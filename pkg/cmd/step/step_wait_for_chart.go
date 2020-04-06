package step

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	optionChartName    = "chart-name"
	optionChartVersion = "chart-version"
	optionChartRepo    = "chart-repo"
	optionRepoUsername = "repo-username"
	optionRepoPassword = "repo-password" // pragma: allowlist secret
)

// WaitForChartOptions contains the command line flags
type WaitForChartOptions struct {
	*step.StepOptions

	ChartName    string
	ChartVersion string
	ChartRepo    string
	RepoUsername string
	RepoPassword string
	Timeout      string
	PollTime     string

	// calculated fields
	TimeoutDuration time.Duration
	PollDuration    time.Duration
}

var (
	// StepWaitForChartLong CLI long description
	StepWaitForChartLong = templates.LongDesc(`
		Waits for the given Chart to be available in a Helm repository

`)
	// StepWaitForChartExample CLI example
	StepWaitForChartExample = templates.Examples(`
		# wait for a chart to be available
		jx step wait-for-chart --chart-name foo --chart-version 1.0.0

`)
)

// NewCmdStepWaitForChart creates the CLI command
func NewCmdStepWaitForChart(commonOpts *opts.CommonOptions) *cobra.Command {
	options := WaitForChartOptions{
		StepOptions: &step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "wait-for-chart",
		Short:   "Waits for the given chart to be available in a helm repository",
		Long:    StepWaitForChartLong,
		Example: StepWaitForChartExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.ChartName, optionChartName, "", "", "Helm chart name to search for [required]")
	cmd.Flags().StringVarP(&options.ChartVersion, optionChartVersion, "", "", "Helm chart version to search for [required]")
	cmd.Flags().StringVarP(&options.ChartRepo, optionChartRepo, "", "https://storage.googleapis.com/chartmuseum.jenkins-x.io", "The repo to search for the helm chart")
	cmd.Flags().StringVarP(&options.RepoUsername, optionRepoUsername, "", "", "Helm Repo username if auth enabled")
	cmd.Flags().StringVarP(&options.RepoPassword, optionRepoPassword, "", "", "Helm Repo password if auth enabled")
	cmd.Flags().StringVarP(&options.Timeout, opts.OptionTimeout, "t", "1h", "The duration before we consider this operation failed")
	cmd.Flags().StringVarP(&options.PollTime, optionPollTime, "", "30s", "The amount of time between polls for the Chart being present")
	return cmd
}

// Run runs the command
func (o *WaitForChartOptions) Run() error {
	var err error
	if o.PollTime != "" {
		o.PollDuration, err = time.ParseDuration(o.PollTime)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.PollTime, optionPollTime, err)
		}
	}
	if o.Timeout != "" {
		o.TimeoutDuration, err = time.ParseDuration(o.Timeout)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.Timeout, opts.OptionTimeout, err)
		}
	}

	if o.ChartName == "" {
		return util.MissingOption(optionChartName)
	}
	if o.ChartVersion == "" {
		return util.MissingOption(optionChartVersion)
	}
	log.Logger().Infof("Waiting for chart %s version %s at %s", util.ColorInfo(o.ChartName), util.ColorInfo(o.ChartVersion), util.ColorInfo(o.ChartRepo))

	dir, err := ioutil.TempDir("", "wait_for_chart")
	if err != nil {
		return errors.Wrap(err, "creating temporary directory")
	}
	defer os.RemoveAll(dir) // clean up

	fn := func() error {
		return o.Helm().FetchChart(o.ChartName, o.ChartVersion, true, dir, o.ChartRepo, o.RepoUsername, o.RepoPassword)
	}

	err = o.RetryQuietlyUntilTimeout(o.TimeoutDuration, o.PollDuration, fn)
	if err == nil {
		log.Logger().Infof("Found chart name %s version %s at %s", util.ColorInfo(o.ChartName), util.ColorInfo(o.ChartVersion), util.ColorInfo(o.ChartRepo))
		return nil
	}
	log.Logger().Warnf("Failed to find chart %s version  %s at %s due to %s", o.ChartName, o.ChartVersion, o.ChartRepo, err)
	return err
}
