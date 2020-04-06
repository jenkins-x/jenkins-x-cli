package opts

import (
	"github.com/jenkins-x/jx/pkg/expose"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
)

// Expose runs expose controller in the given target namespace
func (o *CommonOptions) Expose(devNamespace, targetNamespace, password string) error {
	certClient, err := o.factory.CreateCertManagerClient()
	if err != nil {
		return errors.Wrap(err, "creating cert-manager client")
	}
	versionsDir, _, err := o.CloneJXVersionsRepo("", "")
	if err != nil {
		return errors.Wrapf(err, "failed to clone the Jenkins X versions repository")
	}
	return expose.Expose(o.kubeClient, certClient, devNamespace, targetNamespace, password, o.Helm(), DefaultInstallTimeout, versionsDir)
}

// RunExposecontroller runs exponse controller in the given target dir with the given ingress configuration
func (o *CommonOptions) RunExposecontroller(devNamespace, targetNamespace string, ic kube.IngressConfig, services ...string) error {
	versionsDir, _, err := o.CloneJXVersionsRepo("", "")
	if err != nil {
		return errors.Wrapf(err, "failed to clone the Jenkins X versions repository")
	}
	return expose.RunExposecontroller(devNamespace, targetNamespace, ic, o.kubeClient, o.Helm(),
		DefaultInstallTimeout, versionsDir, services...)
}

// CleanExposecontrollerReources cleans expose controller resources
func (o *CommonOptions) CleanExposecontrollerReources(ns string) {
	expose.CleanExposecontrollerReources(o.kubeClient, ns)
}
