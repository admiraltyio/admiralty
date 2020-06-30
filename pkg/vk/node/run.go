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

	"github.com/virtual-kubelet/virtual-kubelet/log"
	"github.com/virtual-kubelet/virtual-kubelet/node"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/typed/coordination/v1beta1"
)

func Run(ctx context.Context, c Opts, client kubernetes.Interface) error {
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"node": c.NodeName}))

	var leaseClient v1beta1.LeaseInterface
	if c.EnableNodeLease {
		leaseClient = client.CoordinationV1beta1().Leases(corev1.NamespaceNodeLease)
	}

	n := NodeFromOpts(c)
	nodeRunner, err := node.NewNodeController(
		node.NaiveNodeProvider{},
		n,
		client.CoreV1().Nodes(),
		node.WithNodeEnableLeaseV1Beta1(leaseClient, nil),
		node.WithNodeStatusUpdateErrorHandler(func(ctx context.Context, err error) error {
			if !k8serrors.IsNotFound(err) {
				return err
			}

			log.G(ctx).Debug("node not found")
			newNode := n.DeepCopy()
			newNode.ResourceVersion = ""
			_, err = client.CoreV1().Nodes().Create(ctx, newNode, metav1.CreateOptions{})
			if err != nil {
				return err
			}
			log.G(ctx).Debug("created new node")
			return nil
		}),
	)
	if err != nil {
		log.G(ctx).Fatal(err)
	}

	return nodeRunner.Run(ctx)
}
