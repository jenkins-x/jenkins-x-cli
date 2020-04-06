package syntax

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	repoNames = []string{
		"classic",
		"kubernetes",
	}
)

// StepSyntaxValidateBuildPacksOptions contains the command line flags
type StepSyntaxValidateBuildPacksOptions struct {
	step.StepOptions
}

// NewCmdStepSyntaxValidateBuildPacks Creates a new Command object
func NewCmdStepSyntaxValidateBuildPacks(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSyntaxValidateBuildPacksOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "buildpacks",
		Short:   "Validates all available build packs",
		Example: "buildpacks",
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
func (o *StepSyntaxValidateBuildPacksOptions) Run() error {
	var err error

	draftDir, err := util.DraftDir()
	if err != nil {
		return errors.Wrapf(err, "couldn't get root directory of build packs")
	}

	var packNames []string

	errorsByPack := make(map[string][]string)

	for _, repo := range repoNames {
		packsDir := filepath.Join(draftDir, "packs", "github.com", "jenkins-x-buildpacks", fmt.Sprintf("jenkins-x-%s", repo), "packs")
		exists, err := util.DirExists(packsDir)
		if err != nil {
			return errors.Wrapf(err, "error reading packs dir %s", packsDir)
		}
		if !exists {
			return fmt.Errorf("packs directory %s does not exist or is not a directory", packsDir)
		}
		packs, err := ioutil.ReadDir(packsDir)
		if err != nil {
			return errors.Wrapf(err, "there was an error reading %s", packsDir)
		}
		for _, file := range packs {
			if file.IsDir() {
				yamlFile := filepath.Join(packsDir, file.Name(), jenkinsfile.PipelineConfigFileName)
				exists, err := util.FileExists(yamlFile)
				if err != nil {
					return errors.Wrapf(err, "error reading %s", yamlFile)
				}
				if exists {
					data, err := ioutil.ReadFile(yamlFile)
					if err != nil {
						return errors.Wrapf(err, "Failed to load file %s", yamlFile)
					}
					validationErrors, err := util.ValidateYaml(&jenkinsfile.PipelineConfig{}, data)
					if err != nil {
						return errors.Wrapf(err, "failed to validate YAML file %s", yamlFile)
					}
					packID := fmt.Sprintf("%s: %s", repo, file.Name())
					packNames = append(packNames, packID)
					if len(validationErrors) > 0 {
						errorsByPack[packID] = validationErrors
					}
				}
			}
		}
	}
	hasError := false
	sort.Strings(packNames)

	for _, k := range packNames {
		v, exists := errorsByPack[k]
		if !exists {
			log.Logger().Infof("SUCCESS: %s", k)
		} else {
			hasError = true
			log.Logger().Errorf("FAILURE: %s", k)
			for _, e := range v {
				log.Logger().Errorf("\t%s", e)
			}
		}
	}

	if hasError {
		return errors.New("one or more build packs failed validation")
	}
	return nil
}
