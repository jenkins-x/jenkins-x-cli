package cmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/health"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

type DiagnoseOptions struct {
	*opts.CommonOptions
	Namespace string
	Show      []string
}

func NewCmdDiagnose(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DiagnoseOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Print diagnostic information about the Jenkins X installation",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to display the kube resources from. If left out, defaults to the current namespace")
	cmd.Flags().StringArrayVarP(&options.Show, "show", "", []string{"version", "status", "pvc", "pods", "ingresses", "secrets", "configmaps"}, "Determine what information to diagnose")
	return cmd
}

func (o *DiagnoseOptions) Run() error {
	// Get the namespace to run the diagnostics in, and output it
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create kubeClient")
	}

	log.Logger().Infof("Running in namespace: %s", util.ColorInfo(ns))
	if o.showOption("version") {
		err := printStatus(o, "Jenkins X Version", "jx", "version", "--no-version-check")
		if err != nil {
			return err
		}
	}

	if o.showOption("status") {
		err := printStatus(o, "Jenkins X Status", "jx", "status")
		if err != nil {
			return err
		}
	}

	if o.showOption("pvc") {
		err := printStatus(o, "Kubernetes PVCs", "kubectl", "get", "pvc", "--namespace", ns)
		if err != nil {
			return err
		}
	}

	if o.showOption("pods") {
		err := printStatus(o, "Kubernetes Pods", "kubectl", "get", "po", "--namespace", ns)
		if err != nil {
			return err
		}
	}

	if o.showOption("ingresses") {
		err := printStatus(o, "Kubernetes Ingresses", "kubectl", "get", "ingress", "--namespace", ns)
		if err != nil {
			return err
		}
	}

	if o.showOption("secrets") {
		err := printStatus(o, "Kubernetes Secrets", "kubectl", "get", "secrets", "--namespace", ns)
		if err != nil {
			return err
		}
	}

	if o.showOption("configmaps") {
		err := printStatus(o, "Kubernetes Configmaps", "kubectl", "get", "configmaps", "--namespace", ns)
		if err != nil {
			return err
		}
	}

	if o.showOption("health") {
		err = health.Kuberhealthy(kubeClient, ns)
		if err != nil {
			return err
		}
	}

	log.Logger().Info("\nPlease visit https://jenkins-x.io/faq/issues/ for any known issues.")
	log.Logger().Info("\nFinished printing diagnostic information.")
	return nil
}

// Run the specified command (jx status, kubectl get po, etc) and print its output
func printStatus(o *DiagnoseOptions, header string, command string, options ...string) error {
	output, err := o.GetCommandOutput("", command, options...)
	if err != nil {
		log.Logger().Errorf("Unable to get the %s", header)
		return err
	}
	// Print the output of the command, and add a little header at the top for formatting / readability
	log.Logger().Infof("\n%s:\n%s", header, util.ColorInfo(output))
	return nil
}

func (o *DiagnoseOptions) showOption(e string) bool {
	for _, a := range o.Show {
		if a == e {
			return true
		}
	}
	return false
}
