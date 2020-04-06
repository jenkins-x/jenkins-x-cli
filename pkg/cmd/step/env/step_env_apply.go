package env

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/platform"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	helm_cmd "github.com/jenkins-x/jx/pkg/cmd/step/helm"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepEnvApplyOptions contains the command line flags
type StepEnvApplyOptions struct {
	StepEnvOptions

	Namespace          string
	Dir                string
	ReleaseName        string
	Wait               bool
	Force              bool
	DisableHelmVersion bool
	ChangeNs           bool
	Vault              bool
}

var (
	// stepEnvApplyLong long description
	stepEnvApplyLong = templates.LongDesc(`
		Applies the GitOps source code (by default in the current directory) to the Environment.

		This command will lazily create an environment, setup Helm and build and apply any helm charts defined in the env/Chart.yaml
`)

	// StepEnvApplyExample example
	stepEnvApplyExample = templates.Examples(`
		# setup and/or update the helm charts for the environment
		jx step env apply --namespace jx-staging
`)
)

// NewCmdStepEnvApply registers the command
func NewCmdStepEnvApply(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepEnvApplyOptions{
		StepEnvOptions: StepEnvOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Applies the GitOps source code to an environment",
		Aliases: []string{""},
		Long:    stepEnvApplyLong,
		Example: stepEnvApplyExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Kubernetes namespace to apply the helm charts to")
	cmd.Flags().StringVarP(&options.ReleaseName, "name", "r", "", "The name of the release")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The directory to look for the environment chart")
	cmd.Flags().BoolVarP(&options.ChangeNs, "change-namespace", "", false, "Set the given namespace as the current namespace in Kubernetes configuration")
	cmd.Flags().BoolVarP(&options.Vault, "vault", "", false, "Environment secrets are stored in vault")

	// step helm apply flags
	cmd.Flags().BoolVarP(&options.Wait, "wait", "", true, "Wait for Kubernetes readiness probe to confirm deployment")
	cmd.Flags().BoolVarP(&options.Force, "force", "f", true, "Whether to to pass '--force' to helm to help deal with upgrading if a previous promote failed")
	cmd.Flags().BoolVar(&options.DisableHelmVersion, "no-helm-version", false, "Don't set Chart version before applying")

	return cmd
}

// Run performs the comamand
func (o *StepEnvApplyOptions) Run() error {
	var err error
	dir := o.Dir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return errors.Wrap(err, "getting the working directory")
		}
	}

	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	o.SetDevNamespace(ns)

	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return errors.Wrap(err, "creating the API extensions client")
	}
	kube.RegisterAllCRDs(apisClient)
	if err != nil {
		return errors.Wrap(err, "registering all CRDs")
	}

	// now lets find the dev environment to know what kind of helmer to use
	chartFile := filepath.Join(dir, helm.ChartFileName)
	exists, err := util.FileExists(chartFile)
	if err != nil {
		return errors.Wrap(err, "checking if file exits")
	}
	if !exists {
		envDir := filepath.Join(dir, "env")
		chartFile2 := filepath.Join(envDir, helm.ChartFileName)
		exists2, err := util.FileExists(chartFile2)
		if exists2 && err == nil {
			dir = envDir
		} else {
			return fmt.Errorf("there is no Environment chart file at %s or %s\nplease try specify the directory containing the Chart.yaml or env/Chart.yaml with --dir", chartFile, chartFile2)
		}
	}
	devEnvFile := filepath.Join(dir, "templates", "dev-env.yaml")
	exists, err = util.FileExists(chartFile)
	if exists && err == nil {
		// lets setup the Helmer based on the current settings
		log.Logger().Infof("Loading the latest Dev Environment configuration from %s", devEnvFile)

		env := v1.Environment{}
		data, err := ioutil.ReadFile(devEnvFile)
		if err != nil {
			return errors.Wrapf(err, "loading configuration file %s", devEnvFile)
		}
		err = yaml.Unmarshal(data, &env)
		if err != nil {
			return errors.Wrapf(err, "unmarshalling YAML file %s", devEnvFile)
		}

		teamSettings := &env.Spec.TeamSettings

		// disable the modify of the Dev Environment lazily...
		o.ModifyDevEnvironmentFn = func(callback func(env *v1.Environment) error) error {
			callback(&env)
			return nil
		}

		helm := o.NewHelm(false, teamSettings.HelmBinary, teamSettings.NoTiller, teamSettings.HelmTemplate)
		o.SetHelm(helm)

		// ensure there's a development namespace setup
		err = kube.EnsureDevNamespaceCreatedWithoutEnvironment(kubeClient, ns)
		if err != nil {
			return errors.Wrapf(err, "creating namespace %s for development environment", ns)
		}

		if o.ReleaseName == "" {
			o.ReleaseName = platform.JenkinsXPlatformRelease
		}
	} else {
		// ensure there's a development namespace setup
		err = kube.EnsureNamespaceCreated(kubeClient, ns, nil, nil)
		if err != nil {
			return errors.Wrapf(err, "creating  namespace %s for environment", ns)
		}
	}

	// Change the current namesapce before applying the environment step
	if o.ChangeNs {
		_, currentNs, err := o.KubeClientAndNamespace()
		if err != nil {
			errors.Wrap(err, "creating the kube client")
		}
		if currentNs != ns {
			nsOptions := &namespace.NamespaceOptions{
				CommonOptions: o.CommonOptions,
			}
			nsOptions.BatchMode = true
			nsOptions.Args = []string{ns}
			err := nsOptions.Run()
			if err != nil {
				log.Logger().Warnf("Failed to set context to namespace %s: %s", ns, err)
			}
			o.ResetClientsAndNamespaces()
		}
	}

	stepHelmBuild := &helm_cmd.StepHelmBuildOptions{
		StepHelmOptions: helm_cmd.StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: o.CommonOptions,
			},
			Dir: dir,
		},
	}
	err = stepHelmBuild.Run()
	if err != nil {
		return errors.Wrapf(err, "building helm chart in dir %s", dir)
	}

	stepApply := &helm_cmd.StepHelmApplyOptions{
		StepHelmOptions:    stepHelmBuild.StepHelmOptions,
		Namespace:          ns,
		ReleaseName:        o.ReleaseName,
		Wait:               o.Wait,
		DisableHelmVersion: o.DisableHelmVersion,
		Force:              o.Force,
		Vault:              o.Vault,
	}
	err = stepApply.Run()
	if err != nil {
		return errors.Wrapf(err, "applying the helm chart in dir %s", dir)
	}
	log.Logger().Infof("Environment applied in namespace %s", util.ColorInfo(ns))
	return nil
}
