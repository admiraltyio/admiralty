package scheduler

import (
	"testing"

	"admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
	"admiralty.io/multicluster-scheduler/pkg/controllers/nodepool"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeNodePool(name string, clusterName string) *v1alpha1.NodePool {
	return &v1alpha1.NodePool{
		ObjectMeta: metav1.ObjectMeta{
			ClusterName: clusterName,
			Name:        name,
		},
	}
}

func makeNode(name string, clusterName string, nodePoolName string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			ClusterName: clusterName,
			Name:        name,
			Labels: map[string]string{
				nodepool.NodePoolLabel: nodePoolName,
			},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				"attachable-volumes-gce-pd": resource.MustParse("32"),
				"cpu":                       resource.MustParse("940m"),
				"ephemeral-storage":         resource.MustParse("47093746742"),
				"hugepages-2Mi":             resource.MustParse("0"),
				"memory":                    resource.MustParse("2702204Ki"),
				"pods":                      resource.MustParse("110"),
			},
		},
	}
}

func makePod(clusterName string, nodeName string, cpu string, memory string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			ClusterName: clusterName,
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				corev1.Container{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"cpu":    resource.MustParse(cpu),
							"memory": resource.MustParse(memory),
						},
					},
				},
			},
		},
	}
}

func TestSchedule(t *testing.T) {
	tests := []struct {
		description     string
		nodePools       []*v1alpha1.NodePool
		nodes           []*corev1.Node
		pods            []*corev1.Pod
		originalPod     *corev1.Pod
		wantClusterName string
	}{{
		description: "one cluster, one node pool, one node, no pod",
		nodePools: []*v1alpha1.NodePool{
			makeNodePool("np1", "c1"),
		},
		nodes: []*corev1.Node{
			makeNode("n1", "c1", "np1"),
		},
		pods:            []*corev1.Pod{},
		originalPod:     makePod("c1", "", "100m", "32Mi"),
		wantClusterName: "c1",
	}, {
		description: "two clusters, one node pool each, one node each, one pod in cluster 1",
		nodePools: []*v1alpha1.NodePool{
			makeNodePool("np1", "c1"),
			makeNodePool("np1", "c2"),
		},
		nodes: []*corev1.Node{
			makeNode("n1", "c1", "np1"),
			makeNode("n1", "c2", "np1"),
		},
		pods: []*corev1.Pod{
			makePod("c1", "n1", "100m", "32Mi"),
		},
		originalPod:     makePod("c2", "", "100m", "32Mi"),
		wantClusterName: "c2",
	}}
	for _, test := range tests {
		s := New()
		for _, np := range test.nodePools {
			s.SetNodePool(np)
		}
		for _, n := range test.nodes {
			s.SetNode(n)
		}
		for _, p := range test.pods {
			s.SetPod(p)
		}
		clusterName, err := s.Schedule(test.originalPod)
		if err != nil {
			t.Error(test.description, err)
		}
		if clusterName != test.wantClusterName {
			t.Error(test.description, clusterName)
		}
	}
}
