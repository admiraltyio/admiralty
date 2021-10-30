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

package agent

import (
	"context"
	"log"

	"admiralty.io/multicluster-scheduler/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	"admiralty.io/multicluster-scheduler/pkg/name"
)

type Config struct {
	Targets []Target
}

func (c Config) GetKnownFinalizers() []string {
	var knownFinalizers []string
	for _, target := range c.Targets {
		knownFinalizers = append(knownFinalizers, target.GetFinalizer())
	}
	return knownFinalizers
}

type Target struct {
	Name                 string
	ClientConfig         *rest.Config
	Self                 bool // optimization to re-use clients, informers, etc.
	Namespace            string
	ExcludedLabelsRegexp *string
}

func (t Target) GetKey() string {
	return name.FromParts(name.Long, []int{0}, []int{1}, "admiralty", t.Namespace, t.Name)
}

func (t Target) GetFinalizer() string {
	return common.KeyPrefix + name.FromParts(name.Short, nil, []int{0}, t.Namespace, t.Name)
}

// until we watch targets at runtime, we can already load them from objects at startup
func NewFromCRD(ctx context.Context) Config {
	cfg := config.GetConfigOrDie()

	customClient, err := versioned.NewForConfig(cfg)
	utilruntime.Must(err)

	k, err := kubernetes.NewForConfig(cfg)
	utilruntime.Must(err)

	agentCfg := Config{}

	cl, err := customClient.MulticlusterV1alpha1().ClusterTargets().List(ctx, metav1.ListOptions{})
	utilruntime.Must(err)
	for _, t := range cl.Items {
		addClusterTarget(ctx, k, &agentCfg, t)
	}

	l, err := customClient.MulticlusterV1alpha1().Targets(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	utilruntime.Must(err)
	for _, t := range l.Items {
		addTarget(ctx, k, &agentCfg, t)
	}

	return agentCfg
}

func addClusterTarget(ctx context.Context, k *kubernetes.Clientset, agentCfg *Config, t v1alpha1.ClusterTarget) {
	if t.Spec.Self == (t.Spec.KubeconfigSecret != nil) {
		log.Printf("invalid ClusterTarget %s: self XOR kubeconfigSecret != nil", t.Name)
		return
		// TODO validating webhook to catch user error upstream
	}
	var cfg *rest.Config
	if kcfg := t.Spec.KubeconfigSecret; kcfg != nil {
		var err error
		cfg, err = getConfigFromKubeconfigSecretOrDie(ctx, k, kcfg.Namespace, kcfg.Name, kcfg.Key, kcfg.Context)
		if err != nil {
			log.Printf("invalid ClusterTarget %s: %v", t.Name, err)
			return
		}
	} else {
		cfg = config.GetConfigOrDie()
	}

	c := Target{
		Name:                 t.Name,
		ClientConfig:         cfg,
		Namespace:            corev1.NamespaceAll,
		Self:                 t.Spec.Self,
		ExcludedLabelsRegexp: t.Spec.ExcludedLabelsRegexp,
	}
	agentCfg.Targets = append(agentCfg.Targets, c)
}

func addTarget(ctx context.Context, k *kubernetes.Clientset, agentCfg *Config, t v1alpha1.Target) {
	if t.Spec.Self == (t.Spec.KubeconfigSecret != nil) {
		log.Printf("invalid Target %s in namespace %s: self XOR kubeconfigSecret != nil", t.Name, t.Namespace)
		return
		// TODO validating webhook to catch user error upstream
	}
	var cfg *rest.Config
	if kcfg := t.Spec.KubeconfigSecret; kcfg != nil {
		var err error
		cfg, err = getConfigFromKubeconfigSecretOrDie(ctx, k, t.Namespace, kcfg.Name, kcfg.Key, kcfg.Context)
		if err != nil {
			log.Printf("invalid Target %s in namespace %s: %v", t.Name, t.Namespace, err)
			return
		}
	} else {
		cfg = config.GetConfigOrDie()
	}

	c := Target{
		Name:                 t.Name,
		ClientConfig:         cfg,
		Namespace:            t.Namespace,
		Self:                 t.Spec.Self,
		ExcludedLabelsRegexp: t.Spec.ExcludedLabelsRegexp,
	}
	agentCfg.Targets = append(agentCfg.Targets, c)
}

func getConfigFromKubeconfigSecretOrDie(ctx context.Context, k *kubernetes.Clientset, namespace, name, key, context string) (*rest.Config, error) {
	if key == "" {
		key = "config"
	}

	s, err := k.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	cfg0, err := clientcmd.Load(s.Data[key])
	if err != nil {
		return nil, err
	}

	cfg1 := clientcmd.NewDefaultClientConfig(*cfg0, &clientcmd.ConfigOverrides{CurrentContext: context})

	cfg2, err := cfg1.ClientConfig()
	if err != nil {
		return nil, err
	}

	return cfg2, nil
}
