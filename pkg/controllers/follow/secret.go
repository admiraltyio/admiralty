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

package follow

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"admiralty.io/multicluster-controller/pkg/patterns"
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
	kubeclientset kubernetes.Interface
	remoteClients map[string]kubernetes.Interface

	podLister    corelisters.PodLister
	secretLister corelisters.SecretLister

	remoteSecretLister map[string]corelisters.SecretLister

	podIndex cache.Indexer

	selfTargetName string
}

func NewSecretController(
	kubeclientset kubernetes.Interface,
	remoteClients map[string]kubernetes.Interface,

	podInformer coreinformers.PodInformer,
	secretInformer coreinformers.SecretInformer,

	remoteSecretInformers map[string]coreinformers.SecretInformer,

	selfTargetName string) *controller.Controller {

	r := &secretReconciler{
		kubeclientset: kubeclientset,
		remoteClients: remoteClients,

		podLister:    podInformer.Lister(),
		secretLister: secretInformer.Lister(),

		remoteSecretLister: make(map[string]corelisters.SecretLister, len(remoteSecretInformers)),

		podIndex: podInformer.Informer().GetIndexer(),

		selfTargetName: selfTargetName,
	}

	informersSynced := make([]cache.InformerSynced, 2+len(remoteSecretInformers))
	informersSynced[0] = podInformer.Informer().HasSynced
	informersSynced[1] = secretInformer.Informer().HasSynced

	i := 2
	for targetName, informer := range remoteSecretInformers {
		r.remoteSecretLister[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	c := controller.New("secrets-follow", r, informersSynced...)

	secretInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	getSecret := func(namespace, name string) (metav1.Object, error) {
		return r.secretLister.Secrets(namespace).Get(name)
	}
	for _, informer := range remoteSecretInformers {
		informer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController("Secret", getSecret)))
	}

	podInformer.Informer().AddEventHandler(controller.HandleAllWith(enqueueProxyPodsSecrets(c)))
	podInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		proxyPodBySecrets: indexProxyPodBySecrets,
	})

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

	secret, err := r.secretLister.Secrets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	terminating := secret.DeletionTimestamp != nil

	j := -1
	for i, f := range secret.Finalizers {
		if f == common.CrossClusterGarbageCollectionFinalizer {
			j = i
			break
		}
	}
	hasFinalizer := j > -1

	objs, err := r.podIndex.ByIndex(proxyPodBySecrets, fmt.Sprintf("%s/%s", namespace, name))
	utilruntime.Must(err)
	targetNames := map[string]bool{}
	for _, obj := range objs {
		proxyPod := obj.(*corev1.Pod)
		if proxypod.IsScheduled(proxyPod) {
			if targetName := proxypod.GetScheduledClusterName(proxyPod); targetName != r.selfTargetName {
				targetNames[targetName] = true
			}
		}
	}

	// get remote owned secrets
	// eponymous secrets that aren't owned are not included (because we don't want to delete them, see below)
	// include owned secrets in targets that no longer need them (because we shouldn't forget them when deleting)
	remoteSecrets := make(map[string]*corev1.Secret, len(r.remoteSecretLister))
	for targetName, lister := range r.remoteSecretLister {
		remoteSecret, err := lister.Secrets(namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				// error with a target shouldn't block reconciliation with other targets
				d := time.Second
				requeueAfter = &d // named returned
				utilruntime.HandleError(err)
			}
			continue
		}
		if remoteSecret.Labels[common.LabelKeyParentUID] == string(secret.UID) {
			remoteSecrets[targetName] = remoteSecret
		}
	}

	if terminating {
		for targetName := range remoteSecrets {
			if err := r.remoteClients[targetName].CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		}
		if hasFinalizer && len(remoteSecrets) == 0 {
			requeueAfter, err = r.removeFinalizer(ctx, secret, j)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}
	} else if len(targetNames) == 0 {
		// remove extraneous finalizers added pre-0.9.3 (to all config maps and secrets)
		if hasFinalizer && len(remoteSecrets) == 0 {
			requeueAfter, err = r.removeFinalizer(ctx, secret, j)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}
	} else {
		if !hasFinalizer {
			requeueAfter, err = r.addFinalizer(ctx, secret)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}

		for targetName := range targetNames {
			remoteSecret := remoteSecrets[targetName]
			if remoteSecret == nil {
				gold := makeRemoteSecret(secret)
				_, err := r.remoteClients[targetName].CoreV1().Secrets(namespace).Create(ctx, gold, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					// error with a target shouldn't block reconciliation with other targets
					d := time.Second
					requeueAfter = &d // named returned
					utilruntime.HandleError(err)
				}
			} else if !reflect.DeepEqual(remoteSecret.Data, secret.Data) {
				remoteSecretCopy := remoteSecret.DeepCopy()
				remoteSecretCopy.Data = make(map[string][]byte, len(secret.Data))
				for k, v := range secret.Data {
					remoteSecretCopy.Data[k] = make([]byte, len(v))
					copy(remoteSecretCopy.Data[k], v)
				}
				_, err := r.remoteClients[targetName].CoreV1().Secrets(namespace).Update(ctx, remoteSecretCopy, metav1.UpdateOptions{})
				if err != nil {
					// error with a target shouldn't block reconciliation with other targets
					d := time.Second
					requeueAfter = &d // named returned
					utilruntime.HandleError(err)
				}
			}
		}
	}

	// TODO? cleanup remote secrets that aren't referred to by proxy pods

	return requeueAfter, nil
}

func (r secretReconciler) addFinalizer(ctx context.Context, secret *corev1.Secret) (*time.Duration, error) {
	secretCopy := secret.DeepCopy()
	secretCopy.Finalizers = append(secretCopy.Finalizers, common.CrossClusterGarbageCollectionFinalizer)
	if secretCopy.Labels == nil {
		secretCopy.Labels = map[string]string{}
	}
	secretCopy.Labels[common.LabelKeyHasFinalizer] = "true"
	var err error
	if _, err = r.kubeclientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secretCopy, metav1.UpdateOptions{}); err != nil {
		if patterns.IsOptimisticLockError(err) {
			d := time.Second
			return &d, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}

func (r secretReconciler) removeFinalizer(ctx context.Context, secret *corev1.Secret, j int) (*time.Duration, error) {
	secretCopy := secret.DeepCopy()
	secretCopy.Finalizers = append(secretCopy.Finalizers[:j], secretCopy.Finalizers[j+1:]...)
	delete(secretCopy.Labels, common.LabelKeyHasFinalizer)
	var err error
	if _, err = r.kubeclientset.CoreV1().Secrets(secret.Namespace).Update(ctx, secretCopy, metav1.UpdateOptions{}); err != nil {
		if patterns.IsOptimisticLockError(err) {
			d := time.Second
			return &d, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}

func makeRemoteSecret(secret *corev1.Secret) *corev1.Secret {
	gold := &corev1.Secret{}
	gold.Name = secret.Name
	gold.Labels = make(map[string]string, len(secret.Labels)+1)
	gold.Labels[common.LabelKeyParentUID] = string(secret.UID) // cross-cluster "owner reference"
	for k, v := range secret.Labels {
		gold.Labels[k] = v
	}
	gold.Annotations = make(map[string]string, len(secret.Annotations))
	for k, v := range secret.Annotations {
		gold.Annotations[k] = v
	}
	gold.Type = secret.Type
	gold.Data = make(map[string][]byte, len(secret.Data))
	for k, v := range secret.Data {
		gold.Data[k] = make([]byte, len(v))
		copy(gold.Data[k], v)
	}
	return gold
}
