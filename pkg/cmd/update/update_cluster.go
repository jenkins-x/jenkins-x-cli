package update

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// UpdateClusterOptions the flags for running update cluster
type UpdateClusterOptions struct {
	UpdateOptions
	InitOption initcmd.InitOptions
	Flags      initcmd.InitFlags
	Provider   string
}

type UpdateClusterFlags struct {
}

var (
	updateClusterLong = templates.LongDesc(`
		This command updates an existing Kubernetes cluster, it can be used to apply minor changes to a cluster / node pool.
`)

	updateClusterExample = templates.Examples(`

		jx update cluster gke

`)
)

// NewCmdGet creates a command object for the generic "init" action, which
// installs the dependencies required to run the jenkins-x platform on a Kubernetes cluster.
func NewCmdUpdateCluster(commonOpts *opts.CommonOptions) *cobra.Command {
	options := createUpdateClusterOptions(commonOpts, "")

	cmd := &cobra.Command{
		Use:     "cluster [kubernetes provider]",
		Short:   "Updates an existing Kubernetes cluster",
		Long:    fmt.Sprintf(updateClusterLong),
		Example: updateClusterExample,
		Run: func(cmd2 *cobra.Command, args []string) {
			options.Cmd = cmd2
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdUpdateClusterGKE(commonOpts))

	return cmd
}

func createUpdateClusterOptions(commonOpts *opts.CommonOptions, cloudProvider string) UpdateClusterOptions {
	options := UpdateClusterOptions{
		UpdateOptions: UpdateOptions{
			CommonOptions: commonOpts,
		},
		Provider: cloudProvider,
	}
	return options
}

func (o *UpdateClusterOptions) Run() error {
	return o.Cmd.Help()
}
