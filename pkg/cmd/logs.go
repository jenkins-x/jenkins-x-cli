package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type LogsOptions struct {
	*opts.CommonOptions

	Container       string
	Namespace       string
	Environment     string
	Filter          string
	Label           string
	EditEnvironment bool
}

var (
	logs_long = templates.LongDesc(`
		Tails the logs of the newest pod for a Deployment.

`)

	logs_example = templates.Examples(`
		# Tails the log of the latest pod in deployment myapp
		jx logs myapp

		# Tails the log of the container foo in the latest pod in deployment myapp
		jx logs myapp -c foo
`)
)

func NewCmdLogs(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &LogsOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "logs [deployment]",
		Short:   "Tails the log of the latest pod for a deployment",
		Long:    logs_long,
		Example: logs_example,
		Aliases: []string{"log"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Container, "container", "c", "", "The name of the container to log")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to look for the Deployment. Defaults to the current namespace")
	cmd.Flags().StringVarP(&options.Environment, "env", "e", "", "the Environment to look for the Deployment. Defaults to the current environment")
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filters the available deployments if no deployment argument is provided")
	cmd.Flags().StringVarP(&options.Label, "label", "l", "", "The label to filter the pods if no deployment argument is provided")
	cmd.Flags().BoolVarP(&options.EditEnvironment, "edit", "d", false, "Use my Edit Environment to look for the Deployment pods")
	return cmd
}

func (o *LogsOptions) Run() error {
	args := o.Args

	client, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if ns == "" {
		env := o.Environment
		if env != "" {
			ns, err = kube.GetEnvironmentNamespace(jxClient, devNs, env)
			if err != nil {
				return err
			}
		}
		if ns == "" && o.EditEnvironment {
			ns, err = kube.GetEditEnvironmentNamespace(jxClient, devNs)
			if err != nil {
				return err
			}
		}
	}
	if ns == "" {
		ns = curNs
	}
	names, err := kube.GetDeploymentNames(client, ns, o.Filter)
	if err != nil {
		return fmt.Errorf("Could not find deployments in namespace %s with filter %s: %s", ns, o.Filter, err)
	}
	if len(names) == 0 {
		if o.Filter == "" {
			return fmt.Errorf("There are no Deployments")
		} else {
			return fmt.Errorf("There are no Deployments matching filter: " + o.Filter)
		}
	}
	name := ""
	if len(args) == 0 {
		if o.Label == "" {
			n, err := util.PickName(names, "Pick Deployment:", "", o.GetIOFileHandles())
			if err != nil {
				return err
			}
			name = n
		}
	} else {
		name = args[0]
		if util.StringArrayIndex(names, name) < 0 {
			return util.InvalidArg(name, names)
		}
	}

	for {
		pod := ""
		if o.Label != "" {
			selector, err := parseSelector(o.Label)
			if err != nil {
				return err
			}
			pod, err = o.WaitForReadyPodForSelectorLabels(client, ns, selector, false)
			if err != nil {
				return err
			}
			if pod == "" {
				return fmt.Errorf("No pod found for namespace %s with selector %s", ns, o.Label)
			}
		} else {
			pod, err = o.WaitForReadyPodForDeployment(client, ns, name, names, false)
			if err != nil {
				return err
			}
			if pod == "" {
				return fmt.Errorf("No pod found for namespace %s with name %s", ns, name)
			}
		}
		err = o.TailLogs(ns, pod, o.Container)
		if err != nil {
			return nil
		}
	}
}

func parseSelector(selectorText string) (map[string]string, error) {
	selector, err := metav1.ParseToLabelSelector(selectorText)
	if err != nil {
		return nil, err
	}
	return selector.MatchLabels, nil
}
