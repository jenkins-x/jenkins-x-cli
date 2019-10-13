package verify

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/helm"
	"sigs.k8s.io/yaml"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

const (
	jxInterpretPipelineEnvKey = "JX_INTERPRET_PIPELINE"
	configRepoURLEnvKey       = "REPO_URL"
	configRepoRefEnvKey       = "BASE_CONFIG_REF"
)

// StepVerifyEnvironmentsOptions contains the command line flags
type StepVerifyEnvironmentsOptions struct {
	StepVerifyOptions
	Dir string
}

// NewCmdStepVerifyEnvironments creates the `jx step verify pod` command
func NewCmdStepVerifyEnvironments(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyEnvironmentsOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "environments",
		Aliases: []string{"environment", "env"},
		Short:   "Verifies that the Environments have valid git repositories setup - lazily creating them if needed",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", fmt.Sprintf("The directory to look for the %s file, by default the current working directory", config.RequirementsConfigFileName))
	return cmd
}

// Run implements this command
func (o *StepVerifyEnvironmentsOptions) Run() error {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	requirements, _, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}
	info := util.ColorInfo

	envMap, names, err := kube.GetEnvironments(jxClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to load Environments in namespace %s", ns)
	}
	for _, name := range names {
		env := envMap[name]
		gitURL := env.Spec.Source.URL
		if gitURL != "" && (env.Spec.Kind == v1.EnvironmentKindTypePermanent || (env.Spec.Kind == v1.EnvironmentKindTypeDevelopment && requirements.GitOps)) {
			log.Logger().Infof("Validating git repository for %s environment at URL %s\n", info(name), info(gitURL))

			err = o.validateGitRepository(name, requirements, env, gitURL)
			if err != nil {
				return err
			}
		}
	}

	err = o.storeRequirementsInTeamSettings(requirements)
	if err != nil {
		return err
	}

	log.Logger().Infof("The environment git repositories look good\n")
	fmt.Println()

	return nil
}

func (o *StepVerifyEnvironmentsOptions) storeRequirementsInTeamSettings(requirements *config.RequirementsConfig) error {
	log.Logger().Infof("Storing the requirements in team settings in the dev environment\n")
	err := o.ModifyDevEnvironment(func(env *v1.Environment) error {
		log.Logger().Debugf("Updating the TeamSettings with: %+v", requirements)
		reqBytes, err := yaml.Marshal(requirements)
		if err != nil {
			return errors.Wrap(err, "there was a problem marshalling the requirements file to include it in the TeamSettings")
		}
		env.Spec.TeamSettings.BootRequirements = string(reqBytes)
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "there was a problem saving the current state of the requirements.yaml file in TeamSettings in the dev environment")
	}
	return nil
}

// isJXBoot returns true if this code is executed as part of jx boot, false otherwise.
func (o *StepVerifyEnvironmentsOptions) isJXBoot() bool {
	// sort of a hack to determine that `jx boot` is executed opposed to running as a pipeline build
	// see step_create_task where JX_INTERPRET_PIPELINE is set when the pipeline is executed in interpret mode
	// which in turn is set by `jx boot` (HF)
	return os.Getenv(jxInterpretPipelineEnvKey) == "true"
}

// readEnvironment returns the repository URL as well as the git ref for original boot config repo.
// An error is returned in case any of the require environment variables needed to setup the environment repository
// is missing.
func (o *StepVerifyEnvironmentsOptions) readEnvironment() (string, string, error) {
	var missingRepoURLErr, missingReoRefErr error

	fromGitURL, foundURL := os.LookupEnv(configRepoURLEnvKey)
	if !foundURL {
		missingRepoURLErr = errors.Errorf("the environment variable %s must be specified", configRepoURLEnvKey)
	}
	gitRef, foundRef := os.LookupEnv(configRepoRefEnvKey)
	if !foundRef {
		missingReoRefErr = errors.Errorf("the environment variable %s must be specified", configRepoRefEnvKey)
	}

	err := util.CombineErrors(missingRepoURLErr, missingReoRefErr)

	if err == nil {
		log.Logger().Debugf("Defined %s env variable value: %s", configRepoURLEnvKey, fromGitURL)
		log.Logger().Debugf("Defined %s env variable value: %s", configRepoRefEnvKey, gitRef)
	}

	return fromGitURL, gitRef, err
}

func (o *StepVerifyEnvironmentsOptions) modifyPipelineGitEnvVars(dir string) error {
	parameterValues, err := helm.LoadParametersValuesFile(dir)
	if err != nil {
		return errors.Wrap(err, "failed to load parameters values file")
	}
	username := util.GetMapValueAsStringViaPath(parameterValues, "pipelineUser.username")
	email := util.GetMapValueAsStringViaPath(parameterValues, "pipelineUser.email")

	if username != "" && email != "" {
		fileName := filepath.Join(dir, config.ProjectConfigFileName)
		projectConf, err := config.LoadProjectConfigFile(fileName)
		if err != nil {
			return errors.Wrapf(err, "failed to load project config file %s", fileName)
		}

		envVars := projectConf.PipelineConfig.Pipelines.Release.Pipeline.Environment

		if !o.envVarsHasEntry(envVars, "GIT_AUTHOR_NAME") {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "GIT_AUTHOR_NAME",
				Value: username,
			})
		}

		if !o.envVarsHasEntry(envVars, "GIT_AUTHOR_EMAIL") {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "GIT_AUTHOR_EMAIL",
				Value: email,
			})
		}

		projectConf.PipelineConfig.Pipelines.Release.Pipeline.Environment = envVars

		err = projectConf.SaveConfig(fileName)
		if err != nil {
			return errors.Wrapf(err, "failed to write to %s", fileName)
		}

		err = os.Setenv("GIT_AUTHOR_NAME", username)
		if err != nil {
			return errors.Wrap(err, "failed to set GIT_AUTHOR_NAME env variable")
		}
		err = os.Setenv("GIT_AUTHOR_EMAIL", email)
		if err != nil {
			return errors.Wrap(err, "failed to set GIT_AUTHOR_EMAIL env variable")
		}
	}
	return nil
}

func (o *StepVerifyEnvironmentsOptions) envVarsHasEntry(envVars []corev1.EnvVar, key string) bool {
	for _, entry := range envVars {
		if entry.Name == key {
			return true
		}
	}
	return false
}

func (o *StepVerifyEnvironmentsOptions) validateGitRepository(name string, requirements *config.RequirementsConfig, environment *v1.Environment, gitURL string) error {
	message := fmt.Sprintf("for environment %s", environment.Name)
	envGitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL %s and %s", gitURL, message)
	}
	authConfigSvc, err := o.CreatePipelineUserGitAuthConfigService()
	if err != nil {
		return err
	}
	return o.createEnvironmentRepository(name, requirements, authConfigSvc, environment, gitURL, envGitInfo)
}

func (o *StepVerifyEnvironmentsOptions) createEnvironmentRepository(name string, requirements *config.RequirementsConfig, authConfigSvc auth.ConfigService, environment *v1.Environment, gitURL string, envGitInfo *gits.GitRepository) error {
	envDir, err := ioutil.TempDir("", "jx-env-repo-")
	if err != nil {
		return err
	}

	// TODO - this is hard coded to GitHub and needs to be extended to support other git providers (HF)
	gitKind := gits.KindGitHub
	public := requirements.Cluster.EnvironmentGitPublic
	prefix := ""

	gitServerURL := envGitInfo.HostURL()
	server, userAuth := authConfigSvc.Config().GetPipelineAuth()
	helmValues, err := o.createEnvironmentHelmValues(requirements, environment)
	if err != nil {
		return err
	}
	batchMode := o.BatchMode
	forkGitURL := kube.DefaultEnvironmentGitRepoURL

	if server == nil {
		return fmt.Errorf("no auth server found for git server %s from gitURL %s", gitServerURL, gitURL)
	}
	if userAuth == nil {
		return fmt.Errorf("no pipeline user found for git server %s from gitURL %s", gitServerURL, gitURL)
	}
	if userAuth.IsInvalid() {
		return errors.Wrapf(err, "validating user '%s' of server '%s'", userAuth.Username, server.Name)
	}

	gitter := o.Git()

	if name == kube.LabelValueDevEnvironment || environment.Spec.Kind == v1.EnvironmentKindTypeDevelopment {
		if requirements.GitOps {
			provider, err := envGitInfo.CreateProviderForUser(server, userAuth, gitKind, gitter)
			if err != nil {
				return errors.Wrap(err, "unable to create git provider")
			}
			err = o.createDevEnvironmentRepository(envGitInfo, public, provider, gitter, requirements)
			if err != nil {
				return err
			}
		}
	} else {
		gitRepoOptions := &gits.GitRepositoryOptions{
			ServerURL:                gitServerURL,
			ServerKind:               gitKind,
			Username:                 userAuth.Username,
			ApiToken:                 userAuth.Password,
			Owner:                    envGitInfo.Organisation,
			RepoName:                 envGitInfo.Name,
			Public:                   public,
			IgnoreExistingRepository: true,
		}

		_, _, err = kube.DoCreateEnvironmentGitRepo(batchMode, authConfigSvc, environment, forkGitURL, envDir, gitRepoOptions, helmValues, prefix, gitter, o.ResolveChartMuseumURL, o.In, o.Out, o.Err)
		if err != nil {
			return errors.Wrapf(err, "failed to create git repository for gitURL %s", gitURL)
		}
	}
	return nil
}

func (o *StepVerifyEnvironmentsOptions) createDevEnvironmentRepository(envGitInfo *gits.GitRepository, public bool, provider gits.GitProvider, gitter gits.Gitter, requirements *config.RequirementsConfig) error {
	fromGitURL, fromBaseRef, err := o.readEnvironment()
	if err != nil {
		return err
	}

	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "resolving %s to absolute path", o.Dir)
	}

	environmentRepo, err := o.createDevEnvironmentRemote(envGitInfo, dir, fromGitURL, fromBaseRef, !public, requirements, provider, gitter)
	if err != nil {
		return errors.Wrapf(err, "creating remote for dev environment %s", envGitInfo.Name)
	}

	if o.isJXBoot() {
		err = o.pushDevEnvironmentUpdates(environmentRepo, dir, provider, gitter)
		if err != nil {
			return errors.Wrapf(err, "error updating dev environment for %s", envGitInfo.Name)
		}

		// Add a remote for the user that references the boot config that they originally used
		err = gitter.SetRemoteURL(dir, "jenkins-x", fromGitURL)
		if err != nil {
			return errors.Wrapf(err, "setting jenkins-x remote to boot config %s", fromGitURL)
		}
	}
	return nil
}

func (o *StepVerifyEnvironmentsOptions) createDevEnvironmentRemote(gitInfo *gits.GitRepository, localRepoDir string, fromGitURL string, fromGitRef string, privateRepo bool, requirements *config.RequirementsConfig, provider gits.GitProvider, gitter gits.Gitter) (*gits.GitRepository, error) {
	if fromGitURL == config.DefaultBootRepository && fromGitRef == "master" {
		// If the GitURL is not overridden and the GitRef is set to it's default value then look up the version number
		resolver, err := o.CreateVersionResolver(requirements.VersionStream.URL, requirements.VersionStream.Ref)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create version resolver")
		}
		fromGitRef, err = resolver.ResolveGitVersion(fromGitURL)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to resolve version for https://github.com/jenkins-x/jenkins-x-boot-config.git")
		}
		if fromGitRef == "" {
			log.Logger().Infof("Attempting to resolve version for upstream boot config %s", util.ColorInfo(config.DefaultBootRepository))
			fromGitRef, err = resolver.ResolveGitVersion(config.DefaultBootRepository)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to resolve version for https://github.com/jenkins-x/jenkins-x-boot-config.git")
			}
		}
	}

	commitish, err := gits.FindTagForVersion(localRepoDir, fromGitRef, gitter)
	if err != nil {
		log.Logger().Debugf(errors.Wrapf(err, "finding tag for %s", fromGitRef).Error())
		commitish = fmt.Sprintf("%s/%s", "origin", fromGitRef)
		log.Logger().Debugf("set commitish to '%s'", commitish)
	}

	duplicateInfo, err := gits.DuplicateGitRepoFromCommitish(gitInfo.Organisation, gitInfo.Name, fromGitURL, commitish, "master", privateRepo, provider, gitter)
	if err != nil {
		return nil, errors.Wrapf(err, "duplicating %s to %s/%s", fromGitURL, gitInfo.Organisation, gitInfo.Name)
	}
	return duplicateInfo, nil
}

func (o *StepVerifyEnvironmentsOptions) pushDevEnvironmentUpdates(environmentRepo *gits.GitRepository, localRepoDir string, provider gits.GitProvider, gitter gits.Gitter) error {
	_, _, _, _, err := gits.ForkAndPullRepo(environmentRepo.CloneURL, localRepoDir, "master", "master", provider, gitter, environmentRepo.Name)
	if err != nil {
		return errors.Wrapf(err, "forking and pulling %s", environmentRepo.CloneURL)
	}

	err = o.modifyPipelineGitEnvVars(localRepoDir)
	if err != nil {
		return errors.Wrap(err, "failed to modify dev environment config")
	}

	hasChanges, err := gitter.HasChanges(localRepoDir)
	if err != nil {
		return errors.Wrap(err, "unable to check for changes")
	}

	if hasChanges {
		err = gitter.Add(localRepoDir, ".")
		if err != nil {
			return errors.Wrap(err, "unable to add stage commit")
		}

		err = gitter.CommitDir(localRepoDir, "chore(config): update configuration")
		if err != nil {
			return errors.Wrapf(err, "unable to commit changes to environment repo in %s", localRepoDir)
		}
	}

	userDetails := provider.UserAuth()
	authenticatedPushURL, err := gitter.CreateAuthenticatedURL(environmentRepo.CloneURL, &userDetails)
	if err != nil {
		return errors.Wrapf(err, "failed to create push URL for %s", environmentRepo.CloneURL)
	}
	err = gitter.Push(localRepoDir, authenticatedPushURL, true, "master")
	if err != nil {
		return errors.Wrapf(err, "unable to push %s to %s", localRepoDir, environmentRepo.URL)
	}
	log.Logger().Infof("Pushed Git repository to %s\n\n", util.ColorInfo(environmentRepo.HTMLURL))

	return nil
}

func (o *StepVerifyEnvironmentsOptions) createEnvironmentHelmValues(requirements *config.RequirementsConfig, environment *v1.Environment) (config.HelmValuesConfig, error) {
	envCfg, err := requirements.Environment(environment.GetName())
	if err != nil || envCfg == nil {
		return config.HelmValuesConfig{}, errors.Wrapf(err,
			"looking the configuration of environment %q in the requirements configuration", environment.GetName())
	}
	domain := requirements.Ingress.Domain
	if envCfg.Ingress.Domain != "" {
		domain = envCfg.Ingress.Domain
	}
	useHTTP := "true"
	tlsAcme := "false"

	if requirements.Ingress.TLS.Enabled {
		useHTTP = "false"
		tlsAcme = "true"
	}
	exposer := "Ingress"
	helmValues := config.HelmValuesConfig{
		ExposeController: &config.ExposeController{
			Config: config.ExposeControllerConfig{
				Domain:      domain,
				Exposer:     exposer,
				HTTP:        useHTTP,
				TLSAcme:     tlsAcme,
				URLTemplate: config.ExposeDefaultURLTemplate,
			},
		},
	}

	// set the exposecontroller helm values needed to create an ingress rule with TLS pointing to the right secret containing the cert
	secretName := ""
	if requirements.Ingress.TLS.Production {
		helmValues.ExposeController.Production = true
		secretName = fmt.Sprintf("tls-%s-p", domain)
	} else {
		secretName = fmt.Sprintf("tls-%s-s", domain)
	}

	// only set the secret name if TLS is enabled else exposecontroller thinks the ingress needs TLS
	if requirements.Ingress.TLS.Enabled {
		helmValues.ExposeController.Config.TLSSecretName = strings.Replace(secretName, ".", "-", -1)
	}

	return helmValues, nil
}
