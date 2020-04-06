package applications

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/flagger"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Deployment represents an application deployment in a single environment
type Deployment struct {
	*appsv1.Deployment
}

// Environment represents an environment in which an application has been
// deployed
type Environment struct {
	v1.Environment
	Deployments []Deployment
}

// Application represents an application in jx
type Application struct {
	*v1.SourceRepository
	Environments map[string]Environment
}

// List is a collection of applications
type List struct {
	Items []Application
}

// Environments loops through all applications in a list and returns a map with
// all the unique environments
func (l List) Environments() map[string]v1.Environment {
	envs := make(map[string]v1.Environment)

	for _, a := range l.Items {
		for name, env := range a.Environments {
			if _, ok := envs[name]; !ok {
				envs[name] = env.Environment
			}
		}
	}

	return envs
}

// Name returns the application name
func (a Application) Name() string {
	return naming.ToValidName(a.SourceRepository.Spec.Repo)
}

// IsPreview returns true if the environment is a preview environment
func (e Environment) IsPreview() bool {
	return e.Environment.Spec.Kind == v1.EnvironmentKindTypePreview
}

// Version returns the deployment version
func (d Deployment) Version() string {
	return kube.GetVersion(&d.Deployment.ObjectMeta)
}

// Pods returns the ratio of pods that are ready/replicas
func (d Deployment) Pods() string {
	pods := ""
	ready := d.Deployment.Status.ReadyReplicas

	if d.Deployment.Spec.Replicas != nil && ready > 0 {
		replicas := util.Int32ToA(*d.Deployment.Spec.Replicas)
		pods = util.Int32ToA(ready) + "/" + replicas
	}

	return pods
}

// URL returns a deployment URL
func (d Deployment) URL(kc kubernetes.Interface, a Application) string {
	url, _ := services.FindServiceURL(kc, d.Deployment.Namespace, a.Name())
	return url
}

// GetApplications fetches all Applications
func GetApplications(factory clients.Factory) (List, error) {
	list := List{
		Items: make([]Application, 0),
	}

	client, namespace, err := factory.CreateJXClient()
	if err != nil {
		return list, errors.Wrap(err, "failed to create a jx client from applications.GetApplications")
	}

	// fetch ALL repositories
	srList, err := client.JenkinsV1().SourceRepositories(namespace).List(metav1.ListOptions{})
	if err != nil {
		return list, errors.Wrapf(err, "failed to find any SourceRepositories in namespace %s", namespace)
	}

	// fetch all environments
	envMap, _, err := kube.GetOrderedEnvironments(client, namespace)
	if err != nil {
		return list, errors.Wrapf(err, "failed to fetch environments in namespace %s", namespace)
	}

	// only keep permanent environments
	permanentEnvsMap := map[string]*v1.Environment{}
	for _, env := range envMap {
		if env.Spec.Kind.IsPermanent() {
			permanentEnvsMap[env.Spec.Namespace] = env
		}
	}

	// copy repositories that aren't environments to our applications list
	for _, sr := range srList.Items {
		if !kube.IsIncludedInTheGivenEnvs(permanentEnvsMap, &sr) {
			srCopy := sr
			list.Items = append(list.Items, Application{&srCopy, make(map[string]Environment)})
		}
	}

	kubeClient, _, err := factory.CreateKubeClient()

	// fetch deployments by environment (excluding dev)
	deployments := make(map[string]map[string]appsv1.Deployment)
	for _, env := range permanentEnvsMap {
		if env.Spec.Kind != v1.EnvironmentKindTypeDevelopment {
			envDeployments, err := kube.GetDeployments(kubeClient, env.Spec.Namespace)
			if err != nil {
				return list, err
			}

			deployments[env.Spec.Namespace] = envDeployments
		}
	}

	err = list.appendMatchingDeployments(permanentEnvsMap, deployments)
	if err != nil {
		return list, err
	}

	return list, nil
}

func getDeploymentAppNameInEnvironment(d appsv1.Deployment, e *v1.Environment) (string, error) {
	labels, err := metav1.LabelSelectorAsMap(d.Spec.Selector)
	if err != nil {
		return "", err
	}

	name := kube.GetAppName(labels["app"], e.Spec.Namespace)
	return name, nil
}

func (l List) appendMatchingDeployments(envs map[string]*v1.Environment, deps map[string]map[string]appsv1.Deployment) error {
	for _, app := range l.Items {
		for envName, env := range envs {
			for _, dep := range deps[envName] {
				depAppName, err := getDeploymentAppNameInEnvironment(dep, env)
				if err != nil {
					return errors.Wrap(err, "getting app name")
				}
				if depAppName == app.Name() && !flagger.IsCanaryAuxiliaryDeployment(dep) {
					depCopy := dep
					app.Environments[env.Name] = Environment{
						*env,
						[]Deployment{{&depCopy}},
					}
				}
			}
		}
	}

	return nil
}
