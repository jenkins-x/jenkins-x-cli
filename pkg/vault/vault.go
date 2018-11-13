package vault

import (
	"fmt"
	"github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultNumVaults      = 2
	vaultImage            = "vault:0.11.2"
	bankVaultsImage       = "banzaicloud/bank-vaults:latest"
	gcpServiceAccountEnv  = "GOOGLE_APPLICATION_CREDENTIALS"
	gcpServiceAccountPath = "/etc/gcp/service-account.json"

	vaultAuthName     = "auth"
	vaultAuthType     = "kubernetes"
	vaultAuthTTL      = "1h"
	vaultAuthSaSuffix = "auth-sa"
)

// Vault stores some details of a Vault resource
type Vault struct {
	Name                   string
	Namespace              string
	URL                    string
	AuthServiceAccountName string
}

// GCPConfig keeps the configuration for Google Cloud
type GCPConfig struct {
	ProjectId   string
	KmsKeyring  string
	KmsKey      string
	KmsLocation string
	GcsBucket   string
}

type GCSConfig struct {
	Bucket    string `json:"bucket"`
	HaEnabled string `json:"ha_enabled"`
}

type VaultAuths []VaultAuth

type VaultAuth struct {
	Roles []VaultRole `json:"roles"`
	Type  string      `json:"type"`
}

type VaultRole struct {
	BoundServiceAccountNames      string `json:"bound_service_account_names"`
	BoundServiceAccountNamespaces string `json:"bound_service_account_namespaces"`
	Name                          string `json:"name"`
	Policies                      string `json:"policies"`
	TTL                           string `json:"ttl"`
}

type VaultPolicies []VaultPolicy

type VaultPolicy struct {
	Name  string `json:"name"`
	Rules string `json:"rules"`
}

type Tcp struct {
	Address    string `json:"address"`
	TlsDisable bool   `json:"tls_disable"`
}

type Listener struct {
	Tcp Tcp `json:"tcp"`
}

type Telemetry struct {
	StatsdAddress string `json:"statsd_address"`
}

type Storage struct {
	GCS GCSConfig `json:"gcs"`
}

// VaultGcpServiceAccountSecretName builds the secret name where the GCP service account is stored
func VaultGcpServiceAccountSecretName(vaultName string) string {
	return fmt.Sprintf("%s-gcp-sa", vaultName)
}

// CreateVault creates a new vault backed by GCP KMS and storage
func CreateVault(vaultOperatorClient versioned.Interface, name string, ns string,
	gcpServiceAccountSecretName string, gcpConfig *GCPConfig, authServiceAccount string,
	authServiceAccountNamespace string, secretsPathPrefix string) error {

	if secretsPathPrefix == "" {
		secretsPathPrefix = DefaultSecretsPathPrefix
	}
	pathRule := &PathRule{
		Path: []PathPolicy{{
			Prefix:       secretsPathPrefix,
			Capabilities: DefaultSecretsCapabiltities,
		}},
	}
	vaultRule, err := pathRule.String()
	if err != nil {
		return errors.Wrap(err, "encoding the polcies for secret path")
	}

	vault := &v1alpha1.Vault{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Vault",
			APIVersion: "vault.banzaicloud.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: v1alpha1.VaultSpec{
			Size:            defaultNumVaults,
			Image:           vaultImage,
			BankVaultsImage: bankVaultsImage,
			ServiceType:     string(v1.ServiceTypeClusterIP),
			Config: map[string]interface{}{
				"api_addr":           fmt.Sprintf("http://%s.%s:8200", name, ns),
				"disable_clustering": true,
				"listener": Listener{
					Tcp: Tcp{
						Address:    "0.0.0.0:8200",
						TlsDisable: true,
					},
				},
				"storage": Storage{
					GCS: GCSConfig{
						Bucket:    gcpConfig.GcsBucket,
						HaEnabled: "true",
					},
				},
				"telemetry": Telemetry{
					StatsdAddress: "localhost:9125",
				},
				"ui": true,
			},
			ExternalConfig: map[string]interface{}{
				vaultAuthName: []VaultAuth{
					{
						Roles: []VaultRole{
							{

								BoundServiceAccountNames:      authServiceAccount,
								BoundServiceAccountNamespaces: authServiceAccountNamespace,
								Name:                          authServiceAccount,
								Policies:                      PathRulesName,
								TTL:                           vaultAuthTTL,
							},
						},
						Type: vaultAuthType,
					},
				},
				PoliciesName: []VaultPolicy{
					{
						Name:  PathRulesName,
						Rules: vaultRule,
					},
				},
			},
			UnsealConfig: v1alpha1.UnsealConfig{
				Google: &v1alpha1.GoogleUnsealConfig{
					KMSKeyRing:    gcpConfig.KmsKeyring,
					KMSCryptoKey:  gcpConfig.KmsKey,
					KMSLocation:   gcpConfig.KmsLocation,
					KMSProject:    gcpConfig.ProjectId,
					StorageBucket: gcpConfig.GcsBucket,
				},
			},
			CredentialsConfig: v1alpha1.CredentialsConfig{
				Env:        gcpServiceAccountEnv,
				Path:       gcpServiceAccountPath,
				SecretName: gcpServiceAccountSecretName,
			},
		},
	}

	_, err = vaultOperatorClient.Vault().Vaults(ns).Create(vault)
	return err
}

// FindVault  checks if a vault is available
func FindVault(vaultOperatorClient versioned.Interface, name string, ns string) bool {
	_, err := vaultOperatorClient.Vault().Vaults(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return true
}

// VaultAuthServiceAccountName returns the vault service account name
func VaultAuthServiceAccountName(vaultName string) string {
	return fmt.Sprintf("%s-%s", vaultName, vaultAuthSaSuffix)
}

// GetVaults returns all vaults available in a given namespaces
func GetVaults(client kubernetes.Interface, vaultOperatorClient versioned.Interface, ns string) ([]*Vault, error) {
	vaultList, err := vaultOperatorClient.Vault().Vaults(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "listing vaults in namespace '%s'", ns)
	}

	vaults := []*Vault{}
	for _, v := range vaultList.Items {
		vaultName := v.Name
		vaultAuthSaName := VaultAuthServiceAccountName(vaultName)
		vaultURL, err := services.FindServiceURL(client, ns, vaultName)
		if err != nil {
			vaultURL = ""
		}
		vault := Vault{
			Name:                   vaultName,
			Namespace:              ns,
			URL:                    vaultURL,
			AuthServiceAccountName: vaultAuthSaName,
		}
		vaults = append(vaults, &vault)
	}
	return vaults, nil
}

// DeleteVault delete a Vault resource
func DeleteVault(vaultOperatorClient versioned.Interface, name string, ns string) error {
	return vaultOperatorClient.Vault().Vaults(ns).Delete(name, &metav1.DeleteOptions{})
}
