package cmd

import (
	"io"
	"io/ioutil"
	"strings"

	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
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
	InstallOptions

	Version       string
	ReleaseName   string
	Chart         string
	Namespace     string
	Set           string
	AlwaysUpgrade bool

	InstallFlags InstallFlags
}

// NewCmdUpgradePlatform defines the command
func NewCmdUpgradePlatform(f Factory, in terminal.FileReader, out terminal.FileWriter, errOut io.Writer) *cobra.Command {
	options := &UpgradePlatformOptions{
		InstallOptions: InstallOptions{
			CommonOptions: CommonOptions{
				Factory: f,
				In:      in,
				Out:     out,
				Err:     errOut,
			},
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
			CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "The Namespace to promote to")
	cmd.Flags().StringVarP(&options.ReleaseName, "name", "n", "jenkins-x", "The release name")
	cmd.Flags().StringVarP(&options.Chart, "chart", "c", "jenkins-x/jenkins-x-platform", "The Chart to upgrade")
	cmd.Flags().StringVarP(&options.Version, "version", "v", "", "The specific platform version to upgrade to")
	cmd.Flags().StringVarP(&options.Set, "set", "s", "", "The helm parameters to pass in while upgrading")
	cmd.Flags().BoolVarP(&options.AlwaysUpgrade, "always-upgrade", "", false, "If set to true, jx will upgrade platform Helm chart even if requested version is already installed.")
	cmd.Flags().BoolVarP(&options.Flags.CleanupTempFiles, "cleanup-temp-files", "", true, "Cleans up any temporary values.yaml used by helm install [default true]")

	options.addCommonFlags(cmd)
	options.InstallFlags.addCloudEnvOptions(cmd)

	return cmd
}

// Run implements the command
func (o *UpgradePlatformOptions) Run() error {
	targetVersion := o.Version
	err := o.Helm().UpdateRepo()
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

	wrkDir := ""

	if targetVersion == "" {
		io := &InstallOptions{}
		io.CommonOptions = o.CommonOptions
		io.Flags = o.InstallFlags
		wrkDir, err = io.cloneJXCloudEnvironmentsRepo()
		if err != nil {
			return err
		}
		targetVersion, err = LoadVersionFromCloudEnvironmentsDir(wrkDir)
		if err != nil {
			return err
		}
	}

	// Current version
	var currentVersion string
	output, err := o.Helm().ListCharts()
	if err != nil {
		log.Warnf("Failed to find helm installs: %s\n", err)
		return err
	} else {
		for _, line := range strings.Split(output, "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) > 4 && strings.TrimSpace(fields[0]) == "jenkins-x" {
				for _, f := range fields[4:] {
					f = strings.TrimSpace(f)
					if strings.HasPrefix(f, jxChartPrefix) {
						currentVersion = strings.TrimPrefix(f, jxChartPrefix)
					}
				}
			}
		}
	}
	if currentVersion == "" {
		return errors.New("Jenkins X platform helm chart is not installed.")
	}

	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}
	log.Infof("Using provider %s from team settings\n", util.ColorInfo(settings.KubeProvider))

	helmConfig := &o.CreateEnvOptions.HelmValuesConfig
	exposeController := helmConfig.ExposeController
	if exposeController != nil && exposeController.Config.Domain == "" {
		helmConfig.ExposeController.Config.Domain = o.InitOptions.Flags.Domain
	}

	// clone the environments repo
	if wrkDir == "" {
		wrkDir, err = o.cloneJXCloudEnvironmentsRepo()
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

	secretsFileName := filepath.Join(dir, GitSecretsFile)
	adminSecretsFileName := filepath.Join(dir, AdminSecretsFile)
	configFileName := filepath.Join(dir, ExtraValuesFile)

	secretResources := o.KubeClientCached.CoreV1().Secrets(ns)
	oldSecret, err := secretResources.Get(JXInstallConfig, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get the jx secret resource")
	}

	if oldSecret == nil {
		return errors.Wrap(err, "old secret doesn't exist, aborting")
	}

	if targetVersion != currentVersion {
		log.Infof("Upgrading platform from version %s to version %s\n", util.ColorInfo(currentVersion), util.ColorInfo(targetVersion))
	} else if o.AlwaysUpgrade {
		log.Infof("Rerunning platform version %s\n", util.ColorInfo(targetVersion))
	} else {
		log.Infof("Already installed platform version %s. Skipping upgrade process.\n", util.ColorInfo(targetVersion))
		return nil
	}

	cloudEnvironmentValuesLocation := filepath.Join(makefileDir, CloudEnvValuesFile)
	cloudEnvironmentSecretsLocation := filepath.Join(makefileDir, CloudEnvSecretsFile)
	cloudEnvironmentSopsLocation := filepath.Join(makefileDir, CloudEnvSopsConfigFile)

	secretsFileNameExists, err := util.FileExists(secretsFileName)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if %s exist", secretsFileName)
	}
	if !secretsFileNameExists {
		log.Infof("Creating %s from %s", util.ColorInfo(secretsFileName), util.ColorInfo(JXInstallConfig))
		err = ioutil.WriteFile(secretsFileName, oldSecret.Data[GitSecretsFile], 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to write the config file %s", secretsFileName)
		}
	}

	adminSecretsFileNameExists, err := util.FileExists(adminSecretsFileName)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if %s exist", adminSecretsFileName)
	}
	if !adminSecretsFileNameExists {
		log.Infof("Creating %s from %s", util.ColorInfo(adminSecretsFileName), util.ColorInfo(JXInstallConfig))
		err = ioutil.WriteFile(adminSecretsFileName, oldSecret.Data[AdminSecretsFile], 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to write the config file %s", adminSecretsFileName)
		}
	}

	configFileNameExists, err := util.FileExists(configFileName)
	if err != nil {
		return errors.Wrapf(err, "unable to determine if %s exist", configFileName)
	}
	if !configFileNameExists {
		log.Infof("Creating %s from %s", util.ColorInfo(configFileName), util.ColorInfo(JXInstallConfig))
		err = ioutil.WriteFile(configFileName, oldSecret.Data[ExtraValuesFile], 0644)
		if err != nil {
			return errors.Wrapf(err, "failed to write the config file %s", configFileName)
		}
	}

	sopsFileExists, err := util.FileExists(cloudEnvironmentSopsLocation)
	if err != nil {
		return errors.Wrap(err, "failed to look for "+cloudEnvironmentSopsLocation)
	}

	if sopsFileExists {
		log.Infof("Attempting to decrypt secrets file %s\n", util.ColorInfo(cloudEnvironmentSecretsLocation))
		// need to decrypt secrets now
		err = o.Helm().DecryptSecrets(cloudEnvironmentSecretsLocation)
		if err != nil {
			return errors.Wrap(err, "failed to decrypt "+cloudEnvironmentSecretsLocation)
		}

		cloudEnvironmentSecretsDecryptedLocation := filepath.Join(makefileDir, CloudEnvSecretsFile+".dec")
		decryptedSecretsFile, err := util.FileExists(cloudEnvironmentSecretsDecryptedLocation)
		if err != nil {
			return errors.Wrap(err, "failed to look for "+cloudEnvironmentSecretsDecryptedLocation)
		}

		if decryptedSecretsFile {
			log.Infof("Successfully decrypted %s\n", util.ColorInfo(cloudEnvironmentSecretsDecryptedLocation))
			cloudEnvironmentSecretsLocation = cloudEnvironmentSecretsDecryptedLocation
		}
	}

	valueFiles := []string{cloudEnvironmentValuesLocation, secretsFileName, adminSecretsFileName, configFileName, cloudEnvironmentSecretsLocation}
	valueFiles, err = helm.AppendMyValues(valueFiles)
	if err != nil {
		return errors.Wrap(err, "failed to append the myvalues.yaml file")
	}

	values := []string{}
	if o.Set != "" {
		values = append(values, o.Set)
	}

	for _, v := range valueFiles {
		o.Debugf("Adding values file %s\n", util.ColorInfo(v))
	}

	err = o.Helm().UpgradeChart(o.Chart, o.ReleaseName, ns, &targetVersion, false, nil, false, false, values, valueFiles)
	if err != nil {
		return errors.Wrap(err, "unable to upgrade helm chart")
	}

	if o.Flags.CleanupTempFiles {
		err = os.Remove(secretsFileName)
		if err != nil {
			return errors.Wrap(err, "failed to cleanup the secrets file")
		}

		if !configFileNameExists {
			err = os.Remove(configFileName)
			if err != nil {
				return errors.Wrap(err, "failed to cleanup the config file")
			}
		}
	}

	return nil
}
