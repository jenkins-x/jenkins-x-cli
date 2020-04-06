package nexus

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StepNexusDropOptions contains the command line flags
type StepNexusDropOptions struct {
	StepNexusOptions
}

var (
	StepNexusDropLong = templates.LongDesc(`
		This pipeline step command drops a Staging Nexus Repository

`)

	StepNexusDropExample = templates.Examples(`
		jx step nexus drop

`)
)

func NewCmdStepNexusDrop(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepNexusDropOptions{
		StepNexusOptions: StepNexusOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "drop",
		Short:   "Drops a staging nexus release",
		Aliases: []string{"nexus_stage"},
		Long:    StepNexusDropLong,
		Example: StepNexusDropExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	//cmd.Flags().StringVarP(&options.Flags.Version, VERSION, "v", "", "version number for the tag [required]")
	return cmd
}

func (o *StepNexusDropOptions) Run() error {
	repoIds, err := o.findStagingRepoIds()
	if err != nil {
		return err
	}
	if len(repoIds) == 0 {
		log.Logger().Infof("No Nexus staging repository ids found in %s", util.ColorInfo(statingRepositoryProperties))
		return nil
	}
	return o.dropRepositories(repoIds, "Dropping staging repositories")
}
