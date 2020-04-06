package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// DeleteGitOptions are the flags for delete commands
type DeleteGitOptions struct {
	*opts.CommonOptions
}

// NewCmdDeleteGit creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdDeleteGit(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteGitOptions{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "git",
		Short: "Deletes one or more Git resources",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		SuggestFor: []string{"remove", "rm"},
	}

	cmd.AddCommand(NewCmdDeleteGitServer(commonOpts))
	cmd.AddCommand(NewCmdDeleteGitToken(commonOpts))
	return cmd
}

// Run implements this command
func (o *DeleteGitOptions) Run() error {
	return o.Cmd.Help()
}
