package pr

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type StepPROptions struct {
	step.StepOptions
}

// NewCmdStepPR Steps a command object for the "step pr" command
func NewCmdStepPR(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepPROptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "pr",
		Short: "pipeline step pr",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdStepPRComment(commonOpts))
	cmd.AddCommand(NewCmdStepPRLabels(commonOpts))

	return cmd
}

// Run implements this command
func (o *StepPROptions) Run() error {
	return o.Cmd.Help()
}
