package nexus

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StepNexusReleaseOptions contains the command line flags
type StepNexusReleaseOptions struct {
	StepNexusOptions

	DropOnFailure bool
}

var (
	StepNexusReleaseLong = templates.LongDesc(`
		This pipeline step command releases a Nexus staging repository
`)

	StepNexusReleaseExample = templates.Examples(`
		jx step nexus release

`)
)

func NewCmdStepNexusRelease(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepNexusReleaseOptions{
		StepNexusOptions: StepNexusOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "release",
		Short:   "Releases a staging nexus release",
		Aliases: []string{"nexus_stage"},
		Long:    StepNexusReleaseLong,
		Example: StepNexusReleaseExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.DropOnFailure, "drop-on-fail", "d", true, "Should we drop the repository on failure")
	return cmd
}

func (o *StepNexusReleaseOptions) Run() error {
	repoIds, err := o.findStagingRepoIds()
	if err != nil {
		return err
	}
	m := map[string]string{}

	if len(repoIds) == 0 {
		log.Logger().Infof("No Nexus staging repository ids found in %s", util.ColorInfo(statingRepositoryProperties))
		return nil
	}
	var answer error
	for _, repoId := range repoIds {
		err = o.releaseRepository(repoId)
		if err != nil {
			m[repoId] = fmt.Sprintf("Failed to release %s due to %s", repoId, err)
			if answer != nil {
				answer = err
			}
		}
	}
	if len(m) > 0 && o.DropOnFailure {
		for repoId, message := range m {
			err = o.dropRepository(repoId, message)
			if answer != nil {
				answer = err
			}
		}
	}
	return answer
}
