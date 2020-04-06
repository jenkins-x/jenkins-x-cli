package verify

import (
	"fmt"
	"net/mail"
	"os"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cloud/gke/externaldns"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	verifyIngressLong = templates.LongDesc(`
		Verifies the ingress configuration defaulting the ingress domain if necessary
`)

	verifyIngressExample = templates.Examples(`
		# populate the ingress domain if not using a configured 'ingress.domain' setting
		jx step verify ingress

			`)
)

// StepVerifyIngressOptions contains the command line flags
type StepVerifyIngressOptions struct {
	step.StepOptions

	Dir              string
	Namespace        string
	Provider         string
	IngressNamespace string
	IngressService   string
	ExternalIP       string
	LazyCreate       bool
	LazyCreateFlag   string
}

// StepVerifyIngressResults stores the generated results
type StepVerifyIngressResults struct {
	Pipeline    *pipelineapi.Pipeline
	Task        *pipelineapi.Task
	PipelineRun *pipelineapi.PipelineRun
}

// NewCmdStepVerifyIngress Creates a new Command object
func NewCmdStepVerifyIngress(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyIngressOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "ingress",
		Short:   "Verifies the ingress configuration defaulting the ingress domain if necessary",
		Long:    verifyIngressLong,
		Example: verifyIngressExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the values.yaml file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to install into. Defaults to $DEPLOY_NAMESPACE if not")

	cmd.Flags().StringVarP(&options.IngressNamespace, "ingress-namespace", "", opts.DefaultIngressNamesapce, "The namespace for the Ingress controller")
	cmd.Flags().StringVarP(&options.IngressService, "ingress-service", "", opts.DefaultIngressServiceName, "The name of the Ingress controller Service")
	cmd.Flags().StringVarP(&options.ExternalIP, "external-ip", "", "", "The external IP used to access ingress endpoints from outside the Kubernetes cluster. For bare metal on premise clusters this is often the IP of the Kubernetes master. For cloud installations this is often the external IP of the ingress LoadBalancer.")
	cmd.Flags().StringVarP(&options.Provider, "provider", "", "", "Cloud service providing the Kubernetes cluster.  Supported providers: "+cloud.KubernetesProviderOptions())
	cmd.Flags().StringVarP(&options.LazyCreateFlag, "lazy-create", "", "", fmt.Sprintf("Specify true/false as to whether to lazily create missing resources. If not specified it is enabled if Terraform is not specified in the %s file", config.RequirementsConfigFileName))
	return cmd
}

// Run implements this command
func (o *StepVerifyIngressOptions) Run() error {
	var err error
	if o.Dir == "" {
		o.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	info := util.ColorInfo
	ns := o.Namespace
	if ns == "" {
		ns = os.Getenv("DEPLOY_NAMESPACE")
	}
	if ns != "" {
		if ns == "" {
			return fmt.Errorf("no default namespace found")
		}
	}
	requirements, requirementsFileName, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load Jenkins X requirements")
	}

	o.LazyCreate, err = requirements.IsLazyCreateSecrets(o.LazyCreateFlag)
	if err != nil {
		return errors.Wrapf(err, "failed to see if lazy create flag is set %s", o.LazyCreateFlag)
	}

	if requirements.Cluster.Provider == "" {
		log.Logger().Warnf("No provider configured\n")
	}

	if requirements.Ingress.Domain == "" {
		err = o.discoverIngressDomain(requirements, requirementsFileName)
		if err != nil {
			return errors.Wrapf(err, "failed to discover the Ingress domain")
		}
	}

	// if we're using GKE and folks have provided a domain, i.e. we're  not using the Jenkins X default nip.io
	if requirements.Ingress.Domain != "" && !requirements.Ingress.IsAutoDNSDomain() && requirements.Cluster.Provider == cloud.GKE {
		// then it may be a good idea to enable external dns and TLS
		if !requirements.Ingress.ExternalDNS {
			log.Logger().Info("using a custom domain and GKE, you can enable external dns and TLS")
		} else if !requirements.Ingress.TLS.Enabled {
			log.Logger().Info("using GKE with external dns, you can also now enable TLS")
		}

		if requirements.Ingress.ExternalDNS {
			log.Logger().Infof("validating the external-dns secret in namespace %s\n", info(ns))

			kubeClient, err := o.KubeClient()
			if err != nil {
				return errors.Wrap(err, "creating kubernetes client")
			}

			cloudDNSSecretName := requirements.Ingress.CloudDNSSecretName
			if cloudDNSSecretName == "" {
				cloudDNSSecretName = gke.GcpServiceAccountSecretName(kube.DefaultExternalDNSReleaseName)
				requirements.Ingress.CloudDNSSecretName = cloudDNSSecretName
			}

			err = kube.ValidateSecret(kubeClient, cloudDNSSecretName, externaldns.ServiceAccountSecretKey, ns)
			if err != nil {
				if o.LazyCreate {
					log.Logger().Infof("attempting to lazily create the external-dns secret %s\n", info(ns))

					_, err = externaldns.CreateExternalDNSGCPServiceAccount(o.GCloud(), kubeClient, kube.DefaultExternalDNSReleaseName, ns,
						requirements.Cluster.ClusterName, requirements.Cluster.ProjectID)
					if err != nil {
						return errors.Wrap(err, "creating the ExternalDNS GCP Service Account")
					}
					// lets rerun the verify step to ensure its all sorted now
					err = kube.ValidateSecret(kubeClient, cloudDNSSecretName, externaldns.ServiceAccountSecretKey, ns)
				}
			}
			if err != nil {
				return errors.Wrap(err, "validating external-dns secret")
			}

			err = o.GCloud().EnableAPIs(requirements.Cluster.ProjectID, "dns")
			if err != nil {
				return errors.Wrap(err, "unable to enable 'dns' api")
			}
		}
	}

	// TLS uses cert-manager to ask LetsEncrypt for a signed certificate
	if requirements.Ingress.TLS.Enabled {
		if requirements.Cluster.Provider != cloud.GKE {
			log.Logger().Warnf("Note that we have only tested TLS support on Google Container Engine with external-dns so far. This may not work!")
		}

		if requirements.Ingress.IsAutoDNSDomain() {
			return fmt.Errorf("TLS is not supported with automated domains like %s, you will need to use a real domain you own", requirements.Ingress.Domain)
		}
		_, err = mail.ParseAddress(requirements.Ingress.TLS.Email)
		if err != nil {
			return errors.Wrap(err, "You must provide a valid email address to enable TLS so you can receive notifications from LetsEncrypt about your certificates")
		}
	}

	return requirements.SaveConfig(requirementsFileName)
}

func (o *StepVerifyIngressOptions) discoverIngressDomain(requirements *config.RequirementsConfig, requirementsFileName string) error {
	client, err := o.KubeClient()
	var domain string
	if err != nil {
		return errors.Wrap(err, "getting the kubernetes client")
	}

	if requirements.Ingress.Domain != "" {
		return nil
	}

	if o.Provider == "" {
		o.Provider = requirements.Cluster.Provider
		if o.Provider == "" {
			log.Logger().Warnf("No provider configured\n")
		}
	}
	domain, err = o.GetDomain(client, "",
		o.Provider,
		o.IngressNamespace,
		o.IngressService,
		o.ExternalIP)
	if err != nil {
		return errors.Wrapf(err, "getting a domain for ingress service %s/%s", o.IngressNamespace, o.IngressService)
	}
	if domain == "" {
		hasHost, err := o.waitForIngressControllerHost(client, o.IngressNamespace, o.IngressService)
		if err != nil {
			return errors.Wrapf(err, "getting a domain for ingress service %s/%s", o.IngressNamespace, o.IngressService)
		}
		if hasHost {
			domain, err = o.GetDomain(client, "",
				o.Provider,
				o.IngressNamespace,
				o.IngressService,
				o.ExternalIP)
			if err != nil {
				return errors.Wrapf(err, "getting a domain for ingress service %s/%s", o.IngressNamespace, o.IngressService)
			}
		} else {
			log.Logger().Warnf("could not find host for  ingress service %s/%s\n", o.IngressNamespace, o.IngressService)
		}
	}

	if domain == "" {
		return fmt.Errorf("failed to discover domain for ingress service %s/%s", o.IngressNamespace, o.IngressService)
	}
	requirements.Ingress.Domain = domain
	err = requirements.SaveConfig(requirementsFileName)
	if err != nil {
		return errors.Wrapf(err, "failed to save changes to file: %s", requirementsFileName)
	}
	log.Logger().Infof("defaulting the domain to %s and modified %s\n", util.ColorInfo(domain), util.ColorInfo(requirementsFileName))
	return nil
}

func (o *StepVerifyIngressOptions) waitForIngressControllerHost(kubeClient kubernetes.Interface, ns, serviceName string) (bool, error) {
	loggedWait := false
	serviceInterface := kubeClient.CoreV1().Services(ns)

	if serviceName == "" || ns == "" {
		return false, nil
	}
	_, err := serviceInterface.Get(serviceName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	fn := func() (bool, error) {
		svc, err := serviceInterface.Get(serviceName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// lets get the ingress service status
		for _, lb := range svc.Status.LoadBalancer.Ingress {
			if lb.Hostname != "" || lb.IP != "" {
				return true, nil
			}
		}

		if !loggedWait {
			loggedWait = true
			log.Logger().Infof("waiting for external Host on the ingress service %s in namespace %s ...", serviceName, ns)
		}
		return false, nil
	}
	err = o.RetryUntilTrueOrTimeout(time.Minute*5, time.Second*3, fn)
	if err != nil {
		return false, err
	}
	return true, nil
}
