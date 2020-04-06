package release

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

//StepUpdateReleaseOptions are the common options for all update steps
type StepUpdateReleaseOptions struct {
	step.StepUpdateOptions
	Owner      string
	Repository string
	Version    string
}

// NewCmdStepUpdateRelease Steps a command object for the "step" command
func NewCmdStepUpdateRelease(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepUpdateReleaseOptions{
		StepUpdateOptions: step.StepUpdateOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "release-status",
		Aliases: []string{""},
		Short:   "update release-status [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepUpdateReleaseGitHub(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepUpdateReleaseOptions) Run() error {
	return o.Cmd.Help()
}

//AddStepUpdateReleaseFlags adds the common flags for all update release steps to the cmd and stores them in o
func AddStepUpdateReleaseFlags(cmd *cobra.Command, o *StepUpdateReleaseOptions) {
	cmd.Flags().StringVarP(&o.Owner, "owner", "", "o", "The owner of the git repository")
	cmd.Flags().StringVarP(&o.Repository, "repository", "r", "", "The git repository")
	cmd.Flags().StringVarP(&o.Version, "version", "v", "", "The version to udpate. If no version is found an error is returned")
}

// ValidateOptions validates the common options for all PR creation steps
func (o *StepUpdateReleaseOptions) ValidateOptions() error {
	if o.Owner == "" {
		return util.MissingOption("owner")
	}
	if o.Repository == "" {
		return util.MissingOption("repository")
	}
	if o.Version == "" {
		return util.MissingOption("version")
	}
	return nil
}
