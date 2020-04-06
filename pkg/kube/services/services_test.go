// +build unit

package services_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExtractServiceSchemePortDefault(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     80,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "http", schema)
	assert.Equal(t, "80", port)
}
func TestExtractServiceSchemePortHttps(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     443,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "https", schema)
	assert.Equal(t, "443", port)
}
func TestExtractServiceSchemePortHttpsFirst(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     80,
				},
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     443,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "https", schema)
	assert.Equal(t, "443", port)
}

func TestExtractServiceSchemePortHttpsOdd(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "brian",
					Protocol: "TCP",
					Port:     443,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "https", schema)
	assert.Equal(t, "443", port)
}

func TestExtractServiceSchemePortHttpsNamed(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "dave",
					Protocol: "UDP",
					Port:     800,
				},
				{
					Name:     "brian",
					Protocol: "TCP",
					Port:     444,
				},
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     443,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "https", schema)
	assert.Equal(t, "443", port)
}

func TestExtractServiceSchemePortHttpNamed(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "dave",
					Protocol: "UDP",
					Port:     800,
				},
				{
					Name:     "brian",
					Protocol: "TCP",
					Port:     444,
				},
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     8083,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "http", schema)
	assert.Equal(t, "8083", port)
}

func TestExtractServiceSchemePortHttpNotNamed(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "TCP",
					Port:     8088,
				},
				{
					Name:     "alan",
					Protocol: "TCP",
					Port:     80,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "http", schema)
	assert.Equal(t, "80", port)
}

func TestExtractServiceSchemePortNamedPrefHttps(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "ssh",
					Protocol: "UDP",
					Port:     22,
				},
				{
					Name:     "hiddenhttp",
					Protocol: "TCP",
					Port:     8083,
				},
				{
					Name:     "sctp-tunneling",
					Protocol: "TCP",
					Port:     9899,
				},
				{
					Name:     "https",
					Protocol: "TCP",
					Port:     8443,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "https", schema)
	assert.Equal(t, "8443", port)
}

func TestExtractServiceSchemePortInconclusive(t *testing.T) {
	t.Parallel()
	s := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-spring-boot-demo2",
			Namespace: "default-staging",
			Labels: map[string]string{
				"chart": "preview-0.0.0-SNAPSHOT-PR-29-28",
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "ssh",
					Protocol: "UDP",
					Port:     22,
				},
				{
					Name:     "hiddenhttp",
					Protocol: "TCP",
					Port:     8083,
				},
				{
					Name:     "sctp-tunneling",
					Protocol: "TCP",
					Port:     9899,
				},
			},
		},
	}
	schema, port, _ := services.ExtractServiceSchemePort(s)
	assert.Equal(t, "", schema)
	assert.Equal(t, "", port)
}
