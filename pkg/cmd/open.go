package cmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

type OpenOptions struct {
	ConsoleOptions
}

var (
	open_long = templates.LongDesc(`
		Opens a named service in the browser.

		You can use the '--url' argument to just display the URL without opening it`)

	open_example = templates.Examples(`
		# Open the Nexus console in a browser
		jx open jenkins-x-sonatype-nexus

		# Print the Nexus console URL but do not open a browser
		jx open jenkins-x-sonatype-nexus -u

		# List all the service URLs
		jx open`)
)

func NewCmdOpen(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &OpenOptions{
		ConsoleOptions: ConsoleOptions{
			GetURLOptions: get.GetURLOptions{
				GetOptions: get.GetOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "open",
		Short:   "Open a service in a browser",
		Long:    open_long,
		Example: open_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addConsoleFlags(cmd)
	return cmd
}

func (o *OpenOptions) Run() error {
	if len(o.Args) == 0 {
		return o.GetURLOptions.Run()
	}
	name := o.Args[0]
	return o.ConsoleOptions.Open(name, name)
}
