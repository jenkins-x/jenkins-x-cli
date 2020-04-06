package syntax

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	validatePipeline = templates.Examples(`
		# validates the jenkins-x.yml in the current directory
		jx step syntax validate pipeline

		# validates the jenkins-x-bdd.yml file in the current directory
		jx step syntax validate pipeline --context bdd

			`)
)

// StepSyntaxValidatePipelineOptions contains the command line flags
type StepSyntaxValidatePipelineOptions struct {
	step.StepOptions

	Context string
	Dir     string
}

// NewCmdStepSyntaxValidatePipeline Creates a new Command object
func NewCmdStepSyntaxValidatePipeline(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSyntaxValidatePipelineOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "pipeline",
		Short:   "Validates a pipeline YAML file",
		Long:    "Validates the pipeline YAML file in the current directory for the given context, or jenkins-x.yml by default",
		Example: validatePipeline,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Context, "context", "c", "", "The context for the pipeline YAML to validate instead of the default.")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The directory to query to find the pipeline YAML file")

	return cmd
}

// Run implements this command
func (o *StepSyntaxValidatePipelineOptions) Run() error {
	var err error
	dir := o.Dir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	dirExists, err := util.DirExists(dir)
	if err != nil {
		return errors.Wrapf(err, "error reading directory %s", dir)
	}
	if !dirExists {
		return fmt.Errorf("directory %s does not exist or is not a directory", dir)
	}

	pipelineFileName := "jenkins-x.yml"
	if o.Context != "" {
		pipelineFileName = fmt.Sprintf("jenkins-x-%s.yml", o.Context)
	}

	pipelineFile := filepath.Join(dir, pipelineFileName)
	fileExists, err := util.FileExists(pipelineFile)
	if err != nil {
		return errors.Wrapf(err, "error reading pipeline file %s", pipelineFile)
	}
	if !fileExists {
		return fmt.Errorf("pipeline file %s does not exist or is not a file", pipelineFile)
	}

	data, err := ioutil.ReadFile(pipelineFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to load file %s", pipelineFile)
	}
	validationErrors, err := util.ValidateYaml(&config.ProjectConfig{}, data)
	if err != nil {
		return errors.Wrapf(err, "failed to perform schema validation of pipeline YAML file %s", pipelineFile)
	}
	if len(validationErrors) > 0 {
		log.Logger().Errorf("One or more schema validation errors for %s:", pipelineFile)
		for _, e := range validationErrors {
			log.Logger().Errorf("\t%s", e)
		}
		return errors.New("FAILURE")
	}

	projectConfig := &config.ProjectConfig{}
	err = yaml.Unmarshal(data, projectConfig)
	if err != nil {
		return errors.Wrapf(err, "error loading pipeline YAML file %s", pipelineFile)
	}

	hasErrors := false

	if projectConfig.PipelineConfig != nil {
		if &projectConfig.PipelineConfig.Pipelines != nil {
			for name, lifecycle := range projectConfig.PipelineConfig.Pipelines.AllMap() {
				if lifecycle.Pipeline != nil {
					validateErr := lifecycle.Pipeline.Validate(context.Background())
					if validateErr != nil {
						hasErrors = true
						log.Logger().Errorf("Validation errors in lifecycle %s:\n\t%s", name, validateErr)
					}
				}
			}
		} else {
			log.Logger().Infof("No lifecycles defined in %s", pipelineFile)
		}
	}

	if hasErrors {
		return errors.New("FAILURE")
	}
	log.Logger().Infof("Successfully validated %s", pipelineFile)

	return nil
}
