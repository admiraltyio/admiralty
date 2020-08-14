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

const proxyPodByConfigMaps = "proxyPodByConfigMaps"

type configMapReconciler struct {
	kubeclientset kubernetes.Interface
	remoteClients map[string]kubernetes.Interface

	podLister       corelisters.PodLister
	configMapLister corelisters.ConfigMapLister

	remoteConfigMapLister map[string]corelisters.ConfigMapLister

	podIndex cache.Indexer

	selfTargetKeys map[string]bool
}

func NewConfigMapController(
	kubeclientset kubernetes.Interface,
	remoteClients map[string]kubernetes.Interface,

	podInformer coreinformers.PodInformer,
	configMapInformer coreinformers.ConfigMapInformer,

	remoteConfigMapInformers map[string]coreinformers.ConfigMapInformer,

	selfTargetKeys map[string]bool) *controller.Controller {

	r := &configMapReconciler{
		kubeclientset: kubeclientset,
		remoteClients: remoteClients,

		podLister:       podInformer.Lister(),
		configMapLister: configMapInformer.Lister(),

		remoteConfigMapLister: make(map[string]corelisters.ConfigMapLister, len(remoteConfigMapInformers)),

		podIndex: podInformer.Informer().GetIndexer(),

		selfTargetKeys: selfTargetKeys,
	}

	informersSynced := make([]cache.InformerSynced, 2+len(remoteConfigMapInformers))
	informersSynced[0] = podInformer.Informer().HasSynced
	informersSynced[1] = configMapInformer.Informer().HasSynced

	i := 2
	for targetName, informer := range remoteConfigMapInformers {
		r.remoteConfigMapLister[targetName] = informer.Lister()
		informersSynced[i] = informer.Informer().HasSynced
		i++
	}

	c := controller.New("config-maps-follow", r, informersSynced...)

	configMapInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(c.EnqueueObject))

	getConfigMap := func(namespace, name string) (metav1.Object, error) {
		return r.configMapLister.ConfigMaps(namespace).Get(name)
	}
	for _, informer := range remoteConfigMapInformers {
		informer.Informer().AddEventHandler(controller.HandleAllWith(c.EnqueueRemoteController("ConfigMap", getConfigMap)))
	}

	podInformer.Informer().AddEventHandler(controller.HandleAllWith(enqueueProxyPodsConfigMaps(c)))
	utilruntime.Must(podInformer.Informer().AddIndexers(map[string]cache.IndexFunc{
		proxyPodByConfigMaps: indexProxyPodByConfigMaps,
	}))

	return c
}

func enqueueProxyPodsConfigMaps(c *controller.Controller) func(obj interface{}) {
	return func(obj interface{}) {
		keys, _ := indexProxyPodByConfigMaps(obj)
		for _, key := range keys {
			c.EnqueueKey(key)
		}
	}
}

func indexProxyPodByConfigMaps(obj interface{}) ([]string, error) {
	pod, ok := obj.(*corev1.Pod)
	if !ok || !proxypod.IsProxy(pod) {
		return nil, nil
	}
	var keys []string
	for _, vol := range pod.Spec.Volumes {
		if cm := vol.ConfigMap; cm != nil {
			keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, cm.Name))
		}
		if proj := vol.Projected; proj != nil {
			for _, src := range proj.Sources {
				if cm := src.ConfigMap; cm != nil {
					keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, cm.Name))
				}
			}
		}
	}
	for _, c := range pod.Spec.Containers {
		keys = indexContainerByConfigMap(c, keys, pod)
	}
	for _, c := range pod.Spec.InitContainers {
		keys = indexContainerByConfigMap(c, keys, pod)
	}
	return keys, nil
}

func indexContainerByConfigMap(c corev1.Container, keys []string, pod *corev1.Pod) []string {
	for _, envVar := range c.Env {
		if from := envVar.ValueFrom; from != nil {
			if cm := from.ConfigMapKeyRef; cm != nil {
				keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, cm.Name))
			}
		}
	}
	for _, src := range c.EnvFrom {
		if cm := src.ConfigMapRef; cm != nil {
			keys = append(keys, fmt.Sprintf("%s/%s", pod.Namespace, cm.Name))
		}
	}
	return keys
}

func (r configMapReconciler) Handle(obj interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	key := obj.(string)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	utilruntime.Must(err)

	configMap, err := r.configMapLister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	terminating := configMap.DeletionTimestamp != nil

	j := -1
	for i, f := range configMap.Finalizers {
		if f == common.CrossClusterGarbageCollectionFinalizer {
			j = i
			break
		}
	}
	hasFinalizer := j > -1

	objs, err := r.podIndex.ByIndex(proxyPodByConfigMaps, fmt.Sprintf("%s/%s", namespace, name))
	utilruntime.Must(err)
	targetNames := map[string]bool{}
	for _, obj := range objs {
		proxyPod := obj.(*corev1.Pod)
		if proxypod.IsScheduled(proxyPod) {
			if targetName := proxypod.GetScheduledClusterName(proxyPod); !r.selfTargetKeys[targetName] {
				targetNames[targetName] = true
			}
		}
	}

	// get remote owned configMaps
	// eponymous configMaps that aren't owned are not included (because we don't want to delete them, see below)
	// include owned configMaps in targets that no longer need them (because we shouldn't forget them when deleting)
	remoteConfigMaps := make(map[string]*corev1.ConfigMap)
	for targetName, lister := range r.remoteConfigMapLister {
		remoteConfigMap, err := lister.ConfigMaps(namespace).Get(name)
		if err != nil {
			if !errors.IsNotFound(err) {
				// error with a target shouldn't block reconciliation with other targets
				d := time.Second
				requeueAfter = &d // named returned
				utilruntime.HandleError(err)
			}
			continue
		}
		if controller.ParentControlsChild(remoteConfigMap, configMap) {
			remoteConfigMaps[targetName] = remoteConfigMap
		}
	}

	if terminating {
		for targetName := range remoteConfigMaps {
			if err := r.remoteClients[targetName].CoreV1().ConfigMaps(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
				return nil, err
			}
		}
		if hasFinalizer && len(remoteConfigMaps) == 0 {
			requeueAfter, err = r.removeFinalizer(ctx, configMap, j)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}
	} else if len(targetNames) == 0 {
		// remove extraneous finalizers added pre-0.9.3 (to all config maps and secrets)
		if hasFinalizer && len(remoteConfigMaps) == 0 {
			requeueAfter, err = r.removeFinalizer(ctx, configMap, j)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}
	} else {
		if !hasFinalizer {
			requeueAfter, err = r.addFinalizer(ctx, configMap)
			if requeueAfter != nil || err != nil {
				return requeueAfter, err
			}
		}

		for targetName := range targetNames {
			remoteConfigMap := remoteConfigMaps[targetName]
			if remoteConfigMap == nil {
				gold := makeRemoteConfigMap(configMap)
				_, err := r.remoteClients[targetName].CoreV1().ConfigMaps(namespace).Create(ctx, gold, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					// error with a target shouldn't block reconciliation with other targets
					d := time.Second
					requeueAfter = &d // named returned
					utilruntime.HandleError(err)
				}
			} else if !reflect.DeepEqual(remoteConfigMap.Data, configMap.Data) || !reflect.DeepEqual(remoteConfigMap.BinaryData, configMap.BinaryData) {
				remoteConfigMapCopy := remoteConfigMap.DeepCopy()
				remoteConfigMapCopy.Data = make(map[string]string, len(configMap.Data))
				for k, v := range configMap.Data {
					remoteConfigMapCopy.Data[k] = v
				}
				remoteConfigMapCopy.BinaryData = make(map[string][]byte, len(configMap.BinaryData))
				for k, v := range configMap.BinaryData {
					remoteConfigMapCopy.BinaryData[k] = make([]byte, len(v))
					copy(remoteConfigMapCopy.BinaryData[k], v)
				}
				_, err := r.remoteClients[targetName].CoreV1().ConfigMaps(namespace).Update(ctx, remoteConfigMapCopy, metav1.UpdateOptions{})
				if err != nil {
					// error with a target shouldn't block reconciliation with other targets
					d := time.Second
					requeueAfter = &d // named returned
					utilruntime.HandleError(err)
				}
			}
		}
	}

	// TODO? cleanup remote configMaps that aren't referred to by proxy pods

	return requeueAfter, nil
}

func (r configMapReconciler) addFinalizer(ctx context.Context, configMap *corev1.ConfigMap) (*time.Duration, error) {
	configMapCopy := configMap.DeepCopy()
	configMapCopy.Finalizers = append(configMapCopy.Finalizers, common.CrossClusterGarbageCollectionFinalizer)
	if configMapCopy.Labels == nil {
		configMapCopy.Labels = map[string]string{}
	}
	configMapCopy.Labels[common.LabelKeyHasFinalizer] = "true"
	var err error
	if _, err = r.kubeclientset.CoreV1().ConfigMaps(configMap.Namespace).Update(ctx, configMapCopy, metav1.UpdateOptions{}); err != nil {
		if patterns.IsOptimisticLockError(err) {
			d := time.Second
			return &d, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}

func (r configMapReconciler) removeFinalizer(ctx context.Context, configMap *corev1.ConfigMap, j int) (*time.Duration, error) {
	configMapCopy := configMap.DeepCopy()
	configMapCopy.Finalizers = append(configMapCopy.Finalizers[:j], configMapCopy.Finalizers[j+1:]...)
	delete(configMapCopy.Labels, common.LabelKeyHasFinalizer)
	var err error
	if _, err = r.kubeclientset.CoreV1().ConfigMaps(configMap.Namespace).Update(ctx, configMapCopy, metav1.UpdateOptions{}); err != nil {
		if patterns.IsOptimisticLockError(err) {
			d := time.Second
			return &d, nil
		} else {
			return nil, err
		}
	}
	return nil, nil
}

func makeRemoteConfigMap(configMap *corev1.ConfigMap) *corev1.ConfigMap {
	gold := &corev1.ConfigMap{}
	gold.Name = configMap.Name
	gold.Labels = make(map[string]string, len(configMap.Labels))
	for k, v := range configMap.Labels {
		gold.Labels[k] = v
	}
	gold.Annotations = make(map[string]string, len(configMap.Annotations))
	for k, v := range configMap.Annotations {
		gold.Annotations[k] = v
	}
	controller.AddRemoteControllerReference(gold, configMap)
	gold.Data = make(map[string]string, len(configMap.Data))
	for k, v := range configMap.Data {
		gold.Data[k] = v
	}
	gold.BinaryData = make(map[string][]byte, len(configMap.BinaryData))
	for k, v := range configMap.BinaryData {
		gold.BinaryData[k] = make([]byte, len(v))
		copy(gold.BinaryData[k], v)
	}
	return gold
}
