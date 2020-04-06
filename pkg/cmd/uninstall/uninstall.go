package uninstall

import (
	"fmt"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UninstallOptions struct {
	*opts.CommonOptions

	Namespace        string
	Context          string
	Force            bool // Force uninstallation - programmatic use only - do not expose to the user
	KeepEnvironments bool
}

var (
	uninstall_long = templates.LongDesc(`
		Uninstalls the Jenkins X platform from a Kubernetes cluster. This will remove all Jenkins X components, secrets, config and namespaces including any environment related namespaces`)
	uninstall_example = templates.Examples(`
		# Uninstall the Jenkins X platform
		jx uninstall`)
)

const (
	kubeSystemNs     = "kube-system"
	jxIngressRelease = "jxing"
)

func NewCmdUninstall(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UninstallOptions{
		CommonOptions: commonOpts,
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
			helper.CheckErr(err)
		},
	}
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
		if !o.BatchMode {
			envNamespaces := o.getAllNamespaces(namespace, jxClient)
			help := "All Jenkins X components will be removed and the following namespaces will be deleted:"
			for _, envNamespace := range envNamespaces {
				help += "\n " + envNamespace
			}

			if answer, err := util.Confirm("Uninstall JX - this command will remove all JX components and delete all namespaces created by Jenkins X. Do you wish to continue?", false,
				help, o.GetIOFileHandles()); !answer {
				return err
			}
		}
		if o.BatchMode || o.Context != "" {
			targetContext = o.Context
		} else {
			msg := fmt.Sprintf("This action will permanently delete Jenkins X from the Kubernetes context %s. "+
				"Please type in the name of the context to confirm:", util.ColorInfo(currentContext))
			helpMsg := "To prevent accidental uninstallation from the wrong cluster, you must enter the current Kubernetes context."
			targetContext, err = util.PickValue(msg, "", true, helpMsg, o.GetIOFileHandles())
			if err != nil {
				return err
			}
		}
		if targetContext != currentContext {
			return fmt.Errorf("the context '%s' must match the current context '%s' to uninstall", targetContext, currentContext)
		}
	}

	log.Logger().Infof("Removing installation of Jenkins X in team namespace %s", util.ColorInfo(namespace))

	envMap, envNames, err := kube.GetEnvironments(jxClient, namespace)
	if err != nil {
		log.Logger().Warnf("Failed to find Environments. Probably not installed yet?. Error: %s", err)
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
				log.Logger().Warnf("Failed to uninstall environment chart %s: %s", release, err)
			}
		}
	}

	var errs []error
	errs = o.DeleteReleaseIfPresent(namespace, "jx-prow", errs, false)
	errs = o.DeleteReleaseIfPresent(namespace, "jenkins-x", errs, false)
	errs = o.DeleteReleaseIfPresent(kubeSystemNs, jxIngressRelease, errs, true)

	err = jxClient.JenkinsV1().Environments(namespace).DeleteCollection(&meta_v1.DeleteOptions{}, meta_v1.ListOptions{})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to delete the environments in namespace %s: %s", namespace, err))
	}
	err = o.cleanupNamespaces(namespace, envNames, envMap)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to cleanup namespaces in namespace %s: %s", namespace, err))
	}
	if len(errs) > 0 {
		return util.CombineErrors(errs...)
	}
	log.Logger().Infof("Jenkins X has been successfully uninstalled from team namespace %s", namespace)
	return nil
}

func (o *UninstallOptions) getAllNamespaces(namespace string, jxClient versioned.Interface) []string {
	envNamespaces := []string{namespace}

	if jxClient != nil {
		envMap, envNames, err := kube.GetEnvironments(jxClient, namespace)
		if err != nil {
			log.Logger().Warnf("Failed to find Environments. Error: %s", err)
			return envNamespaces
		}

		for _, env := range envNames {
			envResource := envMap[env]
			envNamespace := namespace + "-" + env
			if envResource != nil {
				envNamespace = envResource.Spec.Namespace
			}
			if envNamespace != "" && envNamespace != namespace {
				envNamespaces = append(envNamespaces, envNamespace)
			}
		}
	}

	return envNamespaces
}

func (o *UninstallOptions) cleanupNamespaces(namespace string, envNames []string, envMap map[string]*v1.Environment) error {
	client, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "getting the kube client")
	}
	errs := []error{}
	err = o.deleteNamespace(namespace)
	if err != nil {
		errs = append(errs, fmt.Errorf("deleting namespace %s: %s", namespace, err))
	}
	if !o.KeepEnvironments {
		for _, env := range envNames {
			envNamespaces := []string{namespace + "-" + env}

			envResource := envMap[env]
			envNamespace := ""
			if envResource != nil {
				envNamespace = envResource.Spec.Namespace
			}
			if envNamespace != "" && envNamespaces[0] != envNamespace && envNamespace != namespace {
				envNamespaces = append(envNamespaces, envNamespace)
			}
			for _, envNamespace := range envNamespaces {
				_, err := client.CoreV1().Namespaces().Get(envNamespace, meta_v1.GetOptions{})
				if err != nil {
					continue
				}
				err = o.deleteNamespace(envNamespace)
				if err != nil {
					errs = append(errs, fmt.Errorf("deleting environment namespace %s: %s", envNamespace, err))
				}
			}
		}
	}
	return util.CombineErrors(errs...)
}

func (o *UninstallOptions) deleteNamespace(namespace string) error {
	client, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "getting the kube client")
	}
	_, err = client.CoreV1().Namespaces().Get(namespace, meta_v1.GetOptions{})
	if err != nil {
		// There is nothing to delete if the namespace cannot be retrieved
		return nil
	}
	log.Logger().Infof("deleting namespace %s", util.ColorInfo(namespace))
	err = client.CoreV1().Namespaces().Delete(namespace, &meta_v1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "deleting the namespace '%s' from Kubernetes cluster", namespace)
	}
	return nil
}

// DeleteReleaseIfPresent deletes the given chart in the given namespace and adds any erro to the passed errors slice
// as it checks if the release is present before deleting, it can be forced to delete in case it doesn't find it because
// of an unrelated error
func (o *UninstallOptions) DeleteReleaseIfPresent(ns string, releaseName string, errors []error, force bool) []error {
	if err := o.Helm().StatusRelease(ns, releaseName); err == nil || force {
		err := o.Helm().DeleteRelease(ns, releaseName, true)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to uninstall the %s helm chart in namespace %s: %s", releaseName, ns, err))
		}
	} else {
		log.Logger().Warnf("Not deleting %s because the release is not installed", releaseName)
	}
	return errors
}
