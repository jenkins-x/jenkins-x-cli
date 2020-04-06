package pki

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	kubeservices "github.com/jenkins-x/jx/pkg/kube/services"
	certmng "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	certclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// CertSecretPrefix used as prefix for all certificate object names
const CertSecretPrefix = "tls-"

// Certificate keeps some information related to a certificate issued by cert-manager
type Certificate struct {
	// Name certificate name
	Name string
	//Namespace certificate namespace
	Namespace string
}

// WaitCertificateIssuedReady wait for a certificate issued by cert-manager until is ready or the timeout is reached
func WaitCertificateIssuedReady(client certclient.Interface, name string, ns string, timeout time.Duration) error {
	err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		cert, err := client.CertmanagerV1alpha1().Certificates(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		isReady := cert.HasCondition(certmng.CertificateCondition{
			Type:   certmng.CertificateConditionReady,
			Status: certmng.ConditionTrue,
		})
		if !isReady {
			return false, nil
		}
		log.Logger().Infof("Ready Cert: %s", util.ColorInfo(name))
		return true, nil
	})
	if err != nil {
		return errors.Wrapf(err, "waiting for certificate %q to be ready in namespace %q.", name, ns)
	}
	return nil
}

// WaitCertificateExists waits until the timeout for the certificate with the provided name to be available in the certificates list
func WaitCertificateExists(client certclient.Interface, name string, ns string, timeout time.Duration) error {
	err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		_, err := client.CertmanagerV1alpha1().Certificates(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return errors.Wrapf(err, "waiting for certificate %q to be created in namespace %q.", name, ns)
	}
	return nil
}

// CleanAllCerts removes all certs and their associated secrets which hold a TLS certificated issued by cert-manager
func CleanAllCerts(client kubernetes.Interface, certclient certclient.Interface, ns string) error {
	return cleanCerts(client, certclient, ns, func(cert string) bool {
		return strings.HasPrefix(cert, CertSecretPrefix)
	})
}

// CleanCerts removes the certs and their associated secrets which hold a TLS certificate issued by cert-manager
func CleanCerts(client kubernetes.Interface, certclient certclient.Interface, ns string, filter []Certificate) error {
	allowed := make(map[string]bool)
	for _, cert := range filter {
		allowed[cert.Name] = true
	}
	return cleanCerts(client, certclient, ns, func(cert string) bool {
		_, ok := allowed[cert]
		return ok
	})
}

func cleanCerts(client kubernetes.Interface, certclient certclient.Interface, ns string, allow func(string) bool) error {
	certsClient := certclient.Certmanager().Certificates(ns)
	certsList, err := certsClient.List(metav1.ListOptions{})
	if err != nil {
		// there are no certificates to clean
		return nil
	}
	for _, c := range certsList.Items {
		if allow(c.GetName()) {
			err := certsClient.Delete(c.GetName(), &metav1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "deleting the cert %s/%s", ns, c.GetName())
			}
		}
	}
	// delete the tls related secrets so we dont reuse old ones when switching from http to https
	secrets, err := client.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "listing the secrets in namespace %q", ns)
	}
	for _, s := range secrets.Items {
		if allow(s.GetName()) {
			err := client.CoreV1().Secrets(ns).Delete(s.Name, &metav1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "deleting the tls secret %s/%s", ns, s.GetName())
			}
		}
	}
	return nil
}

// String returns the certificate information in a string format
func (c Certificate) String() string {
	return fmt.Sprintf("%s/%s", c.Namespace, c.Name)
}

// WatchCertificatesIssuedReady starts watching for ready certificate in the given namespace.
// If the namespace is empty, it will watch the entire cluster. The caller can stop watching by cancelling the context.
func WatchCertificatesIssuedReady(ctx context.Context, client certclient.Interface, ns string) (<-chan Certificate, error) {
	watcher, err := client.Certmanager().Certificates(ns).Watch(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "watching certificates in namespace %q", ns)
	}
	results := make(chan Certificate)
	go func() {
		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case e := <-watcher.ResultChan():
				if e.Type == watch.Added || e.Type == watch.Modified {
					cert, ok := e.Object.(*certmng.Certificate)
					if ok {
						if isCertReady(cert) {
							result := Certificate{
								Name:      cert.GetName(),
								Namespace: cert.GetNamespace(),
							}
							results <- result
						}
					}
				}
			}
		}
	}()

	return results, nil
}

func isCertReady(cert *certmng.Certificate) bool {
	return cert.HasCondition(certmng.CertificateCondition{
		Type:   certmng.CertificateConditionReady,
		Status: certmng.ConditionTrue,
	})
}

// GetIssuedReadyCertificates returns the current ready certificates in the given namespace
func GetIssuedReadyCertificates(client certclient.Interface, ns string) ([]Certificate, error) {
	certs := make([]Certificate, 0)
	certsList, err := client.Certmanager().Certificates(ns).List(metav1.ListOptions{})
	if err != nil {
		return certs, errors.Wrapf(err, "listing certificates in namespace %q", ns)
	}
	for _, cert := range certsList.Items {
		if isCertReady(&cert) {
			certs = append(certs, Certificate{
				Name:      cert.GetName(),
				Namespace: cert.GetNamespace(),
			})
		}
	}
	return certs, nil
}

// ToCertificates converts a list of services into a list of certificates. The certificate name is built from
// the application label of the service.
func ToCertificates(services []*v1.Service) []Certificate {
	result := make([]Certificate, 0)
	for _, svc := range services {
		app := kubeservices.ServiceAppName(svc)
		cert := CertSecretPrefix + app
		ns := svc.GetNamespace()
		result = append(result, Certificate{
			Name:      cert,
			Namespace: ns,
		})
	}
	return result
}
