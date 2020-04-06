package create

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/edit"
	"github.com/jenkins-x/jx/pkg/packages"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	glooNamespace           = "gloo-system"
	glooClusterIngressProxy = "clusteringress-proxy"
	knativeServeNamespace   = "knative-serving"
	knativeServeConfig      = "config-domain"
)

var (
	createAddonGlooLong = templates.LongDesc(`
		Create a Gloo and Knative Serve addon for creating serverless applications
`)

	createAddonGlooExample = templates.Examples(`
		# Create the Gloo addon 
		jx create addon gloo
	`)
)

// CreateAddonGlooOptions the options for the create spring command
type CreateAddonGlooOptions struct {
	CreateAddonOptions

	GlooNamespace         string
	ClusterIngressProxy   string
	KnativeServeNamespace string
	KnativeServeConfigMap string
}

// NewCmdCreateAddonGloo creates a command object for the "create" command
func NewCmdCreateAddonGloo(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonGlooOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "gloo",
		Short:   "Create a Gloo and Knative Serve addon for creating serverless applications",
		Aliases: []string{"knative-serve"},
		Long:    createAddonGlooLong,
		Example: createAddonGlooExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.GlooNamespace, "namespace", "n", glooNamespace, "The gloo system namespace")
	cmd.Flags().StringVarP(&options.ClusterIngressProxy, "ingress", "i", glooClusterIngressProxy, "The name of the gloo cluster ingress proxy Service")
	cmd.Flags().StringVarP(&options.KnativeServeNamespace, "knative-namespace", "k", knativeServeNamespace, "The knative serving namespace")
	cmd.Flags().StringVarP(&options.KnativeServeConfigMap, "knative-configmap", "c", knativeServeConfig, "The knative serving ConfigMap name")

	return cmd
}

// Run implements the command
func (o *CreateAddonGlooOptions) Run() error {
	if o.GlooNamespace == "" {
		o.GlooNamespace = glooNamespace
	}
	if o.ClusterIngressProxy == "" {
		o.ClusterIngressProxy = glooClusterIngressProxy
	}
	if o.KnativeServeNamespace == "" {
		o.KnativeServeNamespace = knativeServeNamespace
	}
	if o.KnativeServeConfigMap == "" {
		o.KnativeServeConfigMap = knativeServeConfig
	}

	// lets ensure glooctl is installed
	_, shouldInstall, err := packages.ShouldInstallBinary("glooctl")
	if err != nil {
		return errors.Wrapf(err, "failed to check if we need to install glooctl")
	}

	if shouldInstall {
		log.Logger().Infof("installing %s", util.ColorInfo("glooctl"))
		err = o.InstallGlooctl()
		if err != nil {
			return errors.Wrapf(err, "failed to install glooctl")
		}
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	// lets try load the gloo cluster service
	_, err = kubeClient.CoreV1().Services(o.GlooNamespace).Get(o.ClusterIngressProxy, metav1.GetOptions{})
	if err != nil {
		// we may not have installed gloo yet so lets do that now...
		err = o.RunCommandVerbose("glooctl", "install", "knative")
		if err != nil {
			return errors.Wrapf(err, "failed to install gloo")
		}
	}

	ip, err := o.getGlooDomain(kubeClient)
	if err != nil {
		return err
	}
	if ip == "" {
		return fmt.Errorf("failed to find the external LoadBalancer IP of the Gloo cluster ingress proxy service %s in namespace %s", o.ClusterIngressProxy, o.GlooNamespace)
	}
	externalDomain := ip + ".nip.io"
	err = o.updateKnativeServeDomain(kubeClient, externalDomain)
	if err != nil {
		return errors.Wrapf(err, "failed to update the gloo domain")
	}

	eo := &edit.EditDeployKindOptions{}
	eo.CommonOptions = o.CommonOptions
	eo.Team = true
	eo.Kind = opts.DeployKindKnative
	return eo.Run()
}

func (o *CreateAddonGlooOptions) getGlooDomain(kubeClient kubernetes.Interface) (string, error) {
	loggedWait := false
	ip := ""
	fn := func() (bool, error) {
		svc, err := kubeClient.CoreV1().Services(o.GlooNamespace).Get(o.ClusterIngressProxy, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// lets get the gloo ingress service
		for _, lb := range svc.Status.LoadBalancer.Ingress {
			ip = lb.IP
			if ip != "" {
				return true, nil
			}
		}

		if !loggedWait {
			loggedWait = true
			log.Logger().Infof("waiting for external IP on Gloo cluster ingress proxy service %s in namespace %s ...", o.ClusterIngressProxy, o.GlooNamespace)
		}
		return false, nil
	}
	err := o.RetryUntilTrueOrTimeout(time.Minute*5, time.Second*3, fn)
	if ip == "" || err != nil {
		return "", err
	}
	log.Logger().Infof("using external IP of gloo LoadBalancer: %s", util.ColorInfo(ip))
	return ip, nil
}

func (o *CreateAddonGlooOptions) updateKnativeServeDomain(kubeClient kubernetes.Interface, domain string) error {
	// lets get the knative serving ConfigMap
	knativeConfigMaps := kubeClient.CoreV1().ConfigMaps(o.KnativeServeNamespace)
	cm, err := knativeConfigMaps.Get(o.KnativeServeConfigMap, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to load the Knative Serve ConfigMap %s in namespace %s", o.KnativeServeConfigMap, o.KnativeServeNamespace)
	}
	cm.Data = map[string]string{
		domain: "",
	}
	_, err = knativeConfigMaps.Update(cm)
	if err != nil {
		return errors.Wrapf(err, "failed to save the Knative Serve ConfigMap %s in namespace %s", o.KnativeServeConfigMap, o.KnativeServeNamespace)
	}
	return nil
}
