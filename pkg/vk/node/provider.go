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
	"context"

	vknode "github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"

	"admiralty.io/multicluster-scheduler/pkg/controllers/resources"
)

// NodeProvider accepts a callback from virtual-kubelet's node controller,
// and exposes it to our upstream resources controller, for node status updates.
// If we updated node status directly, without informing virtual-kubelet,
// virtual-kubelet would override our changes every minute (with node leases enabled; more often otherwise);
// which would trigger a reconcile on our end, and another override on vk's end, etc. (controllers disagreeing).
// When virtual-kubelet is notified of what we think the node status should be,
// it keeps it in memory, and call kube-apiserver on our behalf.
type NodeProvider struct {
	cb func(*corev1.Node)
}

var _ vknode.NodeProvider = &NodeProvider{}
var _ resources.NodeStatusUpdater = &NodeProvider{}

func (p *NodeProvider) Ping(ctx context.Context) error {
	return ctx.Err()
}

func (p *NodeProvider) NotifyNodeStatus(ctx context.Context, cb func(*corev1.Node)) {
	p.cb = cb
}

func (p *NodeProvider) UpdateNodeStatus(node *corev1.Node) {
	if p.cb != nil {
		p.cb(node)
	}
}
