package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	deleteAddonFlaggerLong = templates.LongDesc(`
		Deletes the Flagger addon
`)

	deleteAddonFlaggerExample = templates.Examples(`
		# Deletes the Flagger addon
		jx delete addon flagger
	`)
)

// DeleteAddonFlaggerOptions the options for the create spring command
type DeleteAddonFlaggerOptions struct {
	DeleteAddonOptions

	ReleaseName string
	Namespace   string
}

// NewCmdDeleteAddonFlagger defines the command
func NewCmdDeleteAddonFlagger(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteAddonFlaggerOptions{
		DeleteAddonOptions: DeleteAddonOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "flagger",
		Short:   "Deletes the Flagger addon",
		Long:    deleteAddonFlaggerLong,
		Example: deleteAddonFlaggerExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.ReleaseName, opts.OptionRelease, "r", "flagger", "The chart release name")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", create.DefaultFlaggerNamespace, "The Namespace to delete from")
	options.addFlags(cmd)
	return cmd
}

// Run implements the command
func (o *DeleteAddonFlaggerOptions) Run() error {
	if o.ReleaseName == "" {
		return util.MissingOption(opts.OptionRelease)
	}
	// Delete from the 'istio-system' namespace, not from 'jx'
	err := o.Helm().DeleteRelease(o.Namespace, o.ReleaseName, o.Purge)
	if err != nil {
		return errors.Wrap(err, "Failed to delete Flagger chart")
	}
	err = o.Helm().DeleteRelease(o.Namespace, o.ReleaseName+"-grafana", o.Purge)
	if err != nil {
		return errors.Wrap(err, "Failed to delete Flagger Grafana chart")
	}
	return err
}
