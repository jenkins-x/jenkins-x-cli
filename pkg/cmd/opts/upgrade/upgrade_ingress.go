package upgrade

import (
	"context"
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/update"

	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube/pki"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	survey "gopkg.in/AlecAivazis/survey.v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	exposecontroller = "exposecontroller"

	certsIssuedReadyTimeout = 5 * time.Minute
)

// UpgradeIngressOptions the options for the create spring command
type UpgradeIngressOptions struct {
	*opts.CommonOptions

	SkipCertManager     bool
	Cluster             bool
	Force               bool
	Namespaces          []string
	Version             string
	TargetNamespaces    []string
	Services            []string
	SkipResourcesUpdate bool
	WaitForCerts        bool
	ConfigNamespace     string

	IngressConfig kube.IngressConfig
}

// Run implements the command
func (o *UpgradeIngressOptions) Run() error {
	client, devNamespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return fmt.Errorf("cannot connect to Kubernetes cluster: %v", err)
	}

	jxClient, ns, err := o.JXClient()
	if err != nil {
		return errors.Wrap(err, "error obtaining the JX Client")
	}

	devEnv, err := jxClient.JenkinsV1().Environments(ns).Get("dev", metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "error obtaining the ")
	}

	if devEnv.Spec.TeamSettings.BootRequirements != "" {
		return errors.New(`jx upgrade ingress shouldn't be used in a Jenkins X Boot cluster.
For more documentation on Ingress configuration see: [https://jenkins-x.io/docs/getting-started/setup/boot/#ingress](https://jenkins-x.io/docs/getting-started/setup/boot/#ingress)`)
	}

	previousWebHookEndpoint := ""
	if !o.SkipResourcesUpdate {
		previousWebHookEndpoint, err = o.GetWebHookEndpoint()
		if err != nil {
			return errors.Wrap(err, "getting the webhook endpoint")
		}
	}

	// if existing ingress exist in the namespaces ask do you want to delete them?
	ingressToDelete, err := o.getExistingIngressRules()
	if err != nil {
		return errors.Wrap(err, "getting the existing ingress rules")
	}

	// wizard to ask for config values
	err = o.confirmExposecontrollerConfig()
	if err != nil {
		return errors.Wrap(err, ""+
			"configure exposecontroller")
	}

	// confirm values
	if !o.BatchMode {
		if answer, err := util.Confirm(fmt.Sprintf("Using config values %v, ok?", o.IngressConfig), true, "", o.GetIOFileHandles()); err != nil {
			return err
		} else if !answer {
			log.Logger().Infof("Terminating")
			return nil
		}
	}

	// save details to a configmap
	_, err = kube.SaveAsConfigMap(client, kube.ConfigMapIngressConfig, devNamespace, o.IngressConfig)
	if err != nil {
		return errors.Wrap(err, "saving ingress config into a configmap")
	}

	// ensure cert-manager is installed
	if o.IngressConfig.TLS {
		err = o.ensureCertmanagerSetup()
		if err != nil {
			return errors.Wrap(err, "ensure cert-manager setup")
		}
	}

	// clear the service annotations
	err = o.CleanServiceAnnotations(o.Services...)
	if err != nil {
		return errors.Wrap(err, "cleaning service annotations")
	}

	// annotate any service that has expose=true with correct cert-manager staging / prod annotation
	var services []*v1.Service
	if o.IngressConfig.TLS {
		services, err = o.AnnotateExposedServicesWithCertManager(o.Services...)
		if err != nil {
			return errors.Wrap(err, "annotating the exposed service with cert-manager")
		}
	}

	// remove the ingress resource in order to allow the ingress-controller to recreate them
	for name, namespace := range ingressToDelete {
		log.Logger().Infof("Deleting ingress %s/%s", namespace, name)
		err := client.ExtensionsV1beta1().Ingresses(namespace).Delete(name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("cannot delete ingress rule %s in namespace %s: %v", name, namespace, err)
		}
	}

	// start watching and collecting ready certificates
	var notReadyCertsCh <-chan map[pki.Certificate]bool
	ctx, cancel := context.WithTimeout(context.Background(), certsIssuedReadyTimeout)
	defer cancel()
	if o.IngressConfig.TLS && o.WaitForCerts {
		certsCh, err := o.watchReadyCertificates(ctx)
		if err != nil {
			return errors.Wrap(err, "start watching ready certificates")
		}
		notReadyCertsCh = o.startCollectingReadyCertificates(ctx, services, certsCh)
	}

	// run the expose-controller to create the ingress rules
	err = o.createIngressRules()
	if err != nil {
		return errors.Wrap(err, "creating the ingress rules")
	}

	log.Logger().Info("Ingress rules recreated")

	if o.IngressConfig.TLS {
		if o.WaitForCerts {
			log.Logger().Info("Waiting for TLS certificates to be issued...")
			select {
			case certs := <-notReadyCertsCh:
				cancel()
				if len(certs) == 0 {
					log.Logger().Info("All TLS certificates are ready")
				} else {
					log.Logger().Warn("Following TLS certificates are not ready:")
					for cert := range certs {
						log.Logger().Warnf("%s", cert)
					}
					return errors.New("not all TLS certificates are ready")
				}
			case <-ctx.Done():
				log.Logger().Warn("Timeout reached while waiting for TLS certificates to be ready")
			}
		} else {
			log.Logger().Warn("It can take around 5 minutes for Cert Manager to get certificates from Lets Encrypt and update Ingress rules")
			log.Logger().Info("Use the following commands to diagnose any issues:")
			log.Logger().Infof("jx logs %s -n %s", pki.CertManagerDeployment, pki.CertManagerNamespace)
			log.Logger().Info("kubectl describe certificates")
			log.Logger().Info("kubectl describe issuers\n")
		}
	}

	// update all resource dependent to the ingress endpoints
	if !o.SkipResourcesUpdate {
		err = o.updateResources(previousWebHookEndpoint)
		if err != nil {
			return errors.Wrap(err, "unable to update resources for webhook change")
		}
	}

	return nil
}

func (o *UpgradeIngressOptions) watchReadyCertificates(ctx context.Context) (<-chan pki.Certificate, error) {
	client, err := o.CertManagerClient()
	if err != nil {
		return nil, errors.Wrap(err, "creating the cert-manager client")
	}

	// watch certificates across all namesapces
	namespace := ""
	certsCh, err := pki.WatchCertificatesIssuedReady(ctx, client, namespace)
	if err != nil {
		return nil, errors.Wrap(err, "start watching certificates")
	}
	return certsCh, nil
}

func (o *UpgradeIngressOptions) startCollectingReadyCertificates(ctx context.Context, services []*v1.Service,
	certsCh <-chan pki.Certificate) <-chan map[pki.Certificate]bool {
	resultCh := make(chan map[pki.Certificate]bool)
	go func() {
		certs := pki.ToCertificates(services)
		certsMap := make(map[pki.Certificate]bool)
		for _, cert := range certs {
			certsMap[cert] = true
		}

		log.Logger().Infof("Expecting certificates: %v", certs)

		for {
			select {
			case cert := <-certsCh:
				log.Logger().Infof("Ready Cert: %s", util.ColorInfo(cert))
				delete(certsMap, cert)
				// check if all expected certificates are received
				if len(certsMap) == 0 {
					// send a map with no certificates to indicate success
					resultCh <- certsMap
					return
				}
			case <-ctx.Done():
				// send the current state of the certificates map
				resultCh <- certsMap
				return
			}
		}
	}()
	return resultCh
}

func (o *UpgradeIngressOptions) updateResources(previousWebHookEndpoint string) error {
	_, _, err := o.JXClient()
	if err != nil {
		return errors.Wrap(err, "failed to get jxclient")
	}

	isProwEnabled, err := o.IsProw()
	if err != nil {
		return errors.Wrap(err, "checking if is prow")
	}

	if !isProwEnabled {
		err = o.UpdateJenkinsURL(o.TargetNamespaces)
		if err != nil {
			return errors.Wrap(err, "upgrade jenkins URL")
		}
	}

	updatedWebHookEndpoint, err := o.GetWebHookEndpoint()
	if err != nil {
		return errors.Wrap(err, "retrieving the webhook endpoint")
	}

	log.Logger().Infof("Previous webhook endpoint %s", previousWebHookEndpoint)
	log.Logger().Infof("Updated webhook endpoint %s", updatedWebHookEndpoint)
	updateWebHooks := true
	if !o.BatchMode {
		if answer, err := util.Confirm("Do you want to update all existing webhooks?", true, "", o.GetIOFileHandles()); err != nil {
			return err
		} else if !answer {
			updateWebHooks = false
		}
	}

	if updateWebHooks {
		err := o.updateWebHooks(previousWebHookEndpoint, updatedWebHookEndpoint)
		if err != nil {
			return errors.Wrap(err, "unable to update webhooks")
		}
	}
	return nil
}

func (o *UpgradeIngressOptions) isIngressForServices(ingress *v1beta1.Ingress) bool {
	services := o.Services
	if len(services) == 0 {
		// allow all ingresses if no services filter is defined
		return true
	}
	rules := ingress.Spec.Rules
	for _, rule := range rules {
		http := rule.IngressRuleValue.HTTP
		if http == nil {
			continue
		}
		for _, path := range http.Paths {
			service := path.Backend.ServiceName
			i := util.StringArrayIndex(services, service)
			if i >= 0 {
				return true
			}
		}
	}
	return false
}

func (o *UpgradeIngressOptions) getExistingIngressRules() (map[string]string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	existingIngressNames := map[string]string{}
	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return existingIngressNames, err
	}
	var confirmMessage string
	if o.Cluster {
		confirmMessage = "Existing ingress rules found in the cluster.  Confirm to delete all and recreate them"

		ings, err := client.ExtensionsV1beta1().Ingresses("").List(metav1.ListOptions{})
		if err != nil {
			return existingIngressNames, fmt.Errorf("cannot list all ingresses in cluster: %v", err)
		}
		for _, i := range ings.Items {
			if i.Annotations[services.ExposeGeneratedByAnnotation] == exposecontroller {
				if o.isIngressForServices(&i) {
					existingIngressNames[i.Name] = i.Namespace
				}
			}
		}

		nsList, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
		for _, n := range nsList.Items {
			o.TargetNamespaces = append(o.TargetNamespaces, n.Name)
		}

	} else if len(o.Namespaces) > 0 {
		confirmMessage = fmt.Sprintf("Existing ingress rules found in namespaces %v namespace.  Confirm to delete and recreate them", o.Namespaces)
		// loop round each
		for _, n := range o.Namespaces {
			ings, err := client.ExtensionsV1beta1().Ingresses(n).List(metav1.ListOptions{})
			if err != nil {
				return existingIngressNames, fmt.Errorf("cannot list all ingresses in cluster: %v", err)
			}
			for _, i := range ings.Items {
				if i.Annotations[services.ExposeGeneratedByAnnotation] == exposecontroller {
					if o.isIngressForServices(&i) {
						existingIngressNames[i.Name] = i.Namespace
					}
				}
			}
			o.TargetNamespaces = append(o.TargetNamespaces, n)
		}
	} else {
		confirmMessage = "Existing ingress rules found in current namespace.  Confirm to delete and recreate them"
		// fall back to current ns only
		log.Logger().Infof("Looking for existing ingress rules in current namespace %s", currentNamespace)

		ings, err := client.ExtensionsV1beta1().Ingresses(currentNamespace).List(metav1.ListOptions{})
		if err != nil {
			return existingIngressNames, fmt.Errorf("cannot list all ingresses in cluster: %v", err)
		}
		for _, i := range ings.Items {
			if i.Annotations[services.ExposeGeneratedByAnnotation] == exposecontroller {
				if o.isIngressForServices(&i) {
					existingIngressNames[i.Name] = i.Namespace
				}
			}
		}
		o.TargetNamespaces = append(o.TargetNamespaces, currentNamespace)
	}

	if len(existingIngressNames) == 0 {
		return existingIngressNames, nil
	}

	if !o.BatchMode {
		confirm := &survey.Confirm{
			Message: confirmMessage,
			Default: true,
		}
		flag := true
		err = survey.AskOne(confirm, &flag, nil, surveyOpts)
		if err != nil {
			return existingIngressNames, err
		}
		if !flag {
			return existingIngressNames, errors.New("Not able to automatically delete existing ingress rules.  Either delete manually or change the scope the command should run in")
		}
	}

	return existingIngressNames, nil
}

func (o *UpgradeIngressOptions) confirmExposecontrollerConfig() error {
	// get current ingress config to use as existing defaults
	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	// select the namespace from where to read the ingress-config config map
	devNamespace, _, err := kube.GetDevNamespace(client, currentNamespace)
	if err != nil {
		return fmt.Errorf("cannot find a dev team namespace to get existing exposecontroller config from. %v", err)
	}
	configNamespace := devNamespace
	if o.ConfigNamespace != "" {
		configNamespace = o.ConfigNamespace
	}

	// Overwrites the ingress config with the values from config map only if this config map exists
	urlTemplate := o.IngressConfig.UrlTemplate
	domain := o.IngressConfig.Domain
	ic, err := kube.GetIngressConfig(client, configNamespace)
	if err == nil {
		// TODO: Add the rest of the Ingress-related info as arguments and assign to `o.IngressConfig` only those that were not specified, instead of the whole `ic`.`
		o.IngressConfig = ic
		if urlTemplate != "" {
			// Template must be surrounded by quotes
			if !strings.HasPrefix(urlTemplate, "\"") && !strings.HasPrefix(urlTemplate, "'") {
				urlTemplate = "\"" + urlTemplate + "\""
			}
			o.IngressConfig.UrlTemplate = urlTemplate
		}
		if domain != "" {
			o.IngressConfig.Domain = domain
		}
	}

	if o.BatchMode {
		if err := checkEmtptyIngressConfig(o.IngressConfig.Exposer, "exposer"); err != nil {
			return err
		}
		if err := checkEmtptyIngressConfig(o.IngressConfig.Domain, "domain"); err != nil {
			return err
		}
		if o.IngressConfig.TLS {
			if err := checkEmtptyIngressConfig(o.IngressConfig.Issuer, "issuer"); err != nil {
				return err
			}
			if err := checkEmtptyIngressConfig(o.IngressConfig.Email, "email"); err != nil {
				return err
			}
		}
	} else {
		o.IngressConfig.Exposer, err = util.PickNameWithDefault([]string{"Ingress", "Route"}, "Expose type", o.IngressConfig.Exposer, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}

		o.IngressConfig.Domain, err = util.PickValue("Domain:", o.IngressConfig.Domain, true, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}

		if !strings.HasSuffix(o.IngressConfig.Domain, "nip.io") {
			if !o.BatchMode {
				o.IngressConfig.TLS, err = util.Confirm("If your network is publicly available would you like to enable cluster wide TLS?", true, "Enables cert-manager and configures TLS with signed certificates from LetsEncrypt", o.GetIOFileHandles())
				if err != nil {
					return err
				}
			}

			if o.IngressConfig.TLS {
				log.Logger().Infof("If testing LetsEncrypt you should use staging as you may be rate limited using production.")
				clusterIssuer, err := util.PickNameWithDefault([]string{"staging", "production"}, "Use LetsEncrypt staging or production?", "production", "", o.GetIOFileHandles())
				// if the cluster issuer is production the string needed by letsencrypt is prod
				if clusterIssuer == "production" {
					clusterIssuer = "prod"
				}
				if err != nil {
					return err
				}
				o.IngressConfig.Issuer = "letsencrypt-" + clusterIssuer

				if o.IngressConfig.Email == "" {
					email1, err := o.GetCommandOutput("", "git", "config", "user.email")
					if err != nil {
						return err
					}

					o.IngressConfig.Email = strings.TrimSpace(email1)
				}

				o.IngressConfig.Email, err = util.PickValue("Email address to register with LetsEncrypt:", o.IngressConfig.Email, true, "", o.GetIOFileHandles())
				if err != nil {
					return err
				}
			}
		}
		o.IngressConfig.UrlTemplate, err = util.PickValue("URLTemplate (press <Enter> to keep the current value):", o.IngressConfig.UrlTemplate, false, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	return nil
}

func checkEmtptyIngressConfig(value string, name string) error {
	if value == "" {
		return fmt.Errorf("%v config value must not be empty", name)
	}
	return nil
}

func (o *UpgradeIngressOptions) createIngressRules() error {
	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	certmngClient, err := o.CertManagerClient()
	if err != nil {
		return errors.Wrap(err, "creating the cert-manager client")
	}
	devNamespace, _, err := kube.GetDevNamespace(client, currentNamespace)
	if err != nil {
		return fmt.Errorf("cannot find a dev team namespace to get existing exposecontroller config from. %v", err)
	}
	for _, n := range o.TargetNamespaces {
		o.CleanExposecontrollerReources(n)

		if len(o.Services) > 0 {
			services, err := services.GetServicesByName(client, n, o.Services)
			if err != nil {
				return err
			}
			certs := pki.ToCertificates(services)
			err = pki.CleanCerts(client, certmngClient, n, certs)
			if err != nil {
				return err
			}
		} else {
			err := pki.CleanAllCerts(client, certmngClient, n)
			if err != nil {
				return err
			}
		}

		err := pki.CreateCertManagerResources(certmngClient, n, o.IngressConfig)
		if err != nil {
			return err
		}

		err = o.RunExposecontroller(devNamespace, n, o.IngressConfig, o.Services...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *UpgradeIngressOptions) ensureCertmanagerSetup() error {
	if !o.SkipCertManager {
		return o.EnsureCertManager()
	}
	return nil
}

// AnnotateExposedServicesWithCertManager annotates exposed services with cert manager
func (o *UpgradeIngressOptions) AnnotateExposedServicesWithCertManager(svcs ...string) ([]*v1.Service, error) {
	result := make([]*v1.Service, 0)
	client, err := o.KubeClient()
	if err != nil {
		return result, err
	}
	for _, n := range o.TargetNamespaces {
		issuer := o.IngressConfig.Issuer
		if issuer == "" {
			return result, fmt.Errorf("no issuer was configured for cert manager")
		}
		clusterIssuer := o.IngressConfig.ClusterIssuer
		services, err := services.AnnotateServicesWithCertManagerIssuer(client, n, issuer, clusterIssuer, svcs...)
		if err != nil {
			return result, err
		}
		result = append(result, services...)
	}
	return result, nil
}

// CleanServiceAnnotations cleans service annotations
func (o *UpgradeIngressOptions) CleanServiceAnnotations(svcs ...string) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	for _, n := range o.TargetNamespaces {
		err := services.CleanServiceAnnotations(client, n, svcs...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o *UpgradeIngressOptions) updateWebHooks(oldHookEndpoint string, newHookEndpoint string) error {
	if oldHookEndpoint == newHookEndpoint && !o.Force {
		log.Logger().Infof("Webhook URL unchanged. Use %s to force updating", util.ColorInfo("--force"))
		return nil
	}

	log.Logger().Infof("Updating all webHooks from %s to %s", util.ColorInfo(oldHookEndpoint), util.ColorInfo(newHookEndpoint))

	updateWebHook := update.UpdateWebhooksOptions{
		CommonOptions: o.CommonOptions,
	}

	authConfigService, err := o.GitAuthConfigService()
	if err != nil {
		return errors.Wrap(err, "failed to create git auth service")
	}

	gitServer := authConfigService.Config().CurrentServer
	git, err := o.GitProviderForGitServerURL(gitServer, "github", "")
	if err != nil {
		return errors.Wrap(err, "unable to determine git provider")
	}

	// user
	userAuth := git.UserAuth()
	username := userAuth.Username

	// organisation
	organisation, err := gits.PickOrganisation(git, username, o.GetIOFileHandles())
	updateWebHook.Username = ReturnUserNameIfPicked(organisation, username)
	if err != nil {
		return errors.Wrap(err, "unable to determine git provider")
	}

	if o.CommonOptions.Verbose {
		log.Logger().Infof("Updating all webHooks for org %s and/or username %s", organisation, updateWebHook.Username)
	}

	updateWebHook.PreviousHookUrl = oldHookEndpoint
	updateWebHook.Org = organisation
	updateWebHook.DryRun = false

	return updateWebHook.Run()
}

// ReturnUserNameIfPicked checks to see if PickOrganisation returned ""
// this will happen if you picked the username as organization
// which is valid in this scenario and allows code further down
// to select the appropriate API to call (user or org based)
func ReturnUserNameIfPicked(organisation string, username string) string {
	if organisation == "" && username != "" {
		return username
	}
	return ""
}
