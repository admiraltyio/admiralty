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

package schedule

import (
	"fmt"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns/decorator"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	schedulerconfig "admiralty.io/multicluster-scheduler/pkg/config/scheduler"
	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
)

func NewController(c *cluster.Cluster, s Scheduler, schedCfg *schedulerconfig.Config) (*controller.Controller, error) {
	pendingDecisions := make(pendingDecisions)
	client, err := c.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client: %v", err)
	}
	return decorator.NewController(c, &v1alpha1.PodObservation{}, applier{
		schedCfg: schedCfg,
		scheduler: schedulerShim{
			schedCfg:         schedCfg,
			client:           client,
			pendingDecisions: pendingDecisions,
			scheduler:        s,
		},
		pendingDecisions: pendingDecisions,
	}, controller.WatchOptions{Namespaces: schedCfg.Namespaces})
}

type applier struct {
	schedCfg         *schedulerconfig.Config
	scheduler        SchedulerShim
	pendingDecisions pendingDecisions
	// Note: this makes the reconciler NOT compatible with MaxConccurentReconciles > 1
	// TODO add mutex if we want concurrent reconcilers
}

func (r applier) NeedUpdate(obj interface{}) (bool, error) {
	podObs := obj.(*v1alpha1.PodObservation)
	pod := podObs.Status.LiveState
	if _, ok := pod.Annotations[common.AnnotationKeyElect]; !ok {
		// not a proxy pod

		// but it could be a delegate pod, in which case we want to remove the corresponding pod decision from the pending map
		k := key{
			clusterName: r.schedCfg.GetObservationClusterName(podObs),
			namespace:   pod.Namespace,
			name:        pod.Name,
		}
		delete(r.pendingDecisions, k)

		return false, nil
	}

	clusterName := pod.Annotations[common.AnnotationKeyClusterName]
	if clusterName != "" {
		// already scheduled
		// bind controller will check if allowed
		return false, nil
	}

	return true, nil
}

func (r applier) Mutate(obj interface{}) error {
	podObs := obj.(*v1alpha1.PodObservation)
	pod := podObs.Status.LiveState
	srcPodManifest, ok := pod.Annotations[common.AnnotationKeySourcePodManifest]
	if !ok {
		return fmt.Errorf("no source pod manifest on proxy pod")
	}
	srcPod := &corev1.Pod{}
	if err := yaml.Unmarshal([]byte(srcPodManifest), srcPod); err != nil {
		return fmt.Errorf("cannot unmarshal source pod manifest: %v", err)
	}

	srcClusterName := r.schedCfg.GetObservationClusterName(podObs)
	srcPod.ClusterName = srcClusterName // so basic scheduler's Schedule() can default to source cluster
	// if no cluster can accommodate the delegate pod, Schedule() sets clusterName to srcPod.ClusterName
	clusterName, err := r.scheduler.Schedule(srcPod)
	if err != nil {
		// TODO: pod observations pending cluster if not enough resources and node pools not elastic
		// rather than send back to original cluster
		// basic scheduler's Schedule() handles error already, but a different implementation could return an error
		runtime.HandleError(fmt.Errorf("cannot schedule proxy pod observation %s in namespace %s (handled: scheduling to original cluster instead): %v", podObs.Name, podObs.Namespace, err))
		clusterName = srcClusterName
	}

	srcPod.ClusterName = clusterName // set ClusterName because scheduler expects it on pending decisions
	r.pendingDecisions[key{
		clusterName: clusterName,
		namespace:   pod.Namespace,
		name:        pod.Name,
	}] = srcPod

	podObs.Status.LiveState.Annotations[common.AnnotationKeyClusterName] = clusterName
	return nil
}

type pendingDecisions map[key]*corev1.Pod

type key struct {
	clusterName string
	namespace   string
	name        string
}

type SchedulerShim interface {
	Schedule(pod *corev1.Pod) (string, error)
}
