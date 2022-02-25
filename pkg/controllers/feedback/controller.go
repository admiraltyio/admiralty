/*
 * Copyright 2022 The Multicluster-Scheduler Authors.
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
	"context"
	"fmt"
	"reflect"
	"time"

	"admiralty.io/multicluster-scheduler/pkg/config/agent"
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
	target agent.Target

	kubeclientset   kubernetes.Interface
	customclientset clientset.Interface

	podsLister         corelisters.PodLister
	podChaperonsLister listers.PodChaperonLister

	recorder record.EventRecorder
}

// NewController returns a new feedback controller
func NewController(
	target agent.Target,
	kubeclientset kubernetes.Interface,
	customclientset clientset.Interface,
	podInformer coreinformers.PodInformer,
	podChaperonInformer informers.PodChaperonInformer) *controller.Controller {

	utilruntime.Must(customscheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	r := &reconciler{
		target: target,

		kubeclientset:   kubeclientset,
		customclientset: customclientset,

		podsLister:         podInformer.Lister(),
		podChaperonsLister: podChaperonInformer.Lister(),

		recorder: recorder,
	}

	getPod := func(namespace, name string) (metav1.Object, error) { return r.podsLister.Pods(namespace).Get(name) }
	c := controller.New("feedback", r, podInformer.Informer().HasSynced, podChaperonInformer.Informer().HasSynced)

	enqueueProxyPod := func(obj interface{}) {
		pod := obj.(*corev1.Pod)
		if proxypod.IsProxy(pod) {
			c.EnqueueObject(obj)
		}
	}

	podInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueueProxyPod))
	podChaperonInformer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController("Pod", getPod)))

	return c
}

func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	proxyPod, err := c.podsLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("proxy pod '%s' in work queue no longer exists", key))
			return nil, nil
		}

		return nil, err
	}

	proxyPodTerminating := proxyPod.DeletionTimestamp != nil

	proxyPodHasFinalizer, j := controller.HasFinalizer(proxyPod.Finalizers, c.target.Finalizer)

	var candidate *multiclusterv1alpha1.PodChaperon
	l, err := c.podChaperonsLister.PodChaperons(namespace).List(labels.SelectorFromSet(map[string]string{common.LabelKeyParentUID: string(proxyPod.UID)}))
	if err != nil {
		return nil, err
	}
	if len(l) > 1 {
		return nil, fmt.Errorf("more than one candidate in target cluster")
	}
	if len(l) == 1 {
		candidate = l[0]
	}

	didSomething := false

	virtualNodeName := proxypod.GetScheduledClusterName(proxyPod)
	if proxyPodTerminating || virtualNodeName != "" && virtualNodeName != c.target.VirtualNodeName {
		if candidate != nil {
			if err := c.customclientset.MulticlusterV1alpha1().PodChaperons(namespace).Delete(ctx, candidate.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
			didSomething = true
		} else if proxyPodHasFinalizer {
			if proxyPod, err = c.removeFinalizer(ctx, proxyPod, j); err != nil {
				return nil, err
			}
			didSomething = true
		}
	}

	if candidate != nil && virtualNodeName == c.target.VirtualNodeName {
		delegate := candidate

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
			if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).Update(ctx, podCopy, metav1.UpdateOptions{}); err != nil {
				return nil, err
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
			if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).UpdateStatus(ctx, podCopy, metav1.UpdateOptions{}); err != nil {
				return nil, err
			}
			didSomething = true
		}
	}

	if didSomething {
		c.recorder.Event(proxyPod, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	}
	return nil, nil
}

func (c reconciler) removeFinalizer(ctx context.Context, pod *corev1.Pod, j int) (*corev1.Pod, error) {
	podCopy := pod.DeepCopy()
	podCopy.Finalizers = append(podCopy.Finalizers[:j], podCopy.Finalizers[j+1:]...)
	return c.kubeclientset.CoreV1().Pods(pod.Namespace).Update(ctx, podCopy, metav1.UpdateOptions{})
}
