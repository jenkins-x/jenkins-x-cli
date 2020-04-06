package compliance

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/heptio/sonobuoy/pkg/client"
	"github.com/heptio/sonobuoy/pkg/plugin/aggregation"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	complianceStatusLong = templates.LongDesc(`
		Retrieves the current status of the compliance tests
	`)

	complianceStatusExample = templates.Examples(`
		# Get the status
		jx compliance status
	`)
)

// ComplianceStatusOptions options for "compliance status" command
type ComplianceStatusOptions struct {
	*opts.CommonOptions
}

// NewCmdComplianceStatus creates a command object for the "compliance status" action, which
// retrieve the status of E2E compliance tests
func NewCmdComplianceStatus(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ComplianceStatusOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Retrieves the status of compliance tests",
		Long:    complianceStatusLong,
		Example: complianceStatusExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	return cmd
}

// Run implements the "compliance status" command
func (o *ComplianceStatusOptions) Run() error {
	cc, err := o.ComplianceClient()
	if err != nil {
		return errors.Wrap(err, "could not create the compliance client")
	}
	cfg := &client.StatusConfig{
		Namespace: complianceNamespace,
	}
	status, err := cc.GetStatus(cfg)
	if err != nil {
		log.Logger().Infof("No compliance status found. Use %s command to start the compliance tests.", util.ColorInfo("jx compliance run"))
		log.Logger().Infof("You can watch the logs with %s command.", util.ColorInfo("jx compliance logs -f"))
		return nil
	}
	log.Logger().Info(hummanReadableStatus(status.Status))
	return nil
}

func hummanReadableStatus(status string) string {
	switch status {
	case aggregation.RunningStatus:
		return "Compliance tests are still running, it can take up to 60 minutes."
	case aggregation.FailedStatus:
		return "Compliance tests have failed. You can check what happened with `jx compliance results`."
	case aggregation.CompleteStatus:
		return "Compliance tests completed. Use `jx compliance results` to display the results."
	default:
		return fmt.Sprintf("Compliance tests are in unknown state %q.", status)
	}
}
