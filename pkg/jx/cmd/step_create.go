package cmd

import (
	"github.com/spf13/cobra"
)

// StepCreateOptions contains the command line flags
type StepCreateOptions struct {
	StepOptions
}

// NewCmdStepCreate Steps a command object for the "step" command
func NewCmdStepCreate(commonOpts *CommonOptions) *cobra.Command {
	options := &StepCreateOptions{
		StepOptions: StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "create",
		Short: "create [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepCreateBuild(commonOpts))
	cmd.AddCommand(NewCmdStepCreateBuildTemplate(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepCreateOptions) Run() error {
	return o.Cmd.Help()
}
