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

package feedback

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-controller/pkg/controller"
	"admiralty.io/multicluster-controller/pkg/patterns"
	"admiralty.io/multicluster-controller/pkg/patterns/gc"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewController(agent *cluster.Cluster, scheduler *cluster.Cluster, observationNamespace string,
	agentClientset *kubernetes.Clientset) (*controller.Controller, error) {

	agentClient, err := agent.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for agent cluster: %v", err)
	}
	schedulerClient, err := scheduler.GetDelegatingClient()
	if err != nil {
		return nil, fmt.Errorf("getting delegating client for scheduler cluster: %v", err)
	}

	co := controller.New(&reconciler{
		agent:          agentClient,
		scheduler:      schedulerClient,
		agentClientset: agentClientset,
		agentConfig:    agent.Config,
	}, controller.Options{})

	if err := co.WatchResourceReconcileObject(scheduler, &v1alpha1.PodObservation{}, controller.WatchOptions{
		Namespace:     observationNamespace,
		LabelSelector: labels.SelectorFromValidatedSet(labels.Set{gc.LabelParentClusterName: agent.GetClusterName()}),
		CustomPredicate: func(obj interface{}) bool {
			podObs := obj.(*v1alpha1.PodObservation)
			return podObs.Status.DelegateState != nil
		},
	}); err != nil {
		return nil, fmt.Errorf("setting up pod observation watch on scheduler cluster: %v", err)
	}

	return co, nil
}

type reconciler struct {
	agent          client.Client
	scheduler      client.Client
	agentConfig    *rest.Config
	agentClientset *kubernetes.Clientset
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	podObs := &v1alpha1.PodObservation{}
	if err := r.scheduler.Get(context.Background(), req.NamespacedName, podObs); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get pod observation %s in namespace %s in scheduler cluster: %v", req.Name, req.Namespace, err)
		}
		return reconcile.Result{}, nil
	}
	delegatePod := podObs.Status.DelegateState // not nil thanks to watchoptions custompredicate

	proxyPodNs, ok := podObs.Labels[gc.LabelParentNamespace]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("pod observation %s in namespace %s in scheduler cluster is missing label %s",
			req.Name, req.Namespace, gc.LabelParentNamespace)
	}
	proxyPodName, ok := podObs.Labels[gc.LabelParentName]
	if !ok {
		return reconcile.Result{}, fmt.Errorf("pod observation %s in namespace %s in scheduler cluster is missing label %s",
			req.Name, req.Namespace, gc.LabelParentName)
	}
	// TODO use controller ref instead?

	proxyPod := &corev1.Pod{}
	if err := r.agent.Get(context.Background(), types.NamespacedName{Namespace: proxyPodNs, Name: proxyPodName}, proxyPod); err != nil {
		if !errors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("cannot get proxy pod %s in namespace %s in agent cluster: %v", proxyPodName, proxyPodNs, err)
		}
		return reconcile.Result{}, nil
	}

	mcProxyPodAnnotations, otherProxyPodAnnotations := filterAnnotations(proxyPod.Annotations)
	_, otherDelegatePodAnnotations := filterAnnotations(delegatePod.Annotations)

	if !reflect.DeepEqual(otherProxyPodAnnotations, otherDelegatePodAnnotations) {
		for k, v := range otherDelegatePodAnnotations {
			mcProxyPodAnnotations[k] = v
		}
		proxyPod.Annotations = mcProxyPodAnnotations
		if err := r.agent.Update(context.Background(), proxyPod); err != nil {
			if patterns.IsOptimisticLockError(err) {
				// TODO watch proxy pods instead, to requeue when the cache is updated
				oneSec, _ := time.ParseDuration("1s")
				return reconcile.Result{RequeueAfter: oneSec}, nil
			}
			return reconcile.Result{}, fmt.Errorf("cannot update proxy pod %s in namespace %s in agent cluster: %v", proxyPod.Name, proxyPod.Namespace, err)
		}
	}

	if proxyPod.Status.Phase == corev1.PodRunning {
		if delegatePod.Status.Phase == corev1.PodSucceeded {
			for _, c := range proxyPod.Spec.Containers {
				if err := r.sendSignal(proxyPod.Name, proxyPod.Namespace, c.Name, "SIGUSR1"); err != nil {
					return reconcile.Result{}, fmt.Errorf("cannot send SIGUSR1=PodSucceeded to container %s in pod %s in namespace %s in agent cluster: %v", c.Name, proxyPod.Name, proxyPod.Namespace, err)
				}
			}
		} else if delegatePod.Status.Phase == corev1.PodFailed {
			for _, c := range proxyPod.Spec.Containers {
				if err := r.sendSignal(proxyPod.Name, proxyPod.Namespace, c.Name, "-SIGUSR2"); err != nil {
					return reconcile.Result{}, fmt.Errorf("cannot send SIGUSR1=PodFailed to container %s in pod %s in namespace %s in agent cluster: %v", c.Name, proxyPod.Name, proxyPod.Namespace, err)
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) sendSignal(pod string, namespace string, container string, signal string) error {
	if err := r.exec(pod, namespace, container, true, true, "kill", "-"+signal, "1"); err != nil {
		if !strings.Contains(err.Error(), "command terminated with exit code 137") &&
			!strings.Contains(err.Error(), "container not found") {
			// "exit code 137" is expected, "container not found" could happen if some containers have been killed in a previous pass
			return err
		}
	}
	return nil
}

func (r *reconciler) exec(pod string, namespace string, container string, stdout bool, stderr bool, command ...string) error {
	execRequest := r.agentClientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		Param("stdout", fmt.Sprintf("%v", stdout)).
		Param("stderr", fmt.Sprintf("%v", stderr)).
		Param("container", container)

	for _, cmd := range command {
		execRequest = execRequest.Param("command", cmd)
	}

	exec, err := remotecommand.NewSPDYExecutor(r.agentConfig, "POST", execRequest.URL())
	if err != nil {
		return fmt.Errorf("cannot create SPDY executor to POST to %s (agent cluster): %v", execRequest.URL(), err)
	}

	var stdOut bytes.Buffer
	var stdErr bytes.Buffer
	if err := exec.Stream(remotecommand.StreamOptions{
		Stdout: &stdOut,
		Stderr: &stdErr,
		Tty:    false,
	}); err != nil {
		return fmt.Errorf("streaming error: %v", err)
	}

	return nil
}

func filterAnnotations(annotations map[string]string) (map[string]string, map[string]string) {
	mcAnnotations := make(map[string]string)
	otherAnnotations := make(map[string]string)
	for k, v := range annotations {
		if strings.HasPrefix(k, common.KeyPrefix) {
			mcAnnotations[k] = v
		} else {
			otherAnnotations[k] = v
		}
	}
	return mcAnnotations, otherAnnotations
}
