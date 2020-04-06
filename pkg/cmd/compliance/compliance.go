package compliance

import (
	"github.com/heptio/sonobuoy/pkg/buildinfo"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// TODO change the name of the namespace to something more jx specific (e.g. jx-compliance), but
// at this time the Sonobuoy does not run properly into a custom namespace.
const complianceNamespace = "heptio-sonobuoy"

// kubeConformanceImage is the URL of the docker image to run for the kube conformance tests
const kubeConformanceImage = "gcr.io/heptio-images/kube-conformance:latest"

// compliance is the URL of the docker image to run for the Sonobuoy aggregator and workers
var complianceWorkerImage = "gcr.io/heptio-images/sonobuoy:" + buildinfo.Version

// ComplianceOptions options for compliance command
type ComplianceOptions struct {
	*opts.CommonOptions
}

// NewCompliance creates a command object for the generic "compliance" action, which
// executes the compliance tests against a Kubernetes cluster
func NewCompliance(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ComplianceOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "compliance ACTION [flags]",
		Short: "Run compliance tests against Kubernetes cluster",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdComplianceStatus(commonOpts))
	cmd.AddCommand(NewCmdComplianceResults(commonOpts))
	cmd.AddCommand(NewCmdComplianceRun(commonOpts))
	cmd.AddCommand(NewCmdComplianceDelete(commonOpts))
	cmd.AddCommand(NewCmdComplianceLogs(commonOpts))

	return cmd
}

// Run implements the compliance root command
func (o *ComplianceOptions) Run() error {
	return o.Cmd.Help()
}
