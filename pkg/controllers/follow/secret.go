/*
 * Copyright 2023 The Multicluster-Scheduler Authors.
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

package follow

import (
	"context"
	"fmt"
	"reflect"
	"time"

	agentconfig "admiralty.io/multicluster-scheduler/pkg/config/agent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"admiralty.io/multicluster-scheduler/pkg/model/proxypod"
)

const proxyPodBySecrets = "proxyPodBySecrets"

type secretReconciler struct {
	clusterName string
	target      agentconfig.Target

	kubeclientset kubernetes.Interface
	remoteClient  kubernetes.Interface

	podLister    corelisters.PodLister
	secretLister corelisters.SecretLister

	remoteSecretLister corelisters.SecretLister

	podIndex cache.Indexer
}

func NewSecretController(
	clusterName string,
	target agentconfig.Target,

	kubeclientset kubernetes.Interface,
	remoteClient kubernetes.Interface,

	podInformer coreinformers.PodInformer,
	secretInformer coreinformers.SecretInformer,

	remoteSecretInformer coreinformers.SecretInformer) *controller.Controller {

	r := &secretReconciler{
		clusterName: clusterName,
		target:      target,

		kubeclientset: kubeclientset,
		remoteClient:  remoteClient,

		podLister:    podInformer.Lister(),
		secretLister: secretInformer.Lister(),

		remoteSecretLister: remoteSecretInformer.Lister(),

		podIndex: podInformer.Informer().GetIndexer(),
	}

	c := controller.New("secrets-follow", r, podInformer.Informer().HasSynced, secretInformer.Informer().HasSynced, remoteSecretInformer.Informer().HasSynced)

	secretInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	remoteSecretInformer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController(clusterName)))

	podInformer.Informer().AddEventHandler(controller.HandleAllWith(enqueueProxyPodsSecrets(c)))
	utilruntime.Must(podInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		proxyPodBySecrets: indexProxyPodBySecrets,
	}))

	// TODO follow ingresses (TLS) when ingresses follows pods

	return c
}

func enqueueProxyPodsSecrets(c *controller.Controller) func(obj interface{}) {
	return func(obj interface{}) {
		keys, _ := indexProxyPodBySecrets(obj)
		for _, key := range keys {
			c.EnqueueKey(key)
		}
	}
}

func indexProxyPodBySecrets(obj interface{}) ([]string, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok || !proxypod.IsProxy(pod) {
		return nil, nil
	}
	var keys []string
	for _, vol := range pod.Spec.Volumes {
		if s := vol.Secret; s != nil {
			keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, s.SecretName))
		}
		if proj := vol.Projected; proj != nil {
			for _, src := range proj.Sources {
				if s := src.Secret; s != nil {
					keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, s.Name))
				}
			}
		}
	}
	for _, c := range pod.Spec.Containers {
		keys = indexContainerBySecret(c, keys, pod)
	}
	for _, c := range pod.Spec.InitContainers {
		keys = indexContainerBySecret(c, keys, pod)
	}
	for _, s := range pod.Spec.ImagePullSecrets {
		keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, s.Name))
	}
	return keys, nil
}

func indexContainerBySecret(c corev1.Container, keys []string, pod *corev1.Pod) []string {
	for _, envVar := range c.Env {
		if from := envVar.ValueFrom; from != nil {
			if s := from.SecretKeyRef; s != nil {
				keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, s.Name))
			}
		}
	}
	for _, src := range c.EnvFrom {
		if s := src.SecretRef; s != nil {
			keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, s.Name))
		}
	}
	return keys
}

func (r secretReconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	remoteSecret, err := r.remoteSecretLister.Secrets(namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	secret, err := r.secretLister.Secrets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			if remoteSecret != nil && controller.IsRemoteControlled(remoteSecret, r.clusterName) {
				if err := r.remoteClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
					return nil, fmt.Errorf("cannot delete orphaned secret: %v", err)
				}
			}
			return nil, nil
		}
		return nil, err
	}

	terminating := secret.DeletionTimestamp != nil

	hasFinalizer, j := controller.HasFinalizer(secret.Finalizers, r.target.Finalizer)

	shouldFollow := r.shouldFollow(namespace, name)

	// get remote owned secrets
	// eponymous secrets that aren't owned are not included (because we don't want to delete them, see below)
	// include owned secrets in targets that no longer need them (because we shouldn't forget them when deleting)
	if remoteSecret != nil && !controller.ParentControlsChild(remoteSecret, secret) {
		return nil, nil
	}

	if terminating {
		if remoteSecret != nil {
			if err := r.remoteClient.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		} else if hasFinalizer {
			_, err = r.removeFinalizer(ctx, secret, j)
			if err != nil {
				return nil, err
			}
		}
	} else if shouldFollow {
		if !hasFinalizer {
			secret, err = r.addFinalizer(ctx, secret)
			if err != nil {
				return nil, err
			}
		}

		if remoteSecret == nil {
			gold := r.makeRemoteSecret(secret)
			_, err := r.remoteClient.CoreV1().Secrets(namespace).Create(ctx, gold, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return nil, err
			}
		} else if !reflect.DeepEqual(remoteSecret.Data, secret.Data) ||
			remoteSecret.Labels[common.LabelKeyParentClusterName] != r.clusterName {

			remoteSecretCopy := remoteSecret.DeepCopy()
			remoteSecretCopy.Data = make(map[string][]byte, len(secret.Data))
			for k, v := range secret.Data {
				remoteSecretCopy.Data[k] = make([]byte, len(v))
				copy(remoteSecretCopy.Data[k], v)
			}
			// add or update parent cluster name
			// labels is non-nil because it includes parent UID
			remoteSecretCopy.Labels[common.LabelKeyParentClusterName] = r.clusterName

			_, err := r.remoteClient.CoreV1().Secrets(namespace).Update(ctx, remoteSecretCopy, metav1.UpdateOptions{})
			if err != nil {
				return nil, err
			}
		}
	}

	// TODO? cleanup remote secrets that aren't referred to by proxy pods

	return requeueAfter, nil
}

func (r secretReconciler) shouldFollow(namespace, name string) bool {
	objs, err := r.podIndex.ByIndex(proxyPodBySecrets, fmt.Sprintf("%s/%s", namespace, name))
	utilruntime.Must(err)
	for _, obj := range objs {
		proxyPod := obj.(*corev1.Pod)
		if proxypod.GetScheduledClusterName(proxyPod) == r.target.VirtualNodeName {
			return true
		}
	}
	return false
}

func (r secretReconciler) addFinalizer(ctx context.Context, secret *corev1.Secret) (*corev1.Secret, error) {
	secretCopy := secret.DeepCopy()
	secretCopy.Finalizers = append(secretCopy.Finalizers, r.target.Finalizer)
	if secretCopy.Labels == nil {
		secretCopy.Labels = map[string]string{}
	}
	secretCopy.Labels[common.LabelKeyHasFinalizer] = "true"
	return r.kubeclientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secretCopy, metav1.UpdateOptions{})
}

func (r secretReconciler) removeFinalizer(ctx context.Context, secret *corev1.Secret, j int) (*corev1.Secret, error) {
	secretCopy := secret.DeepCopy()
	secretCopy.Finalizers = append(secretCopy.Finalizers[:j], secretCopy.Finalizers[j+1:]...)
	return r.kubeclientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secretCopy, metav1.UpdateOptions{})
}

func (r secretReconciler) makeRemoteSecret(secret *corev1.Secret) *corev1.Secret {
	gold := &corev1.Secret{}
	gold.Name = secret.Name
	gold.Labels = make(map[string]string, len(secret.Labels))
	for k, v := range secret.Labels {
		gold.Labels[k] = v
	}
	gold.Annotations = make(map[string]string, len(secret.Annotations))
	for k, v := range secret.Annotations {
		gold.Annotations[k] = v
	}
	controller.AddRemoteControllerReference(gold, secret, r.clusterName)
	gold.Type = secret.Type
	gold.Data = make(map[string][]byte, len(secret.Data))
	for k, v := range secret.Data {
		gold.Data[k] = make([]byte, len(v))
		copy(gold.Data[k], v)
	}
	return gold
}
