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

package feedback

import (
	"fmt"
	"reflect"
	"time"

	"admiralty.io/multicluster-controller/pkg/patterns"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"

	multiclusterv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	customscheme "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned/scheme"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
)

// this file is modified from k8s.io/sample-controller

const controllerAgentName = "admiralty"

const (
	// SuccessSynced is used as part of the Event 'reason' when a proxy pod is synced
	SuccessSynced = "Synced"
	// MessageResourceSynced is the message used for an Event fired when a proxy pod
	// is synced successfully
	MessageResourceSynced = "proxy pod synced successfully"
)

type reconciler struct {
	kubeclientset    kubernetes.Interface
	customclientsets map[string]clientset.Interface

	podsLister          corelisters.PodLister
	podChaperonsListers map[string]listers.PodChaperonLister

	recorder record.EventRecorder
}

// NewController returns a new chaperon controller
func NewController(
	kubeclientset kubernetes.Interface,
	customclientsets map[string]clientset.Interface,
	podInformer coreinformers.PodInformer,
	podChaperonInformers map[string]informers.PodChaperonInformer) *controller.Controller {

	utilruntime.Must(customscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	r := &reconciler{
		kubeclientset:    kubeclientset,
		customclientsets: customclientsets,

		podsLister:          podInformer.Lister(),
		podChaperonsListers: make(map[string]listers.PodChaperonLister, len(podChaperonInformers)),

		recorder: recorder,
	}

	informersSynced := make([]cache.InformerSynced, len(podChaperonInformers)+1)
	informersSynced[0] = podInformer.Informer().HasSynced

	i := 1
	for targetName, informer := range podChaperonInformers {
		r.podChaperonsListers[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	getPod := func(namespace, name string) (metav1.Object, error) { return r.podsLister.Pods(namespace).Get(name) }
	c := controller.New("feedback", r, informersSynced...)

	enqueueProxyPod := func(obj interface{}) {
		pod := obj.(*corev1.Pod)
		if proxypod.IsProxy(pod) {
			c.EnqueueObject(obj)
		}
	}

	podInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueueProxyPod))
	for _, informer := range podChaperonInformers {
		informer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController("Pod", getPod)))
	}

	return c
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the proxy pod resource
// with the current status of the resource.
func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	// Convert the namespace/name string into a distinct namespace and name
	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	// Get the proxy pod resource with this namespace/name
	proxyPod, err := c.podsLister.Pods(namespace).Get(name)
	if err != nil {
		// The proxy pod resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("proxy pod '%s' in work queue no longer exists", key))
			return nil, nil
		}

		return nil, err
	}

	proxyPodTerminating := proxyPod.DeletionTimestamp != nil

	j := -1
	for i, f := range proxyPod.Finalizers {
		if f == common.CrossClusterGarbageCollectionFinalizer {
			j = i
			break
		}
	}
	proxyPodHasFinalizer := j > -1

	candidates := make(map[string]*multiclusterv1alpha1.PodChaperon)
	for targetName, lister := range c.podChaperonsListers {
		l, err := lister.PodChaperons(namespace).List(labels.SelectorFromSet(map[string]string{common.LabelKeyParentUID: string(proxyPod.UID)}))
		if err != nil {
			return nil, err
		}
		if len(l) > 1 {
			return nil, fmt.Errorf("more than one candidate in target cluster")
		}
		if len(l) == 1 {
			candidates[targetName] = l[0]
		}
	}

	didSomething := false

	if proxyPodTerminating {
		for targetName, chap := range candidates {
			if err := c.customclientsets[targetName].MulticlusterV1alpha1().PodChaperons(namespace).Delete(chap.Name, nil); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
			didSomething = true
		}
		if len(candidates) == 0 && proxyPodHasFinalizer {
			podCopy := proxyPod.DeepCopy()
			podCopy.Finalizers = append(podCopy.Finalizers[:j], podCopy.Finalizers[j+1:]...)
			var err error
			if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).Update(podCopy); err != nil {
				if patterns.IsOptimisticLockError(err) {
					d := time.Second
					return &d, nil
				} else {
					return nil, err
				}
			}
			didSomething = true
		}
	} else if !proxyPodHasFinalizer {
		podCopy := proxyPod.DeepCopy()
		podCopy.Finalizers = append(podCopy.Finalizers, common.CrossClusterGarbageCollectionFinalizer)
		var err error
		if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).Update(podCopy); err != nil {
			if patterns.IsOptimisticLockError(err) {
				d := time.Second
				return &d, nil
			} else {
				return nil, err
			}
		}
		didSomething = true
	}

	if proxypod.IsScheduled(proxyPod) {
		delegate, ok := candidates[proxypod.GetScheduledClusterName(proxyPod)]
		if ok {
			mcProxyPodAnnotations, otherProxyPodAnnotations := common.SplitLabelsOrAnnotations(proxyPod.Annotations)
			_, otherDelegatePodAnnotations := common.SplitLabelsOrAnnotations(delegate.Annotations)

			needUpdate := !reflect.DeepEqual(otherProxyPodAnnotations, otherDelegatePodAnnotations)
			if needUpdate {
				for k, v := range otherDelegatePodAnnotations {
					mcProxyPodAnnotations[k] = v
				}
				podCopy := proxyPod.DeepCopy()
				podCopy.Annotations = mcProxyPodAnnotations

				var err error
				if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).Update(podCopy); err != nil {
					if patterns.IsOptimisticLockError(err) {
						d := time.Second
						return &d, nil
					} else {
						return nil, err
					}
				}
				didSomething = true
			}

			// we can't group annotation and status updates into an update,
			// because general update ignores status

			needStatusUpdate := deep.Equal(proxyPod.Status, delegate.Status) != nil
			if needStatusUpdate {
				podCopy := proxyPod.DeepCopy()
				podCopy.Status = delegate.Status

				var err error
				if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).UpdateStatus(podCopy); err != nil {
					if patterns.IsOptimisticLockError(err) {
						d := time.Second
						return &d, nil
					} else {
						return nil, err
					}
				}
				didSomething = true
			}
		}
	}

	if didSomething {
		c.recorder.Event(proxyPod, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	}
	return nil, nil
}
