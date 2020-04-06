package expose

import (
	"bytes"
	"fmt"
	"path/filepath"
	"reflect"
	"strconv"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/helm/pkg/chartutil"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	// APIServicePathAnnotationKey the standard annotation to indicate the exposed URL should include an API path in addition to the domain name
	APIServicePathAnnotationKey = "api.service.kubernetes.io/path"

	// ExposeLabelKeys the labels used to indicate if a Service should be exposed
	ExposeLabelKeys = []string{"fabric8.io/expose", "jenkins-x.io/expose"}

	// ExposeAnnotationKey annotation on a Service to store its exposed URL
	ExposeAnnotationKey = "fabric8.io/exposeUrl"

	// ExposeHostNameAsAnnotationKey annotation to indicate the annotation name to expose the service host name
	ExposeHostNameAsAnnotationKey = "fabric8.io/exposeHostNameAs"

	// ExposePortAnnotationKey annotation to expose the service port
	ExposePortAnnotationKey = "fabric8.io/exposePort"

	ingressTemplateFileName = "ingress.tmpl.yaml"
)

// StepExposeOptions contains the command line flags
type StepExposeOptions struct {
	*opts.CommonOptions
	Dir             string
	IngressTemplate string
	Namespace       string
	LabelSelector   string
}

var (
	stepExposeLong = templates.LongDesc(`
		"This step generates Ingress resources for exposed services
`)

	stepExposeExample = templates.Examples(`
		# runs the expose step as part of the Staging/Production pipeline
		# looking in the env folder for the ingress template 'ingress.tmpl.yaml'
		jx step expose -d env
`)
)

// NewCmdStepExpose creates the command
func NewCmdStepExpose(commonOpts *opts.CommonOptions) *cobra.Command {
	o := StepExposeOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "expose",
		Short:   "This step generates Ingress resources for exposed services",
		Long:    stepExposeLong,
		Example: stepExposeExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.IngressTemplate, "template", "t", "", "the go template to generate the Ingress YAML for each service")
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", ".", "the directory to look for the install requirements file")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "", "", "the namespace that Jenkins X will be booted into. If not specified it defaults to $DEPLOY_NAMESPACE")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "s", "", "the optional label selector to only process a subset of the Service resources")
	return cmd
}

// Run runs the command
func (o *StepExposeOptions) Run() error {
	info := util.ColorInfo
	if o.IngressTemplate == "" {
		o.IngressTemplate = filepath.Join(o.Dir, ingressTemplateFileName)
	}
	ns, err := o.GetDeployNamespace(o.Namespace)
	if err != nil {
		return err
	}

	requirements, _, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements YAML")
	}

	if requirements == nil {
		return errors.Errorf("unable to find requirements file in '%s'", o.Dir)
	}

	if requirements.Cluster.Namespace == "" {
		requirements.Cluster.Namespace = ns
	}

	// if we have no domain lets look for the old exposecontroller chart configuration in the `expose` sub chart in the
	// values.yaml file
	err = o.loadExposeControllerConfigFromValuesYAML(requirements)
	if err != nil {
		return errors.Wrapf(err, "failed to load exposecontroller configuration from helm values.yaml")
	}
	if requirements.Ingress.Domain == "" {
		return fmt.Errorf("could not detect the domain name to expose via a 'jx-requirements.yml' file in %s or a parent directory nor in the values.yaml file in expose.config.domain", o.Dir)
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrapf(err, "failed to create kubernetes client")
	}

	serviceInterface := kubeClient.CoreV1().Services(ns)
	serviceList, err := serviceInterface.List(metav1.ListOptions{
		LabelSelector: o.LabelSelector,
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Logger().Warnf("No services found in namespace %s: %s", ns, err.Error())
			return nil
		}
		return errors.Wrapf(err, "failed to query services in namespace %s", ns)
	}

	for _, svc := range serviceList.Items {
		if !IsExposedService(&svc) {
			continue
		}
		oldAnnotations := map[string]string{}
		for k, v := range svc.Annotations {
			oldAnnotations[k] = v
		}

		ingress, err := o.createIngress(requirements, kubeClient, ns, &svc, o.IngressTemplate)
		if err != nil {
			return err
		}

		if ingress == nil {
			continue
		}
		exposedURL, err := o.applyIngressAndUpdateService(kubeClient, ns, &svc, ingress)
		if err != nil {
			return errors.Wrapf(err, "failed to modify the Service %s in namespace %s", svc.Name, ns)
		}

		if reflect.DeepEqual(svc.Annotations, oldAnnotations) {
			log.Logger().Infof("did not need to annotate Service %s in namespace %s with the exposed URL", info(svc.Name), info(ns))
		} else {
			// lets save the modified service due to updating of the label
			_, err = serviceInterface.Update(&svc)
			if err != nil {
				return errors.Wrapf(err, "failed to modify the annotated Service %s to namespace %s", svc.Name, ns)
			}
			log.Logger().Infof("annotated Service %s in namespace %s with the exposed URL %s", info(svc.Name), info(ns), info(exposedURL))
		}
	}
	return nil
}

// creates an Ingres resource from a template if it exists
func (o *StepExposeOptions) createIngress(requirements *config.RequirementsConfig, kubeClient kubernetes.Interface, ns string, service *corev1.Service, fileName string) (*v1beta1.Ingress, error) {
	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to check if file exists %s", fileName)
	}
	if !exists {
		log.Logger().Warnf("failed to find file %s\n", fileName)
		return nil, nil
	}
	data, err := readYamlTemplate(fileName, requirements, service)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load vault ingress template file %s", fileName)
	}

	answer := &v1beta1.Ingress{}
	err = yaml.Unmarshal(data, answer)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load Ingress from result of template file %s", fileName)
	}
	return answer, nil
}

// readYamlTemplate evaluates the given go template file and returns the output data
func readYamlTemplate(templateFile string, requirements *config.RequirementsConfig, svc *corev1.Service) ([]byte, error) {
	_, name := filepath.Split(templateFile)
	funcMap := helm.NewFunctionMap()
	tmpl, err := template.New(name).Option("missingkey=error").Funcs(funcMap).ParseFiles(templateFile)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse Secrets template: %s", templateFile)
	}

	requirementsMap, err := requirements.ToMap()
	if err != nil {
		return nil, errors.Wrapf(err, "failed turn requirements into a map: %v", requirements)
	}

	svcMap, err := createServiceMap(svc)
	if err != nil {
		return nil, errors.Wrapf(err, "failed turn Service into a map: %v", svc)
	}

	templateData := map[string]interface{}{
		"Requirements": chartutil.Values(requirementsMap),
		"Environments": chartutil.Values(requirements.EnvironmentMap()),
		"Service":      chartutil.Values(svcMap),
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute Secrets template: %s", templateFile)
	}
	data := buf.Bytes()
	return data, nil
}

func createServiceMap(service *corev1.Service) (map[string]interface{}, error) {
	m, err := util.ToObjectMap(service)
	if err != nil {
		return m, err
	}
	m["name"] = service.Name

	port := 0
	exposePort := service.Annotations[ExposePortAnnotationKey]
	if exposePort != "" {
		port, err = strconv.Atoi(exposePort)
		if err != nil {
			port = 0
		}
	}
	if port == 0 {
		for _, p := range service.Spec.Ports {
			port = int(p.Port)
			break
		}
	}
	m["port"] = port

	return m, nil
}

func (o *StepExposeOptions) applyIngressAndUpdateService(client kubernetes.Interface, ns string, svc *corev1.Service, ingress *v1beta1.Ingress) (string, error) {
	if ingress.Labels == nil {
		ingress.Labels = map[string]string{}
	}
	ingress.Labels["provider"] = "jenkins-x"
	hasOwner := false
	for _, o := range ingress.OwnerReferences {
		if o.UID == svc.UID {
			hasOwner = true
			break
		}
	}
	if !hasOwner {
		ingress.OwnerReferences = append(ingress.OwnerReferences, kube.ServiceOwnerRef(svc))
	}

	name := ingress.Name
	_, err := client.ExtensionsV1beta1().Ingresses(ns).Get(name, metav1.GetOptions{})
	createIngress := false
	if err != nil {
		if apierrors.IsNotFound(err) {
			createIngress = true
		} else {
			return "", errors.Wrapf(err, "could not check for existing ingress %s/%s", ns, name)
		}
	}

	info := util.ColorInfo
	if createIngress {
		_, err = client.ExtensionsV1beta1().Ingresses(ns).Create(ingress)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create ingress %s/%s", ns, ingress.Name)
		}
		log.Logger().Infof("created Ingress %s in namespace %s", info(name), info(ns))
	} else {
		_, err = client.ExtensionsV1beta1().Ingresses(ns).Update(ingress)
		if err != nil {
			return "", errors.Wrapf(err, "failed to update ingress %s/%s", ns, ingress.Name)
		}
		log.Logger().Infof("updated Ingress %s in namespace %s", info(name), info(ns))
	}

	if svc.Annotations == nil {
		svc.Annotations = map[string]string{}
	}
	hostName := services.IngressHost(ingress)
	exposeURL := services.IngressURL(ingress)

	path := svc.Annotations[APIServicePathAnnotationKey]
	if path == "" {
		path = svc.Annotations["fabric8.io/ingress.path"]
	}
	if len(path) > 0 {
		exposeURL = util.UrlJoin(exposeURL, path)
	}
	svc.Annotations[ExposeAnnotationKey] = exposeURL

	if key := svc.Annotations[ExposeHostNameAsAnnotationKey]; len(key) > 0 {
		svc.Annotations[key] = hostName
	}
	return exposeURL, nil
}

func (o *StepExposeOptions) loadExposeControllerConfigFromValuesYAML(requirements *config.RequirementsConfig) error {
	values, err := helm.LoadValuesFile(filepath.Join(o.Dir, helm.ValuesFileName))
	if err != nil {
		return err
	}
	ic := &requirements.Ingress
	if ic.Domain == "" {
		ic.Domain = util.GetMapValueAsStringViaPath(values, "expose.config.domain")
	}

	// lets detect TLS
	if util.GetMapValueAsStringViaPath(values, "expose.config.tlsacme") == "true" || util.GetMapValueAsStringViaPath(values, "expose.config.http") == "false" {
		ic.TLS.Enabled = true
	}

	// lets populate the sub domain if its missing
	if ic.NamespaceSubDomain == "" {
		ic.NamespaceSubDomain = "-" + requirements.Cluster.Namespace + "."
	}
	return nil
}

// IsExposedService returns true if this service should be exposed
func IsExposedService(svc *corev1.Service) bool {
	labels := svc.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	for _, l := range ExposeLabelKeys {
		if labels[l] == "true" {
			return true
		}
	}
	return false
}
