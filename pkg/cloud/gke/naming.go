package gke

import "fmt"

// BucketName creates a Bucket name for a given service name
func BucketName(serviceName string) string {
	return generateName(serviceName, "bucket")
}

// ServiceAccountName creates a service account name for a given service and cluster name
func ServiceAccountName(clusterName, serviceAbbreviation string) string {
	return generateName(clusterName, serviceAbbreviation)
}

// KeyringName creates a keyring name for a given service and cluster name
func KeyringName(serviceName string) string {
	return generateName(serviceName, "keyring")
}

// KeyName creates a key name for a given service and cluster name
func KeyName(serviceName string) string {
	return generateName(serviceName, "key")
}

// GcpServiceAccountSecretName builds the secret name where the GCP service account is stored
func GcpServiceAccountSecretName(serviceName string) string {
	return generateName(serviceName, "gcp-sa")
}

func generateName(serviceName string, name string) string {
	return fmt.Sprintf("%s-%s", serviceName, name)
}
