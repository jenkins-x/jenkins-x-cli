package cluster

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepClusterOptions contains the command line flags and other helper objects
type StepClusterOptions struct {
	step.StepOptions
	ClusterOptions opts.ClusterOptions
}

// NewCmdStepCluster Creates a new Command object
func NewCmdStepCluster(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepClusterOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "cluster [kind]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepClusterLabel(commonOpts))
	cmd.AddCommand(NewCmdStepClusterLock(commonOpts))
	cmd.AddCommand(NewCmdStepClusterUnlock(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepClusterOptions) Run() error {
	return o.Cmd.Help()
}
