package verify

import (
	"fmt"
	"io/ioutil"

	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
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
)

// StepVerifyEnvironmentsOptions contains the command line flags
type StepVerifyEnvironmentsOptions struct {
	StepVerifyOptions
	Dir            string
	LazyCreate     bool
	LazyCreateFlag string
}

// NewCmdStepVerifyEnvironments creates the `jx step verify pod` command
func NewCmdStepVerifyEnvironments(commonOpts *opts.CommonOptions) *cobra.Command {

	options := &StepVerifyEnvironmentsOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: opts.StepOptions{
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
	cmd.Flags().StringVarP(&options.LazyCreateFlag, "lazy-create", "", "", fmt.Sprintf("Specify true/false as to whether to lazily create missing resources. If not specified it is enabled if Terraform is not specified in the %s file", config.RequirementsConfigFileName))
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the install requirements file")
	return cmd
}

// Run implements this command
func (o *StepVerifyEnvironmentsOptions) Run() error {
	lazyCreate := true
	jxClient, ns, err := o.JXClientAndDevNamespace()
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
		if gitURL != "" && name != kube.LabelValueDevEnvironment && env.Spec.Kind == v1.EnvironmentKindTypePermanent {
			log.Logger().Infof("validating git repository for %s at URL %s\n", info(name), info(gitURL))

			err = o.validateGitRepoitory(env, gitURL, lazyCreate)
			if err != nil {
				return err
			}
		}
	}

	log.Logger().Infof("the git repositories for the environments look good\n")
	fmt.Println()
	return nil
}

func (o *StepVerifyEnvironmentsOptions) validateGitRepoitory(environment *v1.Environment, gitURL string, lazyCreate bool) error {
	message := fmt.Sprintf("for environment %s", environment.Name)
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL %s and %s", gitURL, message)
	}
	authConfigSvc, err := o.CreatePipelineUserGitAuthConfigService()
	if err != nil {
		return err
	}
	return o.createEnvGitRepository(authConfigSvc, environment, gitURL, gitInfo)
}

func (o *StepVerifyEnvironmentsOptions) createEnvGitRepository(authConfigSvc auth.ConfigService, environment *v1.Environment, gitURL string, gitInfo *gits.GitRepository) error {
	log.Logger().Infof("creating environment %s git repository for URL: %s to namespace %s\n", util.ColorInfo(environment.Name), util.ColorInfo(gitURL), util.ColorInfo(environment.Spec.Namespace))

	envDir, err := ioutil.TempDir("", "jx-env-repo-")
	if err != nil {
		return err
	}

	// TODO
	gitKind := gits.KindGitHub
	privateRepo := false
	prefix := ""

	gitServerURL := gitInfo.HostURL()
	server, userAuth := authConfigSvc.Config().GetPipelineAuth()
	helmValues := config.HelmValuesConfig{}
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

	gitRepoOptions := &gits.GitRepositoryOptions{
		ServerURL:  gitServerURL,
		ServerKind: gitKind,
		Username:   userAuth.Username,
		ApiToken:   userAuth.Password,
		Owner:      gitInfo.Organisation,
		RepoName:   gitInfo.Name,
		Private:    privateRepo,
	}

	_, _, err = kube.DoCreateEnvironmentGitRepo(batchMode, authConfigSvc, environment, forkGitURL, envDir, gitRepoOptions, helmValues, prefix, o.Git(), o.ResolveChartMuseumURL, o.In, o.Out, o.Err)
	if err != nil {
		return fmt.Errorf("failed to create git repository for gitURL %s", gitURL)
	}
	return nil
}
