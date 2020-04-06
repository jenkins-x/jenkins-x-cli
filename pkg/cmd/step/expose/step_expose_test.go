// +build unit

package expose_test

import (
	"fmt"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/expose"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/config"
	"k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestStepExpose(t *testing.T) {
	t.Parallel()

	type testData struct {
		Dir, Domain, NamespaceSubDomain string
		TLSEnabled                      bool
	}

	ns := "jx-staging"
	subDomain := "-" + ns + "."

	tests := []testData{
		{
			Dir:                "test_data/http",
			Domain:             "35.195.192.178.nip.io",
			NamespaceSubDomain: subDomain,
			TLSEnabled:         false,
		},
		{
			Dir:                "test_data/https",
			Domain:             "my-domain.com",
			NamespaceSubDomain: subDomain,
			TLSEnabled:         true,
		},
	}

	for _, td := range tests {
		commonOpts := opts.NewCommonOptionsWithFactory(nil)
		o := &expose.StepExposeOptions{
			Dir: td.Dir,
		}
		o.CommonOptions = &commonOpts
		o.Namespace = ns

		testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
			[]runtime.Object{},
			[]runtime.Object{},
			gits.NewGitCLI(),
			nil,
			helm.NewHelmCLI("helm", helm.V2, "", true),
			resources_test.NewMockInstaller(),
		)

		kubeClient, err := o.KubeClient()
		require.NoError(t, err)

		expectedRequirements := &config.RequirementsConfig{
			Ingress: config.IngressConfig{
				Domain:             td.Domain,
				NamespaceSubDomain: td.NamespaceSubDomain,
				TLS: config.TLSConfig{
					Enabled: td.TLSEnabled,
				},
			},
		}
		ings := []*v1beta1.Ingress{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "update-my-ingress",
				},
			},
		}
		svcs := []*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc1",
					Labels: map[string]string{
						"fabric8.io/expose": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: "TCP",
							Port:     80,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "update-my-ingress",
					Labels: map[string]string{
						"fabric8.io/expose": "true",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: "TCP",
							Port:     8080,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dont-expose",
					Labels: map[string]string{
						"fabric8.io/expose": "false",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: "TCP",
							Port:     80,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "no-expose-label",
					Labels: map[string]string{
						"fabric8.io/expose": "false",
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Protocol: "TCP",
							Port:     80,
						},
					},
				},
			},
		}
		for _, ing := range ings {
			_, err = kubeClient.ExtensionsV1beta1().Ingresses(ns).Create(ing)
			require.NoError(t, err, "failed to create Ingress %s in namespace %s", ing.Name, ns)
		}

		for _, svc := range svcs {
			_, err = kubeClient.CoreV1().Services(ns).Create(svc)
			require.NoError(t, err, "failed to create Service %s in namespace %s", svc.Name, ns)
		}

		err = o.Run()
		assert.NoError(t, err)

		for _, svc := range svcs {
			if expose.IsExposedService(svc) {
				assertIngressForService(t, kubeClient, ns, svc, expectedRequirements)
			} else {
				assertNoIngressExistsForService(t, kubeClient, ns, svc)
			}
		}
	}
}

func assertIngressForService(t *testing.T, kubeClient kubernetes.Interface, ns string, svc *corev1.Service, expectedRequirements *config.RequirementsConfig) {
	svcName := svc.Name
	ingress, err := kubeClient.ExtensionsV1beta1().Ingresses(ns).Get(svcName, metav1.GetOptions{})
	if assert.NoError(t, err, "finding Ingress %s in namespace %s", svcName, ns) {
		t.Logf("found the Ingress for service %s in namespace %s which should be exposed", svcName, ns)
	}

	rules := ingress.Spec.Rules
	require.Equal(t, 1, len(rules), "Ingress rule count for Ingress %s", svcName)
	rule := rules[0]
	expectedHost := fmt.Sprintf("%s%s%s", svcName, expectedRequirements.Ingress.NamespaceSubDomain, expectedRequirements.Ingress.Domain)
	if assert.Equal(t, expectedHost, rule.Host, "host for Ingress %s in namespace %s", svcName, ns) {
		t.Logf("Ingress %s is at the correct host %s", svcName, expectedHost)
	}
	assert.True(t, rule.HTTP != nil && len(rule.HTTP.Paths) == 1, "has a HTTP path for Ingress %s in namespace %s", svcName, ns)

	path := rule.HTTP.Paths[0]
	if assert.Equal(t, svcName, path.Backend.ServiceName, "has a HTTP path backend service name for Ingress %s in namespace %s", svcName, ns) {
		t.Logf("Ingress %s is using the correct backend service", svcName)
	}

	servicePort := svc.Spec.Ports[0].Port
	if assert.Equal(t, servicePort, path.Backend.ServicePort.IntVal, "the exposed service port on the Ingress %s in namespace %s", svcName, ns) {
		t.Logf("Ingress %s is at the correct service port %d", svcName, int(servicePort))
	}

	scheme := "http://"
	if expectedRequirements.Ingress.TLS.Enabled {
		scheme = "https://"
	}
	expectedURL := scheme + expectedHost
	actualSvc, err := kubeClient.CoreV1().Services(ns).Get(svcName, metav1.GetOptions{})
	require.NoError(t, err, "could not load Service %s", svcName)

	actualURL := actualSvc.Annotations[expose.ExposeAnnotationKey]
	if assert.Equal(t, expectedURL, actualURL, "the exposed service URL was not annotated correctly on the Service %s in namespace %s", svcName, ns) {
		t.Logf("added the exposed URL %s to the Service %s using the annotation %s", actualURL, svcName, expose.ExposeAnnotationKey)
	}

	if expectedRequirements.Ingress.TLS.Enabled {
		tlss := ingress.Spec.TLS
		require.Equal(t, 1, len(tlss), "Ingress TLS count for Ingress %s", svcName)
		tls := tlss[0]
		t.Logf("got TLS secret name %s for Ingress %s", tls.SecretName, ingress.Name)

		require.Equal(t, 1, len(tls.Hosts), "Ingress TLS host should be defined for Ingress %s", svcName)
		host := tls.Hosts[0]
		require.Equal(t, expectedHost, host, "Ingress TLS host for Ingress %s", svcName)
	}
}

func assertNoIngressExistsForService(t *testing.T, kubeClient kubernetes.Interface, ns string, svc *corev1.Service) {
	svcName := svc.Name
	_, err := kubeClient.ExtensionsV1beta1().Ingresses(ns).Get(svcName, metav1.GetOptions{})
	notExposed := err != nil && apierrors.IsNotFound(err)
	if assert.True(t, notExposed, "we should not have around Ingress %s in namespace %s as the service is not meant to be exposed. error: %v", svcName, ns, err) {
		t.Logf("did not create an Ingress for service %s in namespace %s which is not meant to be exposed", svcName, ns)
	}
}
