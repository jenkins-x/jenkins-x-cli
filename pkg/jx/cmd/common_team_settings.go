package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
)

type BranchPatterns struct {
	DefaultBranchPattern string
	ForkBranchPattern    string
}

const (
	defaultBuildPackRef = "2.1"
	defaultHelmBin      = "helm"
)

// TeamSettings returns the team settings
func (o *CommonOptions) TeamSettings() (*v1.TeamSettings, error) {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return nil, err
	}
	err = o.registerEnvironmentCRD()
	if err != nil {
		return nil, fmt.Errorf("Failed to register Environment CRD: %s", err)
	}

	env, err := kube.EnsureDevEnvironmentSetup(jxClient, ns)
	if err != nil {
		return nil, fmt.Errorf("Failed to setup dev Environment in namespace %s: %s", ns, err)
	}
	if env == nil {
		return nil, fmt.Errorf("No Development environment found for namespace %s", ns)
	}

	teamSettings := &env.Spec.TeamSettings
	if teamSettings.BuildPackURL == "" {
		teamSettings.BuildPackURL = JenkinsBuildPackURL
	}
	if teamSettings.BuildPackRef == "" {
		teamSettings.BuildPackRef = defaultBuildPackRef
	}
	return teamSettings, nil
}

// TeamBranchPatterns returns the team branch patterns used to enable CI/CD on branches when creating/importing projects
func (o *CommonOptions) TeamBranchPatterns() (*BranchPatterns, error) {
	teamSettings, err := o.TeamSettings()
	if err != nil {
		return nil, err
	}

	branchPatterns := teamSettings.BranchPatterns
	if branchPatterns == "" {
		branchPatterns = defaultBranchPatterns
	}

	forkBranchPatterns := teamSettings.ForkBranchPatterns
	if forkBranchPatterns == "" {
		forkBranchPatterns = defaultForkBranchPatterns
	}

	return &BranchPatterns{
		DefaultBranchPattern: branchPatterns,
		ForkBranchPattern:    forkBranchPatterns,
	}, nil
}

// TeamHelmBin returns the helm binary used for a team
func (o *CommonOptions) TeamHelmBin() (string, error) {
	helmBin := defaultHelmBin
	teamSettings, err := o.TeamSettings()
	if err != nil {
		return helmBin, err
	}

	helmBin = teamSettings.HelmBinary
	if helmBin == "" {
		helmBin = defaultHelmBin
	}
	return helmBin, nil
}

// ModifyDevEnvironment modifies the development environment settings
func (o *CommonOptions) ModifyDevEnvironment(callback func(env *v1.Environment) error) error {
	apisClient, err := o.CreateApiExtensionsClient()
	if err != nil {
		return errors.Wrap(err, "failed to create the api extensions client")
	}
	kube.RegisterEnvironmentCRD(apisClient)

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the jx client")
	}
	err = o.registerEnvironmentCRD()
	if err != nil {
		return errors.Wrap(err, "failed to register the environment CRD")
	}

	env, err := kube.EnsureDevEnvironmentSetup(jxClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to setup the dev environment for namespace '%s'", ns)
	}
	if env == nil {
		return fmt.Errorf("No Development environment found for namespace %s", ns)
	}
	return o.modifyDevEnvironment(jxClient, ns, callback)
}
