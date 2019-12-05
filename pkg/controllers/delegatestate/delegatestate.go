package delegatestate

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	schedulerconfig "admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(scheduler *cluster.Cluster, schedCfg *schedulerconfig.Config) (*controller.Controller, error) {
	schedulerClient, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client: %v", err)
	}

	co := controller.New(&reconciler{
		scheduler: schedulerClient,
		schedCfg:  schedCfg,
	}, controller.Options{})

	if err := co.WatchResourceReconcileObject(scheduler, &v1alpha1.PodObservation{}, controller.WatchOptions{
		CustomPredicate: func(obj interface{}) bool {
			podObs := obj.(*v1alpha1.PodObservation)
			proxyPodClusterName, isDelegate := podObs.Status.LiveState.Labels[common.LabelKeyProxyPodClusterName]
			delegateClusterName := schedCfg.GetObservationClusterName(podObs)
			_, isAllowed := schedCfg.PairedClustersByCluster[proxyPodClusterName][delegateClusterName]
			// isAllowed prevents an attacker in an untrusted cluster from sending feedback via fake annotations
			return isDelegate && isAllowed
		},
	}); err != nil {
		return nil, fmt.Errorf("setting up pod observation watch: %v", err)
	}

	return co, nil
}

type reconciler struct {
	scheduler client.Client
	schedCfg  *schedulerconfig.Config
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	podObs := &v1alpha1.PodObservation{}
	if err := r.scheduler.Get(context.Background(), req.NamespacedName, podObs); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get pod observation %s in namespace %s: %v",
				req.Name, req.Namespace, err)
		}
		return reconcile.Result{}, nil
	}

	pod := podObs.Status.LiveState

	proxyPodClusterName, ok := pod.Labels[common.LabelKeyProxyPodClusterName]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("pod observation %s in namespace %s is missing label %s",
			req.Name, req.Namespace, common.LabelKeyProxyPodClusterName)
	}
	proxyPodNs, ok := pod.Labels[common.LabelKeyProxyPodNamespace]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("pod observation %s in namespace %s is missing label %s",
			req.Name, req.Namespace, common.LabelKeyProxyPodNamespace)
	}
	proxyPodName, ok := pod.Labels[common.LabelKeyProxyPodName]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("pod observation %s in namespace %s is missing label %s",
			req.Name, req.Namespace, common.LabelKeyProxyPodName)
	}

	proxyPodObsList := &v1alpha1.PodObservationList{}
	s := labels.SelectorFromValidatedSet(labels.Set{
		gc.LabelParentClusterName: proxyPodClusterName,
		gc.LabelParentNamespace:   proxyPodNs,
		gc.LabelParentName:        proxyPodName,
	})
	if err := r.scheduler.List(context.Background(), &client.ListOptions{
		Namespace:     r.schedCfg.NamespaceForCluster[proxyPodClusterName],
		LabelSelector: s,
	}, proxyPodObsList); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list pod obs in namespace %s with label selector %s: %v",
			proxyPodNs, s, err)
	}
	if len(proxyPodObsList.Items) == 0 {
		return reconcile.Result{}, fmt.Errorf("proxy pod obs not found in namespace %s with label selector %s",
			proxyPodNs, s)
	} else if len(proxyPodObsList.Items) > 1 {
		return reconcile.Result{}, fmt.Errorf("found duplicate proxy pod obs in namespace %s with label selector %s",
			proxyPodNs, s)
	}
	proxyPodObs := &proxyPodObsList.Items[0]

	if !reflect.DeepEqual(proxyPodObs.Status.DelegateState, podObs.Status.LiveState) {
		proxyPodObs.Status.DelegateState = podObs.Status.LiveState
		if err := r.scheduler.Update(context.Background(), proxyPodObs); err != nil {
			if patterns.IsOptimisticLockError(err) {
				// TODO watch proxy pod observations instead, to requeue when the cache is updated
				oneSec, _ := time.ParseDuration("1s")
				return reconcile.Result{RequeueAfter: oneSec}, nil
			}
			return reconcile.Result{}, fmt.Errorf("cannot update proxy pod obs %s in namespace %s: %v",
				proxyPodObs.Name, proxyPodObs.Namespace, err)
		}
	}

	return reconcile.Result{}, nil
}
