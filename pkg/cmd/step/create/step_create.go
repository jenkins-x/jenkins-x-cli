package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/create/helmfile"
	"github.com/jenkins-x/jx/pkg/cmd/step/create/pr"
	"github.com/spf13/cobra"
)

// NewCmdStepCreate Steps a command object for the "step" command
func NewCmdStepCreate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &step.StepCreateOptions{
		StepOptions: step.StepOptions{
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
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepCreateDevPodWorkpace(commonOpts))
	cmd.AddCommand(helmfile.NewCmdCreateHelmfile(commonOpts))
	cmd.AddCommand(NewCmdStepCreateJenkinsConfig(commonOpts))
	cmd.AddCommand(NewCmdStepCreateTask(commonOpts))
	cmd.AddCommand(NewCmdStepCreateInstallValues(commonOpts))
	cmd.AddCommand(NewCmdStepCreateValues(commonOpts))
	cmd.AddCommand(pr.NewCmdStepCreatePr(commonOpts))
	cmd.AddCommand(NewCmdStepCreateTemplatedConfig(commonOpts))
	return cmd
}

//StepCreateCommand is the options for NewCmdStepCreate
type StepCreateCommand struct {
	step.StepCreateOptions
}

// Run implements this command
func (o *StepCreateCommand) Run() error {
	return o.Cmd.Help()
}
