package helm

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/mholt/archiver"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	configio "github.com/jenkins-x/jx/pkg/io"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/platform"
	"github.com/jenkins-x/jx/pkg/secreturl/fakevault"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/vault"
)

// StepHelmApplyOptions contains the command line flags
type StepHelmApplyOptions struct {
	StepHelmOptions

	Namespace          string
	ReleaseName        string
	Wait               bool
	Force              bool
	DisableHelmVersion bool
	Boot               bool
	Vault              bool
	NoVault            bool
	NoMasking          bool
	ProviderValuesDir  string
}

var (
	StepHelmApplyLong = templates.LongDesc(`
		Applies the helm chart in a given directory.

		This step is usually used to apply any GitOps promotion changes into a Staging or Production cluster.

        Environment Variables:
		- JX_NO_DELETE_TMP_DIR="true" - prevents the removal of the temporary directory.
`)

	StepHelmApplyExample = templates.Examples(`
		# apply the chart in the env folder to namespace jx-staging
		jx step helm apply --dir env --namespace jx-staging

`)

	defaultValueFileNames = []string{"values.yaml", "myvalues.yaml", helm.SecretsFileName, filepath.Join("env", helm.SecretsFileName)}
)

func NewCmdStepHelmApply(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepHelmApplyOptions{
		StepHelmOptions: StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Applies the helm chart in a given directory",
		Aliases: []string{""},
		Long:    StepHelmApplyLong,
		Example: StepHelmApplyExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addStepHelmFlags(cmd)

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "The Kubernetes namespace to apply the helm chart to")
	cmd.Flags().StringVarP(&options.ReleaseName, "name", "n", "", "The name of the release")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "", true, "Wait for Kubernetes readiness probe to confirm deployment")
	cmd.Flags().BoolVarP(&options.Force, "force", "f", true, "Whether to to pass '--force' to helm to help deal with upgrading if a previous promote failed")
	cmd.Flags().BoolVar(&options.DisableHelmVersion, "no-helm-version", false, "Don't set Chart version before applying")
	cmd.Flags().BoolVarP(&options.Vault, "vault", "", false, "Helm secrets are stored in vault")
	cmd.Flags().BoolVarP(&options.Boot, "boot", "", false, "In Boot mode we load the Version Stream from the 'jx-requirements.yml' and use that to replace any missing versions in the 'requirements.yaml' file from the Version Stream")
	cmd.Flags().BoolVarP(&options.NoVault, "no-vault", "", false, "Disables loading secrets from Vault. e.g. if bootstrapping core services like Ingress before we have a Vault")
	cmd.Flags().BoolVarP(&options.NoMasking, "no-masking", "", false, "The effective 'values.yaml' file is output to the console with parameters masked. Enabling this flag will show the unmasked secrets in the console output")
	cmd.Flags().StringVarP(&options.ProviderValuesDir, "provider-values-dir", "", "", "The optional directory of kubernetes provider specific override values.tmpl.yaml files a kubernetes provider specific folder")

	return cmd
}

func (o *StepHelmApplyOptions) Run() error {
	var err error
	chartName := o.Dir
	dir := o.Dir
	releaseName := o.ReleaseName

	// let allow arguments to be passed in like for `helm install releaseName dir`
	args := o.Args
	if releaseName == "" && len(args) > 0 {
		releaseName = args[0]
	}
	if dir == "" && len(args) > 1 {
		dir = args[1]
	}

	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if !o.DisableHelmVersion {
		(&StepHelmVersionOptions{
			StepHelmOptions: StepHelmOptions{
				StepOptions: step.StepOptions{
					CommonOptions: &opts.CommonOptions{},
				},
			},
		}).Run()
	}
	helmBinary, noTiller, helmTemplate, err := o.TeamHelmBin()
	if err != nil {
		return err
	}

	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	err = kube.EnsureNamespaceCreated(kubeClient, ns, nil, nil)
	if err != nil {
		return err
	}

	_, devNs, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}

	if releaseName == "" {
		if devNs == ns {
			releaseName = platform.JenkinsXPlatformRelease
		} else {
			releaseName = ns

			if helmBinary != "helm" || noTiller || helmTemplate {
				releaseName = "jx"
			}
		}
	}
	info := util.ColorInfo

	path, err := filepath.Abs(dir)
	if err != nil {
		return errors.Wrapf(err, "could not find absolute path of dir %s", dir)
	}
	dir = path

	devGitInfo, err := o.FindGitInfo(dir)
	if err != nil {
		log.Logger().Warnf("could not find a git repository in the directory %s: %s\n", dir, err.Error())
	}
	rootTmpDir, err := ioutil.TempDir("", "jx-helm-apply-")
	if err != nil {
		return errors.Wrapf(err, "failed to create a temporary directory to apply the helm chart")
	}
	if os.Getenv("JX_NO_DELETE_TMP_DIR") != "true" {
		defer os.RemoveAll(rootTmpDir)
	}

	if release, err := kube.AcquireBuildLock(kubeClient, devNs, ns); err != nil {
		return errors.Wrapf(err, "fail to acquire the lock")
	} else {
		defer release()
	}

	// lets use the same child dir name as the original as helm is quite particular about the name of the directory it runs from
	_, name := filepath.Split(dir)
	if name == "" {
		return fmt.Errorf("could not find the relative name of the directory %s", dir)
	}
	tmpDir := filepath.Join(rootTmpDir, name)
	log.Logger().Debugf("Copying the helm source directory %s to a temporary location for building and applying %s\n", info(dir), info(tmpDir))

	err = os.MkdirAll(tmpDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to helm temporary dir %s", tmpDir)
	}
	err = util.CopyDir(dir, tmpDir, true)
	if err != nil {
		return errors.Wrapf(err, "failed to copy helm dir %s to temporary dir %s", dir, tmpDir)
	}
	dir = tmpDir
	log.Logger().Debugf("Applying helm chart at %s as release name %s to namespace %s", info(dir), info(releaseName), info(ns))

	o.Helm().SetCWD(dir)

	valueFiles := []string{}
	for _, name := range defaultValueFileNames {
		file := filepath.Join(dir, name)
		exists, err := util.FileExists(file)
		if exists && err == nil {
			valueFiles = append(valueFiles, file)
		}
	}

	vaultSecretLocation := o.GetSecretsLocation() == secrets.VaultLocationKind
	if vaultSecretLocation && o.NoVault {
		// lets install a fake secret URL client to avoid spurious vault errors
		o.SetSecretURLClient(fakevault.NewFakeClient())
	}
	if (vaultSecretLocation || o.Vault) && !o.NoVault {
		store := configio.NewFileStore()
		secretsFiles, err := o.fetchSecretFilesFromVault(dir, store)
		if err != nil {
			return errors.Wrap(err, "fetching secrets files from vault")
		}
		for _, sf := range secretsFiles {
			if util.StringArrayIndex(valueFiles, sf) < 0 {
				log.Logger().Debugf("adding secret file %s", sf)
				valueFiles = append(valueFiles, sf)
			}
		}
		defer func() {
			for _, secretsFile := range secretsFiles {
				err := util.DestroyFile(secretsFile)
				if err != nil {
					log.Logger().Warnf("Failed to cleanup the secrets files (%s): %v",
						strings.Join(secretsFiles, ", "), err)
				}
			}
		}()
	}

	requirements, requirementsFileName, err := o.getRequirements()
	if err != nil {
		return errors.Wrap(err, "loading the requirements")
	}

	secretURLClient, err := o.GetSecretURLClient(secrets.ToSecretsLocation(string(requirements.SecretStorage)))
	if err != nil {
		return errors.Wrap(err, "failed to create a Secret RL client")
	}

	DefaultEnvironments(requirements, devGitInfo)

	funcMap, err := o.createFuncMap(requirements)
	if err != nil {
		return err
	}
	chartValues, params, err := helm.GenerateValues(requirements, funcMap, dir, nil, true, secretURLClient)
	if err != nil {
		return errors.Wrapf(err, "generating values.yaml for tree from %s", dir)
	}
	if o.ProviderValuesDir != "" && requirementsFileName != "" {
		chartValues, err = o.overwriteProviderValues(requirements, requirementsFileName, chartValues, params, o.ProviderValuesDir)
		if err != nil {
			return errors.Wrapf(err, "failed to overwrite provider values in dir: %s", dir)
		}
	}

	chartValuesFile := filepath.Join(dir, helm.ValuesFileName)
	err = ioutil.WriteFile(chartValuesFile, chartValues, 0755)
	if err != nil {
		return errors.Wrapf(err, "writing values.yaml for tree to %s", chartValuesFile)
	}
	log.Logger().Debugf("Wrote chart values.yaml %s generated from directory tree", chartValuesFile)

	data, err := ioutil.ReadFile(chartValuesFile)
	if err != nil {
		log.Logger().Warnf("failed to load file %s: %s", chartValuesFile, err.Error())
	} else {
		log.Logger().Debugf("generated helm %s", chartValuesFile)

		valuesText := string(data)
		if !o.NoMasking {
			masker := kube.NewLogMaskerFromMap(params.AsMap())
			valuesText = masker.MaskLog(valuesText)
		}

		log.Logger().Debugf("\n%s\n", util.ColorStatus(valuesText))
	}

	log.Logger().Debugf("Using values files: %s", strings.Join(valueFiles, ", "))

	if o.Boot {
		err = o.replaceMissingVersionsFromVersionStream(requirements, dir)
		if err != nil {
			return errors.Wrapf(err, "failed to replace missing versions in the requirements.yaml in dir %s", dir)
		}
	}

	_, err = o.HelmInitDependencyBuild(dir, o.DefaultReleaseCharts(), valueFiles)
	if err != nil {
		return err
	}

	// Now let's unpack all the dependencies and apply the vault URLs
	dependencies, err := filepath.Glob(filepath.Join(dir, "charts", "*.tgz"))
	if err != nil {
		return errors.Wrapf(err, "finding chart dependencies in %s", filepath.Join(dir, "charts"))
	}
	for _, src := range dependencies {
		dest, err := ioutil.TempDir("", "")
		if err != nil {
			return errors.Wrapf(err, "creating temp dir")
		}
		err = archiver.Unarchive(src, dest)
		if err != nil {
			return errors.Wrapf(err, "untarring %s to %s", src, dest)
		}
		err = os.Remove(src)
		if err != nil {
			return errors.Wrapf(err, "removing %s", src)
		}
		filepath.Walk(dest, func(path string, info os.FileInfo, err error) error {
			if filepath.Base(path) == helm.ValuesFileName {

				newFiles, cleanup, err := helm.DecorateWithSecrets([]string{path}, secretURLClient)
				defer cleanup()
				if err != nil {
					return errors.Wrapf(err, "decorating %s with secrets", path)
				}
				err = util.CopyFile(newFiles[0], path)
				if err != nil {
					return errors.Wrapf(err, "moving decorated file %s to %s", newFiles[0], path)
				}
			}
			return nil
		})
		dirs, err := filepath.Glob(filepath.Join(dest, "*"))
		if err != nil {
			return errors.Wrapf(err, "list %s", filepath.Join(dest, "*"))
		}
		err = archiver.Archive(dirs, src)
	}

	err = o.applyAppsTemplateOverrides(chartName)
	if err != nil {
		return errors.Wrap(err, "applying app chart overrides")
	}
	err = o.applyTemplateOverrides(chartName)
	if err != nil {
		return errors.Wrap(err, "applying chart overrides")
	}

	setValues, setStrings := o.getChartValues(ns)

	helmOptions := helm.InstallChartOptions{
		Chart:       chartName,
		ReleaseName: releaseName,
		Ns:          ns,
		NoForce:     !o.Force,
		SetValues:   setValues,
		SetStrings:  setStrings,
		ValueFiles:  valueFiles,
		Dir:         dir,
	}
	if o.Boot {
		helmOptions.VersionsGitURL = requirements.VersionStream.URL
		helmOptions.VersionsGitRef = requirements.VersionStream.Ref
	}

	if o.Wait {
		helmOptions.Wait = true
		err = o.InstallChartWithOptionsAndTimeout(helmOptions, "600")
	} else {
		err = o.InstallChartWithOptions(helmOptions)
	}
	if err != nil {
		return errors.Wrapf(err, "upgrading helm chart '%s'", chartName)
	}
	return nil
}

// getRequirements tries to load the requirements either from the team settings or local requirements file
func (o *StepHelmApplyOptions) getRequirements() (*config.RequirementsConfig, string, error) {
	// Try to load first the requirements from current directory
	requirements, requirementsFileName, err := config.LoadRequirementsConfig(o.Dir)
	if err == nil {
		return requirements, requirementsFileName, nil
	}

	// When no requirements file is found, try to load the requirements from team settings
	jxClient, ns, err := o.JXClient()
	if err != nil {
		return nil, "", errors.Wrap(err, "getting the jx client")
	}
	teamSettings, err := kube.GetDevEnvTeamSettings(jxClient, ns)
	if err != nil {
		return nil, "", errors.Wrap(err, "getting the team setting from the cluster")
	}

	requirements, err = config.GetRequirementsConfigFromTeamSettings(teamSettings)
	if err != nil {
		return nil, "", errors.Wrap(err, "getting the requirements from team settings")
	}
	// TODO: Workaround for non-boot clusters. Remove when we get rid of jx install. (APB)
	if requirements == nil {
		requirements = config.NewRequirementsConfig()
		requirementsFileName = config.RequirementsConfigFileName
		return requirements, requirementsFileName, nil
	}

	return requirements, "", nil
}

// DefaultEnvironments ensures we have valid values for environment owner and repository names.
// if none are configured lets default them from smart defaults
func DefaultEnvironments(c *config.RequirementsConfig, devGitInfo *gits.GitRepository) {
	defaultOwner := c.Cluster.EnvironmentGitOwner
	clusterName := c.Cluster.ClusterName
	for i := range c.Environments {
		env := &c.Environments[i]
		if !c.GitOps {
			if env.Key == kube.LabelValueDevEnvironment && devGitInfo != nil {
				if env.Owner == "" {
					env.Owner = devGitInfo.Organisation
				}
				if env.Repository == "" {
					env.Repository = devGitInfo.Name
				}
				if env.GitServer == "" {
					env.GitServer = devGitInfo.HostURL()
				}
				if env.GitKind == "" {
					env.GitKind = gits.SaasGitKind(env.GitServer)
				}
			}
		}
		if env.Owner == "" {
			env.Owner = defaultOwner
		}
		if env.Repository == "" {
			if clusterName != "" {
				env.Repository = naming.ToValidName("environment-" + clusterName + "-" + env.Key)
			} else {
				log.Logger().Warnf("there is no 'cluster.clusterName' value set in the 'jx-requirements.yml' file. Please specify the 'repository' for environment: %s", env.Key)
			}
		}
	}
}

func (o *StepHelmApplyOptions) applyTemplateOverrides(chartName string) error {
	log.Logger().Debugf("Applying chart overrides")
	templateOverrides, err := filepath.Glob(chartName + "/../*/templates/*.yaml")
	for _, overrideSrc := range templateOverrides {
		if !strings.Contains(overrideSrc, "/env/") {
			data, err := ioutil.ReadFile(overrideSrc)
			if err == nil {
				writeTemplateParts := strings.Split(overrideSrc, string(os.PathSeparator))
				depChartsDir := filepath.Join(chartName, "charts")
				depChartName := writeTemplateParts[len(writeTemplateParts)-3]
				templateName := writeTemplateParts[len(writeTemplateParts)-1]
				depChartDir := filepath.Join(depChartsDir, depChartName)
				if _, err := os.Stat(depChartDir); os.IsNotExist(err) {
					// If there is no charts/<depChartName> dir it means that it's not a dependency of this chart
					continue
				}
				// If the chart directory does not exist explode the tgz
				if exists, err := util.DirExists(depChartDir); err == nil && !exists {
					chartArchives, _ := filepath.Glob(filepath.Join(depChartsDir, depChartName+"*.tgz"))
					if len(chartArchives) == 1 {
						log.Logger().Debugf("Exploding chart %s", chartArchives[0])
						archiver.Unarchive(chartArchives[0], depChartsDir)
						// Remove the unexploded chart
						os.Remove(chartArchives[0])
					}
				}
				overrideDst := filepath.Join(depChartDir, "templates", templateName)
				log.Logger().Debugf("Copying chart override %s", overrideSrc)
				err = ioutil.WriteFile(overrideDst, data, util.DefaultWritePermissions)
				if err != nil {
					log.Logger().Warnf("Error copying template %s to %s %v", overrideSrc, overrideDst, err)
				}

			}
		}
	}
	return err
}

func (o *StepHelmApplyOptions) applyAppsTemplateOverrides(chartName string) error {
	log.Logger().Debugf("Applying Apps chart overrides")
	templateOverrides, err := filepath.Glob(chartName + "/../*/*/templates/app.yaml")
	for _, overrideSrc := range templateOverrides {
		data, err := ioutil.ReadFile(overrideSrc)
		if err == nil {
			writeTemplateParts := strings.Split(overrideSrc, string(os.PathSeparator))
			depChartsDir := filepath.Join(chartName, "charts")
			depChartName := writeTemplateParts[len(writeTemplateParts)-3]
			templateName := writeTemplateParts[len(writeTemplateParts)-1]
			depChartDir := filepath.Join(depChartsDir, depChartName)
			chartArchives, _ := filepath.Glob(filepath.Join(depChartsDir, depChartName+"*.tgz"))
			if len(chartArchives) == 1 {
				uuid, _ := uuid.NewUUID()
				log.Logger().Debugf("Exploding App chart %s", chartArchives[0])
				explodedChartTempDir := filepath.Join(os.TempDir(), uuid.String())
				if err = archiver.Unarchive(chartArchives[0], explodedChartTempDir); err != nil {
					return defineAppsChartOverridingError(chartName, err)
				}
				overrideDst := filepath.Join(explodedChartTempDir, depChartName, "templates", templateName)
				log.Logger().Debugf("Copying chart override %s", overrideSrc)
				err = ioutil.WriteFile(overrideDst, data, util.DefaultWritePermissions)
				if err != nil {
					log.Logger().Warnf("Error copying template %s to %s %v", overrideSrc, overrideDst, err)
				}
				if err = os.Remove(chartArchives[0]); err != nil {
					return defineAppsChartOverridingError(chartName, err)
				}
				if err = archiver.Archive([]string{filepath.Join(explodedChartTempDir, depChartName)}, chartArchives[0]); err != nil {
					return defineAppsChartOverridingError(chartName, err)
				}
				if err = os.RemoveAll(explodedChartTempDir); err != nil {
					log.Logger().Warnf("There was a problem deleting the temp folder %s", depChartDir)
				}
			}
		}
	}
	return err
}

func defineAppsChartOverridingError(chartName string, err error) error {
	return errors.Wrapf(err, "there was a problem overriding the chart %s", chartName)
}

func (o *StepHelmApplyOptions) fetchSecretFilesFromVault(dir string, store configio.ConfigStore) ([]string, error) {
	log.Logger().Debugf("Fetching secrets from vault into directory %q", dir)
	files := []string{}
	client, err := o.SystemVaultClient("")
	if err != nil {
		return files, errors.Wrap(err, "retrieving the system Vault")
	}
	secretNames, err := client.List(vault.GitOpsSecretsPath)
	if err != nil {
		return files, errors.Wrap(err, "listing the GitOps secrets in Vault")
	}
	secretPaths := []string{}
	for _, secretName := range secretNames {
		if secretName == vault.GitOpsTemplatesPath {
			templatesPath := vault.GitOpsSecretPath(vault.GitOpsTemplatesPath)
			templatesSecretNames, err := client.List(templatesPath)
			if err == nil {
				for _, templatesSecretName := range templatesSecretNames {
					templateSecretPath := vault.GitOpsTemplatesPath + templatesSecretName
					secretPaths = append(secretPaths, templateSecretPath)
				}
			}
		} else {
			secretPaths = append(secretPaths, secretName)
		}
	}

	for _, secretPath := range secretPaths {
		gitopsSecretPath := vault.GitOpsSecretPath(secretPath)
		secret, err := client.ReadYaml(gitopsSecretPath)
		if err != nil {
			return files, errors.Wrapf(err, "retrieving the secret %q from Vault", secretPath)
		}
		if secret == "" {
			return files, fmt.Errorf("secret %q is empty", secretPath)
		}
		secretFile := filepath.Join(dir, secretPath)
		err = store.Write(secretFile, []byte(secret))
		if err != nil {
			return files, errors.Wrapf(err, "saving the secret file %q", secretFile)
		}
		log.Logger().Debugf("Saved secrets file %s", util.ColorInfo(secretFile))
		files = append(files, secretFile)
	}
	return files, nil
}
