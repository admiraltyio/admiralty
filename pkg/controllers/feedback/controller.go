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
	"k8s.io/klog/v2"

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

const podChaperonByPodNamespacedName = "podChaperonByPodNamespacedName"

type reconciler struct {
	clusterName string
	target      agent.Target

	kubeclientset   kubernetes.Interface
	customclientset clientset.Interface

	podsLister         corelisters.PodLister
	podChaperonsLister listers.PodChaperonLister

	podChaperonIndex cache.Indexer
}

// NewController returns a new feedback controller
func NewController(
	clusterName string,
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

	r := &reconciler{
		clusterName: clusterName,
		target:      target,

		kubeclientset:   kubeclientset,
		customclientset: customclientset,

		podsLister:         podInformer.Lister(),
		podChaperonsLister: podChaperonInformer.Lister(),

		podChaperonIndex: podChaperonInformer.Informer().GetIndexer(),
	}

	c := controller.New("feedback", r, podInformer.Informer().HasSynced, podChaperonInformer.Informer().HasSynced)

	enqueueProxyPod := func(obj interface{}) {
		pod := obj.(*corev1.Pod)
		if proxypod.IsProxy(pod) {
			c.EnqueueObject(obj)
		}
	}

	podInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueueProxyPod))
	podChaperonInformer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController(clusterName)))

	utilruntime.Must(podChaperonInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		podChaperonByPodNamespacedName: controller.IndexByRemoteController(clusterName),
	}))

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
			objs, err := c.podChaperonIndex.ByIndex(podChaperonByPodNamespacedName, key)
			utilruntime.Must(err)
			for _, obj := range objs {
				candidate := obj.(*multiclusterv1alpha1.PodChaperon)
				if err := c.customclientset.MulticlusterV1alpha1().PodChaperons(namespace).Delete(ctx, candidate.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					return nil, fmt.Errorf("cannot delete orphaned pod chaperon: %v", err)
				}
			}
			return nil, nil
		} else {
			return nil, fmt.Errorf("cannot get proxy pod: %v", err)
		}
	}

	proxyPodTerminating := proxyPod.DeletionTimestamp != nil

	proxyPodHasFinalizer, j := controller.HasFinalizer(proxyPod.Finalizers, c.target.Finalizer)

	// get pod chaperon by parent UID (when parent still exists) rather than using index
	// for backward compatibility with existing pod chaperons
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

	virtualNodeName := proxypod.GetScheduledClusterName(proxyPod)
	if proxyPodTerminating || virtualNodeName != "" && virtualNodeName != c.target.VirtualNodeName {
		if candidate != nil {
			if err := c.customclientset.MulticlusterV1alpha1().PodChaperons(namespace).Delete(ctx, candidate.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		} else if proxyPodHasFinalizer {
			if proxyPod, err = c.removeFinalizer(ctx, proxyPod, j); err != nil {
				return nil, err
			}
		}
	}

	if candidate != nil {
		if _, podMissing := candidate.Annotations[common.AnnotationKeyPodMissingSince]; podMissing {
			if err := c.customclientset.MulticlusterV1alpha1().PodChaperons(namespace).Delete(ctx, candidate.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, fmt.Errorf("cannot delete pod chaperon")
			}
		}
	}

	if virtualNodeName == c.target.VirtualNodeName {
		if candidate != nil {
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
			}

			// we can't group annotation and status updates into an update,
			// because general update ignores status
			filteredDelegateStatus := filterContainerStatus(&proxyPod.Spec, delegate.Status)
			needStatusUpdate := deep.Equal(proxyPod.Status, filteredDelegateStatus) != nil
			if needStatusUpdate {
				podCopy := proxyPod.DeepCopy()
				podCopy.Status = filteredDelegateStatus

				var err error
				if proxyPod, err = c.kubeclientset.CoreV1().Pods(namespace).UpdateStatus(ctx, podCopy, metav1.UpdateOptions{}); err != nil {
					return nil, err
				}
			}

			needRemoteUpdate := delegate.Labels[common.LabelKeyParentClusterName] != c.clusterName
			if needRemoteUpdate {
				delegateCopy := delegate.DeepCopy()
				delegateCopy.Labels[common.LabelKeyParentClusterName] = c.clusterName
				var err error
				if delegate, err = c.customclientset.MulticlusterV1alpha1().PodChaperons(namespace).Update(ctx, delegateCopy, metav1.UpdateOptions{}); err != nil {
					return nil, fmt.Errorf("cannot update candidate pod chaperon")
				}
			}
		} else {
			if err = c.kubeclientset.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
				return nil, err
			}
		}
	}

	return nil, nil
}

// filterContainerStatus returns a shallow copy of delegateStatus with container / initContainer / ephemeralContainer
// statuses filtered to containers which actually exist in proxyPodSpec.
// Nothing in the original container status lists in delegateStatus is mutated.
func filterContainerStatus(proxyPodSpec *corev1.PodSpec, delegateStatus corev1.PodStatus) corev1.PodStatus {
	delegateStatus.ContainerStatuses = filterContainerStatuses(delegateStatus.ContainerStatuses, hasContainerName(proxyPodSpec.Containers))
	delegateStatus.InitContainerStatuses = filterContainerStatuses(delegateStatus.InitContainerStatuses, hasContainerName(proxyPodSpec.InitContainers))
	delegateStatus.EphemeralContainerStatuses = filterContainerStatuses(delegateStatus.EphemeralContainerStatuses, hasEphemeralContainerName(proxyPodSpec.EphemeralContainers))
	return delegateStatus
}

// filterContainerStatuses returns a list of the container statuses for which hasName returns true.
// If hasName returns true for all, statuses is returned as-is.
// If hasName returns false for any, a filtered copy is returned.
// statuses is never modified.
func filterContainerStatuses(statuses []corev1.ContainerStatus, hasName func(name string) bool) []corev1.ContainerStatus {
	copied := false
	retval := statuses
	for i, s := range statuses {
		if hasName(s.Name) {
			if copied {
				// if we're working with a copy, append to our copy
				retval = append(retval, s)
			}
			continue
		}

		if !copied {
			// copy statuses up to i
			retval = make([]corev1.ContainerStatus, i, len(statuses)-1)
			copy(retval, statuses[0:i])
			copied = true
		}
	}
	return retval
}

func hasContainerName(containers []corev1.Container) func(name string) bool {
	return func(name string) bool {
		for _, c := range containers {
			if c.Name == name {
				return true
			}
		}
		return false
	}
}

func hasEphemeralContainerName(containers []corev1.EphemeralContainer) func(name string) bool {
	return func(name string) bool {
		for _, c := range containers {
			if c.Name == name {
				return true
			}
		}
		return false
	}
}

func (c *reconciler) removeFinalizer(ctx context.Context, pod *corev1.Pod, j int) (*corev1.Pod, error) {
	podCopy := pod.DeepCopy()
	podCopy.Finalizers = append(podCopy.Finalizers[:j], podCopy.Finalizers[j+1:]...)
	return c.kubeclientset.CoreV1().Pods(pod.Namespace).Update(ctx, podCopy, metav1.UpdateOptions{})
}
