/*
 * Copyright 2023 The Multicluster-Scheduler Authors.
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
	"strings"
	"time"

	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"k8s.io/klog/v2"
)

const ingressByService = "ingressByService"

type ingressReconciler struct {
	clusterName string
	target      agentconfig.Target

	kubeclientset kubernetes.Interface
	remoteClient  kubernetes.Interface

	svcLister     corelisters.ServiceLister
	ingressLister networkinglisters.IngressLister

	remoteIngressLister networkinglisters.IngressLister

	ingressIndex cache.Indexer
}

func NewIngressController(
	clusterName string,
	target agentconfig.Target,

	kubeclientset kubernetes.Interface,
	remoteClient kubernetes.Interface,

	svcInformer coreinformers.ServiceInformer,
	ingressInformer networkinginformers.IngressInformer,

	remoteIngressInformer networkinginformers.IngressInformer) *controller.Controller {

	r := &ingressReconciler{
		clusterName: clusterName,
		target:      target,

		kubeclientset: kubeclientset,
		remoteClient:  remoteClient,

		svcLister:     svcInformer.Lister(),
		ingressLister: ingressInformer.Lister(),

		remoteIngressLister: remoteIngressInformer.Lister(),

		ingressIndex: ingressInformer.Informer().GetIndexer(),
	}

	c := controller.New(
		"ingresses-follow",
		r,
		svcInformer.Informer().HasSynced,
		ingressInformer.Informer().HasSynced,
		remoteIngressInformer.Informer().HasSynced,
	)

	ingressInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	remoteIngressInformer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController(clusterName)))

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
	ingress, ok := obj.(*v1.Ingress)
	if !ok {
		return nil, nil
	}
	var keys []string
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				keys = append(keys, fmt.Sprintf("%s/%s", ingress.Namespace, path.Backend.Service.Name))
			}
		}
	}
	return keys, nil
}

func (r ingressReconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	remoteIngress, err := r.remoteIngressLister.Ingresses(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	ingress, err := r.ingressLister.Ingresses(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			if remoteIngress != nil && controller.IsRemoteControlled(remoteIngress, r.clusterName) {
				if err := r.remoteClient.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					return nil, fmt.Errorf("cannot delete orphaned ingress: %v", err)
				}
			}
			return nil, nil
		}
		return nil, err
	}

	if _, ok := ingress.Annotations[common.AnnotationKeyIsDelegate]; ok {
		return nil, nil
	}

	terminating := ingress.DeletionTimestamp != nil

	hasFinalizer, j := controller.HasFinalizer(ingress.Finalizers, r.target.Finalizer)

	shouldFollow := r.shouldFollow(ingress)

	// get remote owned ingresses
	// eponymous ingresses that aren't owned are not included (because we don't want to delete them, see below)
	// include owned ingresses in targets that no longer need them (because we shouldn't forget them when deleting)
	if remoteIngress != nil && !controller.ParentControlsChild(remoteIngress, ingress) {
		return nil, nil
	}

	if terminating {
		if remoteIngress != nil {
			if err := r.remoteClient.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		} else if hasFinalizer {
			if _, err = r.removeFinalizer(ctx, ingress, j); err != nil {
				return nil, err
			}
		}
	} else if shouldFollow {
		if !hasFinalizer {
			if ingress, err = r.addFinalizer(ctx, ingress); err != nil {
				return nil, err
			}
		}

		if remoteIngress == nil {
			gold := r.makeRemoteIngress(ingress)
			_, err := r.remoteClient.NetworkingV1().Ingresses(namespace).Create(ctx, gold, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return nil, err
			}
		} else if remoteIngressCopy, shouldUpdate := r.shouldUpdate(remoteIngress, ingress); shouldUpdate {
			_, err := r.remoteClient.NetworkingV1().Ingresses(namespace).Update(ctx, remoteIngressCopy, metav1.UpdateOptions{})
			if err != nil {
				return nil, err
			}
		}
	}

	// TODO? cleanup remote ingresses that shouldn't follow anymore

	return nil, nil
}

func (r ingressReconciler) shouldUpdate(remoteIngress *v1.Ingress, ingress *v1.Ingress) (*v1.Ingress, bool) {
	remoteIngressCopy := remoteIngress.DeepCopy()
	shouldUpdate := false
	if !reflect.DeepEqual(remoteIngress.Spec, ingress.Spec) {
		remoteIngressCopy.Spec = *ingress.Spec.DeepCopy()
		shouldUpdate = true
	}
	annotationDiff := make(map[string]string)
	for annotationKey, annotationValue := range ingress.Annotations {
		if !strings.HasPrefix(annotationKey, common.KeyPrefix) {
			if _, ok := remoteIngress.Annotations[annotationKey]; ok {
				if remoteIngress.Annotations[annotationKey] != ingress.Annotations[annotationKey] {
					annotationDiff[annotationKey] = annotationValue
				}
			} else {
				annotationDiff[annotationKey] = annotationValue
			}
		}
	}
	if len(annotationDiff) > 0 {
		klog.Infof("Detected annotation diff: %v", annotationDiff)
		for annotationKey, annotationValue := range annotationDiff {
			remoteIngressCopy.Annotations[annotationKey] = annotationValue
			klog.Infof("Adding annotation to remote Ingress: [%s:%s]", annotationKey, annotationValue)
		}
		shouldUpdate = true
	}
	if remoteIngress.Labels[common.LabelKeyParentClusterName] != r.clusterName {
		// add or update parent cluster name
		// labels is non-nil because it includes parent UID
		remoteIngressCopy.Labels[common.LabelKeyParentClusterName] = r.clusterName
		shouldUpdate = true
	}
	return remoteIngressCopy, shouldUpdate
}

func (r ingressReconciler) shouldFollow(ingress *v1.Ingress) bool {
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			if path.Backend.Service != nil {
				svc, err := r.svcLister.Services(ingress.Namespace).Get(path.Backend.Service.Name)
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
	}
	return false
}

func (r ingressReconciler) addFinalizer(ctx context.Context, ingress *v1.Ingress) (*v1.Ingress, error) {
	ingressCopy := ingress.DeepCopy()
	ingressCopy.Finalizers = append(ingressCopy.Finalizers, r.target.Finalizer)
	if ingressCopy.Labels == nil {
		ingressCopy.Labels = map[string]string{}
	}
	ingressCopy.Labels[common.LabelKeyHasFinalizer] = "true"
	if ingressCopy.Annotations == nil {
		ingressCopy.Annotations = map[string]string{}
	}
	ingressCopy.Annotations[common.AnnotationKeyGlobal] = "true"
	return r.kubeclientset.NetworkingV1().Ingresses(ingress.Namespace).Update(ctx, ingressCopy, metav1.UpdateOptions{})
}

func (r ingressReconciler) removeFinalizer(ctx context.Context, ingress *v1.Ingress, j int) (*v1.Ingress, error) {
	ingressCopy := ingress.DeepCopy()
	ingressCopy.Finalizers = append(ingressCopy.Finalizers[:j], ingressCopy.Finalizers[j+1:]...)
	return r.kubeclientset.NetworkingV1().Ingresses(ingress.Namespace).Update(ctx, ingressCopy, metav1.UpdateOptions{})
}

func (r ingressReconciler) makeRemoteIngress(ingress *v1.Ingress) *v1.Ingress {
	gold := &v1.Ingress{}
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
	controller.AddRemoteControllerReference(gold, ingress, r.clusterName)
	gold.Spec = *ingress.Spec.DeepCopy()
	return gold
}
