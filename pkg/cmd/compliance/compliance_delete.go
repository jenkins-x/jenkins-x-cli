package compliance

import (
	"github.com/heptio/sonobuoy/pkg/client"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	complianceDeleteLong = templates.LongDesc(`
		Deletes the Kubernetes resources allocated by the compliance tests
	`)

	complianceDeleteExample = templates.Examples(`
		# Delete the Kubernetes resources allocated by the compliance test
		jx compliance delete
	`)
)

// ComplianceDeleteOptions options for "compliance delete" command
type ComplianceDeleteOptions struct {
	*opts.CommonOptions
}

// NewCmdComplianceDeletecreates a command object for the "compliance delete" action, which
// delete the Kubernetes resources allocated by the compliance tests
func NewCmdComplianceDelete(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ComplianceDeleteOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "delete",
		Short:   "Deletes the Kubernetes resources allocated by the compliance tests",
		Long:    complianceDeleteLong,
		Example: complianceDeleteExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	return cmd
}

// Run implements the "compliance delete" command
func (o *ComplianceDeleteOptions) Run() error {
	cc, err := o.ComplianceClient()
	if err != nil {
		return errors.Wrap(err, "could not create the compliance client")
	}
	deleteOpts := &client.DeleteConfig{
		Namespace:  complianceNamespace,
		EnableRBAC: false,
		DeleteAll:  true,
	}
	return cc.Delete(deleteOpts)
}
