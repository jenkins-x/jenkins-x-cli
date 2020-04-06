package pr

import (
	"github.com/jenkins-x/jx/pkg/brew"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	createPullRequestBrewLong = templates.LongDesc(`
		Creates a Pull Request on a git repository updating any lines in the Dockerfile that start with FROM, ENV or ARG=
`)

	createPullRequestBrewExample = templates.Examples(`
					`)
)

// StepCreatePullRequestBrewOptions contains the command line flags
type StepCreatePullRequestBrewOptions struct {
	StepCreatePrOptions
	Sha string
}

// NewCmdStepCreatePullRequestBrew Creates a new Command object
func NewCmdStepCreatePullRequestBrew(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestBrewOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "brew",
		Short:   "Creates a Pull Request on a git repository updating the homebrew file",
		Long:    createPullRequestBrewLong,
		Example: createPullRequestBrewExample,
		Aliases: []string{"version pullrequest"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	cmd.Flags().StringVarP(&options.Sha, "sha", "", "", "The sha of the brew archive to update")
	return cmd
}

// ValidateOptions validates the common options for brew pr steps
func (o *StepCreatePullRequestBrewOptions) ValidateBrewOptions() error {
	if err := o.ValidateOptions(false); err != nil {
		return errors.WithStack(err)
	}
	if o.Sha == "" {
		return util.MissingOption("sha")
	}
	if o.SrcGitURL == "" {
		log.Logger().Warnf("srcRepo is not provided so generated PR will not be correctly linked in release notesPR")
	}

	return nil
}

// Run implements this command
func (o *StepCreatePullRequestBrewOptions) Run() error {
	if err := o.ValidateBrewOptions(); err != nil {
		return errors.WithStack(err)
	}
	err := o.CreatePullRequest("brew",
		func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
			oldVersions, _, err := brew.UpdateVersionAndSha(dir, o.Version, o.Sha)
			if err != nil {
				return nil, errors.Wrapf(err, "updating version to %s and sha to %s", o.Version, o.Sha)
			}
			return oldVersions, nil
		})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
