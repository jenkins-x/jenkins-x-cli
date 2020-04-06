// +build unit

package kube_test

import (
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube_mocks "k8s.io/client-go/kubernetes/fake"
)

func TestWaitForDeploymentToBeReady(t *testing.T) {
	t.Parallel()

	node := &v1.Node{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: "test-node-1",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("10"),
				v1.ResourceMemory: resource.MustParse("10G"),
			},
		},
	}

	replicaCount := int32(1)

	labels := make(map[string]string)
	labels["app"] = "ians-app"

	deployment := &appsv1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "jx-testing",
			SelfLink:  "/apis/extensions/v1beta1/namespaces/jx-testing/deployments/test-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaCount,
			Selector: &meta_v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "ians-app",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: meta_v1.ObjectMeta{
					Labels: map[string]string{
						"app": "ians-app",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "web",
							Image: "nginx:1.12",
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									Protocol:      v1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:            1,
			UpdatedReplicas:     1,
			AvailableReplicas:   1,
			UnavailableReplicas: 0,
			ReadyReplicas:       1,
			CollisionCount:      nil,
		},
	}

	kubernetesInterface := kube_mocks.NewSimpleClientset(node, deployment)

	err := kube.WaitForDeploymentToBeReady(kubernetesInterface, "test-deployment", "jx-testing", time.Second*5)

	assert.NoError(t, err, "Should not error")

}
