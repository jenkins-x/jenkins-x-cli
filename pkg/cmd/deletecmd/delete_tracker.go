package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// DeleteTrackerOptions are the flags for delete commands
type DeleteTrackerOptions struct {
	*opts.CommonOptions
}

// NewCmdDeleteTracker creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdDeleteTracker(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteTrackerOptions{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "tracker",
		Short:   "Deletes one or more issue tracker resources",
		Aliases: []string{"jra", "trello", "issue-tracker"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdDeleteTrackerServer(commonOpts))
	cmd.AddCommand(NewCmdDeleteTrackerToken(commonOpts))
	return cmd
}

// Run implements this command
func (o *DeleteTrackerOptions) Run() error {
	return o.Cmd.Help()
}
