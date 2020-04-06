package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	deleteAddonEnvironmentControllerLong = templates.LongDesc(`
		Deletes the Environment Controller
`)

	deleteAddonEnvironmentControllerExample = templates.Examples(`
		# Deletes the environment controller
		jx delete addon envctl
	`)
)

// DeleteAddonEnvironmentControllerOptions the options for the create spring command
type DeleteAddonEnvironmentControllerOptions struct {
	DeleteAddonOptions

	ReleaseName string
	Namespace   string
}

// NewCmdDeleteAddonEnvironmentController creates a command object for the "create" command
func NewCmdDeleteAddonEnvironmentController(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteAddonEnvironmentControllerOptions{
		DeleteAddonOptions: DeleteAddonOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "environment controller",
		Short:   "Deletes the Environment Controller ",
		Aliases: []string{"envctl"},
		Long:    deleteAddonEnvironmentControllerLong,
		Example: deleteAddonEnvironmentControllerExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.ReleaseName, opts.OptionRelease, "r", create.DefaultEnvCtrlReleaseName, "The chart release name")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", create.DefaultEnvCtrlNamespace, "The Namespace to delete from")
	options.addFlags(cmd)
	return cmd
}

// Run implements the command
func (o *DeleteAddonEnvironmentControllerOptions) Run() error {
	o.EnableRemoteKubeCluster()

	if o.ReleaseName == "" {
		return util.MissingOption(opts.OptionRelease)
	}
	err := o.Helm().DeleteRelease(o.Namespace, o.ReleaseName, o.Purge)
	if err != nil {
		return errors.Wrap(err, "Failed to delete environment-controller chart")
	}
	log.Logger().Infof("Addon %s deleted successfully", util.ColorInfo(o.ReleaseName))

	return nil

}
