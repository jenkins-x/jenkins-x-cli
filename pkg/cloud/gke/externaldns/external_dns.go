package externaldns

import (
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
)

const (
	// ServiceAccountSecretKey is the key for the external dns service account secret
	ServiceAccountSecretKey = "credentials.json"
	// DefaultExternalDNSAbbreviation appended to the GCP service account
	DefaultExternalDNSAbbreviation = "dn"
)

var (
	serviceAccountRoles = []string{
		"roles/dns.admin",
	}
)

// CreateExternalDNSGCPServiceAccount creates a service account in GCP for ExternalDNS
func CreateExternalDNSGCPServiceAccount(gcloud gke.GClouder, kubeClient kubernetes.Interface, externalDNSName, namespace, clusterName, projectID string) (string, error) {
	gcpServiceAccountSecretName, err := gcloud.CreateGCPServiceAccount(kubeClient, externalDNSName, DefaultExternalDNSAbbreviation, namespace, clusterName, projectID, serviceAccountRoles, ServiceAccountSecretKey)
	if err != nil {
		return "", errors.Wrap(err, "creating the ExternalDNS GCP Service Account")
	}
	return gcpServiceAccountSecretName, nil
}
