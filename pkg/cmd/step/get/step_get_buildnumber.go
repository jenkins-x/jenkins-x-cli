package get

import (
	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

var (
	getBuildNumberLong = templates.LongDesc(`
		Outputs the current build number from environment variables or using the Downward API inside build pods
`)

	getBuildNumberExample = templates.Examples(`
		# dislay the current build number
		jx step get buildnumber

			`)
)

// StepGetBuildNumberOptions contains the command line flags
type StepGetBuildNumberOptions struct {
	step.StepOptions

	Dir string
}

// NewCmdStepGetBuildNumber Creates a new Command object
func NewCmdStepGetBuildNumber(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepGetBuildNumberOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "buildnumber",
		Short:   "Outputs the current build number from environment variables or using the Downward API inside build pods",
		Long:    getBuildNumberLong,
		Example: getBuildNumberExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

// Run implements this command
func (o *StepGetBuildNumberOptions) Run() error {
	text := builds.GetBuildNumber()
	log.Logger().Infof("%s", text)
	return nil
}
