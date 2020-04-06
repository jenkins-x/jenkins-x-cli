// +build unit

package kube

import (
	"testing"

	certmng "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsStagingCertificate(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()

	const name = "test"
	const ns = "test"
	cert := newCert(name, "staging")

	_, err := client.Certmanager().Certificates(ns).Create(cert)
	assert.NoError(t, err, "should create a test certificate whithout an error")

	isStaging, err := IsStagingCertificate(client, ns)
	assert.NoError(t, err, "should find a matching certificate")
	assert.Equal(t, true, isStaging, "should gave found a staging certificate")
}

func TestIsNotStagingCertificate(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()

	const name = "test"
	const ns = "test"
	cert := newCert(name, "production")
	_, err := client.Certmanager().Certificates(ns).Create(cert)
	assert.NoError(t, err, "should create a test certificate whithout an error")

	isStaging, err := IsStagingCertificate(client, ns)
	assert.NoError(t, err, "should find a matching certificate")
	assert.Equal(t, false, isStaging, "should gave found a production certificate")

}

func TestNoCertificate(t *testing.T) {
	t.Parallel()

	client := fake.NewSimpleClientset()
	const ns = "test"

	_, err := IsStagingCertificate(client, ns)
	assert.Error(t, err, "should find a matching certificate")
}

func newCert(name, service string) *certmng.Certificate {
	labels := map[string]string{}
	labels[labelLetsencryptService] = service
	return &certmng.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
