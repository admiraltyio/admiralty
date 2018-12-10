/*
Copyright 2018 The Multicluster-Scheduler Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodepool

import (
	"context"
	"fmt"
	"reflect"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	NodePoolLabel    = "multicluster.admiralty.io/nodepool"
	GKENodePoolLabel = "cloud.google.com/gke-nodepool"
	AKSNodePoolLabel = "agentpool"

	DefaultNodePool = "default"
)

func NewController(local *cluster.Cluster) (*controller.Controller, error) {
	client, err := local.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for local cluster: %v", err)
	}

	co := controller.New(&reconciler{client: client, scheme: local.GetScheme()}, controller.Options{})

	if err := apis.AddToScheme(local.GetScheme()); err != nil {
		return nil, fmt.Errorf("adding APIs to local cluster's scheme: %v", err)
	}
	if err := co.WatchResourceReconcileObject(local, &v1alpha1.NodePool{}, controller.WatchOptions{}); err != nil {
		return nil, err
	}
	// TODO: when multicluster-controller implements it, use WatchResource with arbitrary handler
	h := &EnqueueRequestForNodePool{Context: local.Name, Queue: co.Queue}
	if err := local.AddEventHandler(&corev1.Node{}, h); err != nil {
		return nil, err
	}

	return co, nil
}

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, r.reconcileNodePool(req.Name)
}

func (r *reconciler) reconcileNodePool(name string) error {
	nodes, err := r.getTaggedNodes(name)
	if err != nil {
		return err
	}

	onp := &v1alpha1.NodePool{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name}, onp); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// node pool not found
		if err := r.createNodePool(name, nodes); err != nil {
			return err
		}
	} else {
		// node pool exists

		// find more nodes by selector
		moreNodes, err := r.getSelectedNodes(onp.Spec.Selector)
		if err != nil {
			return err
		}
		nodes = append(nodes, moreNodes...)

		if err := r.updateNodePool(onp, nodes); err != nil {
			return err
		}
	}

	// Convert GKE and AKS labels.
	// Tag orphan nodes as belonging to the default node pool.
	// Tag nodes selected by custom node pools.
	for _, n := range nodes {
		if n.Labels[NodePoolLabel] != name {
			if n.Labels == nil { // very unlikely, esp. for a node, but not impossible for orphan nodes
				n.Labels = map[string]string{}
			}
			n.Labels[NodePoolLabel] = name
			if err := r.client.Update(context.TODO(), &n); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *reconciler) getTaggedNodes(nodePoolName string) ([]corev1.Node, error) {
	var nodes []corev1.Node

	list := &corev1.NodeList{}
	if err := r.client.List(context.TODO(), client.MatchingLabels(map[string]string{NodePoolLabel: nodePoolName}), list); err != nil {
		return nil, err
	}
	nodes = append(nodes, list.Items...)

	list = &corev1.NodeList{}
	if err := r.client.List(context.TODO(), client.MatchingLabels(map[string]string{GKENodePoolLabel: nodePoolName}), list); err != nil {
		return nil, err
	}
	nodes = append(nodes, list.Items...)

	list = &corev1.NodeList{}
	if err := r.client.List(context.TODO(), client.MatchingLabels(map[string]string{AKSNodePoolLabel: nodePoolName}), list); err != nil {
		return nil, err
	}
	nodes = append(nodes, list.Items...)

	if nodePoolName == DefaultNodePool {
		orphans, err := r.getOrphanNodes()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, orphans...)
	}

	return nodes, nil
}

func (r *reconciler) getOrphanNodes() ([]corev1.Node, error) {
	var nodes []corev1.Node

	list := &corev1.NodeList{}
	o := &client.ListOptions{}
	if err := o.SetLabelSelector("!" + NodePoolLabel); err != nil {
		return nil, err
	}
	if err := r.client.List(context.TODO(), o, list); err != nil {
		return nil, err
	}
	nodes = append(nodes, list.Items...)

	more, err := r.getTaggedNodes("")
	if err != nil {
		return nil, err
	}
	nodes = append(nodes, more...)

	return nodes, nil
}

func (r *reconciler) getSelectedNodes(selector *metav1.LabelSelector) ([]corev1.Node, error) {
	selectorInterface, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	o := &client.ListOptions{LabelSelector: selectorInterface}
	list := &corev1.NodeList{}
	if err := r.client.List(context.TODO(), o, list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *reconciler) createNodePool(name string, nodes []corev1.Node) error {
	nodeCount := int32(len(nodes))
	dnp := &v1alpha1.NodePool{}
	dnp.Name = name
	dnp.Spec = v1alpha1.NodePoolSpec{
		Selector:        metav1.SetAsLabelSelector(map[string]string{NodePoolLabel: name}),
		MinNodeCount:    nodeCount,
		MaxNodeCount:    nodeCount,
		NodeAllocatable: nodes[0].Status.Allocatable.DeepCopy(),
		// assuming all nodes have the same Allocatable ResourceList
	}
	return r.client.Create(context.TODO(), dnp)
}

func (r *reconciler) updateNodePool(onp *v1alpha1.NodePool, nodes []corev1.Node) error {
	// only update NodeAllocatable (price and min/max node counts to be updated by user, for now)
	if reflect.DeepEqual(onp.Spec.NodeAllocatable, nodes[0].Status.Allocatable) {
		return nil
	}
	onp.Spec.NodeAllocatable = nodes[0].Status.Allocatable.DeepCopy()
	// assuming all selected nodes have the same Allocatable ResourceList
	return r.client.Update(context.TODO(), onp)
}
