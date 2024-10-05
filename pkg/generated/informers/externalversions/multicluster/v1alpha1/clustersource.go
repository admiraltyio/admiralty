/*
 * Copyright The Multicluster-Scheduler Authors.
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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	multiclusterv1alpha1 "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	versioned "admiralty.io/multicluster-scheduler/pkg/generated/clientset/versioned"
	internalinterfaces "admiralty.io/multicluster-scheduler/pkg/generated/informers/externalversions/internalinterfaces"
	v1alpha1 "admiralty.io/multicluster-scheduler/pkg/generated/listers/multicluster/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterSourceInformer provides access to a shared informer and lister for
// ClusterSources.
type ClusterSourceInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.ClusterSourceLister
}

type clusterSourceInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterSourceInformer constructs a new informer for ClusterSource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterSourceInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterSourceInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterSourceInformer constructs a new informer for ClusterSource type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterSourceInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MulticlusterV1alpha1().ClusterSources().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.MulticlusterV1alpha1().ClusterSources().Watch(context.TODO(), options)
			},
		},
		&multiclusterv1alpha1.ClusterSource{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterSourceInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterSourceInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterSourceInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&multiclusterv1alpha1.ClusterSource{}, f.defaultInformer)
}

func (f *clusterSourceInformer) Lister() v1alpha1.ClusterSourceLister {
	return v1alpha1.NewClusterSourceLister(f.Informer().GetIndexer())
}
