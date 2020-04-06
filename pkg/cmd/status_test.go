// +build unit

package cmd_test

import (
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd"
	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"

	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube_mocks "k8s.io/client-go/kubernetes/fake"
)

func TestStatusRun(t *testing.T) {
	// Create a fake node
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

	// Create a fake Jenkins deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      "jenkins",
			Namespace: "jx-testing",
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

	annotations := make(map[string]string)
	annotations["fabric8.io/exposeUrl"] = "http://jenkins.testorama.com"

	// Create a fake Jenkins service
	service := &v1.Service{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:        "jenkins",
			Namespace:   "jx-testing",
			Annotations: annotations,
		},
	}

	// mock factory
	factory := cmd_mocks.NewMockFactory()
	// mock Kubernetes interface
	kubernetesInterface := kube_mocks.NewSimpleClientset(node, deployment, service)
	// Override CreateKubeClient to return mock Kubernetes interface
	When(factory.CreateKubeClient()).ThenReturn(kubernetesInterface, "jx-testing", nil)

	// Setup options
	commonOpts := opts.NewCommonOptionsWithFactory(factory)
	commonOpts.Out = os.Stdout
	commonOpts.Err = os.Stderr
	options := &cmd.StatusOptions{
		CommonOptions: &commonOpts,
	}

	err := options.Run()

	assert.NoError(t, err, "Should not error")

}
