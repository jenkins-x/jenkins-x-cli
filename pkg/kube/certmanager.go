package kube

import (
	certmngclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const labelLetsencryptService = "jenkins.io/letsencrypt-service"

// IsStagingCertificate looks at certmanager certificates to find if we are using staging or prod certs
func IsStagingCertificate(client certmngclient.Interface, ns string) (bool, error) {
	certs, err := client.CertmanagerV1alpha1().Certificates(ns).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	// loop over certificates and look for a Jenkins X label to identify if we are using staging or prod certs
	for _, cert := range certs.Items {
		if cert.ObjectMeta.Labels[labelLetsencryptService] == "production" {
			return false, nil
		}
		if cert.ObjectMeta.Labels[labelLetsencryptService] == "staging" {
			return true, nil
		}
	}
	return false, errors.New("no matching certificates found with letsencrypt-service label")
}
