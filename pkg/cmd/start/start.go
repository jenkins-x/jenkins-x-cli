package start

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// Start contains the command line options
type Start struct {
	*opts.CommonOptions
}

var (
	start_long = templates.LongDesc(`
		Starts a process such as a Jenkins pipeline.
`)

	start_example = templates.Examples(`
		# Start a pipeline
		jx start pipeline foo
	`)
)

// NewCmdStart creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdStart(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &Start{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "start TYPE [flags]",
		Short:   "Starts a process such as a pipeline",
		Long:    start_long,
		Example: start_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		SuggestFor: []string{"begin"},
	}

	cmd.AddCommand(NewCmdStartPipeline(commonOpts))
	cmd.AddCommand(NewCmdStartProtection(commonOpts))
	return cmd
}

// Run implements this command
func (o *Start) Run() error {
	return o.Cmd.Help()
}
