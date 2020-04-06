package verify

import (
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepVerifyInstallOptions contains the command line flags
type StepVerifyInstallOptions struct {
	StepVerifyOptions
	Debug           bool
	Dir             string
	Namespace       string
	PodWaitDuration time.Duration
}

// NewCmdStepVerifyInstall creates the `jx step verify pod` command
func NewCmdStepVerifyInstall(commonOpts *opts.CommonOptions) *cobra.Command {

	options := &StepVerifyInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Verifies that an installation is setup correctly",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.Debug, "debug", "", false, "Output logs of any failed pod")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the install requirements file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "the namespace that Jenkins X will be booted into. If not specified it defaults to $DEPLOY_NAMESPACE")
	cmd.Flags().DurationVarP(&options.PodWaitDuration, "pod-wait-time", "w", time.Second, "The default wait time to wait for the pods to be ready")
	return cmd
}

// Run implements this command
func (o *StepVerifyInstallOptions) Run() error {
	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}
	o.SetDevNamespace(ns)

	log.Logger().Infof("verifying the Jenkins X installation in namespace %s\n", util.ColorInfo(ns))

	po := &StepVerifyPodReadyOptions{}
	po.StepOptions = o.StepOptions
	po.Debug = o.Debug
	po.WaitDuration = o.PodWaitDuration
	po.ExcludeBuildPods = true

	log.Logger().Info("verifying pods\n")
	err = po.Run()
	if err != nil {
		return err
	}

	gto := &StepVerifyGitOptions{}
	gto.StepOptions = o.StepOptions
	err = gto.Run()
	if err != nil {
		return err
	}

	requirements, _, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	installValues, err := kube.ReadInstallValues(kubeClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to read install values from namespace %s", ns)
	}
	provider := installValues[kube.KubeProvider]
	if provider == "" {
		log.Logger().Warnf("no %s in the ConfigMap %s. Has values %#v\n", kube.KubeProvider, kube.ConfigMapNameJXInstallConfig, installValues)
		provider = requirements.Cluster.Provider
	}

	if requirements.Kaniko {
		if provider == cloud.GKE {
			err = o.validateKaniko(ns)
			if err != nil {
				return err
			}
		}
	}
	log.Logger().Infof("Installation is currently looking: %s\n", util.ColorInfo("GOOD"))
	return nil
}
