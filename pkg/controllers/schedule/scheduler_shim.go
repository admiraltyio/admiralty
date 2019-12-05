package schedule

import (
	"context"
	"fmt"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	schedulerconfig "admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type schedulerShim struct {
	client           client.Client
	pendingDecisions pendingDecisions
	scheduler        Scheduler
	schedCfg         *schedulerconfig.Config
}

func (r schedulerShim) Schedule(srcPod *corev1.Pod) (string, error) {
	r.scheduler.Reset()

	var namespaces []string
	var clusters map[string]struct{}
	fedName, _ := srcPod.Annotations[common.AnnotationKeyFederationName]
	if fedName == "" {
		srcClusterName := srcPod.ClusterName // set by applier depending on useClusterNamespaces
		namespaces = r.schedCfg.PairedNamespacesByCluster[srcClusterName]
		clusters = r.schedCfg.PairedClustersByCluster[srcClusterName]
	} else {
		namespaces = r.schedCfg.NamespacesByFederation[fedName]
		clusters = r.schedCfg.ClustersByFederation[fedName]
	}
	for _, ns := range namespaces {
		if err := r.getObservations(ns, clusters); err != nil {
			return "", fmt.Errorf("cannot get observations: %v", err)
		}
	}
	// get pending pod decisions so we can count the requests (cluster targeted but no pod obs yet)
	// otherwise a bunch of pods would be scheduled to one cluster before it would appear to be busy
	for _, pod := range r.pendingDecisions {
		if _, ok := clusters[pod.ClusterName]; ok {
			r.scheduler.SetPod(pod)
		}
	}

	return r.scheduler.Schedule(srcPod)
}

func (r *schedulerShim) getObservations(namespace string, clusters map[string]struct{}) error {
	podObsL := &v1alpha1.PodObservationList{}
	if err := r.client.List(context.Background(), &client.ListOptions{Namespace: namespace}, podObsL); err != nil {
		return fmt.Errorf("cannot list pod observations: %v", err)
	}
	for _, podObs := range podObsL.Items {
		phase := podObs.Status.LiveState.Status.Phase
		if phase == corev1.PodPending || phase == corev1.PodRunning {
			pod := podObs.Status.LiveState
			pod.ClusterName = r.schedCfg.GetObservationClusterName(&podObs)
			if _, ok := clusters[pod.ClusterName]; ok {
				r.scheduler.SetPod(pod)
			}
		}
	}

	nodeObsL := &v1alpha1.NodeObservationList{}
	if err := r.client.List(context.Background(), &client.ListOptions{Namespace: namespace}, nodeObsL); err != nil {
		return fmt.Errorf("cannot list node observations: %v", err)
	}
	for _, nodeObs := range nodeObsL.Items {
		node := nodeObs.Status.LiveState
		node.ClusterName = r.schedCfg.GetObservationClusterName(&nodeObs)
		if _, ok := clusters[node.ClusterName]; ok {
			r.scheduler.SetNode(node)
		}
	}

	npObsL := &v1alpha1.NodePoolObservationList{}
	if err := r.client.List(context.Background(), &client.ListOptions{Namespace: namespace}, npObsL); err != nil {
		return fmt.Errorf("cannot list node pool observations: %v", err)
	}
	for _, npObs := range npObsL.Items {
		np := npObs.Status.LiveState
		np.ClusterName = r.schedCfg.GetObservationClusterName(&npObs)
		if _, ok := clusters[np.ClusterName]; ok {
			r.scheduler.SetNodePool(np)
		}
	}

	return nil
}

type Scheduler interface {
	Reset()
	SetPod(p *corev1.Pod)
	SetNode(n *corev1.Node)
	SetNodePool(np *v1alpha1.NodePool)
	Schedule(p *corev1.Pod) (string, error)
}
