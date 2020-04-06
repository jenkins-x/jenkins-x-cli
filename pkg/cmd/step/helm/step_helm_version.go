package helm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/builds"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StepHelmVersionOptions contains the command line flags
type StepHelmVersionOptions struct {
	StepHelmOptions

	Version string
}

var (
	StepHelmVersionLong = templates.LongDesc(`
		Updates version of the Helm Chart.yaml in the given directory 
`)

	StepHelmVersionExample = templates.Examples(`
		# updates the current Helm Chart.yaml to the latest build number version
		jx step helm version

`)
)

func NewCmdStepHelmVersion(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepHelmVersionOptions{
		StepHelmOptions: StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Updates the chart version in the given directory",
		Aliases: []string{""},
		Long:    StepHelmVersionLong,
		Example: StepHelmVersionExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addStepHelmFlags(cmd)

	cmd.Flags().StringVarP(&options.Version, "version", "v", "", "The version to update. If none specified it defaults to $BUILD_NUMBER")

	return cmd
}

func (o *StepHelmVersionOptions) Run() error {
	version := o.Version
	if version == "" {
		version = builds.GetBuildNumber()
	}
	if version == "" {
		return fmt.Errorf("no version specified and could not detect the build number via $BUILD_NUMBER")
	}
	var err error
	dir := o.Dir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	chartFile := filepath.Join(dir, "Chart.yaml")
	exists, err := util.FileExists(chartFile)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no chart exists at %s", chartFile)
	}
	err = helm.SetChartVersion(chartFile, version)
	if err != nil {
		return err
	}
	log.Logger().Infof("Modified file %s to set the chart to version %s", util.ColorInfo(chartFile), util.ColorInfo(version))
	return nil
}
