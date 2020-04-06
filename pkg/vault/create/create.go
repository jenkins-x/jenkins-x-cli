package create

import (
	"fmt"
	"time"

	"github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"
	awsvault "github.com/jenkins-x/jx/pkg/cloud/amazon/vault"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	gkevault "github.com/jenkins-x/jx/pkg/cloud/gke/vault"
	"github.com/jenkins-x/jx/pkg/kube/serviceaccount"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"k8s.io/client-go/kubernetes"
)

const (
	autoCreateTableName = "vault-data"
)

// VaultCreationParam encapsulates the parameters needed to create a Vault instance.
type VaultCreationParam struct {
	VaultName            string
	ClusterName          string
	Namespace            string
	ServiceAccountName   string
	KubeProvider         string
	SecretsPathPrefix    string
	CreateCloudResources bool
	Boot                 bool
	BatchMode            bool
	VaultOperatorClient  versioned.Interface
	KubeClient           kubernetes.Interface
	VersionResolver      versionstream.VersionResolver
	FileHandles          util.IOFileHandles
	GKE                  *GKEParam
	AWS                  *AWSParam
}

// GKEParam encapsulates the parameters needed to create a Vault instance on GKE.
type GKEParam struct {
	ProjectID      string
	Zone           string
	BucketName     string
	KeyringName    string
	KeyName        string
	RecreateBucket bool
}

// GKEParam encapsulates the parameters needed to create a Vault instance on AWS.
type AWSParam struct {
	IAMUsername     string
	S3Bucket        string
	S3Region        string
	S3Prefix        string
	TemplatesDir    string
	DynamoDBTable   string
	DynamoDBRegion  string
	KMSKeyID        string
	KMSRegion       string
	AccessKeyID     string
	SecretAccessKey string
	AutoCreate      bool
}

// VaultCreator defines the interface to create and update Vault instances
type VaultCreator interface {
	CreateOrUpdateVault(param VaultCreationParam) error
}

type defaultVaultCreator struct {
}

// NewVaultCreator creates an instance of the default VaultCreator.
func NewVaultCreator() VaultCreator {
	return &defaultVaultCreator{}
}

func (p *VaultCreationParam) validate() error {
	var validationErrors []error
	if p.VaultName == "" {
		validationErrors = append(validationErrors, errors.New("the Vault name needs to be provided"))
	}

	if p.Namespace == "" {
		validationErrors = append(validationErrors, errors.New("the namespace to create the Vault instance into needs to be provided"))
	}

	if p.KubeClient == nil {
		validationErrors = append(validationErrors, errors.New("a kube client needs to be provided"))
	}

	if p.VaultOperatorClient == nil {
		validationErrors = append(validationErrors, errors.New("a vault operator client needs to be provided"))
	}

	if p.KubeProvider == "" {
		validationErrors = append(validationErrors, errors.New("a kube/cloud provider needs be provided"))
	}

	if p.KubeProvider == cloud.GKE {
		if p.GKE == nil {
			validationErrors = append(validationErrors, errors.Errorf("%s selected as kube provider, but no %s specific parameters provided", cloud.GKE, cloud.GKE))
		}
		err := p.GKE.validate()
		if err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	if p.KubeProvider == cloud.AWS {
		if p.AWS == nil {
			validationErrors = append(validationErrors, errors.Errorf("%s selected as kube provider, but no %s specific parameters provided", cloud.AWS, cloud.AWS))
		}
		if err := p.AWS.validate(); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	return util.CombineErrors(validationErrors...)
}

func (p *GKEParam) validate() error {
	var validationErrors []error
	if p == nil {
		return nil
	}
	if p.ProjectID == "" {
		validationErrors = append(validationErrors, errors.New("the GKE project ID needs to be provided"))
	}
	if p.Zone == "" {
		validationErrors = append(validationErrors, errors.New("the GKE zone needs to be provided"))
	}
	return util.CombineErrors(validationErrors...)
}

func (p *AWSParam) validate() error {
	var validationErrors []error
	if p == nil {
		return nil
	}
	if p.TemplatesDir == "" {
		validationErrors = append(validationErrors, errors.New("the cloud formation template dir needs to be provided"))
	}
	if p.AccessKeyID == "" {
		validationErrors = append(validationErrors, errors.New("the AccessKeyID needs to be provided"))
	}
	if p.SecretAccessKey == "" {
		validationErrors = append(validationErrors, errors.New("the SecretAccessKey needs to be provided"))
	}
	return util.CombineErrors(validationErrors...)
}

// CreateOrUpdateVault creates or updates a Vault instance in the specified namespace.
func (v *defaultVaultCreator) CreateOrUpdateVault(param VaultCreationParam) error {
	err := param.validate()
	if err != nil {
		return err
	}

	vaultAuthServiceAccount, err := v.createAuthServiceAccount(param.KubeClient, param.VaultName, param.ServiceAccountName, param.Namespace)
	if err != nil {
		return errors.Wrap(err, "creating Vault authentication service account")
	}
	log.Logger().Debugf("Created service account '%s' for Vault authentication", util.ColorInfo(vaultAuthServiceAccount))

	images, err := v.dockerImages(param.VersionResolver)
	if err != nil {
		return errors.Wrap(err, "loading docker images from versions repository")
	}

	vaultCRD, err := vault.NewVaultCRD(param.KubeClient, param.VaultName, param.Namespace, images, vaultAuthServiceAccount, param.Namespace, param.SecretsPathPrefix)

	err = v.setCloudProviderSpecificSettings(vaultCRD, param)
	if err != nil {
		return errors.Wrap(err, "unable to set cloud provider specific Vault configuration")
	}

	err = vault.CreateOrUpdateVault(vaultCRD, param.VaultOperatorClient, param.Namespace)
	if err != nil {
		return errors.Wrap(err, "creating vault")
	}

	// wait for vault service to become ready before finishing the provisioning
	err = services.WaitForService(param.KubeClient, param.VaultName, param.Namespace, 1*time.Minute)
	if err != nil {
		return errors.Wrap(err, "waiting for vault service")
	}

	return nil
}

func (v *defaultVaultCreator) dockerImages(resolver versionstream.VersionResolver) (map[string]string, error) {
	images := map[string]string{
		vault.BankVaultsImage: "",
		vault.VaultImage:      "",
	}

	for image := range images {
		version, err := resolver.ResolveDockerImage(image)
		if err != nil {
			return images, errors.Wrapf(err, "resolving docker image %q", image)
		}
		images[image] = version
	}
	return images, nil
}

// createAuthServiceAccount creates a Service Account for the Auth service for vault
func (v *defaultVaultCreator) createAuthServiceAccount(client kubernetes.Interface, vaultName, serviceAccountName string, namespace string) (string, error) {
	if serviceAccountName == "" {
		serviceAccountName = v.authServiceAccountName(vaultName)
	}

	_, err := serviceaccount.CreateServiceAccount(client, namespace, serviceAccountName)
	if err != nil {
		return "", errors.Wrap(err, "creating vault auth service account")
	}
	return serviceAccountName, nil
}

// authServiceAccountName creates a service account name for a given vault and cluster name
func (v *defaultVaultCreator) authServiceAccountName(vaultName string) string {
	return fmt.Sprintf("%s-%s", vaultName, "auth-sa")
}

func (v *defaultVaultCreator) setCloudProviderSpecificSettings(vaultCRD *v1alpha1.Vault, param VaultCreationParam) error {
	var cloudProviderConfig vault.CloudProviderConfig
	var err error

	if param.CreateCloudResources {
		switch param.KubeProvider {
		case cloud.GKE:
			cloudProviderConfig, err = v.vaultGKEConfig(vaultCRD, param)
		case cloud.AWS, cloud.EKS:
			cloudProviderConfig, err = v.vaultAWSConfig(vaultCRD, param)
		default:
			err = errors.Errorf("vault is not supported on cloud provider %s", param.KubeProvider)
		}
		if err != nil {
			return errors.Wrapf(err, "unable to apply cloud provider config")
		}
	} else {
		log.Logger().Warn("Upgrading Vault CRD from within the pipeline. No changes to the cloud provider specific configuration will occur.")

		existingVaultCRD, err := vault.GetVault(param.VaultOperatorClient, vaultCRD.Name, param.Namespace)
		if err != nil {
			return errors.Wrapf(err, "expected to find existing Vault configuration")
		}

		cloudProviderConfig, err = v.extractCloudProviderConfig(existingVaultCRD)
		if err != nil {
			return errors.Wrapf(err, "unable to extract cloud provider specific configuration from Vault CRD %s", vaultCRD.Name)
		}
	}

	vaultCRD.Spec.Config["storage"] = cloudProviderConfig.Storage
	vaultCRD.Spec.Config["seal"] = cloudProviderConfig.Seal
	vaultCRD.Spec.UnsealConfig = cloudProviderConfig.UnsealConfig
	vaultCRD.Spec.CredentialsConfig = cloudProviderConfig.CredentialsConfig
	return nil
}

func (v *defaultVaultCreator) vaultGKEConfig(vaultCRD *v1alpha1.Vault, param VaultCreationParam) (vault.CloudProviderConfig, error) {
	gcloud := &gke.GCloud{}

	err := gcloud.Login("", true)
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "login into GCP")
	}

	args := []string{"config", "set", "project", param.GKE.ProjectID}
	cmd := util.Command{
		Name: "gcloud",
		Args: args,
	}
	_, err = cmd.RunWithoutRetry()
	if err != nil {
		return vault.CloudProviderConfig{}, err
	}

	log.Logger().Debugf("Ensure KMS API is enabled")
	err = gcloud.EnableAPIs(param.GKE.ProjectID, "cloudkms")
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "unable to enable 'cloudkms' API")
	}

	log.Logger().Debugf("Creating GCP service account for Vault backend")
	gcpServiceAccountSecretName, err := gkevault.CreateVaultGCPServiceAccount(gcloud, param.KubeClient, vaultCRD.Name, param.Namespace, param.ClusterName, param.GKE.ProjectID)
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "creating GCP service account")
	}
	log.Logger().Debugf("'%s' service account created", util.ColorInfo(gcpServiceAccountSecretName))

	log.Logger().Debugf("Setting up GCP KMS configuration")
	kmsConfig, err := gkevault.CreateKmsConfig(gcloud, vaultCRD.Name, param.GKE.KeyringName, param.GKE.KeyName, param.GKE.ProjectID)
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "creating KMS configuration")
	}
	log.Logger().Debugf("KMS Key '%s' created in keying '%s'", util.ColorInfo(kmsConfig.Key), util.ColorInfo(kmsConfig.Keyring))

	vaultBucket, err := gkevault.CreateBucket(gcloud, vaultCRD.Name, param.GKE.BucketName, param.GKE.ProjectID, param.GKE.Zone, param.GKE.RecreateBucket, param.BatchMode, param.FileHandles)
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "creating Vault GCS data bucket")
	}
	log.Logger().Infof("GCS bucket '%s' was created for Vault backend", util.ColorInfo(vaultBucket))

	gcpConfig := &vault.GCPConfig{
		ProjectId:   param.GKE.ProjectID,
		KmsKeyring:  kmsConfig.Keyring,
		KmsKey:      kmsConfig.Key,
		KmsLocation: kmsConfig.Location,
		GcsBucket:   vaultBucket,
	}
	return vault.PrepareGKEVaultCRD(gcpServiceAccountSecretName, gcpConfig)
}

func (v *defaultVaultCreator) vaultAWSConfig(vaultCRD *v1alpha1.Vault, param VaultCreationParam) (vault.CloudProviderConfig, error) {
	_, clusterRegion, err := session.GetCurrentlyConnectedRegionAndClusterName()
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "finding default AWS region")
	}

	v.applyDefaultRegionIfEmpty(param.AWS, clusterRegion)

	if param.AWS.AutoCreate {
		domain := "jenkins-x-domain"
		username := param.AWS.IAMUsername
		if username == "" {
			username = "vault_" + clusterRegion
		}

		bucketName := param.AWS.S3Bucket
		if bucketName == "" {
			bucketName = "vault-unseal." + param.AWS.S3Region + "." + domain
		}

		valueUUID, err := uuid.NewV4()
		if err != nil {
			return vault.CloudProviderConfig{}, errors.Wrapf(err, "Generating UUID failed")
		}

		// Create suffix to apply to resources
		suffixString := valueUUID.String()[:7]
		var kmsID, s3Name, tableName, accessID, secretKey *string
		if param.Boot {
			accessID, secretKey, kmsID, s3Name, tableName, err = awsvault.CreateVaultResourcesBoot(awsvault.ResourceCreationOpts{
				Region:          clusterRegion,
				Domain:          domain,
				Username:        username,
				BucketName:      bucketName,
				TableName:       autoCreateTableName,
				AWSTemplatesDir: param.AWS.TemplatesDir,
				AccessKeyID:     param.AWS.AccessKeyID,
				SecretAccessKey: param.AWS.SecretAccessKey,
				UniqueSuffix:    suffixString,
			})
		} else {
			// left for non-boot clusters until deprecation
			accessID, secretKey, kmsID, s3Name, tableName, err = awsvault.CreateVaultResources(awsvault.ResourceCreationOpts{
				Region:     clusterRegion,
				Domain:     domain,
				Username:   username,
				BucketName: bucketName,
				TableName:  autoCreateTableName,
			})
		}

		if err != nil {
			return vault.CloudProviderConfig{}, errors.Wrap(err, "an error occurred while creating the vaultCRD resources")
		}
		if s3Name != nil {
			param.AWS.S3Bucket = *s3Name
		}
		if kmsID != nil {
			param.AWS.KMSKeyID = *kmsID
		}
		if tableName != nil {
			param.AWS.DynamoDBTable = *tableName
		}
		if accessID != nil {
			param.AWS.AccessKeyID = *accessID
		}
		if secretKey != nil {
			param.AWS.SecretAccessKey = *secretKey
		}

	} else {
		if param.AWS.S3Bucket == "" {
			return vault.CloudProviderConfig{}, fmt.Errorf("missing S3 bucket flag")
		}
		if param.AWS.KMSKeyID == "" {
			return vault.CloudProviderConfig{}, fmt.Errorf("missing AWS KMS key id flag")
		}
		if param.AWS.AccessKeyID == "" {
			return vault.CloudProviderConfig{}, fmt.Errorf("missing AWS access key id flag")
		}
		if param.AWS.SecretAccessKey == "" {
			return vault.CloudProviderConfig{}, fmt.Errorf("missing AWS secret access key flag")
		}
	}

	awsServiceAccountSecretName, err := awsvault.StoreAWSCredentialsIntoSecret(param.KubeClient, param.AWS.AccessKeyID, param.AWS.SecretAccessKey, vaultCRD.Name, param.Namespace)
	if err != nil {
		return vault.CloudProviderConfig{}, errors.Wrap(err, "storing the service account credentials into a secret")
	}

	vaultConfig := vault.AWSConfig{
		AutoCreate:          param.AWS.AutoCreate,
		DynamoDBTable:       param.AWS.DynamoDBTable,
		DynamoDBRegion:      param.AWS.DynamoDBRegion,
		AccessKeyID:         param.AWS.AccessKeyID,
		SecretAccessKey:     param.AWS.SecretAccessKey,
		ProvidedIAMUsername: param.AWS.IAMUsername,
		AWSUnsealConfig: v1alpha1.AWSUnsealConfig{
			KMSKeyID:  param.AWS.KMSKeyID,
			KMSRegion: param.AWS.KMSRegion,
			S3Bucket:  param.AWS.S3Bucket,
			S3Prefix:  param.AWS.S3Prefix,
			S3Region:  param.AWS.S3Region,
		},
	}

	return vault.PrepareAWSVaultCRD(awsServiceAccountSecretName, &vaultConfig)
}

func (v *defaultVaultCreator) extractCloudProviderConfig(vaultCRD *v1alpha1.Vault) (vault.CloudProviderConfig, error) {
	var cloudProviderConfig = vault.CloudProviderConfig{}

	storageConfig := vaultCRD.Spec.Config["storage"]
	if storageConfig == nil {
		return cloudProviderConfig, errors.Errorf("unable to find storage config in Vault CRD %s", vaultCRD.Name)
	}
	storage, ok := storageConfig.(map[string]interface{})
	if !ok {
		return cloudProviderConfig, errors.Errorf("unexpected storage config in Vault CRD %s: %v", vaultCRD.Name, storageConfig)
	}

	sealConfig := vaultCRD.Spec.Config["seal"]
	if sealConfig == nil {
		return cloudProviderConfig, errors.Errorf("unable to find seal config in Vault CRD %s", vaultCRD.Name)
	}
	seal, ok := sealConfig.(map[string]interface{})
	if !ok {
		return cloudProviderConfig, errors.Errorf("unexpected seal config in Vault CRD %s: %v", vaultCRD.Name, sealConfig)
	}

	cloudProviderConfig = vault.CloudProviderConfig{
		Storage:           storage,
		Seal:              seal,
		UnsealConfig:      vaultCRD.Spec.UnsealConfig,
		CredentialsConfig: vaultCRD.Spec.CredentialsConfig,
	}

	return cloudProviderConfig, nil
}

// applyDefaultRegionIfEmpty applies the default region to all AWS resources
func (v *defaultVaultCreator) applyDefaultRegionIfEmpty(awsParam *AWSParam, defaultRegion string) {
	if awsParam.DynamoDBRegion == "" {
		log.Logger().Infof("DynamoDBRegion not specified, defaulting to %s", util.ColorInfo(defaultRegion))
		if awsParam.DynamoDBRegion == "" {
			awsParam.DynamoDBRegion = defaultRegion
		}
	}

	if awsParam.KMSRegion == "" {
		log.Logger().Infof("KMSRegion not specified, defaulting to %s", util.ColorInfo(defaultRegion))
		if awsParam.KMSRegion == "" {
			awsParam.KMSRegion = defaultRegion
		}
	}

	if awsParam.S3Region == "" {
		log.Logger().Infof("S3Region not specified, defaulting to %s", util.ColorInfo(defaultRegion))
		if awsParam.S3Region == "" {
			awsParam.S3Region = defaultRegion
		}
	}
}
