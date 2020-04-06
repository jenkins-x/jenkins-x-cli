package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepGetOptions contains the command line flags
type StepGetOptions struct {
	step.StepOptions
}

// NewCmdStepGet Steps a command object for the "step" command
func NewCmdStepGet(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepGetOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "get",
		Short: "get [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepGetBuildNumber(commonOpts))
	cmd.AddCommand(NewCmdStepGetVersionChangeSet(commonOpts))
	cmd.AddCommand(NewCmdStepGetDependencyVersion(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepGetOptions) Run() error {
	return o.Cmd.Help()
}
