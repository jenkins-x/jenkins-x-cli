// +build unit

package pki_test

import (
	"context"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/kube/pki"
	certmng "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	certclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	"github.com/jetstack/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWaitCertificateIssuedReady(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()

	const name = "test"
	const ns = "test"
	cert := newCert(name, certmng.CertificateCondition{
		Type:   certmng.CertificateConditionReady,
		Status: certmng.ConditionTrue,
	})
	_, err := client.Certmanager().Certificates(ns).Create(cert)
	assert.NoError(t, err, "should create a test certificate whithout an error")

	err = pki.WaitCertificateIssuedReady(client, name, ns, 3*time.Second)
	assert.NoError(t, err, "should find a cert in ready state")

	err = client.Certmanager().Certificates(ns).Delete(name, &metav1.DeleteOptions{})
	assert.NoError(t, err, "should delete the test certificate whithout an error")

	cert = newCert(name, certmng.CertificateCondition{
		Type:   certmng.CertificateConditionValidationFailed,
		Status: certmng.ConditionTrue,
	})
	_, err = client.Certmanager().Certificates(ns).Create(cert)
	assert.NoError(t, err, "should create a test certificate whithout an error")

	err = pki.WaitCertificateIssuedReady(client, name, ns, 5*time.Second)
	assert.Error(t, err, "should not find a cert in ready state")
}

func newCert(name string, condition certmng.CertificateCondition) *certmng.Certificate {
	return &certmng.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: certmng.CertificateStatus{
			Conditions: []certmng.CertificateCondition{condition},
		},
	}
}

func TestWatchCertificatesIssuedReady(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		certs map[string][]string
	}{

		"watch one namespace": {
			map[string][]string{
				"test": {"test1, test2"},
			},
		},
		"watch multiple namespaces": {
			map[string][]string{
				"test1": {"tests1, test2"},
				"test2": {"tests1, test2"},
				"test3": {"tests1, test2"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			ns := ""
			if len(tc.certs) == 1 {
				for key := range tc.certs {
					ns = key
				}
			}
			certsCh, err := pki.WatchCertificatesIssuedReady(ctx, client, ns)
			assert.NoError(t, err, "should start watching certificates")

			for ns, certs := range tc.certs {
				for _, name := range certs {
					createAndUpdateCert(t, client, name, ns, certmng.CertificateCondition{
						Type:   certmng.CertificateConditionReady,
						Status: certmng.ConditionTrue,
					})
				}
			}

			results := make(map[string][]string)
		L:
			for {
				select {
				case cert := <-certsCh:
					certs, ok := results[cert.Namespace]
					if !ok {
						results[cert.Namespace] = make([]string, 0)
					}
					results[cert.Namespace] = append(certs, cert.Name)
				case <-ctx.Done():
					break L
				}
			}

			assert.Equal(t, len(tc.certs), len(results))
			for ns, certs := range results {
				expectedCerts, ok := tc.certs[ns]
				assert.True(t, ok)
				assert.Equal(t, expectedCerts, certs)
			}
		})
	}
}

func createAndUpdateCert(t *testing.T, client certclient.Interface, name, ns string, condition certmng.CertificateCondition) {
	cert := newCert(name, certmng.CertificateCondition{})
	cert, err := client.Certmanager().Certificates(ns).Create(cert)
	assert.NoError(t, err, "should create a test certificate without an error")
	cert.Status = certmng.CertificateStatus{
		Conditions: []certmng.CertificateCondition{
			condition,
		},
	}
	_, err = client.Certmanager().Certificates(ns).Update(cert)
	assert.NoError(t, err, "should update the test crtificate without an error")
}
