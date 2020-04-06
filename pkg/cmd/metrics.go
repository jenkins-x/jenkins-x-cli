package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

type MetricsOptions struct {
	*opts.CommonOptions

	Namespace string
	Filter    string
	Duration  string
	Selector  string
	Metric    string
}

var (
	MetricsLong = templates.LongDesc(`
		Gets the metrics of the newest pod for a Deployment.

`)

	MetricsExample = templates.Examples(`
		# displays metrics of the latest pod in deployment myapp
		jx metrics myapp

		# Tails the metrics of the container foo in the latest pod in deployment myapp
		jx metrics myapp -c foo
`)
)

func NewCmdMetrics(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &MetricsOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "metrics [deployment]",
		Short:   "Gets the metrics of the latest pod for a deployment",
		Long:    MetricsLong,
		Example: MetricsExample,
		Aliases: []string{"metrics"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to look for the Deployment. Defaults to the current namespace")
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filters the available deployments if no deployment argument is provided")
	cmd.Flags().StringVarP(&options.Duration, "duration", "d", "", "The duration to query (e.g. 1.5h, 20s, 5m")
	cmd.Flags().StringVarP(&options.Selector, "selector", "s", "", "The pod selector to use to query for pods")
	cmd.Flags().StringVarP(&options.Metric, "metric", "m", "", "The heapster metric name")
	return cmd
}

func (o *MetricsOptions) Run() error {
	args := o.Args

	client, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if ns == "" {
		ns = curNs
	}
	pod := ""
	selector := o.Selector
	if selector == "" {
		names, err := kube.GetDeploymentNames(client, ns, o.Filter)
		if err != nil {
			return err
		}
		name := ""
		if len(args) == 0 {
			if len(names) == 0 {
				return fmt.Errorf("There are no Deployments running")
			}
			n, err := util.PickName(names, "Pick Deployment:", "", o.GetIOFileHandles())
			if err != nil {
				return err
			}
			name = n
		} else {
			name = args[0]
			if util.StringArrayIndex(names, name) < 0 {
				return util.InvalidArg(name, names)
			}
		}

		p, err := o.WaitForReadyPodForDeployment(client, ns, name, names, false)
		if err != nil {
			return err
		}

		pod = p
		if pod == "" {
			return fmt.Errorf("No pod found for namespace %s with name %s", ns, name)
		}
	}

	if o.Duration != "" || o.Metric != "" {
		start := ""
		end := ""
		if o.Duration != "" && pod != "" {
			d, err := time.ParseDuration(o.Duration)
			if err != nil {
				return fmt.Errorf("Failed to parse duration %s due to %s", o.Duration, err)
			}
			e := time.Now()
			s := e.Add(-d)
			start = s.Format(time.RFC3339)
			end = e.Format(time.RFC3339)
		}

		heapster := kube.HeapterConfig{
			KubeClient: client,
		}
		data, err := heapster.GetPodMetrics(ns, pod, selector, o.Metric, start, end)
		if err != nil {
			return err
		}
		log.Logger().Infof("%s", string(data))
		return nil
	}

	if selector != "" {
		args = []string{"top", "pod", "--selector", selector, "--namespace", ns}
	} else {
		args = []string{"top", "pod", pod, "--namespace", ns}
	}

	err = o.RunCommand("kubectl", args...)
	if err != nil {
		return err
	}
	return nil
}
