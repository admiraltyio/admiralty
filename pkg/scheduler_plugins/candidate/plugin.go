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

package candidate

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
)

type Plugin struct {
	handle framework.Handle
	client versioned.Interface
}

var _ framework.PreFilterPlugin = &Plugin{}
var _ framework.ReservePlugin = &Plugin{}
var _ framework.PreBindPlugin = &Plugin{}

// Name is the name of the plugin used in the plugin registry and configurations.
const Name = "candidate"

// Name returns name of the plugin. It is used in logs, etc.
func (pl *Plugin) Name() string {
	return Name
}

func (pl *Plugin) PreFilter(ctx context.Context, state *framework.CycleState, p *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	// reset annotations
	patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyIsReserved + `":null}}}`)
	if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Patch(ctx, p.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return nil, framework.NewStatus(framework.Error, err.Error())
	}
	return nil, nil
}

func (pl *Plugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (pl *Plugin) Reserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyIsReserved + `":"true"}}}`)
	if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Patch(ctx, p.Name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	return nil
}

func (pl *Plugin) Unreserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
}

const waitDuration = 30 * time.Second

func (pl *Plugin) PreBind(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	ctx, cancel := context.WithTimeout(ctx, waitDuration)
	defer cancel()

	// wait until pod is allowed, if ever
	if err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
		return pl.isAllowed(ctx, p)
	}, ctx.Done()); err != nil {
		// condition func doesn't throw, i.e., ctx timed out, pod was never allowed
		return framework.NewStatus(framework.Error, err.Error())
	}

	return nil
}

func (pl *Plugin) isAllowed(ctx context.Context, p *v1.Pod) (bool, error) {
	pod, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Get(ctx, p.Name, metav1.GetOptions{})
	if err != nil {
		// TODO handle retriable vs. not retriable, esp. not found (we assume retriable for now)
		// TODO log
		return false, nil
	}

	if _, ok := pod.Annotations[common.AnnotationKeyIsAllowed]; !ok {
		// pod not allowed (yet?)
		klog.V(1).Infof("candidate %s is not allowed", pod.Name)
		return false, nil
	}

	klog.V(1).Infof("candidate %s is allowed", pod.Name)
	return true, nil
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	cfg := config.GetConfigOrDie()
	client, err := versioned.NewForConfig(cfg)
	utilruntime.Must(err)
	return &Plugin{handle: h, client: client}, nil
}
