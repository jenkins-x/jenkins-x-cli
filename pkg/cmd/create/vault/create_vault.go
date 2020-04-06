package vault

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cloud"
	gkevault "github.com/jenkins-x/jx/pkg/cloud/gke/vault"
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/upgrade"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	kubevault "github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/jenkins-x/jx/pkg/vault/create"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	createVaultLong = templates.LongDesc(`
		Creates a Vault using the vault-operator

        The necessary flags depends on the provider of the kubernetes cluster. 
`)

	createVaultExample = templates.Examples(`
		# Create a new vault  with name my-vault
		jx create vault my-vault

		# Create a new vault with name my-vault in namespace my-vault-namespace
		jx create vault my-vault -n my-vault-namespace
	`)
)

// CreateVaultOptions the options for the create vault command
type CreateVaultOptions struct {
	options.CreateOptions

	GKECreateVaultOptions
	AWSCreateVaultOptions
	ClusterName         string
	Namespace           string
	SecretsPathPrefix   string
	RecreateVaultBucket bool
	NoExposeVault       bool
	BucketName          string
	KeyringName         string
	KeyName             string
	ServiceAccountName  string

	IngressConfig kube.IngressConfig
}

// GKECreateVaultOptions the options for vault on GKE
type GKECreateVaultOptions struct {
	GKEProjectID string
	GKEZone      string
}

// AWSCreateVaultOptions are the AWS specific Vault creation options
type AWSCreateVaultOptions struct {
	kubevault.AWSConfig
	AWSTemplatesDir string
	Boot            bool
}

// NewCmdCreateVault  creates a command object for the "create" command
func NewCmdCreateVault(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateVaultOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
		IngressConfig: kube.IngressConfig{},
	}

	cmd := &cobra.Command{
		Use:     "vault",
		Short:   "Create a new Vault using the vault-operator",
		Long:    createVaultLong,
		Example: createVaultExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	// GKE flags
	cmd.Flags().StringVarP(&options.GKEProjectID, "gke-project-id", "", "", "Google Project ID to use for Vault backend")
	cmd.Flags().StringVarP(&options.GKEZone, "gke-zone", "", "", "The zone (e.g. us-central1-a) where Vault will store the encrypted data")

	AwsCreateVaultOptions(cmd, &options.AWSConfig)

	cmd.Flags().StringVarP(&options.ClusterName, "cluster-name", "", "", "Name of the cluster to install vault")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "Namespace where the Vault is created")
	cmd.Flags().StringVarP(&options.SecretsPathPrefix, "secrets-path-prefix", "p", vault.DefaultSecretsPathPrefix, "Path prefix for secrets used for access control config")
	cmd.Flags().BoolVarP(&options.RecreateVaultBucket, "recreate", "", true, "If the bucket already exists delete it so its created empty for the vault")
	cmd.Flags().BoolVarP(&options.NoExposeVault, "no-expose", "", false, "If enabled disable the exposing of the vault")
	cmd.Flags().StringVarP(&options.BucketName, "bucket-name", "", "", "Specify the bucket name. If empty then the bucket name will be based on the vault name")
	cmd.Flags().StringVarP(&options.KeyringName, "keyring-name", "", "", "Specify the KMS Keyring name. If empty then the keyring name will be based on the vault name")
	cmd.Flags().StringVarP(&options.KeyName, "key-name", "", "", "Specify the KMS Key name. If empty then the key name will be based on the vault & keyring name")
	cmd.Flags().StringVarP(&options.ServiceAccountName, "service-account-name", "", "", "Specify Service Account name used. If empty then the service account name will be based on the vault name")

	return cmd
}

func AwsCreateVaultOptions(cmd *cobra.Command, options *kubevault.AWSConfig) {
	// AWS flags
	cmd.Flags().BoolVarP(&options.AutoCreate, "aws-auto-create", "", false, "Whether to skip creating resource prerequisites automatically")
	cmd.Flags().StringVarP(&options.DynamoDBRegion, "aws-dynamodb-region", "", "", "The region to use for storing values in AWS DynamoDB")
	cmd.Flags().StringVarP(&options.DynamoDBTable, "aws-dynamodb-table", "", "vault-data", "The table in AWS DynamoDB to use for storing values")
	cmd.Flags().StringVarP(&options.KMSRegion, "aws-kms-region", "", "", "The region of the AWS KMS key to encrypt values")
	cmd.Flags().StringVarP(&options.KMSKeyID, "aws-kms-key-id", "", "", "The ID or ARN of the AWS KMS key to encrypt values")
	cmd.Flags().StringVarP(&options.S3Bucket, "aws-s3-bucket", "", "", "The name of the AWS S3 bucket to store values in")
	cmd.Flags().StringVarP(&options.S3Prefix, "aws-s3-prefix", "", "vault-operator", "The prefix to use for storing values in AWS S3")
	cmd.Flags().StringVarP(&options.S3Region, "aws-s3-region", "", "", "The region to use for storing values in AWS S3")
	cmd.Flags().StringVarP(&options.AccessKeyID, "aws-access-key-id", "", "", "Access key id of service account to be used by vault")
	cmd.Flags().StringVarP(&options.SecretAccessKey, "aws-secret-access-key", "", "", "Secret access key of service account to be used by vault")
}

// Run implements the command
func (o *CreateVaultOptions) Run() error {
	vaultName, err := o.vaultName()
	if err != nil {
		return err
	}

	kubeClient, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return errors.Wrap(err, "creating kubernetes client")
	}

	if o.Namespace == "" {
		o.Namespace = currentNamespace
	}

	kubeProvider, err := o.kubeProvider(kubeClient, currentNamespace)
	if err != nil {
		return err
	}

	clusterName, err := o.clusterName()
	if err != nil {
		return err
	}

	vaultOperatorClient, err := o.VaultOperatorClient()
	if err != nil {
		return errors.Wrap(err, "creating vault operator client")
	}

	// Checks if the vault already exists
	found := kubevault.FindVault(vaultOperatorClient, vaultName, o.Namespace)
	if found {
		return fmt.Errorf("vault with name '%s' already exists in namespace '%s'", vaultName, o.Namespace)
	}

	err = kube.EnsureNamespaceCreated(kubeClient, o.Namespace, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure that provided namespace '%s' is created", o.Namespace)
	}

	resolver, err := o.CreateVersionResolver("", "")
	if err != nil {
		return errors.Wrap(err, "creating the docker image version resolver")
	}

	vaultCreateParam := create.VaultCreationParam{
		VaultName:            vaultName,
		Namespace:            o.Namespace,
		ClusterName:          clusterName,
		ServiceAccountName:   o.ServiceAccountName,
		SecretsPathPrefix:    o.SecretsPathPrefix,
		KubeProvider:         kubeProvider,
		KubeClient:           kubeClient,
		VaultOperatorClient:  vaultOperatorClient,
		VersionResolver:      *resolver,
		FileHandles:          o.GetIOFileHandles(),
		CreateCloudResources: true,
		Boot:                 false,
		BatchMode:            o.BatchMode,
	}

	if kubeProvider == cloud.GKE {
		gkeParam := &create.GKEParam{
			ProjectID:      gkevault.GetGoogleProjectID(kubeClient, currentNamespace),
			Zone:           gkevault.GetGoogleZone(kubeClient, currentNamespace),
			BucketName:     o.BucketName,
			KeyringName:    o.KeyringName,
			KeyName:        o.KeyName,
			RecreateBucket: o.RecreateVaultBucket,
		}
		vaultCreateParam.GKE = gkeParam
	}

	if kubeProvider == cloud.AWS {
		awsParam := &create.AWSParam{
			IAMUsername:     o.ProvidedIAMUsername,
			S3Bucket:        o.S3Bucket,
			S3Region:        o.S3Region,
			S3Prefix:        o.S3Prefix,
			TemplatesDir:    o.AWSTemplatesDir,
			DynamoDBTable:   o.DynamoDBTable,
			DynamoDBRegion:  o.DynamoDBRegion,
			KMSKeyID:        o.KMSKeyID,
			KMSRegion:       o.KMSRegion,
			AccessKeyID:     o.AccessKeyID,
			SecretAccessKey: o.SecretAccessKey,
			AutoCreate:      o.AutoCreate,
		}
		vaultCreateParam.AWS = awsParam
	}

	vaultCreator := create.NewVaultCreator()
	err = vaultCreator.CreateOrUpdateVault(vaultCreateParam)
	if err != nil {
		return errors.Wrap(err, "unable to create/update Vault")
	}

	if o.NoExposeVault {
		log.Logger().Debugf("Not exposing vault '%s' since --no-expose=%t", vaultName, o.NoExposeVault)
		return nil
	}
	log.Logger().Infof("Exposing Vault...")
	err = o.exposeVault(vaultName)
	if err != nil {
		return errors.Wrap(err, "exposing vault")
	}
	log.Logger().Infof("Vault '%s' exposed", util.ColorInfo(vaultName))
	return nil
}

func (o *CreateVaultOptions) vaultName() (string, error) {
	var vaultName string
	if len(o.Args) == 1 {
		vaultName = o.Args[0]
	} else if o.BatchMode {
		return "", fmt.Errorf("missing vault name")
	} else {
		// Prompt the user for the vault name
		vaultName, _ = util.PickValue("Vault name:", "", true, "The name of the vault that will be created", o.GetIOFileHandles())
	}
	return vaultName, nil
}

func (o *CreateVaultOptions) kubeProvider(kubeClient kubernetes.Interface, currentNamespace string) (string, error) {
	devNamespace, _, err := kube.GetDevNamespace(kubeClient, currentNamespace)
	data, err := kube.ReadInstallValues(kubeClient, devNamespace)
	kubeProvider := data[kube.KubeProvider]
	if kubeProvider == "" {
		return "", errors.New("unable to determine Kube provider")
	}

	if kubeProvider != cloud.GKE && kubeProvider != cloud.AWS && kubeProvider != cloud.EKS {
		return "", errors.Wrapf(err, "this command only supports the Kubernetes providers %s, %s and %s", cloud.GKE, cloud.AWS, cloud.EKS)
	}
	return kubeProvider, nil
}

func (o *CreateVaultOptions) clusterName() (string, error) {
	var err error
	clusterName := o.ClusterName
	if clusterName == "" {
		clusterName, err = cluster.ShortName(o.Kube())
		if err != nil {
			return "", errors.Wrap(err, "unable to determine the cluster name")
		}
	}
	return clusterName, nil
}

func (o *CreateVaultOptions) exposeVault(vaultService string) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	svc, err := client.CoreV1().Services(o.Namespace).Get(vaultService, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "getting the vault service: %s", vaultService)
	}
	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	if svc.Annotations[kube.AnnotationExpose] == "" {
		svc.Annotations[kube.AnnotationExpose] = "true"
		svc.Annotations[kube.AnnotationExposePort] = vault.DefaultVaultPort
		svc, err = client.CoreV1().Services(o.Namespace).Update(svc)
		if err != nil {
			return errors.Wrapf(err, "updating %s service annotations", vaultService)
		}
	}

	upgradeIngOpts := &upgrade.UpgradeIngressOptions{
		CommonOptions:       o.CommonOptions,
		Namespaces:          []string{o.Namespace},
		Services:            []string{vaultService},
		IngressConfig:       o.IngressConfig,
		SkipResourcesUpdate: true,
		WaitForCerts:        true,
	}
	return upgradeIngOpts.Run()
}
