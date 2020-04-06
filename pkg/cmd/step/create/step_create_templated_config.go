package create

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"text/template"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/helm/pkg/chartutil"
)

var (
	createTemplatedConfigLong = templates.LongDesc(`
		Creates a config file from a Go template file and a jx requirements file
`)

	createTemplatedConfigExample = templates.Examples(`
		# creates a config file from a template file and a jx requirements file
		jx step create templated config -t config.tmpl.yml -c config.yml
`)
)

const (
	templateFileOption    = "template-file"
	parametersFileOption  = "parameters-file"
	requirementsDirOption = "requirements-dir"
	configFileOption      = "config-file"
)

// StepCreateTemplatedConfigOptions command line flags
type StepCreateTemplatedConfigOptions struct {
	step.StepOptions

	TemplateFile    string
	ParametersFile  string
	RequirementsDir string
	ConfigFile      string
}

// NewCmdStepCreateTemplatedConfig Creates a new Command object
func NewCmdStepCreateTemplatedConfig(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreateTemplatedConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "templated config",
		Short:   "Create a YAML config file from a Go template file and a jx requirements file",
		Long:    createTemplatedConfigLong,
		Example: createTemplatedConfigExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.TemplateFile, templateFileOption, "t", "", "The template file used to render the config YAML file")
	cmd.Flags().StringVarP(&options.ParametersFile, parametersFileOption, "p", "", "The values file used as parameters in the template file")
	cmd.Flags().StringVarP(&options.RequirementsDir, requirementsDirOption, "r", ".", "The jx requirements file directory")
	cmd.Flags().StringVarP(&options.ConfigFile, configFileOption, "c", "", "The rendered config YAML file")

	return cmd
}

func (o *StepCreateTemplatedConfigOptions) checkFlags() error {
	if o.TemplateFile == "" {
		return util.MissingArgument(templateFileOption)
	}

	if o.ParametersFile != "" {
		parametersFile := filepath.Base(o.ParametersFile)
		if parametersFile != helm.ParametersYAMLFile {
			return fmt.Errorf("provided parameters file %q must be named %q", parametersFile, helm.ParametersYAMLFile)
		}
	}

	if exists, err := util.FileExists(o.TemplateFile); err != nil || !exists {
		return fmt.Errorf("template file %q provided in option %q does not exist", o.TemplateFile, templateFileOption)
	}

	if o.ConfigFile == "" {
		// Override the rendered config file with the template file if it is not specified
		o.ConfigFile = o.TemplateFile
	}

	return nil
}

// Run implements this command
func (o *StepCreateTemplatedConfigOptions) Run() error {
	if err := o.checkFlags(); err != nil {
		return err
	}
	requirements, _, err := config.LoadRequirementsConfig(o.RequirementsDir)
	if err != nil {
		return errors.Wrapf(err, "loading requirements file form dir %q", o.RequirementsDir)
	}
	data, err := o.renderTemplate(requirements)
	if err != nil {
		return errors.Wrapf(err, "rendering the config template using the requirements from dir %q", o.RequirementsDir)
	}
	if err := ioutil.WriteFile(o.ConfigFile, data, util.DefaultFileWritePermissions); err != nil {
		return errors.Wrapf(err, "writing the rendered config into file %q", o.ConfigFile)
	}

	log.Logger().Infof("Saved the rendered configuration into %s file", o.ConfigFile)

	return nil
}

func (o *StepCreateTemplatedConfigOptions) renderTemplate(requirements *config.RequirementsConfig) ([]byte, error) {
	templateName := filepath.Base(o.TemplateFile)
	tmpl, err := template.New(templateName).Option("missingkey=error").ParseFiles(o.TemplateFile)
	if err != nil {
		return nil, errors.Wrap(err, "parsing the template file")
	}
	requirementsMap, err := requirements.ToMap()
	if err != nil {
		return nil, errors.Wrapf(err, "converting requirements into a map: %v", requirements)
	}
	params, err := o.parameters()
	if err != nil {
		return nil, errors.Wrapf(err, "loading the parameter values")
	}

	tmplData := map[string]interface{}{
		"Requirements": requirementsMap,
		"Parameters":   params,
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, tmplData)
	if err != nil {
		return nil, errors.Wrapf(err, "rendering the template file: %s", o.TemplateFile)
	}
	return buf.Bytes(), nil
}

// parameters loads the parameters from file without solving the secrets URIs
func (o *StepCreateTemplatedConfigOptions) parameters() (chartutil.Values, error) {
	if o.ParametersFile == "" {
		return chartutil.Values{}, nil
	}
	paramsDir := filepath.Dir(o.ParametersFile)
	return helm.LoadParameters(paramsDir, nil)
}
