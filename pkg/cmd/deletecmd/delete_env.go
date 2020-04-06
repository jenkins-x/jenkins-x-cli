package deletecmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	delete_env_long = templates.LongDesc(`
		Deletes one or more environments.
`)

	delete_env_example = templates.Examples(`
		# Deletes an environment
		jx delete env staging
	`)
)

// DeleteEnvOptions the options for the create spring command
type DeleteEnvOptions struct {
	*opts.CommonOptions

	DeleteNamespace bool
}

// NewCmdDeleteEnv creates a command object for the "delete repo" command
func NewCmdDeleteEnv(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteEnvOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "environment",
		Short:   "Deletes one or more Environments",
		Aliases: []string{"env"},
		Long:    delete_env_long,
		Example: delete_env_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	//addDeleteFlags(cmd, &options.CreateOptions)

	cmd.Flags().BoolVarP(&options.DeleteNamespace, "namespace", "n", false, "Delete the namespace for the Environment too?")
	return cmd
}

// Run implements the command
func (o *DeleteEnvOptions) Run() error {
	jxClient, currentNs, err := o.JXClient()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	kube.RegisterEnvironmentCRD(apisClient)

	ns, currentEnv, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}

	envMap, envNames, err := kube.GetEnvironments(jxClient, ns)
	if err != nil {
		return err
	}
	name := ""
	args := o.Args
	if len(args) > 0 {
		for _, arg := range args {
			if util.StringArrayIndex(envNames, arg) < 0 {
				return util.InvalidArg(arg, envNames)
			}
		}
		for _, arg := range args {
			err = o.deleteEnviroment(jxClient, ns, arg, envMap)
			if err != nil {
				return err
			}
		}
	} else {
		name, err = kube.PickEnvironment(envNames, currentEnv, o.GetIOFileHandles())
		if err != nil {
			return err
		}
		err = o.deleteEnviroment(jxClient, ns, name, envMap)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *DeleteEnvOptions) deleteEnviroment(jxClient versioned.Interface, ns string, name string, envMap map[string]*v1.Environment) error {
	err := jxClient.JenkinsV1().Environments(ns).Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Logger().Infof("Deleted environment %s", util.ColorInfo(name))

	env := envMap[name]
	envNs := env.Spec.Namespace
	if envNs == "" {
		return fmt.Errorf("No namespace for environment %s", name)
	}
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	kind := env.Spec.Kind
	if o.DeleteNamespace || !kind.IsPermanent() {
		return client.CoreV1().Namespaces().Delete(envNs, &metav1.DeleteOptions{})
	}
	log.Logger().Infof("To delete the associated namespace %s for environment %s then please run this command", name, envNs)
	log.Logger().Infof(util.ColorInfo("  kubectl delete namespace %s"), envNs)
	return nil
}
