package cmd

import (
	"github.com/jenkins-x/jx/pkg/jx/cmd/opts"
	"github.com/spf13/cobra"
)

// StepSyntaxValidateOptions contains the command line flags
type StepSyntaxValidateOptions struct {
	StepOptions
}

// NewCmdStepSyntaxValidate Steps a command object for the "step" command
func NewCmdStepSyntaxValidate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSyntaxValidateOptions{
		StepOptions: StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "validate [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepSyntaxValidateBuildPacks(commonOpts))
	cmd.AddCommand(NewCmdStepSyntaxValidatePipeline(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepSyntaxValidateOptions) Run() error {
	return o.Cmd.Help()
}
