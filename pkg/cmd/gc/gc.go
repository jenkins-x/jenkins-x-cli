package gc

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// GCOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type GCOptions struct {
	*opts.CommonOptions

	Output string
}

const (
	valid_gc_resources = `Valid resource types include:

    * activities
	* helm
	* previews
	* releases
    `
)

var (
	gc_long = templates.LongDesc(`
		Garbage collect resources

		` + valid_gc_resources + `

`)

	gc_example = templates.Examples(`
		jx gc activities
		jx gc gke
		jx gc helm
		jx gc previews
		jx gc releases

	`)
)

// NewCmdGC creates a command object for the generic "gc" action, which
// retrieves one or more resources from a server.
func NewCmdGC(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GCOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "gc TYPE [flags]",
		Short:   "Garbage collects Jenkins X resources",
		Long:    gc_long,
		Example: gc_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdGCActivities(commonOpts))
	cmd.AddCommand(NewCmdGCPreviews(commonOpts))
	cmd.AddCommand(NewCmdGCGKE(commonOpts))
	cmd.AddCommand(NewCmdGCHelm(commonOpts))
	cmd.AddCommand(NewCmdGCPods(commonOpts))
	cmd.AddCommand(NewCmdGCReleases(commonOpts))

	return cmd
}

// Run implements this command
func (o *GCOptions) Run() error {
	return o.Cmd.Help()
}
