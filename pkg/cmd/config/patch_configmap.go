package config

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	k8sv1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	stepModifyConfigMapLong = templates.LongDesc(`
		This step will take a json patch and attempt to modify the given ConfigMap. This is supposed to be used by
		Helm hooks to modify certain configuration values depending on the chart needs
`)

	stepModifyConfigMapExample = templates.Examples(`
		# Update the plank property in the data section of a config map, which is an embedded yaml file as a string literal called config.yaml:
		jx step patch-config -m config --first-level-property config.yaml -p '["op": "replace", "path": "/plank", "value": {"foo": "bar"}]'
		
		# Update a root level property of a config map using strategic merge:
		jx step patch-config -m config -t strategic -p '{"metadata": {"initializers": {"result": {"status": "newstatus"}}}}'
			`)
)

// StepModifyConfigMapOptions are the options for the step patch-config command
type StepModifyConfigMapOptions struct {
	step.StepOptions
	ConfigMapName      string
	JSONPatch          string
	FirstLevelProperty string
	Type               string
	OutputFormat       string
}

// NewCmdStepPatchConfigMap executes the patch-config step, which applies JSONPatches to ConfigMaps
func NewCmdStepPatchConfigMap(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepModifyConfigMapOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "patch-config",
		Short:   "Modifies a ConfigMap with the given json patch",
		Long:    stepModifyConfigMapLong,
		Example: stepModifyConfigMapExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.ConfigMapName, "config-map", "m", "", "The ConfigMap that will be modified")
	cmd.Flags().StringVarP(&options.FirstLevelProperty, "first-level-property", "", "", "The first level property within \"Data:\" where the json patch will be applied. If left empty, the patch will be applied to the whole ConfigMap")
	cmd.Flags().StringVarP(&options.JSONPatch, "patch", "p", "", "The Json patch that will be applied to the Data within the ConfigMap")
	cmd.Flags().StringVarP(&options.Type, "type", "t", "", "The type of patch being provided; one of [json merge strategic] - If the \"first-level-property\" flag is provided, this has no effect")
	cmd.Flags().StringVarP(&options.OutputFormat, "output", "o", "", "the output of the modified ConfigMap if dry-run was provided")

	return cmd
}

// Run implements this command
func (o *StepModifyConfigMapOptions) Run() error {

	if o.JSONPatch == "" {
		return fmt.Errorf("the json patch to apply can't be empty")
	}

	if o.ConfigMapName == "" {
		return fmt.Errorf("the config map to apply the patch into can't be empty")
	}

	jsonPatch := []byte(o.JSONPatch)
	var updatedConfig *v1.ConfigMap
	var err error
	if o.FirstLevelProperty != "" {
		updatedConfig, err = o.applyPatchToFirstLevelProperty(jsonPatch)
		if err != nil {
			return err
		}
	} else {
		updatedConfig, err = o.applyPatchToRoot(jsonPatch)
		if err != nil {
			return err
		}
	}

	if o.OutputFormat != "" {
		err := renderResult(updatedConfig, o.OutputFormat)
		if err != nil {
			return errors.Wrap(err, "there was a problem rendering the output")
		}
	}

	return nil
}

func (o *StepModifyConfigMapOptions) applyPatchToRoot(jsonPatch []byte) (*v1.ConfigMap, error) {
	kubeClient, ns, err := o.KubeClientAndNamespace()
	var patch types.PatchType
	switch o.Type {
	case "json":
		patch = types.JSONPatchType
	case "merge":
		patch = types.MergePatchType
	case "strategic":
		patch = types.StrategicMergePatchType
	default:
		return nil, errors.New("the provided type is not supported. Please use one of [json merge strategic]")
	}
	updatedConfig, err := kubeClient.CoreV1().ConfigMaps(ns).Patch(o.ConfigMapName, patch, jsonPatch)
	if err != nil {
		return nil, err
	}
	return updatedConfig, nil
}

func (o *StepModifyConfigMapOptions) applyPatchToFirstLevelProperty(jsonPatch []byte) (*v1.ConfigMap, error) {
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem obtaining the Kubernetes client")
	}
	config, err := kubeClient.CoreV1().ConfigMaps(ns).Get(o.ConfigMapName, k8sv1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "there was a problem obtaining the %s ConfigMap", o.ConfigMapName)
	}

	data := config.Data[o.FirstLevelProperty]

	orig := []byte(data)
	orig, err = yaml.YAMLToJSON(orig)
	if err != nil {
		return nil, err
	}

	fmt.Println(o.JSONPatch)

	patch, err := jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return nil, err
	}

	b, err := patch.Apply(orig)
	if err != nil {
		return nil, err
	}

	yamlData, err := yaml.JSONToYAML(b)
	if err != nil {
		return nil, err
	}

	config.Data[o.FirstLevelProperty] = string(yamlData)

	updatedConfig, err := kubeClient.CoreV1().ConfigMaps(ns).Update(config)
	if err != nil {
		return nil, err
	}

	if o.OutputFormat != "" {
		err = renderResult(updatedConfig, o.OutputFormat)
		if err != nil {
			return nil, errors.Wrap(err, "there was a problem rendering the output")
		}
	}

	return updatedConfig, nil
}

func renderResult(value interface{}, format string) error {
	switch format {
	case "json":
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		_, e := fmt.Println(string(data))
		return e
	case "yaml":
		data, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		_, e := fmt.Println(string(data))
		return e
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}
