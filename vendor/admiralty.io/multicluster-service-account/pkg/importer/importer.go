/*
Copyright 2018 The Multicluster-Service-Account Authors.

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

package importer // import "admiralty.io/multicluster-service-account/pkg/importer"

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"admiralty.io/multicluster-service-account/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-service-account/pkg/config"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var ctrlName = "service-account-import-controller"
var saiName = "multicluster.admiralty.io/service-account-import.name"
var remoteSecretUID = "multicluster.admiralty.io/remote-secret.uid"

// Add creates a service account import controller and adds it to a manager (mgr).
// It is implemented by the service-account-import-controller command, via controller-runtime.
// The src argument is a map of client-go rest.Configs and namespaces by service account import name
// or kubeconfig context. The controller watches service account imports in the manager's
// cluster, and ensures secrets exist for each of them, created from the corresponding remote service
// account secrets data and cluster URL, fetched with the source client whose name matches the service
// account import clusterName spec field.
// TODO... import into multiple clusters with multicluster-controller
func Add(mgr manager.Manager, src map[string]config.ConfigAndNamespaceTuple) error {
	srcClients := make(map[string]client.Client)
	srcURLs := make(map[string]string)
	for clusterName, cfgAndNs := range src { // using service account import names or kubeconfig context names as cluster names
		cfg := cfgAndNs.Config
		m, err := apiutil.NewDiscoveryRESTMapper(cfg)
		if err != nil {
			return err
		}
		c, err := client.New(cfg, client.Options{Scheme: scheme.Scheme, Mapper: m})
		if err != nil {
			return err
		}
		srcClients[clusterName] = c
		srcURLs[clusterName] = cfg.Host
	}
	c, err := controller.New(ctrlName, mgr, controller.Options{Reconciler: &reconciler{
		destClient: mgr.GetClient(),
		destScheme: mgr.GetScheme(),
		srcClients: srcClients,
		srcURLs:    srcURLs,
	}})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &v1alpha1.ServiceAccountImport{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &v1alpha1.ServiceAccountImport{},
		})
	if err != nil {
		return err
	}

	// TODO? reconcile from label or annotation to allow creating more secrets, and let them be autopopulated,
	// just like for regular service accounts and their token secrets
	// https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#manually-create-a-service-account-api-token

	// TODO... watch actual ServiceAccount and Secrets in remotes to update Secret if needed (need multicluster-controller)

	return nil
}

type reconciler struct {
	destClient client.Client
	destScheme *runtime.Scheme
	srcClients map[string]client.Client
	srcURLs    map[string]string
}

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {

	// Gather observations (ServiceAccountImport and token Secrets)

	sai := &v1alpha1.ServiceAccountImport{}
	log.Printf("get ServiceAccountImport")
	if err := r.destClient.Get(context.TODO(), req.NamespacedName, sai); err != nil {
		if errors.IsNotFound(err) {
			// Secrets will be garbage collected by GC.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("cannot get ServiceAccountImport: %v", err)
	}

	osl := &corev1.SecretList{}
	log.Printf("list Secrets")
	if err := r.destClient.List(context.TODO(), client.InNamespace(sai.Namespace).MatchingLabels(map[string]string{
		saiName: string(sai.Name)}), osl); err != nil {
		return reconcile.Result{}, fmt.Errorf("cannot list token Secrets: %v", err)
	}

	// Update Secrets slice in ServiceAccountImport Status

	ss := make([]corev1.ObjectReference, 0, len(osl.Items)) // reset to not append duplicates
	for _, os := range osl.Items {
		ss = append(ss, corev1.ObjectReference{Name: os.Name})
	}
	before := make(map[string]struct{}, len(sai.Status.Secrets))
	for _, s := range sai.Status.Secrets {
		before[s.Name] = struct{}{}
	}
	after := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		after[s.Name] = struct{}{}
	}
	if !reflect.DeepEqual(before, after) {
		log.Printf("update Secrets slice in ServiceAccountImport Status")
		sai.Status.Secrets = ss
		if err := r.destClient.Update(context.TODO(), sai); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot update Secrets slice in ServiceAccountImport Status: %v", err)
		}
	}

	// Make desired Secrets from ServiceAccountImport (and corresponding remote ServiceAccount and Secret)

	dss, err := r.makeSecrets(sai)
	if err != nil {
		// TODO... status update and event
		return reconcile.Result{}, fmt.Errorf("cannot make desired Secrets: %v", err)
	}
	// TODO: figure out whether no secret is an error or just a transient possibility
	// if len(dss) == 0 {
	// 	return reconcile.Result{}, fmt.Errorf("remote ServiceAccount has no token")
	// }

	// Compare observation to desired state: create, update or delete Secrets

	// map the observed secrets by remote secret UID for comparison
	osm := make(map[string]*corev1.Secret)
	for _, os := range osl.Items {
		osm[os.Labels[remoteSecretUID]] = &os
	}
	for _, ds := range dss {
		key := ds.Labels[remoteSecretUID]
		os, ok := osm[key]

		if !ok {
			log.Printf("create Secret")
			if err := r.destClient.Create(context.TODO(), ds); err != nil {
				return reconcile.Result{}, fmt.Errorf("cannot create Secret: %v", err)
			}
		} else {
			if !reflect.DeepEqual(ds.Data, os.Data) {
				os.Data = ds.Data
				log.Printf("update Secret")
				if err := r.destClient.Update(context.TODO(), os); err != nil {
					return reconcile.Result{}, fmt.Errorf("cannot update Secret: %v", err)
				}
			}
		}

		delete(osm, key)
	}
	for _, os := range osm {
		log.Printf("delete Secret")
		if err := r.destClient.Delete(context.TODO(), os); err != nil {
			return reconcile.Result{}, fmt.Errorf("cannot delete Secret: %v", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *reconciler) makeSecrets(sai *v1alpha1.ServiceAccountImport) ([]*corev1.Secret, error) {
	clusterName := sai.Spec.ClusterName
	srcClient, ok := r.srcClients[clusterName]
	if !ok {
		return nil, fmt.Errorf("cluster name %s is unknown", sai.ClusterName)
	}

	sa := &corev1.ServiceAccount{}
	log.Printf("get remote ServiceAccount")
	if err := srcClient.Get(context.TODO(), types.NamespacedName{
		Namespace: sai.Spec.Namespace,
		Name:      sai.Spec.Name,
	}, sa); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("remote ServiceAccount doesn't exist")
		}
		return nil, fmt.Errorf("cannot get remote ServiceAccount: %v", err)
	}

	var dss []*corev1.Secret
	for _, rs := range sa.Secrets {
		s := &corev1.Secret{}
		log.Printf("get remote Secret")
		if err := srcClient.Get(context.TODO(), types.NamespacedName{
			Namespace: sa.Namespace, // Namespace is not set on OwnerReference (always the same as controlling ServiceAccount)
			Name:      rs.Name,
		}, s); err != nil {
			if errors.IsNotFound(err) {
				// remote ServiceAccount's token is missing
				// TODO: check whether this can happen (must be temporary if it ever does, unless token controller is deactivated)
				continue
			}
			return nil, fmt.Errorf("cannot get remote ServiceAccount token Secret: %v", err)
		}

		ds := &corev1.Secret{}
		ds.Namespace = sai.Namespace
		ds.GenerateName = sai.Name + "-token-"
		if err := controllerutil.SetControllerReference(sai, ds, r.destScheme); err != nil {
			return nil, fmt.Errorf("cannot set controller reference on destClient token Secret: %v", err)
		}
		ds.Data = s.Data

		// Note: we can't use StringData because ds is to be compared with the observation (StringData is for writes only)
		url := r.srcURLs[clusterName]
		if ds.Data == nil { // very unlikely (the remote secret should have data), but just in case...
			ds.Data = map[string][]byte{"server": []byte(url)}
		} else {
			ds.Data["server"] = []byte(url)
		}

		ds.Labels = map[string]string{
			saiName:         string(sai.Name),
			remoteSecretUID: string(s.UID),
		}

		dss = append(dss, ds)
	}

	return dss, nil
}
