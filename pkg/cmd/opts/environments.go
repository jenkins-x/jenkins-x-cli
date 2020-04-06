package opts

import (
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
)

// RegisterEnvironmentCRD registers the CRD for environmnt
func (o *CommonOptions) RegisterEnvironmentCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterEnvironmentCRD(apisClient)
	return err
}

// GetDevEnv gets the Development Environment CRD as devEnv,
// and also tells the user whether the development environment is using gitOps
func (o *CommonOptions) GetDevEnv() (gitOps bool, devEnv *jenkinsv1.Environment) {
	// We're going to need to know whether the team is using GitOps for the dev env or not,
	// and also access the team settings, so load those
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		log.Logger().Errorf("Error loading team settings. %v", err)
		return false, &jenkinsv1.Environment{}
	} else {
		devEnv, err := kube.GetDevEnvironment(jxClient, ns)
		if err != nil {
			log.Logger().Errorf("Error loading team settings. %v", err)
			return false, &jenkinsv1.Environment{}
		}
		gitOps := false
		if devEnv == nil {
			devEnv = &jenkinsv1.Environment{}
			devEnv.Spec.Namespace = ns
		}
		if devEnv.Spec.Source.URL != "" {
			gitOps = true
		}
		return gitOps, devEnv
	}
}

// ResolveChartMuseumURL resolves the current Chart Museum URL so we can pass it into a remote Environment's
// git repository
func (o *CommonOptions) ResolveChartMuseumURL() (string, error) {
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return "", err
	}
	return services.FindServiceURL(kubeClient, ns, kube.ServiceChartMuseum)
}
