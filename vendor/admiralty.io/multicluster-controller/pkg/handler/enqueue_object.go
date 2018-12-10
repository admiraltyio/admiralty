/*
Copyright 2018 The Multicluster-Controller Authors.

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

package handler // import "admiralty.io/multicluster-controller/pkg/handler"

import (
	"admiralty.io/multicluster-controller/pkg/reconcile"
	"k8s.io/apimachinery/pkg/api/meta"
)

type EnqueueRequestForObject struct {
	Context string
	Queue   Queue
}

func (e *EnqueueRequestForObject) enqueue(obj interface{}) {
	o, err := meta.Accessor(obj)
	if err != nil {
		return
	}

	r := reconcile.Request{Context: e.Context}
	r.Namespace = o.GetNamespace()
	r.Name = o.GetName()

	e.Queue.Add(r)
}

func (e *EnqueueRequestForObject) OnAdd(obj interface{}) {
	e.enqueue(obj)
}

func (e *EnqueueRequestForObject) OnUpdate(oldObj, newObj interface{}) {
	e.enqueue(newObj)
}

func (e *EnqueueRequestForObject) OnDelete(obj interface{}) {
	e.enqueue(obj)
}
