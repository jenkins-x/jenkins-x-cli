package verify

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"
)

// StepVerifyOptions contains the command line flags
type StepVerifyOptions struct {
	step.StepOptions
}

func NewCmdStepVerify(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "verify [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepVerifyBehavior(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyDependencies(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyDNS(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyEnvironments(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyGit(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyIngress(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyInstall(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyPackages(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyPod(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyPreInstall(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyRequirements(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyURL(commonOpts))
	cmd.AddCommand(NewCmdStepVerifyValues(commonOpts))

	return cmd
}

// Run implements this command
func (o *StepVerifyOptions) Run() error {
	return o.Cmd.Help()
}
