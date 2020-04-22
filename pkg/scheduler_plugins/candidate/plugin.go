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

package candidate

import (
	"context"
	"time"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
	"admiralty.io/multicluster-service-account/pkg/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
)

type Plugin struct {
	handle framework.FrameworkHandle
	client versioned.Interface
}

var _ framework.PreFilterPlugin = &Plugin{}
var _ framework.PostFilterPlugin = &Plugin{}
var _ framework.ReservePlugin = &Plugin{}
var _ framework.PermitPlugin = &Plugin{}
var _ framework.PostBindPlugin = &Plugin{}
var _ framework.UnreservePlugin = &Plugin{}

// Name is the name of the plugin used in the plugin registry and configurations.
const Name = "candidate"

// Name returns name of the plugin. It is used in logs, etc.
func (pl *Plugin) Name() string {
	return Name
}

func (pl *Plugin) PreFilter(ctx context.Context, state *framework.CycleState, p *v1.Pod) *framework.Status {
	if proxypod.IsProxy(p) {
		return nil
	}

	// reset annotations
	patch := []byte(`{"metadata":{"annotations":{
"` + common.AnnotationKeyIsReserved + `":null,
"` + common.AnnotationKeyIsUnschedulable + `":null,
"` + common.AnnotationKeyIsAllowed + `":null,
"` + common.AnnotationKeyIsBound + `":null,
"` + common.AnnotationKeyBindingFailed + `":null}}}`)
	if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Patch(p.Name, types.MergePatchType, patch); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	return nil
}

func (pl *Plugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (pl *Plugin) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodes []*v1.Node, filteredNodesStatuses framework.NodeToStatusMap) *framework.Status {
	if proxypod.IsProxy(pod) {
		return nil
	}

	if len(nodes) < 1 {
		patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyIsUnschedulable + `":""}}}`)
		if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(pod.Namespace).Patch(pod.Name, types.MergePatchType, patch); err != nil {
			return framework.NewStatus(framework.Error, err.Error())
		}
	}
	return nil
}

func (pl *Plugin) Reserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	if proxypod.IsProxy(p) {
		return nil
	}

	patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyIsReserved + `":""}}}`)
	if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Patch(p.Name, types.MergePatchType, patch); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	return nil
}

func (pl *Plugin) Permit(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	if proxypod.IsProxy(p) {
		return nil, 0
	}

	if _, ok := p.Annotations[common.AnnotationKeyIsAllowed]; ok {
		return nil, 0
	}

	go func() {
		// wait until the pod is waiting
		if err := wait.PollUntil(10*time.Millisecond, func() (bool, error) {
			wp := pl.handle.GetWaitingPod(p.UID)
			return wp != nil, nil
		}, ctx.Done()); err != nil {
			// condition func doesn't throw, i.e., binding cycle done, pod never made it to waiting
			// TODO log
			return
		}

		// the pod is now waiting, wait until it is allowed, if ever
		if err := wait.PollUntil(time.Second, func() (bool, error) {
			pod, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Get(p.Name, metav1.GetOptions{})
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

			// pod allowed, is it still waiting?
			wp := pl.handle.GetWaitingPod(p.UID)
			if wp == nil {
				// TODO log pod isn't waiting anymore (timed out)
				return true, nil
			}

			return wp.Allow(Name), nil
		}, ctx.Done()); err != nil {
			// condition func doesn't throw, i.e., binding cycle done, pod was never allowed
			return
		}
	}()

	return framework.NewStatus(framework.Wait, ""), 30 * time.Second
}

func (pl *Plugin) PostBind(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
	if proxypod.IsProxy(p) {
		return
	}

	patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyIsBound + `":""}}}`)
	if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Patch(p.Name, types.MergePatchType, patch); err != nil {
		utilruntime.HandleError(err)
	}
}

func (pl *Plugin) Unreserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
	if proxypod.IsProxy(p) {
		return
	}

	patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyBindingFailed + `":""}}}`)
	if _, err := pl.client.MulticlusterV1alpha1().PodChaperons(p.Namespace).Patch(p.Name, types.MergePatchType, patch); err != nil {
		utilruntime.HandleError(err)
	}
}

// New initializes a new plugin and returns it.
func New(_ *runtime.Unknown, h framework.FrameworkHandle) (framework.Plugin, error) {
	cfg, _, err := config.ConfigAndNamespaceForKubeconfigAndContext("", "")
	utilruntime.Must(err)
	client, err := versioned.NewForConfig(cfg)
	utilruntime.Must(err)
	return &Plugin{handle: h, client: client}, nil
}
