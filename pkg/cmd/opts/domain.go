package opts

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cloud/iks"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/surveyutils"
	"github.com/jenkins-x/jx/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetDomain returns the domain name, trying to infer it either from various Kubernetes resources or cloud provider. If no domain
// can be determined, it will prompt to the user for a value.
func (o *CommonOptions) GetDomain(client kubernetes.Interface, domain string, provider string, ingressNamespace string, ingressService string, externalIP string) (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	address := externalIP
	if address == "" {
		info := util.ColorInfo
		log.Logger().Infof("Waiting to find the external host name of the ingress controller Service in namespace %s with name %s",
			info(ingressNamespace), info(ingressService))
		if provider == cloud.KUBERNETES {
			log.Logger().Infof("If you are installing Jenkins X on premise you may want to use the '--on-premise' flag or specify the '--external-ip' flags. See: %s",
				info("https://jenkins-x.io/getting-started/install-on-cluster/#installing-jenkins-x-on-premise"))
		}
		svc, err := client.CoreV1().Services(ingressNamespace).Get(ingressService, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		if svc != nil {
			for _, v := range svc.Status.LoadBalancer.Ingress {
				if v.IP != "" {
					address = v.IP
				} else if v.Hostname != "" {
					address = v.Hostname
				}
			}
		}
	}
	defaultDomain := address

	if provider == cloud.AWS || provider == cloud.EKS {
		if domain != "" {
			err := amazon.RegisterAwsCustomDomain(domain, address)
			return domain, err
		}

		// if we are booting, we want to use nip.io directly if a domain is not provided
		// if one is provided, we'll expose it through external-dns
		// we also check that we are not in cluster to do this, as the domain gets wiped if we are using .nip/xip.io
		// and we need to create a new one again
		if !o.InCluster() && os.Getenv("JX_INTERPRET_PIPELINE") != "true" {
			log.Logger().Infof("\nOn AWS we recommend using a custom DNS name to access services in your Kubernetes cluster to ensure you can use all of your Availability Zones")
			log.Logger().Infof("If you do not have a custom DNS name you can use yet, then you can register a new one here: %s\n",
				util.ColorInfo("https://console.aws.amazon.com/route53/home?#DomainRegistration:"))

			if o.BatchMode {
				return "", fmt.Errorf("Please specify a custom DNS name via --domain when installing on AWS in batch mode")
			}
			for {
				if answer, err := util.Confirm("Would you like to register a wildcard DNS ALIAS to point at this ELB address? ", true,
					"When using AWS we need to use a wildcard DNS alias to point at the ELB host name so you can access services inside Jenkins X and in your Environments.", o.GetIOFileHandles()); err != nil {
					return "", err
				} else if answer {
					customDomain := ""
					prompt := &survey.Input{
						Message: "Your custom DNS name: ",
						Help:    "Enter your custom domain that we can use to setup a Route 53 ALIAS record to point at the ELB host: " + address,
					}
					survey.AskOne(prompt, &customDomain, nil, surveyOpts)
					if customDomain != "" {
						err := amazon.RegisterAwsCustomDomain(customDomain, address)
						return customDomain, err
					}
				} else {
					break
				}
			}
		}
	}

	if provider == cloud.IKS {
		if domain != "" {
			log.Logger().Infof("\nIBM Kubernetes Service will use provided domain. Ensure name is registered with DNS (ex. CIS) and pointing the cluster ingress IP: %s",
				util.ColorInfo(address))
			return domain, nil
		}
		clusterName, err := iks.GetClusterName()
		clusterRegion, err := iks.GetKubeClusterRegion(client)
		if err == nil && clusterName != "" && clusterRegion != "" {
			customDomain := clusterName + "." + clusterRegion + ".containers.appdomain.cloud"
			log.Logger().Infof("\nIBM Kubernetes Service will use the default cluster domain: ")
			log.Logger().Infof("%s", util.ColorInfo(customDomain))
			return customDomain, nil
		}
		log.Logger().Infof("ERROR getting IBM Kubernetes Service will use the default cluster domain:")
		log.Logger().Infof(err.Error())
	}

	if address != "" {
		addNip := true
		aip := net.ParseIP(address)
		if aip == nil {
			log.Logger().Infof("The Ingress address %s is not an IP address. We recommend we try resolve it to a public IP address and use that for the domain to access services externally.",
				util.ColorInfo(address))

			addressIP := ""
			resolve := true
			if !o.BatchMode {
				answer, err := util.Confirm("Would you like wait and resolve this address to an IP address and use it for the domain?", true,
					"Should we convert "+address+" to an IP address so we can access resources externally", o.GetIOFileHandles())
				if err != nil {
					return "", err
				}
				resolve = answer
			}
			if resolve {
				log.Logger().Infof("Waiting for %s to be resolvable to an IP address...", util.ColorInfo(address))
				f := func() error {
					ips, err := net.LookupIP(address)
					if err == nil {
						for _, ip := range ips {
							t := ip.String()
							if t != "" && !ip.IsLoopback() {
								addressIP = t
								return nil
							}
						}
					}
					return fmt.Errorf("Address cannot be resolved yet %s", address)
				}
				o.RetryQuiet(5*6, time.Second*10, f)
			}
			if addressIP == "" {
				addNip = false
				log.Logger().Infof("Still not managed to resolve address %s into an IP address. Please try figure out the domain by hand", address)
			} else {
				log.Logger().Infof("%s resolved to IP %s", util.ColorInfo(address), util.ColorInfo(addressIP))
				address = addressIP
			}
		}
		if addNip && !strings.HasSuffix(address, ".amazonaws.com") {
			defaultDomain = fmt.Sprintf("%s.nip.io", address)
		}
	}

	if domain == "" {
		if o.BatchMode {
			log.Logger().Infof("No domain flag provided so using default %s to generate Ingress rules", defaultDomain)
			return defaultDomain, nil
		}
		log.Logger().Infof("You can now configure a wildcard DNS pointing to the new Load Balancer address %s", util.ColorInfo(address))
		log.Logger().Infof("If you don't have a wildcard DNS setup then create a DNS (A) record and point it at: %s, then use the DNS domain in the next input...", util.ColorInfo(address))

		log.Logger().Info("\nIf you do not have a custom domain setup yet, Ingress rules will be set for magic DNS nip.io.")
		log.Logger().Infof("Once you have a custom domain ready, you can update with the command %s", util.ColorInfo("jx upgrade ingress --cluster"))

		if domain == "" {
			prompt := &survey.Input{
				Message: "Domain",
				Default: defaultDomain,
				Help:    "Enter your custom domain that is used to generate Ingress rules, defaults to the magic DNS nip.io",
			}
			survey.AskOne(prompt, &domain,
				survey.ComposeValidators(survey.Required, surveyutils.NoWhiteSpaceValidator()), surveyOpts)
		}
		if domain == "" {
			domain = defaultDomain
		}
	} else {
		if domain != defaultDomain {
			log.Logger().Infof("You can now configure your wildcard DNS %s to point to %s", domain, address)
		}
	}

	return domain, nil
}
