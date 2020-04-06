package git

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepGitEnvsOptions contains the command line flags
type StepGitEnvsOptions struct {
	step.StepOptions

	ServiceKind string
}

var (
	// StepGitEnvsLong command long description
	StepGitEnvsLong = templates.LongDesc(`
		This pipeline step generates a Git environment variables from the current Git provider pipeline Secrets

`)
	// StepGitEnvsExample command example
	StepGitEnvsExample = templates.Examples(`
		# Sets the Git environment variables for the current GitHub provider
		jx step git envs

		# Sets the Gie environment variables for the current Gtilab provider
		jx step git envs --service-kind=gitlab
`)
)

// NewCmdStepGitEnvs create the 'step git envs' command
func NewCmdStepGitEnvs(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGitEnvsOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "envs",
		Short:   "Creates the Git environment variables for the current pipeline Git credentials",
		Long:    StepGitEnvsLong,
		Example: StepGitEnvsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.ServiceKind, "service-kind", "", "github", "The kind of git service")
	return cmd
}

// Run implements the command
func (o *StepGitEnvsOptions) Run() error {
	gitAuthSvc, err := o.GitAuthConfigService()
	if err != nil {
		return errors.Wrap(err, "creating the git auth config service")
	}

	cfg := gitAuthSvc.Config()
	server := cfg.GetServerByKind(o.ServiceKind)
	if server == nil {
		return fmt.Errorf("no server found of kind %q", o.ServiceKind)
	}
	auth := server.CurrentAuth()
	if auth == nil {
		return fmt.Errorf("server %q has no user auth configured", server.URL)
	}
	_, _ = fmt.Fprintf(o.Out, "export GIT_USERNAME=%s\nexport GIT_API_TOKEN=%s\n", auth.Username, auth.ApiToken)

	return nil
}
