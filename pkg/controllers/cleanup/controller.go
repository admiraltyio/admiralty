/*
 * Copyright 2022 The Multicluster-Scheduler Authors.
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

package cleanup

import (
	"context"
	"fmt"
	"strings"
	"time"

	"admiralty.io/multicluster-scheduler/pkg/common"
	"admiralty.io/multicluster-scheduler/pkg/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	coreinformers "k8s.io/client-go/informers/core/v1"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/networking/v1"
)

type reconciler struct {
	kubeClient kubernetes.Interface

	podLister       corelisters.PodLister
	serviceLister   corelisters.ServiceLister
	ingressLister   networkinglisters.IngressLister
	configMapLister corelisters.ConfigMapLister
	secretLister    corelisters.SecretLister

	knownFinalizers map[string]bool
}

func NewController(
	kubeClient kubernetes.Interface,
	podInformer coreinformers.PodInformer,
	serviceInformer coreinformers.ServiceInformer,
	ingressInformer networkinginformers.IngressInformer,
	configMapInformer coreinformers.ConfigMapInformer,
	secretInformer coreinformers.SecretInformer,
	knownFinalizers []string) *controller.Controller {

	r := &reconciler{
		kubeClient:      kubeClient,
		podLister:       podInformer.Lister(),
		serviceLister:   serviceInformer.Lister(),
		ingressLister:   ingressInformer.Lister(),
		configMapLister: configMapInformer.Lister(),
		secretLister:    secretInformer.Lister(),
		knownFinalizers: map[string]bool{},
	}

	for _, f := range knownFinalizers {
		r.knownFinalizers[f] = true
	}

	c := controller.New(
		"cleanup",
		r,
		podInformer.Informer().HasSynced,
		serviceInformer.Informer().HasSynced,
		ingressInformer.Informer().HasSynced,
		configMapInformer.Informer().HasSynced,
		secretInformer.Informer().HasSynced,
	)

	enqueue := func(kind string) func(o interface{}) {
		return func(o interface{}) {
			m := o.(metav1.Object)
			c.EnqueueKey(key{
				kind:      kind,
				namespace: m.GetNamespace(),
				name:      m.GetName(),
			})
		}
	}

	podInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueue("Pod")))
	serviceInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueue("Service")))
	ingressInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueue("Ingress")))
	configMapInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueue("ConfigMap")))
	secretInformer.Informer().AddEventHandler(controller.HandleAddUpdateWith(enqueue("Secret")))

	return c
}

type key struct {
	kind      string
	namespace string
	name      string
}

func (r reconciler) Handle(k interface{}) (requeueAfter *time.Duration, err error) {
	ctx := context.Background()

	t := k.(key)
	var o metav1.Object
	switch t.kind {
	case "Pod":
		o, err = r.podLister.Pods(t.namespace).Get(t.name)
	case "Service":
		o, err = r.serviceLister.Services(t.namespace).Get(t.name)
	case "Ingress":
		o, err = r.ingressLister.Ingresses(t.namespace).Get(t.name)
	case "ConfigMap":
		o, err = r.configMapLister.ConfigMaps(t.namespace).Get(t.name)
	case "Secret":
		o, err = r.secretLister.Secrets(t.namespace).Get(t.name)
	default:
		err = fmt.Errorf("unknown key kind %s", t.kind)
	}
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot get %s: %v", t.kind, err)
	}

	var unknownFinalizers []string
	for _, f := range o.GetFinalizers() {
		if strings.HasPrefix(f, common.KeyPrefix) && !r.knownFinalizers[f] {
			unknownFinalizers = append(unknownFinalizers, f)
		}
	}

	if len(unknownFinalizers) > 0 {
		switch t.kind {
		case "Pod":
			o, err = r.kubeClient.CoreV1().Pods(t.namespace).Patch(ctx, t.name, types.StrategicMergePatchType, []byte(patch(unknownFinalizers)), metav1.PatchOptions{})
		case "Service":
			o, err = r.kubeClient.CoreV1().Services(t.namespace).Patch(ctx, t.name, types.StrategicMergePatchType, []byte(patch(unknownFinalizers)), metav1.PatchOptions{})
		case "Ingress":
			o, err = r.kubeClient.NetworkingV1().Ingresses(t.namespace).Patch(ctx, t.name, types.StrategicMergePatchType, []byte(patch(unknownFinalizers)), metav1.PatchOptions{})
		case "ConfigMap":
			o, err = r.kubeClient.CoreV1().ConfigMaps(t.namespace).Patch(ctx, t.name, types.StrategicMergePatchType, []byte(patch(unknownFinalizers)), metav1.PatchOptions{})
		case "Secret":
			o, err = r.kubeClient.CoreV1().Secrets(t.namespace).Patch(ctx, t.name, types.StrategicMergePatchType, []byte(patch(unknownFinalizers)), metav1.PatchOptions{})
		default:
			err = fmt.Errorf("unknown key kind %s", t.kind)
		}
		if err != nil {
			return nil, fmt.Errorf("cannot patch %s: %v", t.kind, err)
		}
	}

	return nil, nil
}

func patch(finalizers []string) string {
	return `{"metadata":{"$deleteFromPrimitiveList/finalizers":["` + strings.Join(finalizers, `","`) + `"]}}`
}
