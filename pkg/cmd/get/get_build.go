package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// GetBuildOptions the command line options
type GetBuildOptions struct {
	*opts.CommonOptions

	Output string
}

var (
	get_build_long = templates.LongDesc(`
		Display one or more resources.

		` + valid_resources + `

`)

	get_build_example = templates.Examples(`
		# List all pipelines
		jx get pipeline

		# List all URLs for services in the current namespace
		jx get url
	`)
)

// NewCmdGetBuild creates the command object
func NewCmdGetBuild(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetBuildOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "build [flags]",
		Short:   "Display one or more build resources",
		Long:    get_build_long,
		Example: get_build_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		SuggestFor: []string{"list", "ps"},
	}

	cmd.AddCommand(NewCmdGetBuildLogs(commonOpts))
	cmd.AddCommand(NewCmdGetBuildPods(commonOpts))
	return cmd
}

// Run implements this command
func (o *GetBuildOptions) Run() error {
	return o.Cmd.Help()
}
