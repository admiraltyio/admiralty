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

package node

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"admiralty.io/multicluster-scheduler/pkg/common"
)

func NodeFromOpts(c Opts) *v1.Node {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.NodeName,
			Labels: map[string]string{
				"type": "virtual-kubelet",
				common.LabelAndTaintKeyVirtualKubeletProvider: common.VirtualKubeletProviderName,
				"kubernetes.io/role":                          "cluster",
				//"kubernetes.io/hostname": c.NodeName,
				"alpha.service-controller.kubernetes.io/exclude-balancer": "true",
			},
		},
		Spec: v1.NodeSpec{
			Taints: []v1.Taint{
				{
					Key:    common.LabelAndTaintKeyVirtualKubeletProvider,
					Value:  common.VirtualKubeletProviderName,
					Effect: v1.TaintEffectNoSchedule,
				},
				{
					Key:    common.LabelAndTaintKeyVirtualKubeletProvider,
					Value:  common.VirtualKubeletProviderName,
					Effect: v1.TaintEffectNoExecute,
				},
			},
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				// TODO: configure or change dynamically to always be greater than what's available
				// delegate scheduler plugin actually ensures resources are available in target cluster
				// but proxy scheduling would fail if capacity wasn't set here
				// (maybe configure proxy scheduler to not run capacity check?)
				"cpu":    resource.MustParse("100000"),
				"memory": resource.MustParse("100000Gi"),
				"pods":   resource.MustParse("100000"),
			},
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					LastHeartbeatTime:  metav1.Now(),
					LastTransitionTime: metav1.Now(),
				},
				//{
				//	Type:               v1.NodeMemoryPressure,
				//	Status:             v1.ConditionFalse,
				//	LastHeartbeatTime:  metav1.Now(),
				//	LastTransitionTime: metav1.Now(),
				//},
				//{
				//	Type:               v1.NodeDiskPressure,
				//	Status:             v1.ConditionFalse,
				//	LastHeartbeatTime:  metav1.Now(),
				//	LastTransitionTime: metav1.Now(),
				//},
				//{
				//	Type:               v1.NodePIDPressure,
				//	Status:             v1.ConditionFalse,
				//	LastHeartbeatTime:  metav1.Now(),
				//	LastTransitionTime: metav1.Now(),
				//},
				//{
				//	Type:               v1.NodeNetworkUnavailable,
				//	Status:             v1.ConditionFalse,
				//	LastHeartbeatTime:  metav1.Now(),
				//	LastTransitionTime: metav1.Now(),
				//},
			},
			//Addresses: []v1.NodeAddress{
			//	{
			//		Type:    "InternalIP",
			//		Address: os.Getenv("VKUBELET_POD_IP"),
			//	},
			//},
			//DaemonEndpoints: v1.NodeDaemonEndpoints{
			//	KubeletEndpoint: v1.DaemonEndpoint{
			//		Port: int32(c.ListenPort),
			//	},
			//},
		},
	}

	//node.Status.Allocatable = node.Status.Capacity
	return node
}
