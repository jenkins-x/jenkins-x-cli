package compliance

import (
	"github.com/heptio/sonobuoy/pkg/client"
	"github.com/heptio/sonobuoy/pkg/config"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
)

var (
	complianceRuntLong = templates.LongDesc(`
		Runs the compliance tests
	`)

	complianceRunExample = templates.Examples(`
		# Run the compliance tests
		jx compliance run
	`)
)

// ComplianceRunOptions options for "compliance run" command
type ComplianceRunOptions struct {
	*opts.CommonOptions
}

// NewCmdComplianceRun creates a command object for the "compliance run" action, which
// starts the E2E compliance tests
func NewCmdComplianceRun(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ComplianceRunOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "run",
		Short:   "Runs the compliance tests",
		Long:    complianceRuntLong,
		Example: complianceRunExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	return cmd
}

// Run implements the "compliance run" command
func (o *ComplianceRunOptions) Run() error {
	cc, err := o.ComplianceClient()
	if err != nil {
		return errors.Wrap(err, "could not create the compliance client")
	}
	cfg := o.config()
	if err := cc.Run(cfg); err != nil {
		return errors.Wrap(err, "failed to start the compliance tests")
	}

	log.Logger().Infof("Compliance tests started in namespace %q", complianceNamespace)

	return nil
}

func (o *ComplianceRunOptions) config() *client.RunConfig {
	modeName := client.CertifiedConformance
	mode := modeName.Get()
	genCfg := &client.GenConfig{
		E2EConfig:            &mode.E2EConfig,
		Config:               o.getConfigWithMode(modeName),
		EnableRBAC:           true,
		ImagePullPolicy:      string(v1.PullAlways),
		KubeConformanceImage: kubeConformanceImage,
	}
	return &client.RunConfig{
		GenConfig: *genCfg,
	}
}

func (o *ComplianceRunOptions) getConfigWithMode(mode client.Mode) *config.Config {
	cfg := config.New()
	modeConfig := mode.Get()
	if modeConfig != nil {
		cfg.PluginSelections = modeConfig.Selectors
		cfg.Namespace = complianceNamespace
		cfg.WorkerImage = complianceWorkerImage
	}
	return cfg
}
