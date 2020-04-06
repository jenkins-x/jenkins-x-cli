package report

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepReportOptions contains the command line flags and other helper objects
type StepReportOptions struct {
	step.StepOptions
	OutputDir string
}

// NewCmdStepReport Creates a new Command object
func NewCmdStepReport(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepReportOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "report",
		Short: "report [kind]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepReportChart(commonOpts))
	cmd.AddCommand(NewCmdStepReportImageVersion(commonOpts))
	cmd.AddCommand(NewCmdStepReportJUnit(commonOpts))
	cmd.AddCommand(NewCmdStepReportVersion(commonOpts))
	return cmd
}

// AddReportFlags adds common report flags
func (o *StepReportOptions) AddReportFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.OutputDir, "out-dir", "o", "", "The directory to store the resulting reports in")
}

// Run implements this command
func (o *StepReportOptions) Run() error {
	return o.Cmd.Help()
}

// OutputReport outputs the report to the terminal or a file
func (o *StepReportOptions) OutputReport(report interface{}, fileName string, outputDir string) error {
	data, err := yaml.Marshal(report)
	if err != nil {
		return errors.Wrap(err, "failed to marshal report to YAML")
	}
	if fileName == "" {
		log.Logger().Infof(string(data))
		return nil
	}
	if outputDir == "" {
		outputDir = "."
	}
	err = os.MkdirAll(outputDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrap(err, "failed to create directories")
	}
	yamlFile := filepath.Join(outputDir, fileName)
	err = ioutil.WriteFile(yamlFile, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save report file %s", yamlFile)
	}
	log.Logger().Infof("generated report at %s", util.ColorInfo(yamlFile))
	return nil
}
