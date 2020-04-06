package update

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/update/release"
	"github.com/spf13/cobra"
)

// NewCmdStepUpdate Steps a command object for the "step" command
func NewCmdStepUpdate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &step.StepUpdateOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "update",
		Short: "update [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(release.NewCmdStepUpdateRelease(commonOpts))
	return cmd
}

//StepUpdateCommand is the options for NewCmdStepUpdate
type StepUpdateCommand struct {
	step.StepUpdateOptions
}

// Run implements this command
func (o *StepUpdateCommand) Run() error {
	return o.Cmd.Help()
}
