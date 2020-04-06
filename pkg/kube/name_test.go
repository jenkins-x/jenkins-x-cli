// +build unit

package kube_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetName(t *testing.T) {
	t.Parallel()
	r := &metav1.ObjectMeta{
		Name:      "default-staging-my-spring-boot-demo2-my-spring-boot-demo2-fxfgz",
		Namespace: "default-staging",
		Labels: map[string]string{
			"app": "default-staging-my-spring-boot-demo2-my-spring-boot-demo2",
		},
	}
	assert.Equal(t, "my-spring-boot-demo2", kube.GetName(r), "Get name on first pod")
}

func TestGetPodVersion(t *testing.T) {
	t.Parallel()
	r := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-staging-my-spring-boot-demo2-my-spring-boot-demo2-fxfgz",
			Namespace: "default-staging",
			Labels: map[string]string{
				"app": "default-staging-my-spring-boot-demo2-my-spring-boot-demo2",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "randomthing",
				},
				{
					Image: "foo/my-spring-boot-demo2:1.2.3",
				},
			},
		},
	}
	assert.Equal(t, "1.2.3", kube.GetPodVersion(r, ""), "Get version of the pod")
}
