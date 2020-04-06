package edit

import (
	"fmt"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

var (
	editDeployKindLong = templates.LongDesc(`
		Edits the deploy kind to use for your project or team
`)

	editDeployKindExample = templates.Examples(`
		# Edit the deploy kind for your current project and prompts you to pick one of the available kinds
		jx edit deploy

        # to switch to use Knative Serve deployments
		jx edit deploy knative

        # to switch to normal kubernetes deployments
		jx edit deploy default

        # to switch to use canary deployments (requires flagger and its dependencies)
		jx edit deploy --canary

        # to disable canary deployments and don't ask any more questions
		jx edit deploy --canary=false -b

        # to disable canary deployments and confirm if you want to change the deployment kind and HPA
		jx edit deploy --canary=false

		# Edit the default deploy kind for your team and be prompted for answers
		jx edit deploy --team

		# Set the default for your team to use knative and canary but no HPA
		jx edit deploy --team knative --canary=true --hpa=false
	`)

	deployKinds = []string{opts.DeployKindKnative, opts.DeployKindDefault}

	knativeDeployKey = "knativeDeploy:"
	deployCanaryKey  = "canary:"
	deployHPAKey     = "hpa:"
	enabledKey       = "  enabled:"
)

// EditDeployKindOptions the options for the create spring command
type EditDeployKindOptions struct {
	EditOptions

	Kind          string
	DeployOptions v1.DeployOptions
	Dir           string
	Team          bool
}

// NewCmdEditDeployKind creates a command object for the "create" command
func NewCmdEditDeployKind(commonOpts *opts.CommonOptions) *cobra.Command {
	cmd, _ := NewCmdEditDeployKindAndOption(commonOpts)
	return cmd
}

// NewCmdEditDeployKindAndOption creates a command object for the "create" command
func NewCmdEditDeployKindAndOption(commonOpts *opts.CommonOptions) (*cobra.Command, *EditDeployKindOptions) {
	options := &EditDeployKindOptions{
		EditOptions: EditOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "deploy",
		Short:   "Edits the deploy kind to use for your project or team",
		Long:    editDeployKindLong,
		Example: editDeployKindExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.Team, "team", "t", false, "Edits the team default")
	cmd.Flags().StringVarP(&options.Kind, opts.OptionKind, "k", "", fmt.Sprintf("The kind to use which should be one of: %s", strings.Join(deployKinds, ", ")))
	cmd.Flags().BoolVarP(&options.DeployOptions.Canary, opts.OptionCanary, "", false, "should we use canary rollouts (progressive delivery). e.g. using a Canary deployment via flagger. Requires the installation of flagger and istio/gloo in your cluster")
	cmd.Flags().BoolVarP(&options.DeployOptions.HPA, opts.OptionHPA, "", false, "should we enable the Horizontal Pod Autoscaler.")
	options.Cmd = cmd
	return cmd, options
}

// Run implements the command
func (o *EditDeployKindOptions) Run() error {
	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}
	if o.Team {
		teamDeployOptions := settings.GetDeployOptions()
		name, err := o.pickDeployKind(settings.DeployKind)
		if err != nil {
			return err
		}
		canary, err := o.pickProgressiveDelivery(teamDeployOptions.Canary)
		if err != nil {
			return err
		}
		hpa, err := o.pickHPA(teamDeployOptions.HPA)
		if err != nil {
			return err
		}

		callback := func(env *v1.Environment) error {
			teamSettings := &env.Spec.TeamSettings
			teamSettings.DeployKind = name

			dopt := &v1.DeployOptions{}
			if !canary && !hpa {
				teamSettings.DeployOptions = nil
			} else {
				dopt = &v1.DeployOptions{Canary: canary, HPA: hpa}
				teamSettings.DeployOptions = dopt
			}

			log.Logger().Infof("Setting the team deploy to kind: %s with canary: %s and HPA: %s",
				util.ColorInfo(name), util.ColorInfo(toString(dopt.Canary)), util.ColorInfo(toString(dopt.HPA)))
			return nil
		}
		return o.ModifyDevEnvironment(callback)
	}

	fn := func(text string) (string, error) {
		defaultName, currentDeployOptions := o.FindDefaultDeployKindInValuesYaml(text)
		name := ""

		name, err := o.pickDeployKind(defaultName)
		if err != nil {
			return name, err
		}
		canary, err := o.pickProgressiveDelivery(currentDeployOptions.Canary)
		if err != nil {
			return name, err
		}
		hpa, err := o.pickHPA(currentDeployOptions.HPA)
		if err != nil {
			return name, err
		}
		return o.setDeployKindInValuesYaml(text, name, canary, hpa)
	}
	return o.ModifyHelmValuesFile(o.Dir, fn)
}

// FindDefaultDeployKindInValuesYaml finds the deployment values for the given values.yaml text
func (o *EditDeployKindOptions) FindDefaultDeployKindInValuesYaml(yamlText string) (string, v1.DeployOptions) {
	deployOptions := v1.DeployOptions{}
	// lets try find the current setting
	knativeFlag := ""
	mainSection := ""
	lines := strings.Split(yamlText, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			idx := strings.Index(line, ":")
			if idx > 0 {
				mainSection = line[0 : idx+1]
			}
		}
		if strings.HasPrefix(line, knativeDeployKey) {
			knativeFlag = strings.TrimSpace(line[len(knativeDeployKey):])
		} else if strings.HasPrefix(line, enabledKey) {
			if mainSection == deployCanaryKey {
				deployOptions.Canary = toBool(strings.TrimSpace(line[len(enabledKey):]))
			} else if mainSection == deployHPAKey {
				deployOptions.HPA = toBool(strings.TrimSpace(line[len(enabledKey):]))
			}
		}
	}
	kind := opts.DeployKindDefault
	if knativeFlag == "true" {
		kind = opts.DeployKindKnative
	}
	return kind, deployOptions
}

// setDeployKindInValuesYaml sets the `knativeDeployKey` key to true or false based on the deployment kind
func (o *EditDeployKindOptions) setDeployKindInValuesYaml(yamlText string, deployKind string, progressive bool, hpa bool) (string, error) {
	var buffer strings.Builder

	mainSection := ""
	lines := strings.Split(yamlText, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			idx := strings.Index(line, ":")
			if idx > 0 {
				mainSection = line[0 : idx+1]
			}
		}
		if strings.HasPrefix(line, knativeDeployKey) {
			buffer.WriteString(knativeDeployKey)
			buffer.WriteString(" ")
			if deployKind == opts.DeployKindKnative {
				buffer.WriteString("true")
			} else {
				buffer.WriteString("false")
			}
		} else if strings.HasPrefix(line, enabledKey) {
			if mainSection == deployCanaryKey {
				buffer.WriteString(enabledKey)
				buffer.WriteString(" ")
				buffer.WriteString(toString(progressive))
			} else if mainSection == deployHPAKey {
				buffer.WriteString(enabledKey)
				buffer.WriteString(" ")
				buffer.WriteString(toString(hpa))
			} else {
				buffer.WriteString(line)
			}
		} else {
			buffer.WriteString(line)
		}
		buffer.WriteString("\n")
	}
	return buffer.String(), nil
}

func (o *EditDeployKindOptions) pickDeployKind(defaultName string) (string, error) {
	if defaultName == "" {
		defaultName = opts.DeployKindDefault
	}
	// return the CLI option if specified
	if o.FlagChanged(opts.OptionKind) {
		return o.Kind, nil
	}
	if o.BatchMode {
		return defaultName, nil
	}
	if o.Kind != "" {
		return o.Kind, nil
	}
	args := o.Args
	if len(args) > 0 {
		return args[0], nil
	}
	if util.StringArrayIndex(deployKinds, defaultName) < 0 {
		defaultName = opts.DeployKindDefault
	}
	name, err := util.PickNameWithDefault(deployKinds, "Pick the deployment kind: ", defaultName, "lets you switch between knative serve based deployments and default kubernetes deployments", o.GetIOFileHandles())
	if err != nil {
		return name, err
	}
	if name == "" {
		return name, fmt.Errorf("no kind chosen")
	}
	return name, nil
}

func (o *EditDeployKindOptions) pickProgressiveDelivery(defaultValue bool) (bool, error) {
	// return the CLI option if specified
	if o.FlagChanged(opts.OptionCanary) {
		return o.DeployOptions.Canary, nil
	}
	if o.BatchMode {
		return defaultValue, nil
	}
	return util.Confirm("Would you like to use Canary Delivery", defaultValue, "Canary delivery lets us use Canary rollouts to incrementally test applications", o.GetIOFileHandles())
}

func (o *EditDeployKindOptions) pickHPA(defaultValue bool) (bool, error) {
	// return the CLI option if specified
	if o.FlagChanged(opts.OptionHPA) {
		return o.DeployOptions.HPA, nil
	}
	if o.BatchMode {
		return defaultValue, nil
	}
	return util.Confirm("Would you like to use the Horizontal Pod Autoscaler with deployments", defaultValue, "The Horizontal Pod Autoscaler lets you scale your pods up and down automatically", o.GetIOFileHandles())
}

func toBool(text string) bool {
	return strings.ToLower(text) == "true"
}

func toString(flag bool) string {
	if flag {
		return "true"
	}
	return "false"
}

// ToDeployArguments converts the given deploy kind, canary and HPA to CLI arguments we can parse on the command object
func ToDeployArguments(optionsKind string, kind string, canary bool, hpa bool) []string {
	return []string{"--" + optionsKind + "=" + kind, "--" + opts.OptionCanary + "=" + toString(canary), "--" + opts.OptionHPA + "=" + toString(hpa)}
}
