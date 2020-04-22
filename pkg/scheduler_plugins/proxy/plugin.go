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

package proxy

import (
	"context"
	"fmt"
	"time"

	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	"admiralty.io/multicluster-scheduler/pkg/model/delegatepod"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
	"admiralty.io/multicluster-service-account/pkg/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	framework "k8s.io/kubernetes/pkg/scheduler/framework/v1alpha1"
	"k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

type Plugin struct {
	handle  framework.FrameworkHandle
	targets map[string]*versioned.Clientset
}

var _ framework.FilterPlugin = &Plugin{}
var _ framework.ReservePlugin = &Plugin{}
var _ framework.PermitPlugin = &Plugin{}
var _ framework.PostBindPlugin = &Plugin{}
var _ framework.UnreservePlugin = &Plugin{}

// Name is the name of the plugin used in the plugin registry and configurations.
const Name = "proxy"

// Name returns name of the plugin. It is used in logs, etc.
func (pl *Plugin) Name() string {
	return Name
}

func virtualNodeNameToClusterName(nodeName string) string {
	return nodeName[10:]
}

func (pl *Plugin) getCandidate(proxyPod *v1.Pod, clusterName string) (*v1alpha1.PodChaperon, error) {
	target, ok := pl.targets[clusterName]
	if !ok {
		return nil, fmt.Errorf("no target for cluster name %s", clusterName)
	}
	l, err := target.MulticlusterV1alpha1().PodChaperons(proxyPod.Namespace).List(metav1.ListOptions{LabelSelector: gc.LabelParentUID + "=" + string(proxyPod.UID)})
	if err != nil {
		return nil, err
	}
	if len(l.Items) > 1 {
		return nil, fmt.Errorf("more than one candidate in target cluster")
	}
	if len(l.Items) < 1 {
		return nil, nil
	}
	return &l.Items[0], nil
}

func (pl *Plugin) allowCandidate(c *v1alpha1.PodChaperon, clusterName string) error {
	target, ok := pl.targets[clusterName]
	if !ok {
		return fmt.Errorf("no target for cluster name %s", clusterName)
	}
	patch := []byte(`{"metadata":{"annotations":{"` + common.AnnotationKeyIsAllowed + `":""}}}`)
	_, err := target.MulticlusterV1alpha1().PodChaperons(c.Namespace).Patch(c.Name, types.MergePatchType, patch)
	return err
}

func (pl *Plugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *nodeinfo.NodeInfo) *framework.Status {
	if !proxypod.IsProxy(pod) {
		return nil
	}

	if nodeInfo.Node().Labels[common.LabelAndTaintKeyVirtualKubeletProvider] != common.VirtualKubeletProviderName {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, "")
	}

	targetClusterName := virtualNodeNameToClusterName(nodeInfo.Node().Name)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second) // TODO configure
	defer cancel()

	var isReserved, isUnschedulable bool

	if err := wait.PollImmediateUntil(time.Second, func() (bool, error) {
		c, err := pl.getCandidate(pod, targetClusterName)
		if err != nil {
			if errors.IsForbidden(err) {
				isUnschedulable = true
				return true, nil
			}
			return false, err
		}
		// create candidate if not exists
		if c == nil {
			c, err := delegatepod.MakeDelegatePod(pod)
			if err != nil {
				return false, err
			}

			_, err = pl.targets[targetClusterName].MulticlusterV1alpha1().PodChaperons(c.Namespace).Create(c)
			if err != nil {
				if errors.IsForbidden(err) {
					isUnschedulable = true
					return true, nil
				}
				return false, err
			}

			return false, nil
		}
		_, isReserved = c.Annotations[common.AnnotationKeyIsReserved]
		_, isUnschedulable = c.Annotations[common.AnnotationKeyIsUnschedulable]

		klog.V(1).Infof("candidate %s is reserved? %v unschedulable? %v", c.Name, isReserved, isUnschedulable)

		return isReserved || isUnschedulable, nil
	}, ctx.Done()); err != nil {
		// error or timeout or scheduling cycle done
		return framework.NewStatus(framework.Error, err.Error())
	}

	if isUnschedulable {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, "")
	}

	return nil
}

func (pl *Plugin) Reserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) *framework.Status {
	if !proxypod.IsProxy(p) {
		return nil
	}

	targetClusterName := virtualNodeNameToClusterName(nodeName)
	c, err := pl.getCandidate(p, targetClusterName)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}
	if c == nil {
		return framework.NewStatus(framework.Error, "candidate not found")
	}
	if err = pl.allowCandidate(c, targetClusterName); err != nil {
		return framework.NewStatus(framework.Error, err.Error())
	}

	return nil
}

func (pl *Plugin) Permit(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	if !proxypod.IsProxy(p) {
		return nil, 0
	}

	// wait for candidate to be bound or not
	targetClusterName := virtualNodeNameToClusterName(nodeName)
	c, err := pl.getCandidate(p, targetClusterName)
	if err != nil {
		return framework.NewStatus(framework.Error, err.Error()), 0
	}
	if c == nil {
		return framework.NewStatus(framework.Error, "candidate not found"), 0
	}

	_, isBound := c.Annotations[common.AnnotationKeyIsBound]
	if isBound {
		return nil, 0
	}
	_, bindingFailed := c.Annotations[common.AnnotationKeyBindingFailed]
	if bindingFailed {
		return framework.NewStatus(framework.Unschedulable, "candidate binding failed"), 0
	}

	//_, cond := pod.GetPodCondition(&c.Status, v1.PodScheduled)
	//if cond != nil {
	//	if cond.Status == v1.ConditionTrue { // bound
	//		return nil, 0
	//	} else { // binding failed
	//		return framework.NewStatus(framework.Unschedulable, "candidate binding failed"), 0
	//	}
	//}

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

		// the pod is now waiting, wait for its candidate to be bound or not
		if err := wait.PollUntil(time.Second, func() (bool, error) {
			c, err := pl.getCandidate(p, targetClusterName)
			if err != nil {
				// TODO handle retriable vs. not retriable (we assume retriable for now)
				return false, nil
			}
			if c == nil {
				return false, fmt.Errorf("candidate not found")
			}

			_, isBound = c.Annotations[common.AnnotationKeyIsBound]
			_, bindingFailed = c.Annotations[common.AnnotationKeyBindingFailed]
			if !isBound && !bindingFailed {
				return false, nil
			}
			wp := pl.handle.GetWaitingPod(p.UID)
			if wp == nil {
				// TODO log pod isn't waiting anymore (timed out)
				return true, nil
			}
			if isBound {
				return wp.Allow(Name), nil
			} else { // bindingFailed
				return wp.Reject("candidate binding failed"), nil
			}

			//_, cond := pod.GetPodCondition(&c.Status, v1.PodScheduled)
			//if cond == nil {
			//	return false, nil
			//}
			//wp := pl.handle.GetWaitingPod(p.UID)
			//if wp == nil {
			//	// TODO log pod isn't waiting anymore (timed out)
			//	return true, nil
			//}
			//if cond.Status == v1.ConditionTrue { // bound
			//	return wp.Allow(Name), nil
			//} else { // binding failed
			//	return wp.Reject("candidate binding failed"), nil
			//}
		}, ctx.Done()); err != nil {
			// or binding cycle done, candidate was never bound or not
			return
		}
	}()

	return framework.NewStatus(framework.Wait, ""), 30 * time.Second
}

func (pl *Plugin) PostBind(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
	if !proxypod.IsProxy(p) {
		return
	}

	targetClusterName := virtualNodeNameToClusterName(nodeName)
	for clusterName, target := range pl.targets {
		if clusterName == targetClusterName {
			continue
		}
		err := target.MulticlusterV1alpha1().PodChaperons(p.Namespace).DeleteCollection(nil, metav1.ListOptions{LabelSelector: gc.LabelParentUID + "=" + string(p.UID)})
		utilruntime.HandleError(err)
	}
}

func (pl *Plugin) Unreserve(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) {
	if !proxypod.IsProxy(p) {
		return
	}

	//for _, target := range pl.targets {
	//	err := target.MulticlusterV1alpha1().PodChaperons(p.Namespace).DeleteCollection(nil, metav1.ListOptions{LabelSelector: gc.LabelParentUID + "=" + string(p.UID)})
	//	utilruntime.HandleError(err)
	//}
}

// New initializes a new plugin and returns it.
func New(args *runtime.Unknown, h framework.FrameworkHandle) (framework.Plugin, error) {
	agentCfg := agentconfig.NewFromBytes(args.Raw)
	//clients := make(map[string]*kubernetes.Clientset, len(agentCfg.Targets))
	n := len(agentCfg.Targets)
	if agentCfg.Raw.TargetSelf {
		n++
	}
	clients := make(map[string]*versioned.Clientset, n)
	for _, target := range agentCfg.Targets {
		//kubeClient, err := kubernetes.NewForConfig(target.ClientConfig)
		client, err := versioned.NewForConfig(target.ClientConfig)
		utilruntime.Must(err)

		// TODO... cache podchaperons with lister

		clients[target.Name] = client
	}
	if agentCfg.Raw.TargetSelf {
		cfg, _, err := config.ConfigAndNamespaceForKubeconfigAndContext("", "")
		utilruntime.Must(err)
		client, err := versioned.NewForConfig(cfg)
		utilruntime.Must(err)
		clients[agentCfg.Raw.ClusterName] = client
	}

	return &Plugin{handle: h, targets: clients}, nil
}
