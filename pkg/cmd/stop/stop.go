package stop

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// Stop contains the command line options
type Stop struct {
	*opts.CommonOptions
}

var (
	stopLong = templates.LongDesc(`
		Stops a process such as a Jenkins pipeline.
`)

	stopExample = templates.Examples(`
		# Stop a pipeline
		jx stop pipeline foo
	`)
)

// NewCmdStop creates the command object
func NewCmdStop(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &Stop{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "stop TYPE [flags]",
		Short:   "Stops a process such as a pipeline",
		Long:    stopLong,
		Example: stopExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdStopPipeline(commonOpts))
	return cmd
}

// Run implements this command
func (o *Stop) Run() error {
	return o.Cmd.Help()
}
