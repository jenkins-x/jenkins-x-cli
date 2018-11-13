package cmd

import (
	"fmt"
	"io"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UninstallOptions struct {
	CommonOptions

	Namespace        string
	Context          string
	Force            bool // Force uninstallation - programmatic use only - do not expose to the user
	KeepEnvironments bool
}

var (
	uninstall_long = templates.LongDesc(`
		Uninstalls the Jenkins X platform from a Kubernetes cluster`)
	uninstall_example = templates.Examples(`
		# Uninstall the Jenkins X platform
		jx uninstall`)
)

func NewCmdUninstall(f Factory, in terminal.FileReader, out terminal.FileWriter, errOut io.Writer) *cobra.Command {
	options := &UninstallOptions{
		CommonOptions: CommonOptions{
			Factory: f,
			In:      in,

			Out: out,
			Err: errOut,
		},
	}
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall the Jenkins X platform",
		Long:    uninstall_long,
		Example: uninstall_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
	}
	options.addCommonFlags(cmd)
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The team namespace to uninstall. Defaults to the current namespace.")
	cmd.Flags().StringVarP(&options.Context, "context", "", "", "The kube context to uninstall JX from. This will be compared with the current context to prevent accidental uninstallation from the wrong cluster")
	cmd.Flags().BoolVarP(&options.KeepEnvironments, "keep-environments", "", false, "Don't delete environments. Uninstall Jenkins X only.")
	return cmd
}

func (o *UninstallOptions) Run() error {
	config, _, err := o.Kube().LoadConfig()
	if err != nil {
		return err
	}
	jxClient, _, err := o.JXClient()
	if err != nil {
		return err
	}
	currentContext := kube.CurrentContextName(config)
	namespace := o.Namespace
	if namespace == "" {
		namespace = kube.CurrentNamespace(config)
	}
	var targetContext string
	if !o.Force {
		if o.BatchMode || o.Context != "" {
			targetContext = o.Context
		} else {
			targetContext, err = util.PickValue(fmt.Sprintf("Enter the current context name to confirm "+
				"uninstalllation of the Jenkins X platform from the %s namespace:", util.ColorInfo(namespace)),
				"", true,
				"To prevent accidental uninstallation from the wrong cluster, you must enter the current "+
					"kubernetes context. This can be found with `kubectl config current-context`",
				o.In, o.Out, o.Err)
			if err != nil {
				return err
			}
		}
		if targetContext != currentContext {
			return fmt.Errorf("The context '%s' must match the current context to uninstall", targetContext)
		}
	}

	log.Infof("Removing installation of Jenkins X in team namespace %s\n", util.ColorInfo(namespace))

	err = o.cleanupConfig()
	if err != nil {
		return err
	}
	envNames, err := kube.GetEnvironmentNames(jxClient, namespace)
	if err != nil {
		log.Warnf("Failed to find Environments. Probably not installed yet?. Error: %s\n", err)
	}
	if !o.KeepEnvironments {
		for _, env := range envNames {
			release := namespace + "-" + env
			err := o.Helm().StatusRelease(namespace, release)
			if err != nil {
				continue
			}
			err = o.Helm().DeleteRelease(namespace, release, true)
			if err != nil {
				log.Warnf("Failed to uninstall environment chart %s: %s\n", release, err)
			}
		}
	}
	o.Helm().DeleteRelease(namespace, "jx-prow", true)
	err = o.Helm().DeleteRelease(namespace, "jenkins-x", true)
	if err != nil {
		errc := o.cleanupNamespaces(namespace, envNames)
		if errc != nil {
			errc = errors.Wrap(errc, "failed to cleanup the jenkins-x platform")
			return errc
		}
		return errors.Wrap(err, "failed to purge the jenkins-x chart")
	}
	err = jxClient.JenkinsV1().Environments(namespace).DeleteCollection(&meta_v1.DeleteOptions{}, meta_v1.ListOptions{})
	if err != nil {
		return err
	}
	err = o.cleanupNamespaces(namespace, envNames)
	if err != nil {
		return err
	}
	log.Successf("Jenkins X has been successfully uninstalled from team namespace %s", namespace)
	return nil
}

func (o *UninstallOptions) cleanupNamespaces(namespace string, envNames []string) error {
	client, _, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to get the kube client")
	}
	err = o.deleteNamespace(namespace)
	if err != nil {
		return errors.Wrap(err, "failed to delete team namespace namespace")
	}
	if !o.KeepEnvironments {
		for _, env := range envNames {
			envNamespace := namespace + "-" + env
			_, err := client.CoreV1().Namespaces().Get(envNamespace, meta_v1.GetOptions{})
			if err != nil {
				continue
			}
			err = o.deleteNamespace(envNamespace)
			if err != nil {
				return errors.Wrap(err, "failed to delete environment namespace")
			}
		}
	}
	return nil
}

func (o *UninstallOptions) deleteNamespace(namespace string) error {
	client, _, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to get the kube client")
	}
	err = client.CoreV1().Namespaces().Delete(namespace, &meta_v1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to delete the namespace '%s'", namespace)
	}
	return nil
}

func (o *UninstallOptions) cleanupConfig() error {
	authConfigSvc, err := o.Factory.CreateAuthConfigService(JenkinsAuthConfigFile)
	if err != nil {
		return nil
	}
	server := authConfigSvc.Config().CurrentServer
	err = authConfigSvc.DeleteServer(server)
	if err != nil {
		return err
	}

	chartConfigSvc, err := o.Factory.CreateChartmuseumAuthConfigService()
	if err != nil {
		return err
	}
	server = chartConfigSvc.Config().CurrentServer
	return chartConfigSvc.DeleteServer(server)
}
