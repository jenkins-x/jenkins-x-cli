package create

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/kube/cluster"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/aks"
	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cloud/iks"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/secreturl"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/surveyutils"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

var (
	optionSecretsScheme    = "secrets-scheme"
	localSecretsSecretName = "local-param-secrets" //#nosec
)

var (
	createValuesLong = templates.LongDesc(`
		Creates a values.yaml from a schema
`)

	createValuesExample = templates.Examples(`
		# create the values.yaml file from values.schema.json in the current directory
		jx step create values

		# create the values.yaml file from values.schema.json in the /path/to/values directory
		jx step create values -d /path/to/values

		# create the cheese.yaml file from cheese.schema.json in the current directory 
		jx step create values --name cheese
	
			`)
)

// StepCreateValuesOptions contains the command line flags
type StepCreateValuesOptions struct {
	step.StepCreateOptions

	Dir       string
	Namespace string
	Name      string
	BasePath  string

	Schema     string
	ValuesFile string

	SecretsScheme string
}

// StepCreateValuesResults stores the generated results
type StepCreateValuesResults struct {
	Pipeline    *pipelineapi.Pipeline
	Task        *pipelineapi.Task
	PipelineRun *pipelineapi.PipelineRun
}

// NewCmdStepCreateValues Creates a new Command object
func NewCmdStepCreateValues(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreateValuesOptions{
		StepCreateOptions: step.StepCreateOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "values",
		Short:   "Creates the values.yaml file from a schema",
		Long:    createValuesLong,
		Example: createValuesExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "the directory to look for the <kind>.schema.json and write the <kind>.yaml, defaults to the current directory")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "the namespace Jenkins X is installed into. If not specified it defaults to $DEPLOY_NAMESPACE or else defaults to the current kubernetes namespace")
	cmd.Flags().StringVarP(&options.Schema, "schema", "", "", "the path to the schema file, overrides --dir and --name")
	cmd.Flags().StringVarP(&options.Name, "name", "", "values", "the kind of the file to create (and, by default, the schema name)")
	cmd.Flags().StringVarP(&options.BasePath, "secret-base-path", "", "", fmt.Sprintf("the secret path used to store secrets in vault / file system. Typically a unique name per cluster+team. If none is specified we will default it to the cluster name from the %s file in the current or a parent directory.", config.RequirementsConfigFileName))
	cmd.Flags().StringVarP(&options.ValuesFile, "out", "", "", "the path to the file to create, overrides --dir and --name")
	cmd.Flags().StringVarP(&options.SecretsScheme, optionSecretsScheme, "", "", fmt.Sprintf("the scheme to store/reference any secrets in, valid options are vault and local. If none are specified we will default it from the %s file in the current or a parent directory.", config.RequirementsConfigFileName))
	return cmd
}

// Run implements this command
func (o *StepCreateValuesOptions) Run() error {
	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}
	o.SetDevNamespace(ns)

	if o.Dir == "" {
		o.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	// lets default to the install requirements setting
	requirements, fileName, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}
	exists, err := util.FileExists(fileName)
	if err != nil {
		return err
	}
	info := util.ColorInfo
	if o.SecretsScheme == "" {
		if exists {
			o.SecretsScheme = string(requirements.SecretStorage)
			log.Logger().Infof("defaulting to secret storage scheme %s found from requirements file at %s\n", info(o.SecretsScheme), info(fileName))
		} else {
			log.Logger().Warnf("there is no requirements file at %s\n", fileName)
		}
	}
	if o.BasePath == "" {
		if exists {
			o.BasePath = requirements.Cluster.ClusterName
			log.Logger().Infof("defaulting to secret base path to the cluster name %s found from requirements file at %s\n", info(o.BasePath), info(fileName))
		} else {
			log.Logger().Warnf("there is no requirements file at %s\n", fileName)
		}

	}
	if !(o.SecretsScheme == "vault" || o.SecretsScheme == "local") {
		util.InvalidArgf(optionSecretsScheme, "Use one of vault or local")
	}
	if o.Schema == "" {
		o.Schema = filepath.Join(o.Dir, fmt.Sprintf("%s.schema.json", o.Name))
	}

	err = surveyutils.TemplateSchemaFile(o.Schema, requirements)
	if err != nil {
		return errors.Wrapf(err, "failed to generate %s from template", o.Schema)
	}

	if o.ValuesFile == "" {
		o.ValuesFile = filepath.Join(o.Dir, fmt.Sprintf("%s.yaml", o.Name))
	}

	secretURLClient, err := o.GetSecretURLClient(secrets.ToSecretsLocation(o.SecretsScheme))
	if err != nil {
		return err
	}

	err = o.verifyRegistryConfig(requirements, fileName, secretURLClient)
	if err != nil {
		return err
	}

	err = o.CreateValuesFile(secretURLClient)
	if err != nil {
		return errors.WithStack(err)
	}

	err = o.createExternalRegistryValues(requirements, fileName, secretURLClient)
	if err != nil {
		return errors.Wrap(err, "error enabling access to an external docker registry")
	}

	err = o.createLocalSecretFilesSecret(requirements)
	if err != nil {
		return errors.Wrap(err, "error creating the secret for local secret scheme")
	}

	return nil
}

// CreateValuesFile builds the clients and settings from the team needed to run apps.ProcessValues, and then copies the answer
// to the location requested by the command
func (o *StepCreateValuesOptions) CreateValuesFile(secretURLClient secreturl.Client) error {
	schema, err := ioutil.ReadFile(o.Schema)
	if err != nil {
		return errors.Wrapf(err, "reading schema %s", o.Schema)
	}
	gitOpsURL := ""
	gitOps, devEnv := o.GetDevEnv()
	if gitOps {
		gitOpsURL = devEnv.Spec.Source.URL
	}
	teamName, _, err := o.TeamAndEnvironmentNames()
	if err != nil {
		return errors.Wrapf(err, "getting team name")
	}
	existing, err := helm.LoadValuesFile(o.ValuesFile)
	if err != nil {
		return errors.Wrapf(err, "failed to load values file %s", o.ValuesFile)
	}

	valuesFileName, cleanup, err := apps.ProcessValues(schema, o.Name, gitOpsURL, teamName, o.BasePath, o.BatchMode, false, secretURLClient, existing, o.SecretsScheme, o.GetIOFileHandles(), o.Verbose)
	defer cleanup()
	if err != nil {
		return errors.WithStack(err)
	}
	err = util.CopyFile(valuesFileName, o.ValuesFile)
	if err != nil {
		return errors.Wrapf(err, "moving %s to %s", valuesFileName, o.ValuesFile)
	}
	return nil
}

func (o *StepCreateValuesOptions) verifyRegistryConfig(requirements *config.RequirementsConfig, requirementsFileName string, secretClient secreturl.Client) error {
	log.Logger().Debug("Verifying Registry...")
	registry := requirements.Cluster.Registry
	configJSON := ""
	if registry == "" {
		kubeConfig, _, err := o.Kube().LoadConfig()
		if err != nil {
			return errors.Wrapf(err, "failed to load kube config")
		}

		// lets try default the container registry
		switch requirements.Cluster.Provider {
		case cloud.AWS, cloud.EKS:
			registry, err = amazon.GetContainerRegistryHost()
			if err != nil {
				return err
			}
		case cloud.GKE:
			if requirements.Kaniko && requirements.Webhook != config.WebhookTypeJenkins {
				registry = "gcr.io"
			}
		case cloud.AKS:
			server := kube.CurrentServer(kubeConfig)
			azureCLI := aks.NewAzureRunner()
			resourceGroup, name, cluster, err := azureCLI.GetClusterClient(server)
			if err != nil {
				return errors.Wrap(err, "getting cluster from Azure")
			}
			registryID := ""
			if requirements.Cluster.AzureConfig == nil {
				return errors.New("no azure registry subscription specified in 'jx-requirements.yml' at cluster.azure")
			}
			azureRegistrySubscription := requirements.Cluster.AzureConfig.RegistrySubscription
			if azureRegistrySubscription == "" {
				log.Logger().Info("no azure registry subscription is specified in 'jx-requirements.yml' at cluster.azure.registrySubscription")
			}
			configJSON, registry, registryID, err = azureCLI.GetRegistry(azureRegistrySubscription, resourceGroup, name, registry)
			if err != nil {
				return errors.Wrap(err, "getting registry configuration from Azure")
			}
			azureCLI.AssignRole(cluster, registryID)
			log.Logger().Infof("Assign AKS %s a reader role for ACR %s", util.ColorInfo(server), util.ColorInfo(registry))
		case cloud.IKS:
			client, err := o.KubeClient()
			if err != nil {
				return err
			}
			registry = iks.GetClusterRegistry(client)
			configJSON, err = iks.GetRegistryConfigJSON(registry)
			if err != nil {
				return errors.Wrap(err, "getting IKS registry configuration")
			}

		case cloud.OPENSHIFT:
			registry = "docker-registry.default.svc:5000"
			_, err := o.enableOpenShiftRegistryPermissions(requirements.Cluster.Namespace, registry)
			if err != nil {
				return errors.Wrap(err, "enabling OpenShift registry permissions")
			}
		}

		if registry != "" {
			requirements.Cluster.Registry = registry
			err = requirements.SaveConfig(requirementsFileName)
			if err != nil {
				return errors.Wrapf(err, "failed to save changes to file: %s", requirementsFileName)
			}
		}
	}
	if configJSON != "" {
		// TODO update the secret if its changed
		log.Logger().Warn("jx boot does not yet support automatically populating the container registry secrets automatically...")
	}
	return nil
}

func (o *StepCreateValuesOptions) createExternalRegistryValues(requirements *config.RequirementsConfig, requirementsConfigFile string, secretsClient secreturl.Client) error {
	params, err := helm.LoadParameters(o.Dir, secretsClient)
	if err != nil {
		return err
	}

	if val, exists := params["enableDocker"]; exists && val.(bool) {
		dockerConfig := params["docker"].(map[string]interface{})
		requirements.Cluster.Registry = getValidRegistryPushURL(dockerConfig["url"].(string))
		err = requirements.SaveConfig(requirementsConfigFile)
		if err != nil {
			return errors.Wrap(err, "error saving the modified requirements.yml file")
		}
	}

	return nil
}

// Sometimes the push URL is different than the one needed to configure auth, so we need to get the correct ones
// creating this for future registries
func getValidRegistryPushURL(url string) string {
	switch url {
	case "https://index.docker.io/v1/":
		return "docker.io"
	default:
		log.Logger().Warnf("Unexpected registry auth URL %s - using it as registry push URL", url)
		return url
	}
}

func (o *StepCreateValuesOptions) createLocalSecretFilesSecret(requirements *config.RequirementsConfig) error {
	if o.SecretsScheme == "local" && (os.Getenv("OVERRIDE_IN_CLUSTER_CHECK") == "true" || !cluster.IsInCluster()) {
		kubeClient, ns, err := o.KubeClientAndDevNamespace()
		if err != nil {
			return err
		}
		secretFiles, err := getLocalSecretFilesAsMap(requirements)
		if err != nil {
			return errors.Wrap(err, "there was a problem obtaining the local secret files")
		}
		existingSecret, err := kubeClient.CoreV1().Secrets(ns).Get(localSecretsSecretName, metav1.GetOptions{})
		if err != nil {
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: localSecretsSecretName,
				},
				Data: secretFiles,
			}
			_, err = kubeClient.CoreV1().Secrets(ns).Create(secret)
			if err != nil {
				return errors.Wrap(err, "there was a problem creating the local params secret")
			}
			return nil
		}

		existingSecret.Data = secretFiles
		_, err = kubeClient.CoreV1().Secrets(ns).Update(existingSecret)
		if err != nil {
			return errors.Wrap(err, "there was a problem updating the local params secret")
		}
	}
	return nil
}

func getLocalSecretFilesAsMap(requirements *config.RequirementsConfig) (map[string][]byte, error) {
	dir, err := util.LocalFileSystemSecretsDir()
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem obtaining the local file system for secrets")
	}
	fullSecretsPath := filepath.Join(dir, requirements.Cluster.ClusterName)
	exists, err := util.DirExists(fullSecretsPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if full secrets path exists")
	}
	if !exists {
		return nil, errors.Wrapf(err, "secrets path for cluster name %s doesn't exist", requirements.Cluster.ClusterName)
	}
	files, err := ioutil.ReadDir(fullSecretsPath)
	if err != nil {
		return nil, errors.Wrap(err, "error reading secret files from localStorage")
	}
	secretFiles := make(map[string][]byte)
	for _, f := range files {
		bytes, err := ioutil.ReadFile(filepath.Join(fullSecretsPath, f.Name()))
		if err != nil {
			return nil, errors.Wrap(err, "there was a problem reading a local secrets file")
		}
		secretFiles[f.Name()] = bytes
	}
	return secretFiles, nil
}

func (o *StepCreateValuesOptions) enableOpenShiftRegistryPermissions(ns string, registry string) (string, error) {
	log.Logger().Infof("Enabling permissions for OpenShift registry in namespace %s", ns)
	// Open the registry so any authenticated user can pull images from the jx namespace
	err := o.RunCommand("oc", "adm", "policy", "add-role-to-group", "system:image-puller", "system:authenticated", "-n", ns)
	if err != nil {
		return "", err
	}
	err = o.EnsureServiceAccount(ns, "jenkins-x-registry")
	if err != nil {
		return "", err
	}
	err = o.RunCommand("oc", "adm", "policy", "add-cluster-role-to-user", "registry-admin", "system:serviceaccount:"+ns+":jenkins-x-registry")
	if err != nil {
		return "", err
	}
	registryToken, err := o.GetCommandOutput("", "oc", "serviceaccounts", "get-token", "jenkins-x-registry", "-n", ns)
	if err != nil {
		return "", err
	}
	return `{"auths": {"` + registry + `": {"auth": "` + base64.StdEncoding.EncodeToString([]byte("serviceaccount:"+registryToken)) + `"}}}`, nil
}
