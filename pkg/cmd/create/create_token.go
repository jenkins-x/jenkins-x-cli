package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// CreateTokenOptions the options for the create spring command
type CreateTokenOptions struct {
	options.CreateOptions
}

// NewCmdCreateToken creates a command object for the "create" command
func NewCmdCreateToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateTokenOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Creates a new user token for a service",
		Aliases: []string{"api-token", "password", "pwd"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdCreateTokenAddon(commonOpts))
	return cmd
}

// Run implements this command
func (o *CreateTokenOptions) Run() error {
	return o.Cmd.Help()
}
