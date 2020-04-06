package upgrade

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/config"
	configio "github.com/jenkins-x/jx/pkg/io"
	"github.com/jenkins-x/jx/pkg/platform"
	survey "gopkg.in/AlecAivazis/survey.v1"
	"sigs.k8s.io/yaml"

	"io/ioutil"
	"strings"

	"github.com/jenkins-x/jx/pkg/kube"

	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	core_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	upgrade_platform_long = templates.LongDesc(`
		Upgrades the Jenkins X platform if there is a newer release
`)

	upgrade_platform_example = templates.Examples(`
		# Upgrades the Jenkins X platform 
		jx upgrade platform
	`)
)

// UpgradePlatformOptions the options for the create spring command
type UpgradePlatformOptions struct {
	create.InstallOptions

	Version       string
	ReleaseName   string
	Chart         string
	Namespace     string
	Set           string
	AlwaysUpgrade bool
	UpdateSecrets bool

	InstallFlags create.InstallFlags
}

// NewCmdUpgradePlatform defines the command
func NewCmdUpgradePlatform(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradePlatformOptions{
		InstallOptions: create.InstallOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "platform",
		Short:   "Upgrades the Jenkins X platform if there is a new release available",
		Aliases: []string{"install"},
		Long:    upgrade_platform_long,
		Example: upgrade_platform_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "The Namespace to promote to.")
	cmd.Flags().StringVarP(&options.ReleaseName, "name", "n", platform.JenkinsXPlatformRelease, "The release name.")
	cmd.Flags().StringVarP(&options.Chart, "chart", "c", "jenkins-x/jenkins-x-platform", "The Chart to upgrade.")
	cmd.Flags().StringVarP(&options.Version, "version", "v", "", "The specific platform version to upgrade to.")
	cmd.Flags().StringVarP(&options.Set, "set", "s", "", "The helm parameters to pass in while upgrading, separated by comma, e.g. key1=val1,key2=val2.")
	cmd.Flags().BoolVarP(&options.AlwaysUpgrade, "always-upgrade", "", false, "If set to true, jx will upgrade platform Helm chart even if requested version is already installed.")
	cmd.Flags().BoolVarP(&options.Flags.CleanupTempFiles, "cleanup-temp-files", "", true, "Cleans up any temporary values.yaml used by helm install [default true].")
	cmd.Flags().BoolVarP(&options.UpdateSecrets, "update-secrets", "", false, "Regenerate adminSecrets.yaml on upgrade")

	options.InstallFlags.AddCloudEnvOptions(cmd)

	return cmd
}

// Run implements the command
func (o *UpgradePlatformOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)

	configStore := configio.NewFileStore()
	targetVersion := o.Version
	err := o.Helm().UpdateRepo()
	if err != nil {
		return err
	}
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return errors.Wrap(err, "failed to create the API extensions client")
	}
	err = kube.RegisterAllCRDs(apisClient)
	if err != nil {
		return err
	}
	ns := o.Namespace
	if ns == "" {
		_, ns, err = o.JXClientAndDevNamespace()
		if err != nil {
			return err
		}
	}

	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}

	if "" == settings.KubeProvider {
		log.Logger().Warnf("Unable to determine provider from team settings")

		provider := ""

		prompt := &survey.Select{
			Message: "Select the kube provider:",
			Options: cloud.KubernetesProviders,
			Default: "",
		}
		survey.AskOne(prompt, &provider, nil, surveyOpts)

		err = o.ModifyDevEnvironment(func(env *v1.Environment) error {
			settings = &env.Spec.TeamSettings
			settings.KubeProvider = provider
			return nil
		})
		if err != nil {
			return errors.Wrap(err, "failed to create the API extensions client")
		}
	}

	log.Logger().Infof("Using provider '%s' from team settings", util.ColorInfo(settings.KubeProvider))

	wrkDir := ""

	io := &create.InstallOptions{}
	io.CommonOptions = o.CommonOptions
	io.Flags = o.InstallFlags
	versionsDir, _, err := io.CloneJXVersionsRepo(o.Flags.VersionsRepository, o.Flags.VersionsGitRef)
	if err != nil {
		return err
	}

	if targetVersion == "" {
		targetVersion, err = create.LoadVersionFromCloudEnvironmentsDir(versionsDir, configStore)
		if err != nil {
			return err
		}
	}

	releases, _, err := o.Helm().ListReleases(ns)
	if err != nil {
		return errors.Wrap(err, "list charts releases")
	}
	var currentVersion string
	for name, rel := range releases {
		if name == "jenkins-x" {
			currentVersion = rel.ChartVersion
		}
	}
	if currentVersion == "" {
		return errors.New("Jenkins X platform helm chart is not installed.")
	}

	if targetVersion != currentVersion {
		log.Logger().Infof("Upgrading platform from version %s to version %s", util.ColorInfo(currentVersion), util.ColorInfo(targetVersion))
	} else if o.AlwaysUpgrade {
		log.Logger().Infof("Rerunning platform version %s", util.ColorInfo(targetVersion))
	} else {
		log.Logger().Infof("Already installed platform version %s. Skipping upgrade process.", util.ColorInfo(targetVersion))
		return nil
	}

	isGitOps, devEnv := o.GetDevEnv()
	if isGitOps {
		if devEnv == nil {
			return fmt.Errorf("no Dev environment found")
		}
		if devEnv.Spec.Source.URL == "" {
			return fmt.Errorf("Dev environment does not have source URL")
		}
		return o.upgradePlatformViaGitOps(devEnv, targetVersion, versionsDir, configStore)
	}

	helmConfig := &o.CreateEnvOptions.HelmValuesConfig
	exposeController := helmConfig.ExposeController
	if exposeController != nil && exposeController.Config.Domain == "" {
		helmConfig.ExposeController.Config.Domain = o.InitOptions.Flags.Domain
	}

	// clone the environments repo
	if wrkDir == "" {
		wrkDir, err = o.CloneJXCloudEnvironmentsRepo()
		if err != nil {
			return errors.Wrap(err, "failed to clone the jx cloud environments repo")
		}
	}

	makefileDir := filepath.Join(wrkDir, fmt.Sprintf("env-%s", strings.ToLower(settings.KubeProvider)))
	if _, err := os.Stat(wrkDir); os.IsNotExist(err) {
		return fmt.Errorf("cloud environment dir %s not found", makefileDir)
	}

	// create a temporary file that's used to pass current git creds to helm in order to create a secret for pipelines to tag releases
	dir, err := util.ConfigDir()
	if err != nil {
		return errors.Wrap(err, "failed to create a temporary config dir for Git credentials")
	}

	// file locations
	adminSecretsFileName := filepath.Join(dir, opts.AdminSecretsFile)
	configFileName := filepath.Join(dir, opts.ExtraValuesFile)

	cloudEnvironmentValuesLocation := filepath.Join(makefileDir, opts.CloudEnvValuesFile)
	cloudEnvironmentSecretsLocation := filepath.Join(makefileDir, opts.CloudEnvSecretsFile)
	cloudEnvironmentSopsLocation := filepath.Join(makefileDir, opts.CloudEnvSopsConfigFile)

	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	secretResources := client.CoreV1().Secrets(ns)
	oldSecret, err := secretResources.Get(opts.JXInstallConfig, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get the jx-install-config secret")
	}

	if oldSecret == nil {
		return errors.Wrap(err, "secret jx-install-config doesn't exist, aborting")
	}

	err = o.removeFileIfExists(adminSecretsFileName)
	if err != nil {
		return errors.Wrapf(err, "unable to remove %s if exist", adminSecretsFileName)
	}

	err = o.removeFileIfExists(configFileName)
	if err != nil {
		return errors.Wrapf(err, "unable to remove %s if exist", configFileName)
	}

	log.Logger().Infof("Creating %s from %s", util.ColorInfo(adminSecretsFileName), util.ColorInfo(opts.JXInstallConfig))
	err = ioutil.WriteFile(adminSecretsFileName, oldSecret.Data[opts.AdminSecretsFile], 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write the config file %s", adminSecretsFileName)
	}

	log.Logger().Debugf("%s from %s is %s", opts.AdminSecretsFile, opts.JXInstallConfig, oldSecret.Data[opts.AdminSecretsFile])

	if o.UpdateSecrets {
		// load admin secrets service from adminSecretsFileName
		err = o.AdminSecretsService.NewAdminSecretsConfigFromSecret(adminSecretsFileName)
		if err != nil {
			return errors.Wrap(err, "failed to create the admin secret config service from the secrets file")
		}

		o.AdminSecretsService.NewMavenSettingsXML()
		adminSecrets := &o.AdminSecretsService.Secrets

		log.Logger().Debugf("Rewriting secrets file to %s", util.ColorInfo(adminSecretsFileName))
		err = configStore.WriteObject(adminSecretsFileName, adminSecrets)
		if err != nil {
			return errors.Wrapf(err, "writing the admin secrets in the secrets file '%s'", adminSecretsFileName)
		}

		// save updated admin secretes to Kubernetes
		y, err := yaml.Marshal(adminSecrets)
		if err != nil {
			return errors.Wrapf(err, "unable to marshal admin secrets to yaml: %v", adminSecrets)
		}

		_, err = o.ModifySecret(opts.JXInstallConfig, func(secret *core_v1.Secret) error {
			secret.Data[opts.AdminSecretsFile] = y
			return nil
		})
		if err != nil {
			return errors.Wrapf(err, "unable to save admin secrets to kubernetes secret: %s", opts.JXInstallConfig)
		}

		log.Logger().Debugf("Saved admin secrets to Kubernetes secret %s", util.ColorInfo(opts.JXInstallConfig))
	}

	log.Logger().Infof("Creating %s from %s", util.ColorInfo(configFileName), util.ColorInfo(opts.JXInstallConfig))
	err = ioutil.WriteFile(configFileName, oldSecret.Data[opts.ExtraValuesFile], 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write the config file %s", configFileName)
	}

	sopsFileExists, err := util.FileExists(cloudEnvironmentSopsLocation)
	if err != nil {
		return errors.Wrap(err, "failed to look for "+cloudEnvironmentSopsLocation)
	}

	if sopsFileExists {
		log.Logger().Infof("Attempting to decrypt secrets file %s", util.ColorInfo(cloudEnvironmentSecretsLocation))
		// need to decrypt secrets now
		err = o.Helm().DecryptSecrets(cloudEnvironmentSecretsLocation)
		if err != nil {
			return errors.Wrap(err, "failed to decrypt "+cloudEnvironmentSecretsLocation)
		}

		cloudEnvironmentSecretsDecryptedLocation := filepath.Join(makefileDir, opts.CloudEnvSecretsFile+".dec")
		decryptedSecretsFile, err := util.FileExists(cloudEnvironmentSecretsDecryptedLocation)
		if err != nil {
			return errors.Wrap(err, "failed to look for "+cloudEnvironmentSecretsDecryptedLocation)
		}

		if decryptedSecretsFile {
			log.Logger().Infof("Successfully decrypted %s", util.ColorInfo(cloudEnvironmentSecretsDecryptedLocation))
			cloudEnvironmentSecretsLocation = cloudEnvironmentSecretsDecryptedLocation
		}
	}

	invalidFormat, err := o.checkAdminSecretsForInvalidFormat(adminSecretsFileName)
	if err != nil {
		return errors.Wrap(err, "unable to check adminSecrets.yaml file for invalid format")
	}

	if invalidFormat {
		log.Logger().Warnf("We have detected that the %s file has an invalid format", adminSecretsFileName)

		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Would you like to repair the file?"),
			Default: true,
		}
		survey.AskOne(prompt, &confirm, nil, surveyOpts)

		if confirm {
			err = o.repairAdminSecrets(adminSecretsFileName)
			if err != nil {
				return errors.Wrap(err, "unable to repair adminSecrets.yaml")
			}
		} else {
			log.Logger().Error("Aborting upgrade due to invalid adminSecrets.yaml")
			return nil
		}
	}

	valueFiles := []string{cloudEnvironmentValuesLocation, configFileName, adminSecretsFileName, cloudEnvironmentSecretsLocation}
	valueFiles, err = helm.AppendMyValues(valueFiles)
	if err != nil {
		return errors.Wrap(err, "failed to append the myvalues.yaml file")
	}

	values := []string{}
	if o.Set != "" {
		sets := strings.Split(o.Set, ",")
		values = append(values, sets...)
	}

	for _, v := range valueFiles {
		log.Logger().Debugf("Adding values file %s", util.ColorInfo(v))
	}

	helmOptions := helm.InstallChartOptions{
		Chart:       o.Chart,
		ReleaseName: o.ReleaseName,
		Ns:          ns,
		Version:     targetVersion,
		UpgradeOnly: true,
		NoForce:     true,
		SetValues:   values,
		ValueFiles:  valueFiles,
	}
	err = o.InstallChartWithOptions(helmOptions)
	if err != nil {
		return errors.Wrap(err, "unable to upgrade helm chart")
	}

	if o.Flags.CleanupTempFiles {
		err = o.removeFileIfExists(configFileName)
		if err != nil {
			return errors.Wrap(err, "failed to cleanup the config file")
		}

		err = o.removeFileIfExists(adminSecretsFileName)
		if err != nil {
			return errors.Wrap(err, "failed to cleanup the admin secrets file")
		}
	}

	return nil
}

func (o *UpgradePlatformOptions) removeFileIfExists(fileName string) error {
	fileNameExists, err := util.FileExists(fileName)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if %s exist", fileName)
	}
	if fileNameExists {
		log.Logger().Debugf("Removing values file %s", util.ColorInfo(fileName))
		err = os.Remove(fileName)
		if err != nil {
			return errors.Wrapf(err, "failed to remove %s", fileName)
		}
	}
	return nil
}

func (o *UpgradePlatformOptions) checkAdminSecretsForInvalidFormat(fileName string) (bool, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return false, errors.Wrap(err, "unable to read file")
	}
	return strings.Contains(string(data), "mavensettingsxml"), nil
}

func (o *UpgradePlatformOptions) repairAdminSecrets(fileName string) error {
	admin := config.AdminSecretsConfig{}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrap(err, "unable to read file")
	}

	err = yaml.Unmarshal(data, &admin)
	if err != nil {
		return errors.Wrap(err, "unable to unmarshall secrets")
	}

	// use the correct yaml library to persist
	y, err := yaml.Marshal(admin)
	if err != nil {
		return errors.Wrapf(err, "unable to marshal object to yaml: %v", admin)
	}

	err = ioutil.WriteFile(fileName, y, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "unable to write secrets to file %s", fileName)
	}

	_, err = o.ModifySecret(opts.JXInstallConfig, func(secret *core_v1.Secret) error {
		secret.Data[opts.AdminSecretsFile] = y
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "unable to save admin secrets to kubernetes secret: %s", opts.JXInstallConfig)
	}

	return nil
}

func (o *UpgradePlatformOptions) upgradePlatformViaGitOps(devEnv *v1.Environment, targetVersion string, versionsDir string, configStore configio.ConfigStore) error {
	uopts := &UpgradeAppsOptions{}
	uopts.CommonOptions = o.CommonOptions
	uopts.ReleaseName = platform.JenkinsXPlatformRelease
	uopts.GitOps = true
	uopts.Version = targetVersion
	uopts.Repo = kube.DefaultChartMuseumURL
	uopts.HelmUpdate = true

	//opts.Chart = JenkinsXPlatformChartName
	uopts.Args = []string{platform.JenkinsXPlatformChartName}

	return uopts.Run()
}
