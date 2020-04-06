package restore

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepRestoreOptions contains the command line flags
type StepRestoreOptions struct {
	step.StepOptions
}

// NewCmdStepRestore performs the command setup
func NewCmdStepRestore(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepRestoreOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "restore",
		Short: "restore [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepRestoreFromBackup(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepRestoreOptions) Run() error {
	return o.Cmd.Help()
}
