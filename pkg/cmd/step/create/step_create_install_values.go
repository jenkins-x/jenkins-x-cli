package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	"github.com/spf13/cobra"
)

// NewCmdStepCreateInstallValues Creates a new Command object
func NewCmdStepCreateInstallValues(commonOpts *opts.CommonOptions) *cobra.Command {
	// we cannot use a real alias here as this command is in a different part of the tree
	cmd := verify.NewCmdStepVerifyIngress(commonOpts)
	cmd.Use = "install values"
	return cmd
}
