package boot

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx/pkg/cloud"
	gkevault "github.com/jenkins-x/jx/pkg/cloud/gke/vault"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/kube"
	kubevault "github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	pkgvault "github.com/jenkins-x/jx/pkg/vault"
	"github.com/jenkins-x/jx/pkg/vault/create"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/helm/pkg/chartutil"
)

// StepBootVaultOptions contains the command line flags
type StepBootVaultOptions struct {
	*opts.CommonOptions
	Dir               string
	ProviderValuesDir string
	Namespace         string
}

var (
	stepBootVaultLong = templates.LongDesc(`
		This step boots up Vault in the current cluster if its enabled in the 'jx-requirements.yml' file and is not already installed.

		This step is intended to be used in the Jenkins X Boot Pipeline: https://jenkins-x.io/getting-started/boot/
`)

	stepBootVaultExample = templates.Examples(`
		# boots up Vault if its required
		jx step boot vault
`)
)

// NewCmdStepBootVault creates the command
func NewCmdStepBootVault(commonOpts *opts.CommonOptions) *cobra.Command {
	o := StepBootVaultOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "vault",
		Short:   "This step boots up Vault in the current cluster if its enabled in the 'jx-requirements.yml' file and is not already installed",
		Long:    stepBootVaultLong,
		Example: stepBootVaultExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", ".", fmt.Sprintf("the directory to look for the requirements file: %s", config.RequirementsConfigFileName))
	cmd.Flags().StringVarP(&o.ProviderValuesDir, "provider-values-dir", "", "", "The optional directory of kubernetes provider specific files")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "", "", "the namespace that Jenkins X will be booted into. If not specified it defaults to $DEPLOY_NAMESPACE")

	return cmd
}

// Run runs the command
func (o *StepBootVaultOptions) Run() error {
	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}

	requirements, _, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}

	info := util.ColorInfo
	if requirements.SecretStorage != config.SecretStorageTypeVault {
		log.Logger().Infof("Not attempting to boot Vault as using secret storage: %s\n", info(string(requirements.SecretStorage)))
		return nil
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrapf(err, "failed to create kubernetes client")
	}

	if requirements.Vault.Name == "" {
		requirements.Vault.Name = kubevault.SystemVaultNameForCluster(requirements.Cluster.ClusterName)
	}
	log.Logger().Debugf("Using vault name '%s'", requirements.Vault.Name)

	err = o.installOperator(requirements, ns)
	if err != nil {
		return errors.Wrapf(err, "unable to install Vault operator")
	}

	_, err = o.verifyVaultIngress(requirements, kubeClient, ns, requirements.Vault.Name)
	if err != nil {
		return err
	}

	err = o.storeSystemVaultName(kubeClient, requirements.Vault.Name, ns)
	if err != nil {
		return err
	}

	vaultOperatorClient, err := o.VaultOperatorClient()
	if err != nil {
		return errors.Wrap(err, "creating vault operator client")
	}

	resolver, err := o.CreateVersionResolver(requirements.VersionStream.URL, requirements.VersionStream.Ref)
	if err != nil {
		return errors.Wrap(err, "unable to create version stream resolver")
	}

	provider := requirements.Cluster.Provider

	// only allow to make changes to cloud resources when running locally via `jx boot`
	// when run in pipeline, the pipeline SA does not have permissions to create buckets, etc
	// the assumption is that when the code runs in the pipeline all cloud resources already exist
	// (`jx boot` has been executed once at least)
	createCloudResources := o.IsJXBoot()

	vaultCreateParam := create.VaultCreationParam{
		VaultName:            requirements.Vault.Name,
		Namespace:            ns,
		ClusterName:          requirements.Cluster.ClusterName,
		ServiceAccountName:   requirements.Vault.ServiceAccount,
		SecretsPathPrefix:    pkgvault.DefaultSecretsPathPrefix,
		KubeProvider:         provider,
		KubeClient:           kubeClient,
		VaultOperatorClient:  vaultOperatorClient,
		VersionResolver:      *resolver,
		FileHandles:          o.GetIOFileHandles(),
		CreateCloudResources: createCloudResources,
		Boot:                 true,
		BatchMode:            true,
	}

	if provider == cloud.GKE {
		gkeParam := &create.GKEParam{
			ProjectID:      gkevault.GetGoogleProjectID(kubeClient, ns),
			Zone:           gkevault.GetGoogleZone(kubeClient, ns),
			BucketName:     requirements.Vault.Bucket,
			KeyringName:    requirements.Vault.Keyring,
			KeyName:        requirements.Vault.Key,
			RecreateBucket: requirements.Vault.RecreateBucket,
		}
		vaultCreateParam.GKE = gkeParam
	}

	if provider == cloud.EKS {
		awsParam, err := o.createAWSParam(requirements)
		if err != nil {
			return errors.Wrap(err, "unable to create Vault creation parameter from requirements")
		}
		vaultCreateParam.AWS = &awsParam
	}

	vaultCreator := create.NewVaultCreator()
	err = vaultCreator.CreateOrUpdateVault(vaultCreateParam)
	if err != nil {
		return errors.Wrap(err, "unable to create/update Vault")
	}

	return nil
}

func (o *StepBootVaultOptions) createAWSParam(requirements *config.RequirementsConfig) (create.AWSParam, error) {
	if requirements.Vault.AWSConfig == nil {
		return create.AWSParam{}, errors.New("missing AWS configuration for Vault in requirements")
	}

	awsConfig := requirements.Vault.AWSConfig
	secretAccessKey := os.Getenv("VAULT_AWS_SECRET_ACCESS_KEY")
	accessKeyID := os.Getenv("VAULT_AWS_ACCESS_KEY_ID")
	if !awsConfig.AutoCreate && (checkRequiredResource("dynamoDBTable", awsConfig.DynamoDBTable) ||
		checkRequiredResource("secretAccessKey", secretAccessKey) ||
		checkRequiredResource("accessKeyID", accessKeyID) ||
		checkRequiredResource("kmsKeyId", awsConfig.KMSKeyID) ||
		checkRequiredResource("s3Bucket", awsConfig.S3Bucket)) {
		log.Logger().Info("Some of the required provided values are empty - We will create all resources")
		awsConfig.AutoCreate = true
	}

	templatesDir := filepath.Join(o.Dir, o.ProviderValuesDir, cloud.EKS, "templates")

	defaultRegion := requirements.Cluster.Region
	if defaultRegion == "" {
		return create.AWSParam{}, errors.New("unable to find cluster region in requirements")
	}

	dynamoDBRegion := awsConfig.DynamoDBRegion
	if dynamoDBRegion == "" {
		dynamoDBRegion = defaultRegion
		log.Logger().Infof("Region not specified for DynamoDB, defaulting to %s", util.ColorInfo(defaultRegion))
	}

	kmsRegion := awsConfig.KMSRegion
	if kmsRegion == "" {
		kmsRegion = defaultRegion
		log.Logger().Infof("Region not specified for KMS, defaulting to %s", util.ColorInfo(defaultRegion))

	}

	s3Region := awsConfig.S3Region
	if s3Region == "" {
		s3Region = defaultRegion
		log.Logger().Infof("Region not specified for S3, defaulting to %s", util.ColorInfo(defaultRegion))
	}

	awsParam := create.AWSParam{
		IAMUsername:     awsConfig.ProvidedIAMUsername,
		S3Bucket:        awsConfig.S3Bucket,
		S3Region:        s3Region,
		S3Prefix:        awsConfig.S3Prefix,
		TemplatesDir:    templatesDir,
		DynamoDBTable:   awsConfig.DynamoDBTable,
		DynamoDBRegion:  dynamoDBRegion,
		KMSKeyID:        awsConfig.KMSKeyID,
		KMSRegion:       kmsRegion,
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
		AutoCreate:      awsConfig.AutoCreate,
	}

	return awsParam, nil
}

func (o *StepBootVaultOptions) storeSystemVaultName(kubeClient kubernetes.Interface, vaultName string, ns string) error {
	_, err := kube.DefaultModifyConfigMap(kubeClient, ns, kube.ConfigMapNameJXInstallConfig,
		func(configMap *corev1.ConfigMap) error {
			configMap.Data[kube.SystemVaultName] = vaultName
			configMap.Data[secrets.SecretsLocationKey] = string(secrets.VaultLocationKind)
			return nil
		}, nil)
	if err != nil {
		return errors.Wrapf(err, "saving secrets location in ConfigMap %s in namespace %s", kube.ConfigMapNameJXInstallConfig, ns)
	}
	return nil
}

func (o *StepBootVaultOptions) installOperator(requirements *config.RequirementsConfig, ns string) error {
	tag, err := o.vaultOperatorImageTag(&requirements.VersionStream)
	if err != nil {
		return errors.Wrap(err, "unable to determine Vault operator version")
	}

	releaseName := o.ReleaseName
	if releaseName == "" {
		releaseName = kube.DefaultVaultOperatorReleaseName
	}

	values := []string{
		"image.repository=" + kubevault.VaultOperatorImage,
		"image.tag=" + tag,
	}
	log.Logger().Infof("Installing %s operator with helm values: %v", util.ColorInfo(releaseName), util.ColorInfo(values))

	helmOptions := helm.InstallChartOptions{
		Chart:       kube.ChartVaultOperator,
		ReleaseName: releaseName,
		Version:     o.Version,
		Ns:          ns,
		SetValues:   values,
	}
	err = o.InstallChartWithOptions(helmOptions)
	if err != nil {
		return errors.Wrap(err, "unable to install vault operator")
	}

	log.Logger().Infof("Vault operator installed in namespace %s", ns)
	return nil
}

func (o *StepBootVaultOptions) createIngressConfig(requirements *config.RequirementsConfig) (kube.IngressConfig, error) {
	i := requirements.Ingress
	tls := i.TLS
	ic := kube.IngressConfig{
		Domain:  i.Domain,
		Exposer: "Ingress",
		Email:   tls.Email,
		TLS:     tls.Enabled,
	}
	return ic, nil
}

// verifyVaultIngress verifies there is a Vault ingress and if not create one if there is a file at
func (o *StepBootVaultOptions) verifyVaultIngress(requirements *config.RequirementsConfig, kubeClient kubernetes.Interface, ns string, systemVaultName string) (bool, error) {
	fileName := filepath.Join(o.Dir, "vault-ing.tmpl.yaml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check if file exists %s", fileName)
	}
	if !exists {
		log.Logger().Warnf("failed to find file %s\n", fileName)
		return false, nil
	}
	data, err := readYamlTemplate(fileName, requirements)
	if err != nil {
		return true, errors.Wrapf(err, "failed to load vault ingress template file %s", fileName)
	}

	log.Logger().Infof("Applying vault ingress in namespace %s for vault name %s\n", util.ColorInfo(ns), util.ColorInfo(systemVaultName))

	tmpFile, err := ioutil.TempFile("", "vault-ingress-")
	if err != nil {
		return true, errors.Wrapf(err, "failed to create temporary file for vault YAML")
	}

	tmpFileName := tmpFile.Name()
	err = ioutil.WriteFile(tmpFileName, data, util.DefaultWritePermissions)
	if err != nil {
		return true, errors.Wrapf(err, "failed to save vault ingress YAML file %s", tmpFileName)
	}

	args := []string{"apply", "--force", "-f", tmpFileName, "-n", ns}
	err = o.RunCommand("kubectl", args...)
	if err != nil {
		return true, errors.Wrapf(err, "failed to apply vault ingress YAML")
	}
	return true, nil
}

// vaultOperatorImageTag lookups the vault operator image tag in the version stream
func (o *StepBootVaultOptions) vaultOperatorImageTag(versionStream *config.VersionStreamConfig) (string, error) {
	resolver, err := o.CreateVersionResolver(versionStream.URL, versionStream.Ref)
	if err != nil {
		return "", errors.Wrap(err, "creating the vault-operator docker image version resolver")
	}
	fullImage, err := resolver.ResolveDockerImage(kubevault.VaultOperatorImage)
	if err != nil {
		return "", errors.Wrapf(err, "looking up the vault-operator %q image into the version stream",
			kubevault.VaultOperatorImage)
	}
	parts := strings.Split(fullImage, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("no tag found for image %q in version stream", kubevault.VaultOperatorImage)
	}
	return parts[1], nil
}

// readYamlTemplate evaluates the given go template file and returns the output data
func readYamlTemplate(templateFile string, requirements *config.RequirementsConfig) ([]byte, error) {
	_, name := filepath.Split(templateFile)
	funcMap := helm.NewFunctionMap()
	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap).ParseFiles(templateFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse Ingress template: %s", templateFile)
	}

	requirementsMap, err := requirements.ToMap()
	if err != nil {
		return nil, errors.Wrapf(err, "failed turn requirements into a map: %v", requirements)
	}

	templateData := map[string]interface{}{
		"Requirements": chartutil.Values(requirementsMap),
		"Environments": chartutil.Values(requirements.EnvironmentMap()),
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute Ingress template: %s", templateFile)
	}
	data := buf.Bytes()
	return data, nil
}

func checkRequiredResource(resourceName string, value string) bool {
	if value == "" {
		log.Logger().Warnf("Vault.AutoCreate is false but required property %s is missing", resourceName)
		return true
	}
	return false
}
