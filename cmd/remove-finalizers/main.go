package main

import (
	"admiralty.io/multicluster-controller/pkg/cluster"
	"admiralty.io/multicluster-scheduler/pkg/apis"
	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-service-account/pkg/config"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
	ctx := context.Background()

	patch := `{"metadata":{"$deleteFromPrimitiveList/finalizers":["multicluster.admiralty.io/multiclusterForegroundDeletion"]}}`

	cfg, _, err := config.ConfigAndNamespace()
	utilruntime.Must(err)

	k, err := kubernetes.NewForConfig(cfg)

	clus := cluster.New("local", cfg, cluster.Options{})
	c, err := clus.GetDelegatingClient()
	utilruntime.Must(err)

	err = apis.AddToScheme(clus.GetScheme())
	utilruntime.Must(err)

	p := patchAll{k, c, patch}

	p.patchNamespacedResources()
	p.patchClusterScopedResources()

	// the node pool CRD won't be deleted when multicluster-scheduler Helm release is deleted
	// if the user deletes the CRD later, the node pools will be deleted
	// finalizers would be blocking then
	p.patchNodePools(ctx)
}

type patchAll struct {
	k     *kubernetes.Clientset
	c     client.Client
	patch string
}

func (p patchAll) patchNamespacedResources() {
	p.patchPods()
	p.patchServices()
	p.patchPersistentVolumeClaims()
	p.patchReplicationControllers()
	p.patchReplicaSets()
	p.patchStatefulSets()
	p.patchPodDisruptionBudgets()
}

func (p patchAll) patchPods() {
	l, err := p.k.CoreV1().Pods("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().Pods(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchServices() {
	l, err := p.k.CoreV1().Services("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().Services(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchPersistentVolumeClaims() {
	l, err := p.k.CoreV1().PersistentVolumeClaims("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().PersistentVolumeClaims(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchReplicationControllers() {
	l, err := p.k.CoreV1().ReplicationControllers("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().ReplicationControllers(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchReplicaSets() {
	l, err := p.k.AppsV1().ReplicaSets("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.AppsV1().ReplicaSets(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchStatefulSets() {
	l, err := p.k.AppsV1().StatefulSets("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.AppsV1().StatefulSets(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchPodDisruptionBudgets() {
	l, err := p.k.PolicyV1beta1().PodDisruptionBudgets("").List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.PolicyV1beta1().PodDisruptionBudgets(o.Namespace).Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchClusterScopedResources() {
	p.patchNodes()
	p.patchPersistentVolumes()
}

func (p patchAll) patchNodes() {
	l, err := p.k.CoreV1().Nodes().List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().Nodes().Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchPersistentVolumes() {
	l, err := p.k.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.CoreV1().PersistentVolumes().Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchStorageClasses() {
	l, err := p.k.StorageV1().StorageClasses().List(metav1.ListOptions{})
	utilruntime.Must(err)
	for _, o := range l.Items {
		for _, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				_, err := p.k.StorageV1().StorageClasses().Patch(o.Name, types.StrategicMergePatchType, []byte(p.patch))
				utilruntime.Must(err)
				break
			}
		}
	}
}

func (p patchAll) patchNodePools(ctx context.Context) {
	l := &v1alpha1.NodePoolList{}
	err := p.c.List(ctx, &client.ListOptions{}, l)
	utilruntime.Must(err)
	for _, o := range l.Items {
		for i, f := range o.Finalizers {
			if f == "multicluster.admiralty.io/multiclusterForegroundDeletion" {
				o.Finalizers = append(o.Finalizers[:i], o.Finalizers[i+1:]...)
				err := p.c.Update(ctx, &o) // TODO use patch when we upgrade controller-runtime
				utilruntime.Must(err)
				break
			}
		}
	}
}
