package vault

import (
	"bytes"
	"fmt"

	"github.com/jenkins-x/jx/pkg/util/json"
	"k8s.io/apimachinery/pkg/types"

	"github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/kube/serviceaccount"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	BankVaultsImage    = "banzaicloud/bank-vaults"
	VaultOperatorImage = "banzaicloud/vault-operator"
	VaultImage         = "vault"

	gcpServiceAccountEnv  = "GOOGLE_APPLICATION_CREDENTIALS"
	gcpServiceAccountPath = "/etc/gcp/service-account.json"

	awsServiceAccountEnv  = "AWS_SHARED_CREDENTIALS_FILE"
	awsServiceAccountPath = "/etc/aws/credentials"

	vaultAuthName = "auth"
	vaultAuthType = "kubernetes"
	vaultAuthTTL  = "1h"

	vaultRoleName = "vault-auth"

	vaultSecretEngines      = "secrets"
	defaultNumVaults        = 1
	defaultInternalVaultURL = "http://%s:" + vault.DefaultVaultPort
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

// GCSConfig Google Cloud Storage config for Vault backend
type GCSConfig struct {
	Bucket    string `json:"bucket"`
	HaEnabled string `json:"ha_enabled"`
}

// AWSConfig keeps the vault configuration for AWS
type AWSConfig struct {
	v1alpha1.AWSUnsealConfig
	AutoCreate          bool
	DynamoDBTable       string
	DynamoDBRegion      string
	AccessKeyID         string
	SecretAccessKey     string
	ProvidedIAMUsername string
}

// DynamoDBConfig AWS DynamoDB config for Vault backend
type DynamoDBConfig struct {
	HaEnabled       string `json:"ha_enabled"`
	Region          string `json:"region"`
	Table           string `json:"table"`
	AccessKeyID     string `json:"access_key"`
	SecretAccessKey string `json:"secret_key"`
}

// VaultAuths list of vault authentications
type VaultAuths []VaultAuth

// VaultAuth vault auth configuration
type VaultAuth struct {
	Roles []VaultRole `json:"roles"`
	Type  string      `json:"type"`
}

// VaultRole role configuration for VaultAuth
type VaultRole struct {
	BoundServiceAccountNames      string `json:"bound_service_account_names"`
	BoundServiceAccountNamespaces string `json:"bound_service_account_namespaces"`
	Name                          string `json:"name"`
	Policies                      string `json:"policies"`
	TTL                           string `json:"ttl"`
}

// VaultPolicies list of vault policies
type VaultPolicies []VaultPolicy

// VaultPolicy vault policy
type VaultPolicy struct {
	Name  string `json:"name"`
	Rules string `json:"rules"`
}

// Tcp address for vault server
type Tcp struct {
	Address    string `json:"address"`
	TlsDisable bool   `json:"tls_disable"`
}

// Listener vault server listener
type Listener struct {
	Tcp Tcp `json:"tcp"`
}

// Telemetry address for telemetry server
type Telemetry struct {
	StatsdAddress string `json:"statsd_address"`
}

// Storage configuration for Vault storage
type Storage struct {
	GCS      *GCSConfig      `json:"gcs,omitempty"`
	DynamoDB *DynamoDBConfig `json:"dynamodb,omitempty"`
}

// SecretEngine configuration for secret engine
type SecretEngine struct {
	vaultapi.MountInput
	Path string `json:"path"`
}

// Seal configuration for Vault auto-unseal
type Seal struct {
	GcpCkms *GCPSealConfig `json:"gcpckms,omitempty"`
	AWSKms  *AWSSealConig  `json:"awskms,omitempty"`
}

// GCPSealConfig Google Cloud KMS config for vault auto-unseal
type GCPSealConfig struct {
	Credentials string `json:"credentials,omitempty"`
	Project     string `json:"project,omitempty"`
	Region      string `json:"region,omitempty"`
	KeyRing     string `json:"key_ring,omitempty"`
	CryptoKey   string `json:"crypto_key,omitempty"`
}

// AWSSealConig AWS KMS config for vault auto-unseal
type AWSSealConig struct {
	Region    string `json:"region,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`
	KmsKeyID  string `json:"kms_key_id,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"`
}

// CloudProviderConfig is a wrapper around the cloud provider specific elements of the Vault CRD configuration
type CloudProviderConfig struct {
	Storage           map[string]interface{}
	Seal              map[string]interface{}
	UnsealConfig      v1alpha1.UnsealConfig
	CredentialsConfig v1alpha1.CredentialsConfig
}

// SystemVaultName returns the name of the system vault based on the cluster name
func SystemVaultName(kuber kube.Kuber) (string, error) {
	clusterName, err := cluster.ShortName(kuber)
	if err != nil {
		return "", err
	}
	return SystemVaultNameForCluster(clusterName), nil
}

// SystemVaultNameForCluster returns the system vault name from a given cluster name
func SystemVaultNameForCluster(clusterName string) string {
	shortClusterName := naming.ToValidNameTruncated(clusterName, 16)
	fullName := fmt.Sprintf("%s-%s", vault.SystemVaultNamePrefix, shortClusterName)
	return naming.ToValidNameTruncated(fullName, 22)
}

// PrepareGKEVaultCRD creates a new vault backed by GCP KMS and storage
func PrepareGKEVaultCRD(gcpServiceAccountSecretName string, gcpConfig *GCPConfig) (CloudProviderConfig, error) {
	storage := Storage{
		GCS: &GCSConfig{
			Bucket:    gcpConfig.GcsBucket,
			HaEnabled: "true",
		},
	}
	storageConfig, err := util.ToObjectMap(storage)
	if err != nil {
		return CloudProviderConfig{}, errors.Wrap(err, "error creating storage config")
	}

	seal := Seal{
		GcpCkms: &GCPSealConfig{
			Credentials: gcpServiceAccountPath,
			Project:     gcpConfig.ProjectId,
			Region:      gcpConfig.KmsLocation,
			KeyRing:     gcpConfig.KmsKeyring,
			CryptoKey:   gcpConfig.KmsKey,
		},
	}
	sealConfig, err := util.ToObjectMap(seal)
	if err != nil {
		return CloudProviderConfig{}, errors.Wrap(err, "error creating seal config")
	}

	unsealConfig := v1alpha1.UnsealConfig{
		Google: &v1alpha1.GoogleUnsealConfig{
			KMSKeyRing:    gcpConfig.KmsKeyring,
			KMSCryptoKey:  gcpConfig.KmsKey,
			KMSLocation:   gcpConfig.KmsLocation,
			KMSProject:    gcpConfig.ProjectId,
			StorageBucket: gcpConfig.GcsBucket,
		},
	}
	credentialsConfig := v1alpha1.CredentialsConfig{
		Env:        gcpServiceAccountEnv,
		Path:       gcpServiceAccountPath,
		SecretName: gcpServiceAccountSecretName,
	}
	return CloudProviderConfig{storageConfig, sealConfig, unsealConfig, credentialsConfig}, nil
}

// PrepareAWSVaultCRD creates a new vault backed by AWS KMS and DynamoDB storage
func PrepareAWSVaultCRD(awsServiceAccountSecretName string, awsConfig *AWSConfig) (CloudProviderConfig, error) {
	storage := Storage{
		DynamoDB: &DynamoDBConfig{
			HaEnabled:       "true",
			Region:          awsConfig.DynamoDBRegion,
			Table:           awsConfig.DynamoDBTable,
			AccessKeyID:     awsConfig.AccessKeyID,
			SecretAccessKey: awsConfig.SecretAccessKey,
		},
	}
	storageConfig, err := util.ToObjectMap(storage)
	if err != nil {
		return CloudProviderConfig{}, errors.Wrap(err, "error creating storage config")
	}

	seal := Seal{
		AWSKms: &AWSSealConig{
			Region:    awsConfig.KMSRegion,
			AccessKey: awsConfig.AccessKeyID,
			SecretKey: awsConfig.SecretAccessKey,
			KmsKeyID:  awsConfig.KMSKeyID,
		},
	}
	sealConfig, err := util.ToObjectMap(seal)
	if err != nil {
		return CloudProviderConfig{}, errors.Wrap(err, "error creating seal config")
	}

	unsealConfig := v1alpha1.UnsealConfig{
		AWS: &awsConfig.AWSUnsealConfig,
	}
	credentialsConfig := v1alpha1.CredentialsConfig{
		Env:        awsServiceAccountEnv,
		Path:       awsServiceAccountPath,
		SecretName: awsServiceAccountSecretName,
	}
	return CloudProviderConfig{storageConfig, sealConfig, unsealConfig, credentialsConfig}, nil
}

// NewVaultCRD creates and initializes a new Vault instance.
func NewVaultCRD(kubeClient kubernetes.Interface, name string, ns string, images map[string]string,
	authServiceAccount string, authServiceAccountNamespace string, secretsPathPrefix string) (*v1alpha1.Vault, error) {

	err := createVaultServiceAccount(kubeClient, ns, name)
	if err != nil {
		return nil, errors.Wrapf(err, "creating the vault service account '%s'", name)
	}

	err = ensureVaultRoleBinding(kubeClient, ns, vaultRoleName, name, name)
	if err != nil {
		return nil, errors.Wrapf(err, "ensuring vault cluster role binding '%s' is created", name)
	}

	if secretsPathPrefix == "" {
		secretsPathPrefix = vault.DefaultSecretsPathPrefix
	}
	pathRule := &vault.PathRule{
		Path: []vault.PathPolicy{{
			Prefix:       secretsPathPrefix,
			Capabilities: vault.DefaultSecretsCapabiltities,
		}},
	}
	vaultRule, err := pathRule.String()
	if err != nil {
		return nil, errors.Wrap(err, "encoding the policies for secret path")
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
			Image:           images[VaultImage],
			BankVaultsImage: images[BankVaultsImage],
			ServiceType:     string(v1.ServiceTypeClusterIP),
			ServiceAccount:  name,
			Config: map[string]interface{}{
				"api_addr":           fmt.Sprintf("http://%s.%s:%s", name, ns, vault.DefaultVaultPort),
				"disable_clustering": true,
				"listener": Listener{
					Tcp: Tcp{
						Address:    fmt.Sprintf("0.0.0.0:%s", vault.DefaultVaultPort),
						TlsDisable: true,
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
								Policies:                      vault.PathRulesName,
								TTL:                           vaultAuthTTL,
							},
						},
						Type: vaultAuthType,
					},
				},
				vault.PoliciesName: []VaultPolicy{
					{
						Name:  vault.PathRulesName,
						Rules: vaultRule,
					},
				},
				vaultSecretEngines: []SecretEngine{
					{
						Path: vault.DefaultSecretsPath,
						MountInput: vaultapi.MountInput{
							Type:        "kv",
							Description: "KV secret engine",
							Local:       false,
							SealWrap:    false,
							Options: map[string]string{
								"version": "2",
							},
							Config: vaultapi.MountConfigInput{
								ForceNoCache: true,
							},
						},
					},
				},
			},
		},
	}

	return vault, err
}

func createVaultServiceAccount(client kubernetes.Interface, namespace string, name string) error {
	_, err := serviceaccount.CreateServiceAccount(client, namespace, name)
	if err != nil {
		return errors.Wrap(err, "creating vault service account")
	}
	return nil
}

func ensureVaultRoleBinding(client kubernetes.Interface, namespace string, roleName string,
	roleBindingName string, serviceAccount string) error {
	apiGroups := []string{"authentication.k8s.io"}
	resources := []string{"tokenreviews"}
	verbs := []string{"*"}
	found := kube.IsClusterRole(client, roleName)
	if found {
		err := kube.DeleteClusterRole(client, roleName)
		if err != nil {
			return errors.Wrapf(err, "deleting the existing cluster role '%s'", roleName)
		}
	}
	err := kube.CreateClusterRole(client, namespace, roleName, apiGroups, resources, verbs)
	if err != nil {
		return errors.Wrapf(err, "creating the cluster role '%s' for vault", roleName)
	}

	found = kube.IsClusterRoleBinding(client, roleBindingName)
	if found {
		err := kube.DeleteClusterRoleBinding(client, roleBindingName)
		if err != nil {
			return errors.Wrapf(err, "deleting the existing cluster role binding '%s'", roleBindingName)
		}
	}

	err = kube.CreateClusterRoleBinding(client, namespace, roleBindingName, serviceAccount, roleName)
	if err != nil {
		return errors.Wrapf(err, "creating the cluster role binding '%s' for vault", roleBindingName)
	}
	return nil
}

// FindVault checks if a vault is available
func FindVault(vaultOperatorClient versioned.Interface, name string, ns string) bool {
	_, err := GetVault(vaultOperatorClient, name, ns)
	if err != nil {
		log.Logger().Debugf("vault %s not found in namespace %s, err is %s", name, ns, err)
		return false
	}
	return true
}

// GetVault gets a specific vault
func GetVault(vaultOperatorClient versioned.Interface, name string, ns string) (*v1alpha1.Vault, error) {
	return vaultOperatorClient.VaultV1alpha1().Vaults(ns).Get(name, metav1.GetOptions{})
}

// GetVaults returns all vaults available in a given namespaces
func GetVaults(client kubernetes.Interface, vaultOperatorClient versioned.Interface, ns string, useIngressURL bool) ([]*Vault, error) {
	vaultList, err := vaultOperatorClient.Vault().Vaults(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "listing vaults in namespace '%s'", ns)
	}

	vaults := []*Vault{}
	for _, v := range vaultList.Items {
		vaultName := v.Name
		vaultAuthSaName := GetAuthSaName(v)

		// default to using internal kubernetes service dns name for vault endpoint
		vaultURL := fmt.Sprintf(defaultInternalVaultURL, vaultName)
		if useIngressURL {
			vaultURL, err = services.FindIngressURL(client, ns, vaultName)
			if err != nil || vaultURL == "" {
				log.Logger().Debugf("Cannot finding the vault ingress url for vault %s", vaultName)
				// skip this vault since cannot be used without the ingress
				continue
			}
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

// GetAuthSaName gets the Auth Service Account name for the vault
func GetAuthSaName(vault v1alpha1.Vault) string {
	// This is nasty, but the ExternalConfig member of VaultSpec is just defined as a map[string]interface{} :-(
	authArray := vault.Spec.ExternalConfig["auth"]
	authObject := authArray.([]interface{})[0]
	roleArray := authObject.(map[string]interface{})["roles"]
	roleObject := roleArray.([]interface{})[0]
	name := roleObject.(map[string]interface{})["name"]

	return name.(string)
}

// CreateOrUpdateVault creates the specified Vault CRD if it does not exist or updates it otherwise.
func CreateOrUpdateVault(vault *v1alpha1.Vault, vaultOperatorClient versioned.Interface, ns string) error {
	vaultExists := false
	existingVault, err := GetVault(vaultOperatorClient, vault.Name, ns)
	if err == nil {
		vaultExists = true
	} else {
		statusError, ok := err.(*apierrors.StatusError)
		if ok && statusError.ErrStatus.Code == 404 {
			vaultExists = false
		} else {
			return errors.Wrapf(err, "unable to check for existence of vault '%s' in namespace '%s'", vault.Name, ns)
		}
	}

	if vaultExists {
		vaultName := existingVault.ObjectMeta.Name
		patch, err := json.CreatePatch(existingVault, vault)
		if err != nil {
			return errors.Wrapf(err, "unable to create path for update of of vault '%s' in namespace '%s'", vault.Name, ns)
		}
		if bytes.Equal(patch, []byte("[]")) {
			return nil
		}
		_, err = vaultOperatorClient.VaultV1alpha1().Vaults(ns).Patch(vaultName, types.JSONPatchType, patch)
	} else {
		_, err = vaultOperatorClient.VaultV1alpha1().Vaults(ns).Create(vault)
	}

	op := "create"
	if vaultExists {
		op = "update"
	}
	if err != nil {
		return errors.Wrapf(err, "unable to %s vault '%s' in namespace '%s'", op, vault.Name, ns)
	}
	log.Logger().Infof("Vault '%s' in namespace '%s' %sd ", util.ColorInfo(vault.Name), util.ColorInfo(ns), op)

	return nil
}
