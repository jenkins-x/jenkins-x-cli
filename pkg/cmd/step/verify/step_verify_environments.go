package verify

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/boot"
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
	gitAuthorNameEnvKey     = "GIT_AUTHOR_NAME"
	gitAuthorEmailEnvKey    = "GIT_AUTHOR_EMAIL"
	gitCommitterNameEnvKey  = "GIT_COMMITTER_NAME"
	gitCommitterEmailEnvKey = "GIT_COMMITTER_EMAIL"
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

	requirements, requirementsFileName, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}
	info := util.ColorInfo

	exists, err := util.FileExists(requirementsFileName)
	if err != nil {
		return err
	}

	envMap, names, err := kube.GetEnvironments(jxClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to load Environments in namespace %s", ns)
	}

	if exists {
		// lets store the requirements in the team settings now so that when we create the git auth provider
		// we will be able to detect if we are using GitHub App secrets or not
		err = o.storeRequirementsInTeamSettings(requirements)
		if err != nil {
			return err
		}
	} else {
		devEnv := envMap[kube.LabelValueDevEnvironment]
		if devEnv != nil {
			requirements, err = config.GetRequirementsConfigFromTeamSettings(&devEnv.Spec.TeamSettings)
			if err != nil {
				return errors.Wrap(err, "failed to load requirements from team settings")
			}
		}
	}

	for _, name := range names {
		env := envMap[name]
		gitURL := env.Spec.Source.URL
		if gitURL != "" && (env.Spec.Kind == v1.EnvironmentKindTypePermanent || (env.Spec.Kind == v1.EnvironmentKindTypeDevelopment && requirements.GitOps)) {
			log.Logger().Infof("Validating git repository for %s environment at URL %s\n", info(name), info(gitURL))
			err = o.updateEnvironmentIngressConfig(requirements, requirementsFileName, env)
			if err != nil {
				return errors.Wrapf(err, "updating the ingress config for environment %q", env.GetName())
			}
			err = o.validateGitRepository(name, requirements, env, gitURL)
			if err != nil {
				return err
			}
		}
	}

	log.Logger().Infof("Environment git repositories look good\n")
	fmt.Println()
	return nil
}

func (o *StepVerifyEnvironmentsOptions) prDevEnvironment(gitRepoName string, environmentsOrg string, privateRepo bool, user *auth.UserAuth, requirements *config.RequirementsConfig, server *auth.AuthServer, createPr bool) error {
	fromGitURL := os.Getenv(boot.ConfigRepoURLEnvVarName)
	gitRef := os.Getenv(boot.ConfigBaseRefEnvVarName)

	log.Logger().Debugf("Defined %s env variable value: %s", boot.ConfigRepoURLEnvVarName, fromGitURL)
	log.Logger().Debugf("Defined %s env variable value: %s", boot.ConfigBaseRefEnvVarName, gitRef)

	_, err := gits.ParseGitURL(fromGitURL)
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
		// Also set the gitServer from the requirements.
		env.Spec.TeamSettings.GitServer = requirements.Cluster.GitServer
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "there was a problem saving the current state of the requirements.yaml file in TeamSettings in the dev environment")
	}
	return nil
}

// readEnvironment returns the repository URL as well as the git ref for original boot config repo.
// An error is returned in case any of the require environment variables needed to setup the environment repository
// is missing.
func (o *StepVerifyEnvironmentsOptions) readEnvironment() (string, string, error) {
	var missingRepoURLErr, missingReoRefErr error

	fromGitURL, foundURL := os.LookupEnv(boot.ConfigRepoURLEnvVarName)
	if !foundURL {
		missingRepoURLErr = errors.Errorf("the environment variable %s must be specified", boot.ConfigRepoURLEnvVarName)
	}
	gitRef, foundRef := os.LookupEnv(boot.ConfigBaseRefEnvVarName)
	if !foundRef {
		missingReoRefErr = errors.Errorf("the environment variable %s must be specified", boot.ConfigBaseRefEnvVarName)
	}

	err := util.CombineErrors(missingRepoURLErr, missingReoRefErr)

	if err == nil {
		log.Logger().Debugf("Defined %s env variable value: %s", boot.ConfigRepoURLEnvVarName, fromGitURL)
		log.Logger().Debugf("Defined %s env variable value: %s", boot.ConfigBaseRefEnvVarName, gitRef)
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

		envVars, err = o.setEnvVarInPipelineAndCurrentEnv(gitAuthorNameEnvKey, username, envVars)
		if err != nil {
			return err
		}
		envVars, err = o.setEnvVarInPipelineAndCurrentEnv(gitCommitterNameEnvKey, username, envVars)
		if err != nil {
			return err
		}
		envVars, err = o.setEnvVarInPipelineAndCurrentEnv(gitAuthorEmailEnvKey, email, envVars)
		if err != nil {
			return err
		}
		envVars, err = o.setEnvVarInPipelineAndCurrentEnv(gitCommitterEmailEnvKey, email, envVars)
		if err != nil {
			return err
		}

		projectConf.PipelineConfig.Pipelines.Release.Pipeline.Environment = envVars

		err = projectConf.SaveConfig(fileName)
		if err != nil {
			return errors.Wrapf(err, "failed to write to %s", fileName)
		}
	}
	return nil
}

func (o *StepVerifyEnvironmentsOptions) setEnvVarInPipelineAndCurrentEnv(envVarName string, envVarValue string, envVars []corev1.EnvVar) ([]corev1.EnvVar, error) {
	if !o.envVarsHasEntry(envVars, envVarName) {
		envVars = append(envVars, corev1.EnvVar{
			Name:  envVarName,
			Value: envVarValue,
		})
	}

	err := os.Setenv(envVarName, envVarValue)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set %s env variable", envVarName)
	}

	return envVars, nil
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

	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return errors.Wrap(err, "checking the GitHub app mode")
	}
	var authConfigSvc auth.ConfigService
	if gha {
		authConfigSvc, err = o.GitAuthConfigServiceGitHubAppMode("github")
		if err != nil {
			return errors.Wrap(err, "creating github app auth config service")
		}
	} else {
		authConfigSvc, err = o.GitAuthConfigService()
		if err != nil {
			return errors.Wrap(err, "creating git auth config service")
		}
	}

	return o.createEnvironmentRepository(name, requirements, authConfigSvc, environment, gitURL, envGitInfo)
}

func (o *StepVerifyEnvironmentsOptions) createEnvironmentRepository(name string, requirements *config.RequirementsConfig, authConfigSvc auth.ConfigService, environment *v1.Environment, gitURL string, envGitInfo *gits.GitRepository) error {
	envDir, err := ioutil.TempDir("", "jx-env-repo-")
	if err != nil {
		return errors.Wrap(err, "creating temp dir for environment repository")
	}
	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return errors.Wrap(err, "checking the GitHub app mode")
	}

	gitOwner := envGitInfo.Organisation

	gitKind := requirements.Cluster.GitKind
	if gitKind == "" {
		gitKind = gits.KindGitHub
	}

	public := requirements.Cluster.EnvironmentGitPublic
	prefix := ""

	gitServerURL := envGitInfo.HostURL()
	config := authConfigSvc.Config()
	server, userAuth := config.GetPipelineAuth()

	if gha {
		userAuth = nil
		if server == nil {
			for _, s := range config.Servers {
				if s.URL == gitServerURL {
					server = s
					break
				}
			}
		}
		if server != nil {
			for _, u := range server.Users {
				if gitOwner == u.GithubAppOwner {
					userAuth = u
					break
				}
			}
		}
	}

	helmValues, err := o.createEnvironmentHelmValues(requirements, environment)
	if err != nil {
		return errors.Wrap(err, "creating environment helm values")
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

	gitUserName, gitUserEmail, err := gits.EnsureUserAndEmailSetup(gitter)
	if err != nil {
		return errors.Wrapf(err, "couldn't configure git with user %s and email %s", gitUserName, gitUserEmail)
	}

	if name == kube.LabelValueDevEnvironment || environment.Spec.Kind == v1.EnvironmentKindTypeDevelopment {
		if o.IsJXBoot() && requirements.GitOps && os.Getenv(boot.DisablePushUpdatesToDevEnvironment) != "true" {
			provider, err := envGitInfo.CreateProviderForUser(server, userAuth, gitKind, gitter)
			if err != nil {
				return errors.Wrap(err, "unable to create git provider")
			}
			err = o.handleDevEnvironmentRepository(envGitInfo, public, provider, gitter, requirements)
			if err != nil {
				return errors.Wrap(err, "handle dev environment repository")
			}
		}
	} else {
		gitRepoOptions := &gits.GitRepositoryOptions{
			ServerURL:                gitServerURL,
			ServerKind:               gitKind,
			Username:                 userAuth.Username,
			ApiToken:                 userAuth.Password,
			Owner:                    gitOwner,
			RepoName:                 envGitInfo.Name,
			Public:                   public,
			IgnoreExistingRepository: true,
		}

		_, _, err = kube.DoCreateEnvironmentGitRepo(batchMode, authConfigSvc, environment, forkGitURL, envDir, gitRepoOptions, helmValues, prefix, gitter, o.ResolveChartMuseumURL, o.GetIOFileHandles())
		if err != nil {
			return errors.Wrapf(err, "failed to create git repository for gitURL %s", gitURL)
		}
	}
	return nil
}

func (o *StepVerifyEnvironmentsOptions) handleDevEnvironmentRepository(envGitInfo *gits.GitRepository, public bool, provider gits.GitProvider, gitter gits.Gitter, requirements *config.RequirementsConfig) error {
	fromGitURL, fromBaseRef, err := o.readEnvironment()
	if err != nil {
		return err
	}

	dir, err := filepath.Abs(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "resolving %s to absolute path", o.Dir)
	}

	environmentRepo, err := provider.GetRepository(envGitInfo.Organisation, envGitInfo.Name)
	// Assuming an error implies the repo does not exist. There is currently no way to distinguish between error and non existing repo
	// see https://github.com/jenkins-x/jx/issues/5822
	if err != nil {
		environmentRepo, err = o.createDevEnvironmentRepository(envGitInfo, dir, fromGitURL, fromBaseRef, !public, requirements, provider, gitter)
		if err != nil {
			return errors.Wrapf(err, "creating remote for dev environment %s", envGitInfo.Name)
		}
	}

	err = o.pushDevEnvironmentUpdates(environmentRepo, dir, provider, gitter)
	if err != nil {
		return errors.Wrapf(err, "error updating dev environment for %s", envGitInfo.Name)
	}

	// Add a remote for the user that references the boot config that they originally used
	err = gitter.SetRemoteURL(dir, "jenkins-x", fromGitURL)
	if err != nil {
		return errors.Wrapf(err, "setting jenkins-x remote to boot config %s", fromGitURL)
	}
	return nil
}

func (o *StepVerifyEnvironmentsOptions) createDevEnvironmentRepository(gitInfo *gits.GitRepository, localRepoDir string, fromGitURL string, fromGitRef string, privateRepo bool, requirements *config.RequirementsConfig, provider gits.GitProvider, gitter gits.Gitter) (*gits.GitRepository, error) {
	isDefaultBootURL, err := gits.IsDefaultBootConfigURL(fromGitURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to verify whether %s is the default boot config repository", fromGitURL)
	}
	if isDefaultBootURL && fromGitRef == "master" {
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

	fromGitInfo, err := gits.ParseGitURL(fromGitURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse upstream boot config URL %s", fromGitURL)
	}

	var fromRepo *gits.GitRepository
	// If the to URL and from URL aren't on the same host, pass in a simple repo info to duplicate rather than using the provider.
	if fromGitInfo.Host != gitInfo.Host {
		fromRepo = fromGitInfo
		fromRepo.CloneURL = fromGitURL
		fromRepo.HTMLURL = strings.TrimSuffix(fromGitURL, ".git")
	}

	duplicateInfo, err := gits.DuplicateGitRepoFromCommitish(gitInfo.Organisation, gitInfo.Name, fromGitURL, commitish, "master", privateRepo, provider, gitter, fromRepo)
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

	remoteURL, err := gits.AddUserToURL(environmentRepo.CloneURL, provider.CurrentUsername())
	if err != nil {
		return errors.Wrapf(err, "unable to add username to git url %s", environmentRepo.CloneURL)
	}
	remoteName, err := gits.GetRemoteForURL(localRepoDir, remoteURL, gitter)
	if err != nil {
		return errors.Wrapf(err, "cannot determine remote name for %s", environmentRepo.CloneURL)
	}
	if remoteName == "" {
		return errors.Wrapf(err, "no remote configured for %s", environmentRepo.CloneURL)
	}

	err = gitter.Push(localRepoDir, remoteName, true, "master")
	if err != nil {
		return errors.Wrapf(err, "unable to push %s to %s", localRepoDir, environmentRepo.CloneURL)
	}
	log.Logger().Infof("Pushed Git repository to %s", util.ColorInfo(environmentRepo.HTMLURL))

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
	if envCfg.Ingress.TLS.Enabled {
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
				URLTemplate: getEnvironmentURLTemplate(envCfg),
			},
			Production: envCfg.Ingress.TLS.Production,
		},
	}

	// Only set the secret name if TLS is enabled else exposecontroller thinks the ingress needs TLS
	if envCfg.Ingress.TLS.Enabled {
		secretName := envCfg.Ingress.TLS.SecretName
		if secretName == "" {
			if envCfg.Ingress.TLS.Production {
				secretName = fmt.Sprintf("tls-%s-p", domain)
			} else {
				secretName = fmt.Sprintf("tls-%s-s", domain)
			}
		}
		helmValues.ExposeController.Config.TLSSecretName = strings.ReplaceAll(secretName, ".", "-")
	}

	return helmValues, nil
}

func getEnvironmentURLTemplate(envCfg *config.EnvironmentConfig) string {
	if envCfg.URLTemplate != "" {
		return envCfg.URLTemplate
	}
	return config.ExposeDefaultURLTemplate
}

func (o *StepVerifyEnvironmentsOptions) updateEnvironmentIngressConfig(requirements *config.RequirementsConfig, requirementsFileName string, env *v1.Environment) error {
	if env.Spec.Kind != v1.EnvironmentKindTypeDevelopment {
		return nil
	}

	// Override the dev environment ingress config from main ingress config
	name := env.GetName()
	for i, e := range requirements.Environments {
		if e.Key == name {
			requirements.Environments[i].Ingress = requirements.Ingress
			break
		}
	}

	return requirements.SaveConfig(requirementsFileName)
}
