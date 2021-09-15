/*
 * Copyright 2021 The Multicluster-Scheduler Authors.
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

package main

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"admiralty.io/multicluster-scheduler/pkg/common"
)

func main() {
	ctx := context.Background()

	cfg := config.GetConfigOrDie()

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	p := patchAll{k}

	p.patchPods(ctx)
	p.patchServices(ctx)
	p.patchConfigMaps(ctx)
	p.patchSecrets(ctx)
	p.patchIngresses(ctx)
}

func patch(finalizers []string) string {
	return `{"metadata":{"$deleteFromPrimitiveList/finalizers":["` + strings.Join(finalizers, `","`) + `"]}}`
}

type patchAll struct {
	k *kubernetes.Clientset
}

func (p patchAll) patchPods(ctx context.Context) {
	l, err := p.k.CoreV1().Pods("").List(ctx, metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		var finalizers []string
		for _, f := range o.Finalizers {
			if strings.HasPrefix(f, common.KeyPrefix) {
				finalizers = append(finalizers, f)
			}
		}
		_, err := p.k.CoreV1().Pods(o.Namespace).Patch(ctx, o.Name, types.StrategicMergePatchType, []byte(patch(finalizers)), metav1.PatchOptions{})
		utilruntime.Must(err)
	}
}

func (p patchAll) patchServices(ctx context.Context) {
	l, err := p.k.CoreV1().Services("").List(ctx, metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		var finalizers []string
		for _, f := range o.Finalizers {
			if strings.HasPrefix(f, common.KeyPrefix) {
				finalizers = append(finalizers, f)
			}
		}
		_, err := p.k.CoreV1().Services(o.Namespace).Patch(ctx, o.Name, types.StrategicMergePatchType, []byte(patch(finalizers)), metav1.PatchOptions{})
		utilruntime.Must(err)
	}
}

func (p patchAll) patchConfigMaps(ctx context.Context) {
	l, err := p.k.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		var finalizers []string
		for _, f := range o.Finalizers {
			if strings.HasPrefix(f, common.KeyPrefix) {
				finalizers = append(finalizers, f)
			}
		}
		_, err := p.k.CoreV1().ConfigMaps(o.Namespace).Patch(ctx, o.Name, types.StrategicMergePatchType, []byte(patch(finalizers)), metav1.PatchOptions{})
		utilruntime.Must(err)
	}
}

func (p patchAll) patchSecrets(ctx context.Context) {
	l, err := p.k.CoreV1().Secrets("").List(ctx, metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		var finalizers []string
		for _, f := range o.Finalizers {
			if strings.HasPrefix(f, common.KeyPrefix) {
				finalizers = append(finalizers, f)
			}
		}
		_, err := p.k.CoreV1().Secrets(o.Namespace).Patch(ctx, o.Name, types.StrategicMergePatchType, []byte(patch(finalizers)), metav1.PatchOptions{})
		utilruntime.Must(err)
	}
}

func (p patchAll) patchIngresses(ctx context.Context) {
	l, err := p.k.NetworkingV1beta1().Ingresses("").List(ctx, metav1.ListOptions{LabelSelector: common.LabelKeyHasFinalizer})
	utilruntime.Must(err)
	for _, o := range l.Items {
		var finalizers []string
		for _, f := range o.Finalizers {
			if strings.HasPrefix(f, common.KeyPrefix) {
				finalizers = append(finalizers, f)
			}
		}
		_, err := p.k.NetworkingV1beta1().Ingresses(o.Namespace).Patch(ctx, o.Name, types.StrategicMergePatchType, []byte(patch(finalizers)), metav1.PatchOptions{})
		utilruntime.Must(err)
	}
}
