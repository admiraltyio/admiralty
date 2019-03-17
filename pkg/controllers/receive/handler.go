/*
Copyright 2019 The Multicluster-Scheduler Authors.

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

package receive

import (
	"admiralty.io/multicluster-controller/pkg/handler"
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"admiralty.io/multicluster-controller/pkg/reference"
	"k8s.io/apimachinery/pkg/api/meta"
)

var key string = "multicluster.admiralty.io/controller-reference"

type EnqueueRequestForMulticlusterController struct {
	Queue handler.Queue
}

func (e *EnqueueRequestForMulticlusterController) enqueue(obj interface{}) {
	o, err := meta.Accessor(obj)
	if err != nil {
		return
	}

	if c := reference.GetMulticlusterControllerOf(o); c != nil {
		r := reconcile.Request{Context: c.ClusterName}
		r.Namespace = c.Namespace
		r.Name = c.Name

		e.Queue.Add(r)
		return
	}
}

func (e *EnqueueRequestForMulticlusterController) OnAdd(obj interface{}) {
	e.enqueue(obj)
}

func (e *EnqueueRequestForMulticlusterController) OnUpdate(oldObj, newObj interface{}) {
	e.enqueue(newObj)
}

func (e *EnqueueRequestForMulticlusterController) OnDelete(obj interface{}) {
	e.enqueue(obj)
}
