package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// DeleteJenkinsOptions are the flags for delete commands
type DeleteJenkinsOptions struct {
	*opts.CommonOptions
}

// NewCmdDeleteJenkins creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdDeleteJenkins(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteJenkinsOptions{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "jenkins",
		Short: "Deletes one or more Jenkins resources",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		SuggestFor: []string{"remove", "rm"},
	}

	cmd.AddCommand(NewCmdDeleteJenkinsToken(commonOpts))
	return cmd
}

// Run implements this command
func (o *DeleteJenkinsOptions) Run() error {
	return o.Cmd.Help()
}
