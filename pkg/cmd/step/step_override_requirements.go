package step

import (
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// StepOverrideRequirementsOptions contains the command line flags
type StepOverrideRequirementsOptions struct {
	*opts.CommonOptions
	Dir string
}

// NewCmdStepOverrideRequirements creates the `jx step verify pod` command
func NewCmdStepOverrideRequirements(commonOpts *opts.CommonOptions) *cobra.Command {

	options := &StepOverrideRequirementsOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "override-requirements",
		Short: "Overrides requirements with environment variables to be persisted in the `jx-requirements.yml`",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the install requirements file")

	return cmd
}

// Run implements this command
func (o *StepOverrideRequirementsOptions) Run() error {
	requirements, requirementsFileName, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}

	requirements, err = o.overrideRequirements(requirements, requirementsFileName)
	if err != nil {
		return err
	}

	return nil
}

// gatherRequirements gathers cluster requirements and connects to the cluster if required
func (o *StepOverrideRequirementsOptions) overrideRequirements(requirements *config.RequirementsConfig, requirementsFileName string) (*config.RequirementsConfig, error) {
	log.Logger().Debug("Overriding Requirements...")

	requirements.OverrideRequirementsFromEnvironment(func() gke.GClouder {
		return o.GCloud()
	})

	log.Logger().Debugf("saving %s", requirementsFileName)
	requirements.SaveConfig(requirementsFileName)

	return requirements, nil
}
