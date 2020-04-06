package nexus

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"

	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

const (
	statingRepositoryIdPrefix = "stagingRepository.id="

	statingRepositoryProperties = "target/nexus-staging/staging/*.properties"
)

// StepNexusOptions contains the command line flags
type StepNexusOptions struct {
	step.StepOptions
}

// NewCmdStepNexus Steps a command object for the "step" command
func NewCmdStepNexus(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepNexusOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "nexus",
		Short: "nexus [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepNexusDrop(commonOpts))
	cmd.AddCommand(NewCmdStepNexusRelease(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepNexusOptions) Run() error {
	return o.Cmd.Help()
}

func (o *StepNexusOptions) findStagingRepoIds() ([]string, error) {
	answer := []string{}
	files, err := filepath.Glob(statingRepositoryProperties)
	if err != nil {
		return answer, err
	}
	for _, f := range files {
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return answer, fmt.Errorf("Failed to load file %s: %s", f, err)
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, statingRepositoryIdPrefix) {
				id := strings.TrimSpace(strings.TrimPrefix(line, statingRepositoryIdPrefix))
				if id != "" {
					answer = append(answer, id)
				}
			}
		}
	}
	return answer, nil
}

func (o *StepNexusOptions) dropRepositories(repoIds []string, message string) error {
	var answer error
	for _, repoId := range repoIds {
		err := o.dropRepository(repoId, message)
		if err != nil {
			log.Logger().Warnf("Failed to drop repository %s: %s", util.ColorInfo(repoIds), util.ColorError(err))
			if answer == nil {
				answer = err
			}
		}
	}
	return answer
}

func (o *StepNexusOptions) dropRepository(repoId string, message string) error {
	if repoId == "" {
		return nil
	}
	log.Logger().Infof("Dropping nexus release repository %s", util.ColorInfo(repoId))
	err := o.RunCommand("mvn",
		"org.sonatype.plugins:nexus-staging-maven-plugin:1.6.5:rc-drop",
		"-DserverId=oss-sonatype-staging",
		"-DnexusUrl=https://oss.sonatype.org",
		"-DstagingRepositoryId="+repoId,
		"-Ddescription=\""+message+"\" -DstagingProgressTimeoutMinutes=60")
	if err != nil {
		log.Logger().Infof("Failed to drop repository %s due to: %s", repoId, err)
	} else {
		log.Logger().Infof("Dropped repository %s", util.ColorInfo(repoId))
	}
	return err
}

func (o *StepNexusOptions) releaseRepository(repoId string) error {
	if repoId == "" {
		return nil
	}
	log.Logger().Infof("Releasing nexus release repository %s", util.ColorInfo(repoId))
	options := o
	err := options.RunCommand("mvn",
		"org.sonatype.plugins:nexus-staging-maven-plugin:1.6.5:rc-release",
		"-DserverId=oss-sonatype-staging",
		"-DnexusUrl=https://oss.sonatype.org",
		"-DstagingRepositoryId="+repoId,
		"-Ddescription=\"Next release is ready\" -DstagingProgressTimeoutMinutes=60")
	if err != nil {
		log.Logger().Warnf("Failed to release repository %s due to: %s", repoId, err)
	} else {
		log.Logger().Infof("Released repository %s", util.ColorInfo(repoId))
	}
	return err
}
