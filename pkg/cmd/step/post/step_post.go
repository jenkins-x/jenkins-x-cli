package post

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type StepPostOptions struct {
	*opts.CommonOptions

	DisableImport bool
	OutDir        string
}

// NewCmdStep Steps a command object for the "step" command
func NewCmdStepPost(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepPostOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "post",
		Short: "post step actions",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdStepPostBuild(commonOpts))
	cmd.AddCommand(NewCmdStepPostInstall(commonOpts))
	cmd.AddCommand(NewCmdStepPostRun(commonOpts))

	return cmd
}

// Run implements this command
func (o *StepPostOptions) Run() error {
	return o.Cmd.Help()
}
