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

package reconcile // import "admiralty.io/multicluster-controller/pkg/reconcile"

import (
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Request contains the information needed by a multicluster Reconciler to Reconcile:
// a context, namespace, and name.
type Request struct {
	Context string
	types.NamespacedName
}

// Result is the return type of a Reconciler's Reconcile method.
// By default, the Request is forgotten after it's been processed,
// but you can also requeue it immediately, or after some time.
type Result reconcile.Result

// Reconciler is the interface used by a Controller to reconcile.
type Reconciler interface {
	Reconcile(Request) (Result, error)
}
