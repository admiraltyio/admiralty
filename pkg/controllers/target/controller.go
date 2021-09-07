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

package target

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	multiclusterv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	informers "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/multicluster/v1alpha1"
	listers "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
)

type reconciler struct {
	kubeClient kubernetes.Interface

	clusterTargetLister listers.ClusterTargetLister
	targetLister        listers.TargetLister
	secretLister        corelisters.SecretLister

	clusterTargetIndex cache.Indexer
	targetIndex        cache.Indexer

	installNamespace                string
	controllerManagerDeploymentName string
	proxySchedulerDeploymentName    string

	mu                   sync.Mutex
	targetSpecs          map[string]interface{}
	kubeconfigSecretData map[string]interface{}
}

const (
	clusterTargetByKubeconfigSecret = "clusterTargetByKubeconfigSecret"
	targetByKubeconfigSecret        = "targetByKubeconfigSecret"
)

func NewController(
	kubeClient kubernetes.Interface,
	installNamespace string,
	controllerManagerDeploymentName string,
	proxySchedulerDeploymentName string,
	clusterTargetInformer informers.ClusterTargetInformer,
	targetInformer informers.TargetInformer,
	secretInformer coreinformers.SecretInformer,
) *controller.Controller {

	r := &reconciler{
		kubeClient: kubeClient,

		clusterTargetLister: clusterTargetInformer.Lister(),
		targetLister:        targetInformer.Lister(),
		secretLister:        secretInformer.Lister(),

		clusterTargetIndex: clusterTargetInformer.Informer().GetIndexer(),
		targetIndex:        targetInformer.Informer().GetIndexer(),

		installNamespace:                installNamespace,
		controllerManagerDeploymentName: controllerManagerDeploymentName,
		proxySchedulerDeploymentName:    proxySchedulerDeploymentName,
	}

	c := controller.New("source", r,
		clusterTargetInformer.Informer().HasSynced,
		targetInformer.Informer().HasSynced,
		secretInformer.Informer().HasSynced)

	clusterTargetInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))
	targetInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	utilruntime.Must(clusterTargetInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		clusterTargetByKubeconfigSecret: func(obj interface{}) ([]string, error) {
			ct := obj.(*multiclusterv1alpha1.ClusterTarget)
			if s := ct.Spec.KubeconfigSecret; s != nil {
				return []string{fmt.Sprintf("%s/%s", s.Namespace, s.Name)}, nil
			}
			return nil, nil
		},
	}))
	utilruntime.Must(targetInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		targetByKubeconfigSecret: func(obj interface{}) ([]string, error) {
			t := obj.(*multiclusterv1alpha1.Target)
			if s := t.Spec.KubeconfigSecret; s != nil {
				return []string{fmt.Sprintf("%s/%s", t.Namespace, s.Name)}, nil
			}
			return nil, nil
		},
	}))

	secretInformer.Informer().AddEventHandler(controller.HandleAllWith(func(obj interface{}) {
		secret := obj.(*corev1.Secret)
		ct, err := r.clusterTargetIndex.ByIndex(clusterTargetByKubeconfigSecret, fmt.Sprintf("%s/%s", secret.Namespace, secret.Name))
		utilruntime.Must(err)
		for _, obj := range ct {
			c.EnqueueObject(obj)
		}
		t, err := r.targetIndex.ByIndex(targetByKubeconfigSecret, fmt.Sprintf("%s/%s", secret.Namespace, secret.Name))
		utilruntime.Must(err)
		for _, obj := range t {
			c.EnqueueObject(obj)
		}
	}))

	return c
}

func (c *reconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	clusterTargets, err := c.clusterTargetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	targets, err := c.targetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	targetSpecs := make(map[string]interface{}, len(targets)+len(clusterTargets))
	kubeconfigSecretData := make(map[string]interface{}, len(targets)+len(clusterTargets))
	for _, t := range clusterTargets {
		key := fmt.Sprintf("%s/%s", t.Namespace, t.Name)
		targetSpecs[key] = t.Spec
		if s := t.Spec.KubeconfigSecret; s != nil {
			secret, err := c.secretLister.Secrets(s.Namespace).Get(s.Name)
			if err != nil {
				if !errors.IsNotFound(err) {
					return nil, err
				}
				continue
			}
			kubeconfigSecretData[key] = secret.Data
		}
	}
	for _, t := range targets {
		key := fmt.Sprintf("%s/%s", t.Namespace, t.Name)
		targetSpecs[key] = t.Spec
		if s := t.Spec.KubeconfigSecret; s != nil {
			secret, err := c.secretLister.Secrets(t.Namespace).Get(s.Name)
			if err != nil {
				if !errors.IsNotFound(err) {
					return nil, err
				}
				continue
			}
			kubeconfigSecretData[key] = secret.Data
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if !reflect.DeepEqual(c.targetSpecs, targetSpecs) || !reflect.DeepEqual(c.kubeconfigSecretData, kubeconfigSecretData) {
		// rollout restarts
		g := multierror.Group{}
		for _, deployName := range []string{c.controllerManagerDeploymentName, c.proxySchedulerDeploymentName} {
			deployName := deployName
			g.Go(func() error {
				p := []byte(`{"spec":{"template":{"metadata":{"annotations":{"` + common.AnnotationKeyRestartedAt + `":"` + time.Now().UTC().Format(time.RFC3339) + `"}}}}}`)
				_, err := c.kubeClient.AppsV1().Deployments(c.installNamespace).Patch(ctx, deployName, types.MergePatchType, p, metav1.PatchOptions{})
				return err
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		c.targetSpecs = targetSpecs
		c.kubeconfigSecretData = kubeconfigSecretData
	}

	return nil, nil
}
