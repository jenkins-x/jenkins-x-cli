package opts

import (
	"fmt"
	"os"
	"os/user"
	"reflect"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube/cluster"

	"github.com/jenkins-x/jx/pkg/environments"

	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/users"

	"github.com/jenkins-x/jx/pkg/log"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type BranchPatterns struct {
	DefaultBranchPattern string
	ForkBranchPattern    string
}

const (
	defaultHelmBin            = "helm"
	defaultBranchPatterns     = jenkins.BranchPatternMasterPRsAndFeatures
	defaultForkBranchPatterns = ""
)

// TeamSettings returns the team settings
func (o *CommonOptions) TeamSettings() (*v1.TeamSettings, error) {
	_, teamSettings, err := o.DevEnvAndTeamSettings()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return teamSettings, nil
}

// DevEnvAndTeamSettings returns the Dev Environment and Team settings
func (o *CommonOptions) DevEnvAndTeamSettings() (*v1.Environment, *v1.TeamSettings, error) {
	var teamSettings *v1.TeamSettings
	var devEnv *v1.Environment
	err := o.ModifyDevEnvironment(func(env *v1.Environment) error {
		devEnv = env
		teamSettings = &env.Spec.TeamSettings
		teamSettings.DefaultMissingValues()
		return nil
	})
	return devEnv, teamSettings, err
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

// TeamHelmBin returns the helm binary used for a team and whether a remote tiller is disabled
func (o *CommonOptions) TeamHelmBin() (string, bool, bool, error) {
	helmBin := defaultHelmBin
	teamSettings, err := o.TeamSettings()
	if err != nil {
		return helmBin, false, false, err
	}

	helmBin = teamSettings.HelmBinary
	if helmBin == "" {
		helmBin = defaultHelmBin
	}
	return helmBin, teamSettings.NoTiller, teamSettings.HelmTemplate, nil
}

// ModifyDevEnvironment modifies the development environment settings
func (o *CommonOptions) ModifyDevEnvironment(callback func(env *v1.Environment) error) error {
	if o.ModifyDevEnvironmentFn == nil {
		o.ModifyDevEnvironmentFn = o.DefaultModifyDevEnvironment
	}
	return o.ModifyDevEnvironmentFn(callback)
}

// ModifyDevEnvironment modifies the development environment settings
func (o *CommonOptions) ModifyEnvironment(name string, callback func(env *v1.Environment) error) error {
	if o.ModifyEnvironmentFn == nil {
		o.ModifyEnvironmentFn = o.DefaultModifyEnvironment
	}
	return o.ModifyEnvironmentFn(name, callback)
}

// DefaultModifyDevEnvironment default implementation of modifying the Development environment settings
func (o *CommonOptions) DefaultModifyDevEnvironment(callback func(env *v1.Environment) error) error {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the jx client")
	}
	if o.RemoteCluster {
		env := kube.CreateDefaultDevEnvironment(ns)
		if os.Getenv("JX_HELM3") == "true" {
			env.Spec.TeamSettings.HelmBinary = "helm3"
			env.Spec.TeamSettings.HelmTemplate = false
			env.Spec.TeamSettings.NoTiller = true
		} else if os.Getenv("JX_NO_TILLER") == "true" {
			env.Spec.TeamSettings.HelmTemplate = true
			env.Spec.TeamSettings.NoTiller = true
		}
		return callback(env)
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to create the kube client")
	}

	err = kube.EnsureDevNamespaceCreatedWithoutEnvironment(kubeClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to create the %s Dev namespace", ns)
	}

	env, err := kube.EnsureDevEnvironmentSetup(jxClient, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to setup the dev environment for namespace '%s'", ns)
	}
	if env == nil {
		return fmt.Errorf("No Development environment found for namespace %s", ns)
	}
	return environments.ModifyDevEnvironmentWithNs(jxClient, ns, callback)
}

// HelmfileModifyDevEnvironment default implementation of modifying the Development environment settings without
// eagerly populating kubernetes which we do via helmfile/helm
func (o *CommonOptions) HelmfileModifyDevEnvironment(callback func(env *v1.Environment) error) error {
	_, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the jx client")
	}
	env := kube.CreateDefaultDevEnvironment(ns)
	env.Spec.TeamSettings.HelmBinary = "helm"
	env.Spec.TeamSettings.HelmTemplate = false
	env.Spec.TeamSettings.NoTiller = true
	return callback(env)
}

// ConfigureCommonOptions lets us configure the common options based on the requirements
func (o *CommonOptions) ConfigureCommonOptions(requirements *config.RequirementsConfig) error {
	if requirements.Helmfile {
		// if using helmfile lets disable eagerly creating the dev Environment and instead lets create that via helm
		o.ModifyDevEnvironmentFn = o.HelmfileModifyDevEnvironment
	}
	return nil
}

// defaultModifyEnvironment default implementation of modifying an environment
func (o *CommonOptions) DefaultModifyEnvironment(name string, callback func(env *v1.Environment) error) error {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the jx client")
	}

	environmentInterface := jxClient.JenkinsV1().Environments(ns)
	env, err := environmentInterface.Get(name, metav1.GetOptions{})
	create := false
	if err != nil || env == nil {
		create = true
		env = &v1.Environment{}
	}
	env.Name = name
	err = callback(env)
	if err != nil {
		return errors.Wrapf(err, "failed to call the callback function when modifying Environment %s", name)
	}
	if create {
		log.Logger().Infof("Creating %s Environment in namespace %s", env.Name, ns)
		_, err = environmentInterface.Create(env)
		if err != nil {
			return errors.Wrapf(err, "failed to update Environment %s in namespace %s", name, ns)
		}
	} else {
		log.Logger().Infof("Updating %s Environment in namespace %s", env.Name, ns)
		_, err = environmentInterface.PatchUpdate(env)
		if err != nil {
			return errors.Wrapf(err, "failed to update Environment %s in namespace %s", name, ns)
		}
	}
	return nil
}

// IgnoreModifyEnvironment ignores modifying environments when using separate Staging/Production clusters
func (o *CommonOptions) IgnoreModifyEnvironment(name string, callback func(env *v1.Environment) error) error {
	env := &v1.Environment{}
	env.Name = name
	return callback(env)
}

// IgnoreModifyDevEnvironment ignores modifying the dev environment when using separate Staging/Production clusters
func (o *CommonOptions) IgnoreModifyDevEnvironment(callback func(env *v1.Environment) error) error {
	return o.IgnoreModifyEnvironment(kube.LabelValueDevEnvironment, callback)
}

// RegisterReleaseCRD register Release CRD
func (o *CommonOptions) RegisterReleaseCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterReleaseCRD(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Team CRD")
	}
	return nil
}

// RegisterTeamCRD registers Team CRD
func (o *CommonOptions) RegisterTeamCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterTeamCRD(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Team CRD")
	}
	return nil
}

// RegisterUserCRD registers user CRD
func (o *CommonOptions) RegisterUserCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterUserCRD(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the User CRD")
	}
	return nil
}

// RegisterEnvironmentRoleBindingCRD register RegisterEnvironmentRoleBinding CRD
func (o *CommonOptions) RegisterEnvironmentRoleBindingCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterEnvironmentRoleBindingCRD(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the User CRD")
	}
	return nil
}

// RegisterPipelineActivityCRD register PipelineActivity CRD
func (o *CommonOptions) RegisterPipelineActivityCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterPipelineActivityCRD(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the PipelineActivity CRD")
	}
	return nil
}

// RegisterWorkflowCRD registers Workflow CRD
func (o *CommonOptions) RegisterWorkflowCRD() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterWorkflowCRD(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Workflow CRD")
	}
	return nil
}

// ModifyTeam lazily creates the Team CRD if it does not exist or updates it if it requires a change.
// The Team CRD will be modified in the specified admin namespace.
func (o *CommonOptions) ModifyTeam(adminNs string, teamName string, callback func(env *v1.Team) error) error {
	err := o.RegisterTeamCRD()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	//Ignore admin NS returned here and use the one provided; JXClientAndAdminNamespace is returning the dev NS atm.
	jxClient, _, err := o.JXClientAndAdminNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the jx client")
	}
	ns, err := kube.GetAdminNamespace(kubeClient, adminNs)
	if err != nil {
		return err
	}

	if ns == "" {
		// there is no admin namespace yet so its too early to create a Team resource
		return nil
	}

	teamInterface := jxClient.JenkinsV1().Teams(ns)
	create := false
	team, err := teamInterface.Get(teamName, metav1.GetOptions{})
	if err != nil {
		team = kube.CreateTeam(ns, teamName, nil)
		create = true
	}

	original := *team
	if callback != nil {
		err = callback(team)
		if err != nil {
			return errors.Wrapf(err, "failed process Team %s", teamName)
		}
	}
	if create {
		_, err = teamInterface.Create(team)
		if err != nil {
			return errors.Wrapf(err, "failed create Team %s", teamName)
		}
	} else {
		if !reflect.DeepEqual(&original, team) {
			_, err = teamInterface.PatchUpdate(team)
			if err != nil {
				return errors.Wrapf(err, "failed update Team %s", teamName)
			}
		}
	}
	return nil
}

// ModifyUser lazily creates the user if it does not exist or updates it if it requires a change
func (o *CommonOptions) ModifyUser(userName string, callback func(env *v1.User) error) error {
	err := o.RegisterUserCRD()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the jx client")
	}
	ns, err := kube.GetAdminNamespace(kubeClient, devNs)
	if err != nil {
		return err
	}

	if ns == "" {
		// there is no admin namespace yet so its too early to create a User resource
		return nil
	}

	userInterface := jxClient.JenkinsV1().Users(ns)
	create := false
	user, err := userInterface.Get(userName, metav1.GetOptions{})
	if err != nil {
		user = users.CreateUser(ns, userName, "", "")
		create = true
	}

	original := *user
	if callback != nil {
		err = callback(user)
		if err != nil {
			return errors.Wrapf(err, "failed process User %s", userName)
		}
	}
	if create {
		_, err = userInterface.Create(user)
		if err != nil {
			return errors.Wrapf(err, "failed create User %s", userName)
		}
	} else {
		if !reflect.DeepEqual(&original, user) {
			_, err = userInterface.Update(user)
			if err != nil {
				return errors.Wrapf(err, "failed update User %s", userName)
			}
		}
	}
	return nil
}

// GetUsername returns current user name
func (o *CommonOptions) GetUsername(userName string) (string, error) {
	if userName == "" {
		if !cluster.IsInCluster() {
			u, err := user.Current()
			if err != nil {
				return userName, errors.Wrap(err, "Could not find the current user name. Please pass it in explicitly via the argument '--username'")
			}
			userName = u.Username
		}
	}
	return naming.ToValidNameTruncated(userName, 63), nil
}

// EnableRemoteKubeCluster lets setup this command to work with a remote cluster without a jx install
// so lets disable loading TeamSettings and tiller
func (o *CommonOptions) EnableRemoteKubeCluster() {
	o.RemoteCluster = true
	// let disable loading/modifying team environments as we typically install on empty k8s clusters
	o.ModifyEnvironmentFn = o.IgnoreModifyEnvironment
	o.ModifyDevEnvironmentFn = o.IgnoreModifyDevEnvironment
	helmer := o.NewHelm(false, "", true, true)
	o.SetHelm(helmer)
}
