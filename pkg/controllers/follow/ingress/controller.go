/*
 * Copyright 2020 The Multicluster-Scheduler Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package ingress

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1beta1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/networking/v1beta1"
	"k8s.io/client-go/tools/cache"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
)

const ingressByService = "ingressByService"

type ingressReconciler struct {
	kubeclientset kubernetes.Interface
	remoteClients map[string]kubernetes.Interface

	svcLister     corelisters.ServiceLister
	ingressLister networkinglisters.IngressLister

	remoteIngressLister map[string]networkinglisters.IngressLister

	ingressIndex cache.Indexer

	selfTargetKeys map[string]bool
}

func NewIngressController(
	kubeclientset kubernetes.Interface,
	remoteClients map[string]kubernetes.Interface,

	svcInformer coreinformers.ServiceInformer,
	ingressInformer networkinginformers.IngressInformer,

	remoteIngressInformers map[string]networkinginformers.IngressInformer,

	selfTargetKeys map[string]bool) *controller.Controller {

	r := &ingressReconciler{
		kubeclientset: kubeclientset,
		remoteClients: remoteClients,

		svcLister:     svcInformer.Lister(),
		ingressLister: ingressInformer.Lister(),

		remoteIngressLister: make(map[string]networkinglisters.IngressLister, len(remoteIngressInformers)),

		ingressIndex: ingressInformer.Informer().GetIndexer(),

		selfTargetKeys: selfTargetKeys,
	}

	informersSynced := make([]cache.InformerSynced, 2+len(remoteIngressInformers))
	informersSynced[0] = svcInformer.Informer().HasSynced
	informersSynced[1] = ingressInformer.Informer().HasSynced

	i := 2
	for targetName, informer := range remoteIngressInformers {
		r.remoteIngressLister[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	c := controller.New("ingresses-follow", r, informersSynced...)

	ingressInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	getIngress := func(namespace, name string) (metav1.Object, error) {
		return r.ingressLister.Ingresses(namespace).Get(name)
	}
	for _, informer := range remoteIngressInformers {
		informer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController("Ingress", getIngress)))
	}

	svcInformer.Informer().AddEventHandler(controller.HandleAllWith(r.enqueueIngressForService(c)))
	utilruntime.Must(ingressInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		ingressByService: indexIngressByService,
	}))

	return c
}

func (r ingressReconciler) enqueueIngressForService(c *controller.Controller) func(obj interface{}) {
	return func(obj interface{}) {
		svc := obj.(*corev1.Service)
		objs, err := r.ingressIndex.ByIndex(ingressByService, fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))
		utilruntime.Must(err)
		for _, obj := range objs {
			c.EnqueueObject(obj)
		}
	}
}

func indexIngressByService(obj interface{}) ([]string, error) {
	ingress, ok := obj.(*v1beta1.Ingress)
	if !ok {
		return nil, nil
	}
	var keys []string
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			keys = append(keys, fmt.Sprintf("%s/%s", ingress.Namespace, path.Backend.ServiceName))
		}
	}
	return keys, nil
}

func (r ingressReconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	ingress, err := r.ingressLister.Ingresses(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if _, ok := ingress.Annotations[common.AnnotationKeyIsDelegate]; ok {
		return nil, nil
	}

	terminating := ingress.DeletionTimestamp != nil

	j := -1
	for i, f := range ingress.Finalizers {
		if f == common.CrossClusterGarbageCollectionFinalizer {
			j = i
			break
		}
	}
	hasFinalizer := j > -1

	shouldFollow := r.shouldFollow(ingress)

	// get remote owned ingresses
	// eponymous ingresses that aren't owned are not included (because we don't want to delete them, see below)
	// include owned ingresses in targets that no longer need them (because we shouldn't forget them when deleting)
	remoteIngresses := make(map[string]*v1beta1.Ingress)
	for targetName, lister := range r.remoteIngressLister {
		remoteIngress, err := lister.Ingresses(namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				// error with a target shouldn't block reconciliation with other targets
				d := time.Second
				requeueAfter = &d // named returned
				utilruntime.HandleError(err)
			}
			continue
		}
		if controller.ParentControlsChild(remoteIngress, ingress) {
			remoteIngresses[targetName] = remoteIngress
		}
	}

	if terminating {
		for targetName := range remoteIngresses {
			if err := r.remoteClients[targetName].NetworkingV1beta1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		}
		if hasFinalizer && len(remoteIngresses) == 0 {
			requeueAfter, err = r.removeFinalizer(ctx, ingress, j)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}
	} else if shouldFollow {
		if !hasFinalizer {
			requeueAfter, err = r.addFinalizer(ctx, ingress)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}

		for targetName, remoteClient := range r.remoteClients {
			if r.selfTargetKeys[targetName] {
				continue
			}

			remoteIngress := remoteIngresses[targetName]
			if remoteIngress == nil {
				gold := makeRemoteIngress(ingress)
				_, err := remoteClient.NetworkingV1beta1().Ingresses(namespace).Create(ctx, gold, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					// error with a target shouldn't block reconciliation with other targets
					d := time.Second
					requeueAfter = &d // named returned
					utilruntime.HandleError(err)
				}
			} else if !reflect.DeepEqual(remoteIngress.Spec, ingress.Spec) {
				remoteIngressCopy := remoteIngress.DeepCopy()
				remoteIngressCopy.Spec = *ingress.Spec.DeepCopy()
				_, err := remoteClient.NetworkingV1beta1().Ingresses(namespace).Update(ctx, remoteIngressCopy, metav1.UpdateOptions{})
				if err != nil {
					// error with a target shouldn't block reconciliation with other targets
					d := time.Second
					requeueAfter = &d // named returned
					utilruntime.HandleError(err)
				}
			}
		}
	}

	// TODO? cleanup remote ingresses that shouldn't follow anymore

	return requeueAfter, nil
}

func (r ingressReconciler) shouldFollow(ingress *v1beta1.Ingress) bool {
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			svc, err := r.svcLister.Services(ingress.Namespace).Get(path.Backend.ServiceName)
			if err != nil {
				if !errors.IsNotFound(err) {
					// TODO log
				}
				continue
			}
			if svc.Annotations[common.AnnotationKeyGlobal] == "true" {
				return true
			}
		}
	}
	return false
}

func (r ingressReconciler) addFinalizer(ctx context.Context, ingress *v1beta1.Ingress) (*time.Duration, error) {
	ingressCopy := ingress.DeepCopy()
	ingressCopy.Finalizers = append(ingressCopy.Finalizers, common.CrossClusterGarbageCollectionFinalizer)
	if ingressCopy.Labels == nil {
		ingressCopy.Labels = map[string]string{}
	}
	ingressCopy.Labels[common.LabelKeyHasFinalizer] = "true"
	if ingressCopy.Annotations == nil {
		ingressCopy.Annotations = map[string]string{}
	}
	ingressCopy.Labels[common.LabelKeyHasFinalizer] = "true"
	ingressCopy.Annotations[common.AnnotationKeyGlobal] = "true"
	var err error
	if _, err = r.kubeclientset.NetworkingV1beta1().Ingresses(ingress.Namespace).Update(ctx, ingressCopy, metav1.UpdateOptions{}); err != nil {
		if controller.IsOptimisticLockError(err) {
			d := time.Second
			return &d, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}

func (r ingressReconciler) removeFinalizer(ctx context.Context, ingress *v1beta1.Ingress, j int) (*time.Duration, error) {
	ingressCopy := ingress.DeepCopy()
	ingressCopy.Finalizers = append(ingressCopy.Finalizers[:j], ingressCopy.Finalizers[j+1:]...)
	delete(ingressCopy.Labels, common.LabelKeyHasFinalizer)
	var err error
	if _, err = r.kubeclientset.NetworkingV1beta1().Ingresses(ingress.Namespace).Update(ctx, ingressCopy, metav1.UpdateOptions{}); err != nil {
		if controller.IsOptimisticLockError(err) {
			d := time.Second
			return &d, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}

func makeRemoteIngress(ingress *v1beta1.Ingress) *v1beta1.Ingress {
	gold := &v1beta1.Ingress{}
	gold.Name = ingress.Name
	gold.Labels = make(map[string]string, len(ingress.Labels))
	for k, v := range ingress.Labels {
		gold.Labels[k] = v
	}
	gold.Annotations = make(map[string]string, len(ingress.Annotations))
	for k, v := range ingress.Annotations {
		gold.Annotations[k] = v
	}
	gold.Annotations[common.AnnotationKeyIsDelegate] = ""
	gold.Annotations[common.AnnotationKeyGlobal] = "true"
	controller.AddRemoteControllerReference(gold, ingress)
	gold.Spec = *ingress.Spec.DeepCopy()
	return gold
}
