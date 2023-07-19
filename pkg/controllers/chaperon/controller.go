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

package chaperon

import (
	"context"
	"fmt"
	"reflect"
	"time"

	multiclusterv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	clientset "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	customscheme "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned/scheme"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
	"github.com/go-test/deep"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

// this file is modified from k8s.io/sample-controller

type reconciler struct {
	kubeclientset   kubernetes.Interface
	customclientset clientset.Interface

	podsLister         corelisters.PodLister
	podChaperonsLister listers.PodChaperonLister

	recorder record.EventRecorder
}

func NewController(
	kubeclientset kubernetes.Interface,
	customclientset clientset.Interface,
	podInformer coreinformers.PodInformer,
	podChaperonInformer informers.PodChaperonInformer) *controller.Controller {

	utilruntime.Must(customscheme.AddToScheme(scheme.Scheme))

	r := &reconciler{
		kubeclientset:      kubeclientset,
		customclientset:    customclientset,
		podsLister:         podInformer.Lister(),
		podChaperonsLister: podChaperonInformer.Lister(),
	}

	getPodChaperon := func(namespace, name string) (metav1.Object, error) {
		return r.podChaperonsLister.PodChaperons(namespace).Get(name)
	}

	c := controller.New("chaperon", r, podInformer.Informer().HasSynced, podChaperonInformer.Informer().HasSynced)

	podChaperonInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))
	podInformer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueController("PodChaperon", getPodChaperon)))

	return c
}

func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	podChaperon, err := c.podChaperonsLister.PodChaperons(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	pod, err := c.podsLister.Pods(podChaperon.Namespace).Get(podChaperon.Name)
	if errors.IsNotFound(err) {
		pod, err = c.kubeclientset.CoreV1().Pods(podChaperon.Namespace).Create(ctx, newPod(podChaperon), metav1.CreateOptions{})
	}
	if err != nil {
		return nil, err
	}

	if !metav1.IsControlledBy(pod, podChaperon) {
		return nil, fmt.Errorf("resource %q already exists and is not managed by PodChaperon", pod.Name)
	}

	// TODO: support allowed pod spec updates: containers[*].image, initContainers[*].image, activeDeadlineSeconds, tolerations (only additions to tolerations)
	// (and maintain that current)
	// we can't just update the whole spec

	diff := deep.Equal(podChaperon.Status, pod.Status)
	needStatusUpdate := len(diff) > 0

	mcPodChaperonAnnotations, otherPodChaperonAnnotations := common.SplitLabelsOrAnnotations(podChaperon.Annotations)
	_, otherPodAnnotations := common.SplitLabelsOrAnnotations(pod.Annotations)
	needUpdate := !reflect.DeepEqual(otherPodChaperonAnnotations, otherPodAnnotations)

	if needStatusUpdate {
		podChaperonCopy := podChaperon.DeepCopy()
		pod.Status.DeepCopyInto(&podChaperonCopy.Status)

		var err error
		podChaperon, err = c.customclientset.MulticlusterV1alpha1().PodChaperons(podChaperon.Namespace).UpdateStatus(ctx, podChaperonCopy, metav1.UpdateOptions{})
		if err != nil {
			if controller.IsOptimisticLockError(err) {
				requeueAfter := time.Second
				return &requeueAfter, nil
			}
			return nil, err
		}
	}

	if needUpdate {
		podChaperonCopy := podChaperon.DeepCopy()
		for k, v := range otherPodAnnotations {
			mcPodChaperonAnnotations[k] = v
		}
		podChaperonCopy.Annotations = mcPodChaperonAnnotations

		var err error
		podChaperon, err = c.customclientset.MulticlusterV1alpha1().PodChaperons(podChaperon.Namespace).Update(ctx, podChaperonCopy, metav1.UpdateOptions{})
		if err != nil {
			if controller.IsOptimisticLockError(err) {
				requeueAfter := time.Second
				return &requeueAfter, nil
			}
			return nil, err
		}
	}

	return nil, nil
}

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
