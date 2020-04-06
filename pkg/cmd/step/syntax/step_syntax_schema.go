package syntax

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

// StepSyntaxSchemaOptions contains the command line flags
type StepSyntaxSchemaOptions struct {
	step.StepOptions

	Pipeline     bool
	BuildPack    bool
	Requirements bool
	Pod          bool
	Out          string
}

// NewCmdStepSyntaxSchema Steps a command object for the "step" command
func NewCmdStepSyntaxSchema(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSyntaxSchemaOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "schema",
		Short:   "Output the JSON schema either for jenkins-x.yml files or for build packs' pipeline.yaml files",
		Example: "schema --pipeline",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Out, "out", "o", "", "the name of the output file for the generated JSON schema")
	cmd.Flags().BoolVarP(&options.Pipeline, "pipeline", "", false, "Output the JSON schema for jenkins-x.yml files. Defaults to this option if '--buildpack' is not specified")
	cmd.Flags().BoolVarP(&options.BuildPack, "buildpack", "", false, "Output the JSON schema for build pack pipeline.yaml files")
	cmd.Flags().BoolVarP(&options.Requirements, "requirements", "", false, "Output the JSON schema for jx-requirements.yml files")
	cmd.Flags().BoolVarP(&options.Pod, "pod", "", false, "Output the JSON schema for k8s Pod files")

	return cmd
}

// Run implements this command
func (o *StepSyntaxSchemaOptions) Run() error {
	if o.Requirements == false && o.Pipeline == false && o.BuildPack == false && o.Pod == false {
		// lets default to pipeine
		o.Pipeline = true
	}

	var schemaName string
	var schemaTarget interface{}

	if o.Requirements {
		schemaName = "jx-requirements.yml"
		schemaTarget = &config.RequirementsConfig{}
	} else if o.Pipeline {
		schemaName = "jenkins-x.yml"
		schemaTarget = &config.ProjectConfig{}
	} else if o.Pod {
		schemaName = "pod.yml"
		schemaTarget = &corev1.PodSpec{}
	} else if o.BuildPack {
		if o.Pipeline {
			return errors.New("only one of --pipeline or --buildpack may be specified")
		}
		schemaName = "pipeline.yaml"
		schemaTarget = &jenkinsfile.PipelineConfig{}
	}

	schema := util.GenerateSchema(schemaTarget)
	if schema == nil {
		return fmt.Errorf("could not generate schema for %s", schemaName)
	}

	output := prettyPrintJSON(schema)

	if output == "" {
		tempOutput, err := json.Marshal(schema)
		if err != nil {
			return errors.Wrapf(err, "error outputting schema for %s", schemaName)
		}
		output = string(tempOutput)
	}
	log.Logger().Infof("JSON schema for %s:", schemaName)

	if o.Out != "" {
		err := ioutil.WriteFile(o.Out, []byte(output), util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to save file %s", o.Out)
		}
		log.Logger().Infof("wrote file %s", util.ColorInfo(o.Out))
		return nil
	}
	log.Logger().Infof("%s", output)
	return nil
}

func prettyPrintJSON(input interface{}) string {
	output := &bytes.Buffer{}
	if err := json.NewEncoder(output).Encode(input); err != nil {
		return ""
	}
	formatted := &bytes.Buffer{}
	if err := json.Indent(formatted, output.Bytes(), "", "  "); err != nil {
		return ""
	}
	return string(formatted.Bytes())
}
