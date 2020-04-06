package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/vrischmann/envconfig"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	// autoDNSSuffixes the DNS suffixes of any auto-DNS services
	autoDNSSuffixes = []string{
		".nip.io",
		".xip.io",
		".beesdns.com",
	}
)

const (
	// RequirementsConfigFileName is the name of the requirements configuration file
	RequirementsConfigFileName = "jx-requirements.yml"
	// RequirementsValuesFileName is the name of the requirements configuration file
	RequirementsValuesFileName = "jx-requirements.values.yaml.gotmpl"
	// RequirementDomainIssuerUsername contains the username used for basic auth when requesting a domain
	RequirementDomainIssuerUsername = "JX_REQUIREMENT_DOMAIN_ISSUER_USERNAME"
	// RequirementDomainIssuerPassword contains the password used for basic auth when requesting a domain
	RequirementDomainIssuerPassword = "JX_REQUIREMENT_DOMAIN_ISSUER_PASSWORD"
	// RequirementDomainIssuerURL contains the URL to the service used when requesting a domain
	RequirementDomainIssuerURL = "JX_REQUIREMENT_DOMAIN_ISSUER_URL"
	// RequirementClusterName is the cluster name
	RequirementClusterName = "JX_REQUIREMENT_CLUSTER_NAME"
	// RequirementProject is the cloudprovider project
	RequirementProject = "JX_REQUIREMENT_PROJECT"
	// RequirementZone zone the cluster is in
	RequirementZone = "JX_REQUIREMENT_ZONE"
	// RequirementEnvGitOwner the default git owner for environment repositories if none is specified explicitly
	RequirementEnvGitOwner = "JX_REQUIREMENT_ENV_GIT_OWNER"
	// RequirementEnvGitPublic sets the visibility of the environment repositories as private (subscription required for GitHub Organisations)
	RequirementEnvGitPublic = "JX_REQUIREMENT_ENV_GIT_PUBLIC"
	// RequirementGitPublic sets the visibility of the application repositories as private (subscription required for GitHub Organisations)
	RequirementGitPublic = "JX_REQUIREMENT_GIT_PUBLIC"
	// RequirementExternalDNSServiceAccountName the service account name for external dns
	RequirementExternalDNSServiceAccountName = "JX_REQUIREMENT_EXTERNALDNS_SA_NAME"
	// RequirementVaultName the name for vault
	RequirementVaultName = "JX_REQUIREMENT_VAULT_NAME"
	// RequirementVaultServiceAccountName the service account name for vault
	RequirementVaultServiceAccountName = "JX_REQUIREMENT_VAULT_SA_NAME"
	// RequirementVeleroServiceAccountName the service account name for velero
	RequirementVeleroServiceAccountName = "JX_REQUIREMENT_VELERO_SA_NAME"
	// RequirementVaultKeyringName the keyring name for vault
	RequirementVaultKeyringName = "JX_REQUIREMENT_VAULT_KEYRING_NAME"
	// RequirementVaultKeyName the key name for vault
	RequirementVaultKeyName = "JX_REQUIREMENT_VAULT_KEY_NAME"
	// RequirementVaultBucketName the vault name for vault
	RequirementVaultBucketName = "JX_REQUIREMENT_VAULT_BUCKET_NAME"
	// RequirementVaultRecreateBucket recreate the bucket that vault uses
	RequirementVaultRecreateBucket = "JX_REQUIREMENT_VAULT_RECREATE_BUCKET"
	// RequirementVaultDisableURLDiscovery override the default lookup of the Vault URL, could be incluster service or external ingress
	RequirementVaultDisableURLDiscovery = "JX_REQUIREMENT_VAULT_DISABLE_URL_DISCOVERY"
	// RequirementSecretStorageType the secret storage type
	RequirementSecretStorageType = "JX_REQUIREMENT_SECRET_STORAGE_TYPE"
	// RequirementKanikoServiceAccountName the service account name for kaniko
	RequirementKanikoServiceAccountName = "JX_REQUIREMENT_KANIKO_SA_NAME"
	// RequirementKaniko if kaniko is required
	RequirementKaniko = "JX_REQUIREMENT_KANIKO"
	// RequirementIngressTLSProduction use the lets encrypt production server
	RequirementIngressTLSProduction = "JX_REQUIREMENT_INGRESS_TLS_PRODUCTION"
	// RequirementChartRepository the helm chart repository for jx
	RequirementChartRepository = "JX_REQUIREMENT_CHART_REPOSITORY"
	// RequirementRegistry the container registry for jx
	RequirementRegistry = "JX_REQUIREMENT_REGISTRY"
	// RequirementRepository the artifact repository for jx
	RequirementRepository = "JX_REQUIREMENT_REPOSITORY"
	// RequirementWebhook the webhook handler for jx
	RequirementWebhook = "JX_REQUIREMENT_WEBHOOK"
	// RequirementStorageBackupEnabled if backup storage is required
	RequirementStorageBackupEnabled = "JX_REQUIREMENT_STORAGE_BACKUP_ENABLED"
	// RequirementStorageBackupURL backup storage url
	RequirementStorageBackupURL = "JX_REQUIREMENT_STORAGE_BACKUP_URL"
	// RequirementStorageLogsEnabled if log storage is required
	RequirementStorageLogsEnabled = "JX_REQUIREMENT_STORAGE_LOGS_ENABLED"
	// RequirementStorageLogsURL logs storage url
	RequirementStorageLogsURL = "JX_REQUIREMENT_STORAGE_LOGS_URL"
	// RequirementStorageReportsEnabled if report storage is required
	RequirementStorageReportsEnabled = "JX_REQUIREMENT_STORAGE_REPORTS_ENABLED"
	// RequirementStorageReportsURL report storage url
	RequirementStorageReportsURL = "JX_REQUIREMENT_STORAGE_REPORTS_URL"
	// RequirementStorageRepositoryEnabled if repository storage is required
	RequirementStorageRepositoryEnabled = "JX_REQUIREMENT_STORAGE_REPOSITORY_ENABLED"
	// RequirementStorageRepositoryURL repository storage url
	RequirementStorageRepositoryURL = "JX_REQUIREMENT_STORAGE_REPOSITORY_URL"
	// RequirementGkeProjectNumber is the gke project number
	RequirementGkeProjectNumber = "JX_REQUIREMENT_GKE_PROJECT_NUMBER"
	// RequirementGitAppEnabled if the github app should be used for access tokens
	RequirementGitAppEnabled = "JX_REQUIREMENT_GITHUB_APP_ENABLED"
	// RequirementGitAppURL contains the URL to the github app
	RequirementGitAppURL = "JX_REQUIREMENT_GITHUB_APP_URL"
	// RequirementDevEnvApprovers contains the optional list of users to populate the dev env's OWNERS with
	RequirementDevEnvApprovers = "JX_REQUIREMENT_DEV_ENV_APPROVERS"
	// RequirementVersionsGitRef contains the git ref of the version stream
	RequirementVersionsGitRef = "JX_REQUIREMENT_VERSIONS_GIT_REF"
)

const (
	// BootDeployNamespace environment variable for deployment namespace
	BootDeployNamespace = "DEPLOY_NAMESPACE"
)

// SecretStorageType is the type of storage used for secrets
type SecretStorageType string

const (
	// SecretStorageTypeVault specifies that we use vault to store secrets
	SecretStorageTypeVault SecretStorageType = "vault"
	// SecretStorageTypeLocal specifies that we use the local file system in
	// `~/.jx/localSecrets` to store secrets
	SecretStorageTypeLocal SecretStorageType = "local"
)

// SecretStorageTypeValues the string values for the secret storage
var SecretStorageTypeValues = []string{"local", "vault"}

// WebhookType is the type of a webhook strategy
type WebhookType string

const (
	// WebhookTypeNone if we have yet to define a webhook
	WebhookTypeNone WebhookType = ""
	// WebhookTypeProw specifies that we use prow for webhooks
	// see: https://github.com/kubernetes/test-infra/tree/master/prow
	WebhookTypeProw WebhookType = "prow"
	// WebhookTypeLighthouse specifies that we use lighthouse for webhooks
	// see: https://github.com/jenkins-x/lighthouse
	WebhookTypeLighthouse WebhookType = "lighthouse"
	// WebhookTypeJenkins specifies that we use jenkins webhooks
	WebhookTypeJenkins WebhookType = "jenkins"
)

// WebhookTypeValues the string values for the webhook types
var WebhookTypeValues = []string{"jenkins", "lighthouse", "prow"}

// RepositoryType is the type of a repository we use to store artifacts (jars, tarballs, npm packages etc)
type RepositoryType string

const (
	// RepositoryTypeUnknown if we have yet to configure a repository
	RepositoryTypeUnknown RepositoryType = ""
	// RepositoryTypeArtifactory if you wish to use Artifactory as the artifact repository
	RepositoryTypeArtifactory RepositoryType = "artifactory"
	// RepositoryTypeBucketRepo if you wish to use bucketrepo as the artifact repository. see https://github.com/jenkins-x/bucketrepo
	RepositoryTypeBucketRepo RepositoryType = "bucketrepo"
	// RepositoryTypeNone if you do not wish to install an artifact repository
	RepositoryTypeNone RepositoryType = "none"
	// RepositoryTypeNexus if you wish to use Sonatype Nexus as the artifact repository
	RepositoryTypeNexus RepositoryType = "nexus"
)

// RepositoryTypeValues the string values for the repository types
var RepositoryTypeValues = []string{"none", "bucketrepo", "nexus", "artifactory"}

const (
	// DefaultProfileFile location of profle config
	DefaultProfileFile = "profile.yaml"
	// OpenSourceProfile constant for OSS profile
	OpenSourceProfile = "oss"
	// CloudBeesProfile constant for CloudBees profile
	CloudBeesProfile = "cloudbees"
)

// Overrideable at build time - see Makefile
var (
	// DefaultVersionsURL default version stream url
	DefaultVersionsURL = "https://github.com/jenkins-x/jenkins-x-versions.git"
	// DefaultVersionsRef default version stream ref
	DefaultVersionsRef = "master"
	// DefaultBootRepository default git repo for boot
	DefaultBootRepository = "https://github.com/jenkins-x/jenkins-x-boot-config.git"
	// LatestVersionStringsBucket optional bucket name to search in for latest version strings
	LatestVersionStringsBucket = ""
	// BinaryDownloadBaseURL the base URL for downloading the binary from - will always have "VERSION/jx-OS-ARCH.EXTENSION" appended to it when used
	BinaryDownloadBaseURL = "https://github.com/jenkins-x/jx/releases/download/v"
	// TLSDocURL the URL presented by `jx step verify preinstall` for documentation on configuring TLS
	TLSDocURL = "https://jenkins-x.io/docs/getting-started/setup/boot/#ingress"
)

// EnvironmentConfig configures the organisation and repository name of the git repositories for environments
type EnvironmentConfig struct {
	// Key is the key of the environment configuration
	Key string `json:"key,omitempty"`
	// Owner is the git user or organisation for the repository
	Owner string `json:"owner,omitempty"`
	// Repository is the name of the repository within the owner
	Repository string `json:"repository,omitempty"`
	// GitServer is the URL of the git server
	GitServer string `json:"gitServer,omitempty"`
	// GitKind is the kind of git server (github, bitbucketserver etc)
	GitKind string `json:"gitKind,omitempty"`
	// Ingress contains ingress specific requirements
	Ingress IngressConfig `json:"ingress,omitempty"`
	// RemoteCluster specifies this environment runs on a remote cluster to the development cluster
	RemoteCluster bool `json:"remoteCluster,omitempty"`
	// PromotionStrategy what kind of promotion strategy to use
	PromotionStrategy v1.PromotionStrategyType `json:"promotionStrategy,omitempty"`
	// URLTemplate is the template to use for your environment's exposecontroller generated URLs
	URLTemplate string `json:"urlTemplate,omitempty"`
}

// IngressConfig contains dns specific requirements
type IngressConfig struct {
	// DNS is enabled
	ExternalDNS bool `json:"externalDNS"`
	// CloudDNSSecretName secret name which contains the service account for external-dns and cert-manager issuer to
	// access the Cloud DNS service to resolve a DNS challenge
	CloudDNSSecretName string `json:"cloud_dns_secret_name,omitempty"`
	// Domain to expose ingress endpoints
	Domain string `json:"domain"`
	// IgnoreLoadBalancer if the nginx-controller LoadBalancer service should not be used to detect and update the
	// domain if you are using a dynamic domain resolver like `.nip.io` rather than a real DNS configuration.
	// With this flag enabled the `Domain` value will be used and never re-created based on the current LoadBalancer IP address.
	IgnoreLoadBalancer bool `json:"ignoreLoadBalancer,omitempty"`
	// Exposer the exposer used to expose ingress endpoints. Defaults to "Ingress"
	Exposer string `json:"exposer,omitempty"`
	// NamespaceSubDomain the sub domain expression to expose ingress. Defaults to ".jx."
	NamespaceSubDomain string `json:"namespaceSubDomain"`
	// TLS enable automated TLS using certmanager
	TLS TLSConfig `json:"tls"`
	// DomainIssuerURL contains a URL used to retrieve a Domain
	DomainIssuerURL string `json:"domainIssuerURL,omitempty"`
}

// TLSConfig contains TLS specific requirements
type TLSConfig struct {
	// TLS enabled
	Enabled bool `json:"enabled"`
	// Email address to register with services like LetsEncrypt
	Email string `json:"email"`
	// Production false uses self-signed certificates from the LetsEncrypt staging server, true enables the production
	// server which incurs higher rate limiting https://letsencrypt.org/docs/rate-limits/
	Production bool `json:"production"`
	// SecretName the name of the secret which contains the TLS certificate
	SecretName string `json:"secretName,omitempty"`
}

// JxInstallProfile contains the jx profile info
type JxInstallProfile struct {
	InstallType string
}

// StorageEntryConfig contains dns specific requirements for a kind of storage
type StorageEntryConfig struct {
	// Enabled if the storage is enabled
	Enabled bool `json:"enabled"`
	// URL the cloud storage bucket URL such as 'gs://mybucket' or 's3://foo' or `azblob://thingy'
	// see https://jenkins-x.io/architecture/storage/
	URL string `json:"url"`
}

// StorageConfig contains dns specific requirements
type StorageConfig struct {
	// Logs for storing build logs
	Logs StorageEntryConfig `json:"logs"`
	// Tests for storing test results, coverage + code quality reports
	Reports StorageEntryConfig `json:"reports"`
	// Repository for storing repository artifacts
	Repository StorageEntryConfig `json:"repository"`
	// Backup for backing up kubernetes resource
	Backup StorageEntryConfig `json:"backup"`
}

// AzureConfig contains Azure specific requirements
type AzureConfig struct {
	// RegistrySubscription the registry subscription for defaulting the container registry.
	// Not used if you specify a Registry explicitly
	RegistrySubscription string `json:"registrySubscription,omitempty"`
}

// GKEConfig contains GKE specific requirements
type GKEConfig struct {
	// ProjectNumber the unique project number GKE assigns to a project (required for workload identity).
	ProjectNumber string `json:"projectNumber,omitempty"`
}

// ClusterConfig contains cluster specific requirements
type ClusterConfig struct {
	// AzureConfig the azure specific configuration
	AzureConfig *AzureConfig `json:"azure,omitempty"`
	// ChartRepository the repository URL to deploy charts to
	ChartRepository string `json:"chartRepository,omitempty"`
	// GKEConfig the gke specific configuration
	GKEConfig *GKEConfig `json:"gke,omitempty"`
	// EnvironmentGitOwner the default git owner for environment repositories if none is specified explicitly
	EnvironmentGitOwner string `json:"environmentGitOwner,omitempty"`
	// EnvironmentGitPublic determines whether jx boot create public or private git repos for the environments
	EnvironmentGitPublic bool `json:"environmentGitPublic,omitempty"`
	// GitPublic determines whether jx boot create public or private git repos for the applications
	GitPublic bool `json:"gitPublic,omitempty"`
	// Provider the kubernetes provider (e.g. gke)
	Provider string `json:"provider,omitempty"`
	// Namespace the namespace to install the dev environment
	Namespace string `json:"namespace,omitempty"`
	// ProjectID the cloud project ID e.g. on GCP
	ProjectID string `json:"project,omitempty"`
	// ClusterName the logical name of the cluster
	ClusterName string `json:"clusterName,omitempty"`
	// VaultName the name of the vault if using vault for secrets
	// Deprecated
	VaultName string `json:"vaultName,omitempty"`
	// Region the cloud region being used
	Region string `json:"region,omitempty"`
	// Zone the cloud zone being used
	Zone string `json:"zone,omitempty"`
	// GitName is the name of the default git service
	GitName string `json:"gitName,omitempty"`
	// GitKind is the kind of git server (github, bitbucketserver etc)
	GitKind string `json:"gitKind,omitempty"`
	// GitServer is the URL of the git server
	GitServer string `json:"gitServer,omitempty"`
	// ExternalDNSSAName the service account name for external dns
	ExternalDNSSAName string `json:"externalDNSSAName,omitempty"`
	// Registry the host name of the container registry
	Registry string `json:"registry,omitempty"`
	// VaultSAName the service account name for vault
	// Deprecated
	VaultSAName string `json:"vaultSAName,omitempty"`
	// KanikoSAName the service account name for kaniko
	KanikoSAName string `json:"kanikoSAName,omitempty"`
	// HelmMajorVersion contains the major helm version number. Assumes helm 2.x with no tiller if no value specified
	HelmMajorVersion string `json:"helmMajorVersion,omitempty"`
	// DevEnvApprovers contains an optional list of approvers to populate the initial OWNERS file in the dev env repo
	DevEnvApprovers []string `json:"devEnvApprovers,omitempty"`
	// DockerRegistryOrg the default organisation used for container images
	DockerRegistryOrg string `json:"dockerRegistryOrg,omitempty"`
}

// VaultConfig contains Vault configuration for boot
type VaultConfig struct {
	// Name the name of the vault if using vault for secrets
	Name           string `json:"name,omitempty"`
	Bucket         string `json:"bucket,omitempty"`
	Keyring        string `json:"keyring,omitempty"`
	Key            string `json:"key,omitempty"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
	RecreateBucket bool   `json:"recreateBucket,omitempty"`
	// Optionally allow us to override the default lookup of the Vault URL, could be incluster service or external ingress
	DisableURLDiscovery bool            `json:"disableURLDiscovery,omitempty"`
	AWSConfig           *VaultAWSConfig `json:"aws,omitempty"`
}

// VaultAWSConfig contains all the Vault configuration needed by Vault to be deployed in AWS
type VaultAWSConfig struct {
	VaultAWSUnsealConfig
	AutoCreate          bool   `json:"autoCreate,omitempty"`
	DynamoDBTable       string `json:"dynamoDBTable,omitempty"`
	DynamoDBRegion      string `json:"dynamoDBRegion,omitempty"`
	ProvidedIAMUsername string `json:"iamUserName,omitempty"`
}

// VaultAWSUnsealConfig contains references to existing AWS resources that can be used to install Vault
type VaultAWSUnsealConfig struct {
	KMSKeyID  string `json:"kmsKeyId,omitempty"`
	KMSRegion string `json:"kmsRegion,omitempty"`
	S3Bucket  string `json:"s3Bucket,omitempty"`
	S3Prefix  string `json:"s3Prefix,omitempty"`
	S3Region  string `json:"s3Region,omitempty"`
}

// UnmarshalJSON method handles the rename of EnvironmentGitPrivate to EnvironmentGitPublic.
func (t *ClusterConfig) UnmarshalJSON(data []byte) error {
	// need a type alias to go into infinite loop
	type Alias ClusterConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	err := json.Unmarshal(data, &raw)
	if err != nil {
		return err
	}

	_, gitPublicSet := raw["environmentGitPublic"]
	private, gitPrivateSet := raw["environmentGitPrivate"]

	if gitPrivateSet && gitPublicSet {
		return errors.New("found settings for EnvironmentGitPublic as well as EnvironmentGitPrivate in ClusterConfig, only EnvironmentGitPublic should be used")
	}

	if gitPrivateSet {
		log.Logger().Warn("EnvironmentGitPrivate specified in Cluster EnvironmentGitPrivate is deprecated use EnvironmentGitPublic instead.")
		privateString := string(private)
		if privateString == "true" {
			t.EnvironmentGitPublic = false
		} else {
			t.EnvironmentGitPublic = true
		}
	}
	return nil
}

// VersionStreamConfig contains version stream config
type VersionStreamConfig struct {
	// URL of the version stream to use
	URL string `json:"url"`
	// Ref of the version stream to use
	Ref string `json:"ref"`
}

// VeleroConfig contains the configuration for velero
type VeleroConfig struct {
	// Namespace the namespace to install velero into
	Namespace string `json:"namespace,omitempty"`
	// ServiceAccount the cloud service account used to run velero
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// Schedule of backups
	Schedule string `json:"schedule" envconfig:"JX_REQUIREMENT_VELERO_SCHEDULE"`
	// TimeToLive period for backups to be retained
	TimeToLive string `json:"ttl" envconfig:"JX_REQUIREMENT_VELERO_TTL"`
}

// AutoUpdateConfig contains auto update config
type AutoUpdateConfig struct {
	// Enabled autoupdate
	Enabled bool `json:"enabled"`
	// Schedule cron of auto updates
	Schedule string `json:"schedule"`
}

// GithubAppConfig contains github app config
type GithubAppConfig struct {
	// Enabled this determines whether this install should use the jenkins x github app for access tokens
	Enabled bool `json:"enabled"`
	// Schedule cron of the github app token refresher
	Schedule string `json:"schedule,omitempty"`
	// URL contains a URL to the github app
	URL string `json:"url,omitempty"`
}

// RequirementsValues contains the logical installation requirements in the `jx-requirements.yml` file as helm values
type RequirementsValues struct {
	// RequirementsConfig contains the logical installation requirements
	RequirementsConfig *RequirementsConfig `json:"jxRequirements,omitempty"`
}

// RequirementsConfig contains the logical installation requirements in the `jx-requirements.yml` file when
// installing, configuring or upgrading Jenkins X via `jx boot`
type RequirementsConfig struct {
	// AutoUpdate contains auto update config
	AutoUpdate AutoUpdateConfig `json:"autoUpdate,omitempty"`
	// BootConfigURL contains the url to which the dev environment is associated with
	BootConfigURL string `json:"bootConfigURL,omitempty"`
	// Cluster contains cluster specific requirements
	Cluster ClusterConfig `json:"cluster"`
	// Environments the requirements for the environments
	Environments []EnvironmentConfig `json:"environments,omitempty"`
	// GithubApp contains github app config
	GithubApp *GithubAppConfig `json:"githubApp,omitempty"`
	// GitOps if enabled we will setup a webhook in the boot configuration git repository so that we can
	// re-run 'jx boot' when changes merge to the master branch
	GitOps bool `json:"gitops,omitempty"`
	// Indicates if we are using helmfile and helm 3 to spin up environments. This is currently an experimental
	// feature flag used to implement better Multi-Cluster support. See https://github.com/jenkins-x/jx/issues/6442
	Helmfile bool `json:"helmfile,omitempty"`
	// Kaniko whether to enable kaniko for building docker images
	Kaniko bool `json:"kaniko,omitempty"`
	// Ingress contains ingress specific requirements
	Ingress IngressConfig `json:"ingress"`
	// Repository specifies what kind of artifact repository you wish to use for storing artifacts (jars, tarballs, npm modules etc)
	Repository RepositoryType `json:"repository,omitempty"`
	// SecretStorage how should we store secrets for the cluster
	SecretStorage SecretStorageType `json:"secretStorage,omitempty"`
	// Storage contains storage requirements
	Storage StorageConfig `json:"storage"`
	// Terraform specifies if  we are managing the kubernetes cluster and cloud resources with Terraform
	Terraform bool `json:"terraform,omitempty"`
	// Vault the configuration for vault
	Vault VaultConfig `json:"vault,omitempty"`
	// Velero the configuration for running velero for backing up the cluster resources
	Velero VeleroConfig `json:"velero,omitempty"`
	// VersionStream contains version stream info
	VersionStream VersionStreamConfig `json:"versionStream"`
	// Webhook specifies what engine we should use for webhooks
	Webhook WebhookType `json:"webhook,omitempty"`
}

// NewRequirementsConfig creates a default configuration file
func NewRequirementsConfig() *RequirementsConfig {
	return &RequirementsConfig{
		SecretStorage: SecretStorageTypeLocal,
		Webhook:       WebhookTypeProw,
	}
}

// LoadRequirementsConfig loads the project configuration if there is a project configuration file
// if there is not a file called `jx-requirements.yml` in the given dir we will scan up the parent
// directories looking for the requirements file as we often run 'jx' steps in sub directories.
func LoadRequirementsConfig(dir string) (*RequirementsConfig, string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating absolute path")
	}
	for absolute != "" && absolute != "." && absolute != "/" {
		fileName := filepath.Join(absolute, RequirementsConfigFileName)
		absolute = filepath.Dir(absolute)

		exists, err := util.FileExists(fileName)
		if err != nil {
			return nil, "", err
		}

		if !exists {
			continue
		}

		config, err := LoadRequirementsConfigFile(fileName)
		return config, fileName, err
	}
	return nil, "", errors.New("jx-requirements.yml file not found")
}

// LoadRequirementsConfigFile loads a specific project YAML configuration file
func LoadRequirementsConfigFile(fileName string) (*RequirementsConfig, error) {
	config := &RequirementsConfig{}
	_, err := os.Stat(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "checking if file %s exists", fileName)
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load file %s due to %s", fileName, err)
	}

	validationErrors, err := util.ValidateYaml(config, data)
	if err != nil {
		return nil, fmt.Errorf("failed to validate YAML file %s due to %s", fileName, err)
	}

	if len(validationErrors) > 0 {
		return nil, fmt.Errorf("validation failures in YAML file %s:\n%s", fileName, strings.Join(validationErrors, "\n"))
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML file %s due to %s", fileName, err)
	}

	config.addDefaults()
	config.handleDeprecation()
	return config, nil
}

// GetRequirementsConfigFromTeamSettings reads the BootRequirements string from TeamSettings and unmarshals it
func GetRequirementsConfigFromTeamSettings(settings *v1.TeamSettings) (*RequirementsConfig, error) {
	if settings == nil {
		return nil, nil
	}

	// TeamSettings does not have a real value for BootRequirements, so this is probably not a boot cluster.
	if settings.BootRequirements == "" {
		return nil, nil
	}

	config := &RequirementsConfig{}
	data := []byte(settings.BootRequirements)
	validationErrors, err := util.ValidateYaml(config, data)
	if err != nil {
		return config, fmt.Errorf("failed to validate requirements from team settings due to %s", err)
	}
	if len(validationErrors) > 0 {
		return config, fmt.Errorf("validation failures in requirements from team settings:\n%s", strings.Join(validationErrors, "\n"))
	}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal requirements from team settings due to %s", err)
	}
	return config, nil
}

// IsEmpty returns true if this configuration is empty
func (c *RequirementsConfig) IsEmpty() bool {
	empty := &RequirementsConfig{}
	return reflect.DeepEqual(empty, c)
}

// SaveConfig saves the configuration file to the given project directory
func (c *RequirementsConfig) SaveConfig(fileName string) error {
	c.handleDeprecation()
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", fileName)
	}

	if c.Helmfile {
		y := RequirementsValues{
			RequirementsConfig: c,
		}
		data, err = yaml.Marshal(y)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(path.Join(path.Dir(fileName), RequirementsValuesFileName), data, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to save file %s", RequirementsValuesFileName)
		}
	}

	return nil
}

type environmentsSliceTransformer struct{}

// environmentsSliceTransformer.Transformer is handling the correct merge of two EnvironmentConfig slices
// so we can both append extra items and merge existing ones so we don't lose any data
func (t environmentsSliceTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf([]EnvironmentConfig{}) {
		return func(dst, src reflect.Value) error {
			d := dst.Interface().([]EnvironmentConfig)
			s := src.Interface().([]EnvironmentConfig)
			if dst.CanSet() {
				for i, v := range s {
					if i > len(d)-1 {
						d = append(d, v)
					} else {
						err := mergo.Merge(&d[i], &v, mergo.WithOverride)
						if err != nil {
							return errors.Wrap(err, "error merging EnvironmentConfig slices")
						}
					}
				}
				dst.Set(reflect.ValueOf(d))
			}
			return nil
		}
	}
	return nil
}

// MergeSave attempts to merge the provided RequirementsConfig with the caller's data.
// It does so overriding values in the source struct with non-zero values from the provided struct
// it defines non-zero per property and not for a while embedded struct, meaning that nested properties
// in embedded structs should also be merged correctly.
// if a slice is added a transformer will be needed to handle correctly merging the contained values
func (c *RequirementsConfig) MergeSave(src *RequirementsConfig, requirementsFileName string) error {
	err := mergo.Merge(c, src, mergo.WithOverride, mergo.WithTransformers(environmentsSliceTransformer{}))
	if err != nil {
		return errors.Wrap(err, "error merging jx-requirements.yml files")
	}
	err = c.SaveConfig(requirementsFileName)
	if err != nil {
		return errors.Wrapf(err, "error saving the merged jx-requirements.yml files to %s", requirementsFileName)
	}
	return nil
}

// EnvironmentMap creates a map of maps tree which can be used inside Go templates to access the environment
// configurations
func (c *RequirementsConfig) EnvironmentMap() map[string]interface{} {
	answer := map[string]interface{}{}
	for _, env := range c.Environments {
		k := env.Key
		if k == "" {
			log.Logger().Warnf("missing 'key' for Environment requirements %#v", env)
			continue
		}
		m, err := util.ToObjectMap(&env)
		if err == nil {
			ensureHasFields(m, "owner", "repository", "gitServer", "gitKind")
			answer[k] = m
		} else {
			log.Logger().Warnf("failed to turn environment %s with value %#v into a map: %s\n", k, env, err.Error())
		}
	}
	return answer
}

// Environment looks up the environment configuration based on environment name
func (c *RequirementsConfig) Environment(name string) (*EnvironmentConfig, error) {
	for _, env := range c.Environments {
		if env.Key == name {
			return &env, nil
		}
	}
	return nil, fmt.Errorf("environment %q not found", name)
}

// ToMap converts this object to a map of maps for use in helm templating
func (c *RequirementsConfig) ToMap() (map[string]interface{}, error) {
	m, err := util.ToObjectMap(c)
	if m != nil {
		ensureHasFields(m, "provider", "project", "environmentGitOwner", "gitops", "webhook")
	}
	return m, err
}

// IsCloudProvider returns true if the kubenretes provider is a cloud
func (c *RequirementsConfig) IsCloudProvider() bool {
	p := c.Cluster.Provider
	return p == cloud.GKE || p == cloud.AKS || p == cloud.AWS || p == cloud.EKS || p == cloud.ALIBABA
}

func ensureHasFields(m map[string]interface{}, keys ...string) {
	for _, k := range keys {
		_, ok := m[k]
		if !ok {
			m[k] = ""
		}
	}
}

// MissingRequirement returns an error if there is a missing property in the requirements
func MissingRequirement(property string, fileName string) error {
	return fmt.Errorf("missing property: %s in file %s", property, fileName)
}

// IsLazyCreateSecrets returns a boolean whether secrets should be lazily created
func (c *RequirementsConfig) IsLazyCreateSecrets(flag string) (bool, error) {
	if flag != "" {
		if flag == "true" {
			return true, nil
		} else if flag == "false" {
			return false, nil
		} else {
			return false, util.InvalidOption("lazy-create", flag, []string{"true", "false"})
		}
	} else {
		// lets default from the requirements
		if !c.Terraform {
			return true, nil
		}
	}
	// default to false
	return false, nil
}

// addDefaults lets ensure any missing values have good defaults
func (c *RequirementsConfig) addDefaults() {
	if c.Cluster.Namespace == "" {
		c.Cluster.Namespace = "jx"
	}
	if c.Cluster.GitServer == "" {
		c.Cluster.GitServer = "https://github.com"
	}
	if c.Cluster.GitKind == "" {
		c.Cluster.GitKind = "github"
	}
	if c.Cluster.GitName == "" {
		c.Cluster.GitName = c.Cluster.GitKind
	}
	if c.Ingress.NamespaceSubDomain == "" {
		c.Ingress.NamespaceSubDomain = "-" + c.Cluster.Namespace + "."
	}
	if c.Webhook == WebhookTypeNone {
		if c.Cluster.GitServer == "https://github.com" || c.Cluster.GitServer == "https://github.com/" {
			c.Webhook = WebhookTypeProw
		} else {
			// TODO when lighthouse is GA lets default to it
			// c.Webhook = WebhookTypeLighthouse
		}
	}
	if c.Repository == "" {
		c.Repository = "nexus"
	}
}

func (c *RequirementsConfig) handleDeprecation() {
	if c.Vault.Name != "" {
		c.Cluster.VaultName = c.Vault.Name
	} else {
		c.Vault.Name = c.Cluster.VaultName
	}

	if c.Vault.ServiceAccount != "" {
		c.Cluster.VaultSAName = c.Vault.ServiceAccount
	} else {
		c.Vault.ServiceAccount = c.Cluster.VaultSAName
	}
}

// IsAutoDNSDomain returns true if the domain is configured to use an auto DNS sub domain like
// '.nip.io' or '.xip.io'
func (i *IngressConfig) IsAutoDNSDomain() bool {
	for _, suffix := range autoDNSSuffixes {
		if strings.HasSuffix(i.Domain, suffix) {
			return true
		}
	}
	return false
}

// OverrideRequirementsFromEnvironment allows properties to be overridden with environment variables
func (c *RequirementsConfig) OverrideRequirementsFromEnvironment(gcloudFn func() gke.GClouder) {
	//init envconfig struct tags
	err := envconfig.InitWithOptions(&c, envconfig.Options{AllOptional: true})
	if err != nil {
		log.Logger().Errorf("Unable to init envconfig for override requirements: %s", err)
	}

	if "" != os.Getenv(RequirementClusterName) {
		c.Cluster.ClusterName = os.Getenv(RequirementClusterName)
	}
	if "" != os.Getenv(RequirementProject) {
		c.Cluster.ProjectID = os.Getenv(RequirementProject)
	}
	if "" != os.Getenv(RequirementZone) {
		c.Cluster.Zone = os.Getenv(RequirementZone)
	}
	if "" != os.Getenv(RequirementChartRepository) {
		c.Cluster.ChartRepository = os.Getenv(RequirementChartRepository)
	}
	if "" != os.Getenv(RequirementRegistry) {
		c.Cluster.Registry = os.Getenv(RequirementRegistry)
	}
	if "" != os.Getenv(RequirementEnvGitOwner) {
		c.Cluster.EnvironmentGitOwner = os.Getenv(RequirementEnvGitOwner)
	}
	publicEnvRepo, found := os.LookupEnv(RequirementEnvGitPublic)
	if found {
		if envVarBoolean(publicEnvRepo) {
			c.Cluster.EnvironmentGitPublic = true
		} else {
			c.Cluster.EnvironmentGitPublic = false
		}
	}
	publicAppRepo, found := os.LookupEnv(RequirementGitPublic)
	if found {
		if envVarBoolean(publicAppRepo) {
			c.Cluster.GitPublic = true
		} else {
			c.Cluster.GitPublic = false
		}
	}
	if "" != os.Getenv(RequirementExternalDNSServiceAccountName) {
		c.Cluster.ExternalDNSSAName = os.Getenv(RequirementExternalDNSServiceAccountName)
	}
	if "" != os.Getenv(RequirementVaultName) {
		c.Vault.Name = os.Getenv(RequirementVaultName)
	}
	if "" != os.Getenv(RequirementVaultServiceAccountName) {
		c.Vault.ServiceAccount = os.Getenv(RequirementVaultServiceAccountName)
	}
	if "" != os.Getenv(RequirementVaultKeyringName) {
		c.Vault.Keyring = os.Getenv(RequirementVaultKeyringName)
	}
	if "" != os.Getenv(RequirementVaultKeyName) {
		c.Vault.Key = os.Getenv(RequirementVaultKeyName)
	}
	if "" != os.Getenv(RequirementVaultBucketName) {
		c.Vault.Bucket = os.Getenv(RequirementVaultBucketName)
	}
	if "" != os.Getenv(RequirementVaultRecreateBucket) {
		recreate := os.Getenv(RequirementVaultRecreateBucket)
		if envVarBoolean(recreate) {
			c.Vault.RecreateBucket = true
		} else {
			c.Vault.RecreateBucket = false
		}
	}
	if "" != os.Getenv(RequirementVeleroServiceAccountName) {
		c.Velero.ServiceAccount = os.Getenv(RequirementVeleroServiceAccountName)
	}
	if "" != os.Getenv(RequirementVaultDisableURLDiscovery) {
		disable := os.Getenv(RequirementVaultDisableURLDiscovery)
		if envVarBoolean(disable) {
			c.Vault.DisableURLDiscovery = true
		} else {
			c.Vault.DisableURLDiscovery = false
		}
	}
	if "" != os.Getenv(RequirementSecretStorageType) {
		c.SecretStorage = SecretStorageType(os.Getenv(RequirementSecretStorageType))
	}
	if "" != os.Getenv(RequirementKanikoServiceAccountName) {
		c.Cluster.KanikoSAName = os.Getenv(RequirementKanikoServiceAccountName)
	}
	if "" != os.Getenv(RequirementDomainIssuerURL) {
		c.Ingress.DomainIssuerURL = os.Getenv(RequirementDomainIssuerURL)
	}
	if "" != os.Getenv(RequirementIngressTLSProduction) {
		useProduction := os.Getenv(RequirementIngressTLSProduction)
		if envVarBoolean(useProduction) {
			c.Ingress.TLS.Production = true
			for _, e := range c.Environments {
				e.Ingress.TLS.Production = true
			}
		} else {
			c.Ingress.TLS.Production = false
			for _, e := range c.Environments {
				e.Ingress.TLS.Production = false
			}
		}
	}
	if "" != os.Getenv(RequirementKaniko) {
		kaniko := os.Getenv(RequirementKaniko)
		if envVarBoolean(kaniko) {
			c.Kaniko = true
		}
	}
	if "" != os.Getenv(RequirementRepository) {
		repositoryString := os.Getenv(RequirementRepository)
		c.Repository = RepositoryType(repositoryString)
	}
	if "" != os.Getenv(RequirementWebhook) {
		webhookString := os.Getenv(RequirementWebhook)
		c.Webhook = WebhookType(webhookString)
	}
	if "" != os.Getenv(RequirementStorageBackupEnabled) {
		storageBackup := os.Getenv(RequirementStorageBackupEnabled)
		if envVarBoolean(storageBackup) && "" != os.Getenv(RequirementStorageBackupURL) {
			c.Storage.Backup.Enabled = true
			c.Storage.Backup.URL = os.Getenv(RequirementStorageBackupURL)
		}
	}
	if "" != os.Getenv(RequirementStorageLogsEnabled) {
		storageLogs := os.Getenv(RequirementStorageLogsEnabled)
		if envVarBoolean(storageLogs) && "" != os.Getenv(RequirementStorageLogsURL) {
			c.Storage.Logs.Enabled = true
			c.Storage.Logs.URL = os.Getenv(RequirementStorageLogsURL)
		}
	}
	if "" != os.Getenv(RequirementStorageReportsEnabled) {
		storageReports := os.Getenv(RequirementStorageReportsEnabled)
		if envVarBoolean(storageReports) && "" != os.Getenv(RequirementStorageReportsURL) {
			c.Storage.Reports.Enabled = true
			c.Storage.Reports.URL = os.Getenv(RequirementStorageReportsURL)
		}
	}
	if "" != os.Getenv(RequirementStorageRepositoryEnabled) {
		storageRepository := os.Getenv(RequirementStorageRepositoryEnabled)
		if envVarBoolean(storageRepository) && "" != os.Getenv(RequirementStorageRepositoryURL) {
			c.Storage.Repository.Enabled = true
			c.Storage.Repository.URL = os.Getenv(RequirementStorageRepositoryURL)
		}
	}
	// GKE specific c
	if "" != os.Getenv(RequirementGkeProjectNumber) {
		if c.Cluster.GKEConfig == nil {
			c.Cluster.GKEConfig = &GKEConfig{}
		}

		c.Cluster.GKEConfig.ProjectNumber = os.Getenv(RequirementGkeProjectNumber)
	}
	githubApp, found := os.LookupEnv(RequirementGitAppEnabled)
	if found {
		if c.GithubApp == nil {
			c.GithubApp = &GithubAppConfig{}
		}
		if envVarBoolean(githubApp) {
			c.GithubApp.Enabled = true
		} else {
			c.GithubApp.Enabled = false
		}
	}
	if "" != os.Getenv(RequirementGitAppURL) {
		if c.GithubApp == nil {
			c.GithubApp = &GithubAppConfig{}
		}
		c.GithubApp.URL = os.Getenv(RequirementGitAppURL)
	}
	if "" != os.Getenv(RequirementDevEnvApprovers) {
		rawApprovers := os.Getenv(RequirementDevEnvApprovers)
		for _, approver := range strings.Split(rawApprovers, ",") {
			c.Cluster.DevEnvApprovers = append(c.Cluster.DevEnvApprovers, strings.TrimSpace(approver))
		}
	}
	// set this if its not currently configured
	if c.Cluster.Provider == "gke" {
		if c.Cluster.GKEConfig == nil {
			c.Cluster.GKEConfig = &GKEConfig{}
		}

		if c.Cluster.GKEConfig.ProjectNumber == "" {
			if gcloudFn != nil {
				gcloud := gcloudFn()
				if gcloud != nil {
					projectNumber, err := gcloud.GetProjectNumber(c.Cluster.ProjectID)
					if err != nil {
						log.Logger().Warnf("unable to determine gke project number - %s", err)
					}
					c.Cluster.GKEConfig.ProjectNumber = projectNumber
				}
			}
		}
	}

	if "" != os.Getenv(RequirementVersionsGitRef) {
		c.VersionStream.Ref = os.Getenv(RequirementVersionsGitRef)
	}

}

func envVarBoolean(value string) bool {
	return value == "true" || value == "yes"
}
