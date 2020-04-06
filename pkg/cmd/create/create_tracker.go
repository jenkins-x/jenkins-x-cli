package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// CreateTrackerOptions the options for the create spring command
type CreateTrackerOptions struct {
	options.CreateOptions
}

// NewCmdCreateTracker creates a command object for the "create" command
func NewCmdCreateTracker(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateTrackerOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "tracker",
		Short:   "Creates an issue tracker resource",
		Aliases: []string{"jra", "trello", "issue-tracker"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdCreateTrackerServer(commonOpts))
	cmd.AddCommand(NewCmdCreateTrackerToken(commonOpts))
	return cmd
}

// Run implements this command
func (o *CreateTrackerOptions) Run() error {
	return o.Cmd.Help()
}
