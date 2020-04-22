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

package chaperon

import (
	"fmt"
	"reflect"
	"time"

	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	customscheme "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned/scheme"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
)

// this file is modified from k8s.io/sample-controller

const controllerAgentName = "admiralty"

const (
	// SuccessSynced is used as part of the Event 'reason' when a PodChaperon is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a PodChaperon fails
	// to sync due to a Pod of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Pod already existing
	MessageResourceExists = "Resource %q already exists and is not managed by PodChaperon"
	// MessageResourceSynced is the message used for an Event fired when a PodChaperon
	// is synced successfully
	MessageResourceSynced = "PodChaperon synced successfully"
)

type Controller interface {
	Enqueue(item interface{})
}

type reconciler struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// customclientset is a clientset for our own API group
	customclientset clientset.Interface

	podsLister         corelisters.PodLister
	podsSynced         cache.InformerSynced
	podChaperonsLister listers.PodChaperonLister
	podChaperonsSynced cache.InformerSynced

	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	controller Controller
}

// NewController returns a new chaperon controller
func NewController(
	kubeclientset kubernetes.Interface,
	customclientset clientset.Interface,
	podInformer coreinformers.PodInformer,
	podChaperonInformer informers.PodChaperonInformer) *controller.Controller {

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
		kubeclientset:      kubeclientset,
		customclientset:    customclientset,
		podsLister:         podInformer.Lister(),
		podsSynced:         podInformer.Informer().HasSynced,
		podChaperonsLister: podChaperonInformer.Lister(),
		podChaperonsSynced: podChaperonInformer.Informer().HasSynced,
		recorder:           recorder,
	}

	klog.Info("Setting up event handlers")
	// Set up an event handler for when PodChaperon resources change
	podChaperonInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: r.enqueuePodChaperon,
		UpdateFunc: func(old, new interface{}) {
			r.enqueuePodChaperon(new)
		},
	})
	// Set up an event handler for when Pod resources change. This
	// handler will lookup the owner of the given Pod, and if it is
	// owned by a PodChaperon resource will enqueue that PodChaperon resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Pod resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: r.handleObject,
		UpdateFunc: func(old, new interface{}) {
			r.handleObject(new)
		},
		DeleteFunc: r.handleObject,
	})

	c := controller.New("chaperon", []cache.InformerSynced{podInformer.Informer().HasSynced, podChaperonInformer.Informer().HasSynced}, r)
	r.controller = c
	return c
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the PodChaperon resource
// with the current status of the resource.
func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	// Convert the namespace/name string into a distinct namespace and name
	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil, nil
	}

	// Get the PodChaperon resource with this namespace/name
	podChaperon, err := c.podChaperonsLister.PodChaperons(namespace).Get(name)
	if err != nil {
		// The PodChaperon resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("podChaperon '%s' in work queue no longer exists", key))
			return nil, nil
		}

		return nil, err
	}

	didSomething := false

	// Get the pod with the name specified in PodChaperon.spec
	pod, err := c.podsLister.Pods(podChaperon.Namespace).Get(podChaperon.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		pod, err = c.kubeclientset.CoreV1().Pods(podChaperon.Namespace).Create(newPod(podChaperon))
		didSomething = true
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the Pod is not controlled by this PodChaperon resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(pod, podChaperon) {
		msg := fmt.Sprintf(MessageResourceExists, pod.Name)
		c.recorder.Event(podChaperon, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	// TODO: support allowed pod spec updates: containers[*].image, initContainers[*].image, activeDeadlineSeconds, tolerations (only additions to tolerations)
	// (and maintain that current)
	// we can't just update the whole spec

	diff := deep.Equal(podChaperon.Status, pod.Status)
	needStatusUpdate := len(diff) > 0

	mcPodChaperonAnnotations, otherPodChaperonAnnotations := common.SplitLabelsOrAnnotations(podChaperon.Annotations)
	_, otherPodAnnotations := common.SplitLabelsOrAnnotations(pod.Annotations)
	needUpdate := !reflect.DeepEqual(otherPodChaperonAnnotations, otherPodAnnotations)

	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance

	if needStatusUpdate {
		podChaperonCopy := podChaperon.DeepCopy()
		pod.Status.DeepCopyInto(&podChaperonCopy.Status)

		var err error
		podChaperon, err = c.customclientset.MulticlusterV1alpha1().PodChaperons(podChaperon.Namespace).UpdateStatus(podChaperonCopy)
		if err != nil {
			if patterns.IsOptimisticLockError(err) {
				requeueAfter := time.Second
				return &requeueAfter, nil
			}
			return nil, err
		}
		didSomething = true
	}

	if needUpdate {
		podChaperonCopy := podChaperon.DeepCopy()
		for k, v := range otherPodAnnotations {
			mcPodChaperonAnnotations[k] = v
		}
		podChaperonCopy.Annotations = mcPodChaperonAnnotations

		var err error
		podChaperon, err = c.customclientset.MulticlusterV1alpha1().PodChaperons(podChaperon.Namespace).Update(podChaperonCopy)
		if err != nil {
			if patterns.IsOptimisticLockError(err) {
				requeueAfter := time.Second
				return &requeueAfter, nil
			}
			return nil, err
		}
		didSomething = true
	}

	if didSomething {
		c.recorder.Event(podChaperon, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	}
	return nil, nil
}

// enqueuePodChaperon takes a PodChaperon resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than PodChaperon.
func (c *reconciler) enqueuePodChaperon(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.controller.Enqueue(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the PodChaperon resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that PodChaperon resource to be processed. If the object does not
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
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a PodChaperon, we should not do anything more
		// with it.
		if ownerRef.Kind != "PodChaperon" {
			return
		}

		podChaperon, err := c.podChaperonsLister.PodChaperons(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			klog.V(4).Infof("ignoring orphaned object '%s' of podChaperon '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueuePodChaperon(podChaperon)
		return
	}
}

// newPod creates a new Pod for a PodChaperon resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the PodChaperon resource that 'owns' it.
func newPod(podChaperon *multiclusterv1alpha1.PodChaperon) *corev1.Pod {
	annotations := make(map[string]string)
	for k, v := range podChaperon.Annotations {
		annotations[k] = v
	}
	labels := make(map[string]string)
	for k, v := range podChaperon.Labels {
		labels[k] = v
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podChaperon.Name,
			Namespace:   podChaperon.Namespace,
			Labels:      labels,
			Annotations: annotations,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(podChaperon, multiclusterv1alpha1.SchemeGroupVersion.WithKind("PodChaperon")),
			},
		},
	}
	podChaperon.Spec.DeepCopyInto(&pod.Spec)
	return pod
}
