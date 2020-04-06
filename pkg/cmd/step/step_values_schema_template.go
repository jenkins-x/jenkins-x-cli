package step

import (
	"os"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	valuesSchemaTemplateLong = templates.LongDesc(`
		Creates a JSON schema from a template
`)

	valuesSchemaTemplateExample = templates.Examples(`
		jx step values schema template

			`)
)

const (
	valuesSchemaJsonConfigMapNameEnvVar = "VALUES_SCHEMA_JSON_CONFIG_MAP_NAME"
)

// StepValuesSchemaTemplateOptions contains the command line flags
type StepValuesSchemaTemplateOptions struct {
	step.StepOptions

	ConfigMapName string
	ConfigMapKey  string
	Set           []string
}

// NewCmdStepValuesSchemaTemplate Creates a new Command object
func NewCmdStepValuesSchemaTemplate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepValuesSchemaTemplateOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "values schema template",
		Short:   "Creates a JSON schema from a template",
		Long:    valuesSchemaTemplateLong,
		Example: valuesSchemaTemplateExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.ConfigMapName, "config-map-name", "", "", "The name of the config map to use, "+
		"by default read from the VALUES_SCHEMA_JSON_CONFIG_MAP_NAME environment variable")
	cmd.Flags().StringVarP(&options.ConfigMapKey, "config-map-key", "", "values.schema.json",
		"The name of the key in the config map which contains values.schema.json")
	cmd.Flags().StringArrayVarP(&options.Set, "set", "", make([]string, 0),
		"the values to pass to the template e.g. --set foo=bar. "+
			"Can be specified multiple times They can be accessed as .Values.<name> e.g. .Values.foo")
	return cmd
}

// Run implements this command
func (o *StepValuesSchemaTemplateOptions) Run() error {
	if o.ConfigMapName == "" {
		o.ConfigMapName = os.Getenv(valuesSchemaJsonConfigMapNameEnvVar)
	}
	if o.ConfigMapKey == "" {
		o.ConfigMapKey = "values.schema.json"
	}
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return errors.Wrapf(err, "getting kube client and ns")
	}
	cm, err := kubeClient.CoreV1().ConfigMaps(ns).Get(o.ConfigMapName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "getting config map %s", o.ConfigMapName)
	}
	tmplStr := cm.Data[o.ConfigMapKey]
	values := make(map[string]interface{})
	for _, v := range o.Set {
		parts := strings.Split(v, "=")
		if len(parts) != 2 {
			return errors.Errorf("cannot parse value %s as key=value", v)
		}
		values[parts[0]] = parts[1]
	}
	tmpl, err := template.New("values_schema_json").Parse(tmplStr)
	if err != nil {
		return errors.Wrapf(err, "parsing %s as go template", tmplStr)
	}
	var out strings.Builder
	data := map[string]map[string]interface{}{
		"Values": values,
	}
	err = tmpl.Execute(&out, data)
	if err != nil {
		return errors.Wrapf(err, "executing template %s with values %v", tmplStr, data)
	}
	cm.Data[o.ConfigMapKey] = out.String()
	_, err = kubeClient.CoreV1().ConfigMaps(ns).Update(cm)
	if err != nil {
		return errors.Wrapf(err, "writing %v to configmap %s", cm, o.ConfigMapName)
	}
	return nil
}
