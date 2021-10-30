/*
 * Copyright 2021 The Multicluster-Scheduler Authors.
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

package service

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"admiralty.io/multicluster-scheduler/pkg/model/delegatepod"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
)

type reconciler struct {
	target agentconfig.Target

	kubeclientset kubernetes.Interface
	remoteClient  kubernetes.Interface

	svcLister corelisters.ServiceLister
	podLister corelisters.PodLister

	remoteSvcLister corelisters.ServiceLister

	serviceRerouteEnabled bool
}

func NewController(
	target agentconfig.Target,

	kubeclientset kubernetes.Interface,
	remoteClient kubernetes.Interface,

	epInformer coreinformers.EndpointsInformer,
	svcInformer coreinformers.ServiceInformer,
	podInformer coreinformers.PodInformer,

	remoteSvcInformer coreinformers.ServiceInformer) *controller.Controller {

	r := &reconciler{
		target: target,

		kubeclientset: kubeclientset,
		remoteClient:  remoteClient,

		svcLister: svcInformer.Lister(),
		podLister: podInformer.Lister(),

		remoteSvcLister: remoteSvcInformer.Lister(),

		serviceRerouteEnabled: true, // TODO configurable
	}

	c := controller.New(
		"services-follow",
		r,
		epInformer.Informer().HasSynced,
		svcInformer.Informer().HasSynced,
		podInformer.Informer().HasSynced,
		remoteSvcInformer.Informer().HasSynced,
	)

	svcInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	remoteSvcInformer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController(
		"Service",
		func(namespace, name string) (metav1.Object, error) {
			return r.svcLister.Services(namespace).Get(name)
		},
	)))

	epInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	// no need to watch pods, as pod events will update endpoints
	// we just need the lister to see if pods are delegates in shouldFollow()

	return c
}

func (r reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	svc, err := r.svcLister.Services(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if _, ok := svc.Annotations[common.AnnotationKeyIsDelegate]; ok {
		return nil, nil
	}

	terminating := svc.DeletionTimestamp != nil

	hasFinalizer, j := controller.HasFinalizer(svc.Finalizers, r.target.GetFinalizer())

	shouldFollow, originalSelector, err := r.shouldFollow(svc)
	if err != nil {
		return nil, err
	}

	// get remote owned services
	// eponymous services that aren't owned are not included (because we don't want to delete them, see below)
	remoteSvc, err := r.remoteSvcLister.Services(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if remoteSvc != nil && !controller.ParentControlsChild(remoteSvc, svc) {
		return nil, nil
	}

	if terminating {
		if remoteSvc != nil {
			if err := r.remoteClient.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		} else if hasFinalizer {
			if _, err = r.removeFinalizer(ctx, svc, j); err != nil {
				return nil, err
			}
		}
	} else if shouldFollow {
		svcCopy := svc.DeepCopy()
		needUpdateLocal := false
		if !hasFinalizer {
			needUpdateLocal = true
			r.addFinalizer(svcCopy)
		}
		if svcCopy.Annotations == nil {
			svcCopy.Annotations = map[string]string{}
		}
		if svcCopy.Annotations[common.AnnotationKeyCiliumGlobalService] != "true" {
			needUpdateLocal = true
			svcCopy.Annotations[common.AnnotationKeyCiliumGlobalService] = "true"
		}
		if svcCopy.Annotations[common.AnnotationKeyGlobal] != "true" {
			needUpdateLocal = true
			svcCopy.Annotations[common.AnnotationKeyGlobal] = "true"
		}
		if originalSelector != svcCopy.Annotations[common.AnnotationKeyOriginalSelector] {
			needUpdateLocal = true
			svcCopy.Annotations[common.AnnotationKeyOriginalSelector] = originalSelector
		}
		if r.serviceRerouteEnabled {
			selector, changed := delegatepod.ChangeLabels(svcCopy.Spec.Selector)
			if changed {
				needUpdateLocal = true
				svcCopy.Spec.Selector = selector
			}
		}
		if needUpdateLocal {
			if svc, err = r.kubeclientset.CoreV1().Services(svcCopy.Namespace).Update(ctx, svcCopy, metav1.UpdateOptions{}); err != nil {
				return nil, err
			}
		}

		if remoteSvc == nil {
			gold := makeRemoteService(svc) // at this point, svc includes updates from above (including reroute and cilium)
			_, err := r.remoteClient.CoreV1().Services(namespace).Create(ctx, gold, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return nil, err
			}
		} else {
			spec := svc.Spec.DeepCopy()
			spec.ClusterIP = remoteSvc.Spec.ClusterIP
			if !reflect.DeepEqual(&remoteSvc.Spec, spec) {
				remoteCopy := remoteSvc.DeepCopy()
				remoteCopy.Spec = *spec.DeepCopy()
				_, err := r.remoteClient.CoreV1().Services(namespace).Update(ctx, remoteCopy, metav1.UpdateOptions{})
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// TODO? cleanup remote services that shouldn't follow anymore

	return nil, nil
}

func (r reconciler) shouldFollow(service *corev1.Service) (bool, string, error) {
	// an empty selector would list everything!
	// when it actually means the service doesn't select pods (e.g., uses custom Endpoints or external name)
	if len(service.Spec.Selector) == 0 {
		return false, "", nil
	}
	// delegate pods may not be in the same cluster, so we need to select proxy pods
	// if service was never rerouted, we can use the selector as is
	// if service was rerouted, we need to recover the original selector from annotations
	// (because we can't recover original selector from transformed one:
	// with current algorithm, domains are overwritten; with a future one, they may be hashed, same issue),
	// unless the current selector includes labels NOT prefixed with out domain,
	// which means it's been updated (or the original selector has been reapplied)
	// in that case, use those labels
	selector := labels.Set{}
	for k, v := range service.Spec.Selector {
		if !strings.HasPrefix(k, common.KeyPrefix) { // user shouldn't use our domain (which is ok by convention)
			selector[k] = v
		}
	}
	if len(selector) == 0 {
		s, ok := service.Annotations[common.AnnotationKeyOriginalSelector]
		if !ok {
			return false, "", fmt.Errorf("original selector not found")
		}
		var err error
		selector, err = labels.ConvertSelectorToLabelsMap(s)
		if err != nil {
			return false, "", fmt.Errorf("original selector is invalid (was tampered with?): %v", err)
		}
	}
	pods, err := r.podLister.Pods(service.Namespace).List(labels.SelectorFromValidatedSet(selector))
	if err != nil {
		return false, "", err
	}
	for _, pod := range pods {
		if proxypod.IsProxy(pod) {
			return true, selector.String(), nil
		}
	}
	return false, selector.String(), nil
}

func (r reconciler) addFinalizer(actualCopy *corev1.Service) {
	actualCopy.Finalizers = append(actualCopy.Finalizers, r.target.GetFinalizer())
	if actualCopy.Labels == nil {
		actualCopy.Labels = map[string]string{}
	}
	actualCopy.Labels[common.LabelKeyHasFinalizer] = "true"
}

func (r reconciler) removeFinalizer(ctx context.Context, actual *corev1.Service, j int) (*corev1.Service, error) {
	actualCopy := actual.DeepCopy()
	actualCopy.Finalizers = append(actualCopy.Finalizers[:j], actualCopy.Finalizers[j+1:]...)
	return r.kubeclientset.CoreV1().Services(actual.Namespace).Update(ctx, actualCopy, metav1.UpdateOptions{})
}

func makeRemoteService(actual *corev1.Service) *corev1.Service {
	gold := &corev1.Service{}
	gold.Name = actual.Name
	gold.Labels = make(map[string]string, len(actual.Labels))
	for k, v := range actual.Labels {
		gold.Labels[k] = v
	}
	gold.Annotations = make(map[string]string, len(actual.Annotations))
	for k, v := range actual.Annotations {
		gold.Annotations[k] = v
	}
	gold.Annotations[common.AnnotationKeyIsDelegate] = ""
	controller.AddRemoteControllerReference(gold, actual)
	gold.Spec = *actual.Spec.DeepCopy()
	gold.Spec.ClusterIP = "" // cluster IP given by each cluster (not really a top-level spec)
	return gold
}
