package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

// DeleteJenkinsOptions are the flags for delete commands
type DeleteJenkinsOptions struct {
	CommonOptions
}

// NewCmdDeleteJenkins creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdDeleteJenkins(f Factory, out io.Writer, errOut io.Writer) *cobra.Command {
	options := &DeleteJenkinsOptions{
		CommonOptions{
			Factory: f,
			Out:     out,
			Err:     errOut,
		},
	}

	cmd := &cobra.Command{
		Use:   "jenkins",
		Short: "Deletes one or many jenkins resources",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
		SuggestFor: []string{"remove", "rm"},
	}

	cmd.AddCommand(NewCmdDeleteJenkinsUser(f, out, errOut))
	return cmd
}

// Run implements this command
func (o *DeleteJenkinsOptions) Run() error {
	return o.Cmd.Help()
}
