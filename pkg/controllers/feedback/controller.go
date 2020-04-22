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
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	multiclusterv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	customscheme "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned/scheme"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
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

type Controller interface {
	Enqueue(item interface{})
}

// reconciler is the controller implementation for proxy pod resources
type reconciler struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// customclientset is a clientset for our own API group
	customclientsets map[string]clientset.Interface

	podsLister          corelisters.PodLister
	podsSynced          cache.InformerSynced
	podChaperonsListers map[string]listers.PodChaperonLister
	podChaperonsSynced  []cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	controller Controller
}

// NewController returns a new chaperon controller
func NewController(
	kubeclientset kubernetes.Interface,
	customclientsets map[string]clientset.Interface,
	podInformer coreinformers.PodInformer,
	podChaperonInformers map[string]informers.PodChaperonInformer) *controller.Controller {

	// Create event broadcaster
	// Add custom types to the default Kubernetes Scheme so Events can be
	// logged for custom types.
	utilruntime.Must(customscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	r := &reconciler{
		kubeclientset:    kubeclientset,
		customclientsets: customclientsets,
		podsLister:       podInformer.Lister(),
		recorder:         recorder,
	}

	informersSynced := make([]cache.InformerSynced, len(podChaperonInformers)+1)
	informersSynced[0] = podInformer.Informer().HasSynced

	r.podChaperonsListers = make(map[string]listers.PodChaperonLister, len(podChaperonInformers))
	i := 1
	for targetName, informer := range podChaperonInformers {
		r.podChaperonsListers[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when Pod resources change
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: r.enqueuePod,
		UpdateFunc: func(old, new interface{}) {
			r.enqueuePod(new)
		},
	})
	// Set up an event handler for when PodChaperon resources change. This
	// handler will lookup the owner of the given PodChaperon, and if it is
	// owned by a Pod resource will enqueue that Pod resource for
	// processing. This way, we don't need to implement custom logic for
	// handling PodChaperon resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	for _, informer := range podChaperonInformers {
		informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: r.handleObject,
			UpdateFunc: func(old, new interface{}) {
				r.handleObject(new)
			},
			DeleteFunc: r.handleObject,
		})
	}

	c := controller.New("feedback", informersSynced, r)
	r.controller = c
	return c
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the proxy pod resource
// with the current status of the resource.
func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	// Convert the namespace/name string into a distinct namespace and name
	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil, nil
	}

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
		if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
			j = i
			break
		}
	}
	proxyPodHasFinalizer := j > -1

	candidates := make(map[string]*multiclusterv1alpha1.PodChaperon)
	for targetName, lister := range c.podChaperonsListers {
		l, err := lister.PodChaperons(namespace).List(labels.SelectorFromSet(map[string]string{gc.LabelParentUID: string(proxyPod.UID)}))
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
		podCopy.Finalizers = append(podCopy.Finalizers, "multicluster.admiralty.io/multiclusterForegroundDeletion")
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

// enqueuePod takes a Pod resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Pod.
func (c *reconciler) enqueuePod(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	pod := obj.(*corev1.Pod)
	if proxypod.IsProxy(pod) {
		c.controller.Enqueue(key)
	}
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Pod resource that 'owns' it across clusters. It does this by looking at the
// objects metadata.annotations field for an appropriate OwnerReference.
// It then enqueues that Pod resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *reconciler) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if parentName, ok := object.GetLabels()[common.LabelKeyParentName]; ok {
		pod, err := c.podsLister.Pods(object.GetNamespace()).Get(parentName)
		if err != nil {
			return
		}

		if string(pod.UID) != object.GetLabels()[gc.LabelParentUID] {
			// TODO handle unlikely yet possible cross-cluster UID conflict with signing
			return
		}

		c.enqueuePod(pod)
		return
	}
}
