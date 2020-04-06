package add

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// AddOptions contains the command line options
type AddOptions struct {
	*opts.CommonOptions

	DisableImport bool
	OutDir        string
}

var (
	add_resources = `Valid resource types include:

	* app
    `

	add_long = templates.LongDesc(`
		Adds a new resource.

		` + add_resources + `
`)
)

// NewCmdAdd creates a command object for the "add" command
func NewCmdAdd(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &AddOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Adds a new resource",
		Long:  add_long,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdAddApp(commonOpts))
	return cmd
}

// Run implements this command
func (o *AddOptions) Run() error {
	return o.Cmd.Help()
}
