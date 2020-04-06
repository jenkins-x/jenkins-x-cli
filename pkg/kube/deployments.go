package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/log"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	tools_watch "k8s.io/client-go/tools/watch"
)

// GetDeployments get deployments in the given namespace
func GetDeployments(kubeClient kubernetes.Interface, ns string) (map[string]appsv1.Deployment, error) {
	answer := map[string]appsv1.Deployment{}
	deps, err := kubeClient.AppsV1().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		return answer, err
	}
	for _, d := range deps.Items {
		answer[d.Name] = d
	}
	return answer, nil
}

// GetDeploymentNames get deployment names in the given namespace with filter
func GetDeploymentNames(client kubernetes.Interface, ns string, filter string) ([]string, error) {
	names := []string{}
	list, err := client.AppsV1().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		return names, fmt.Errorf("Failed to load Deployments %s", err)
	}
	for _, d := range list.Items {
		name := d.Name
		if filter == "" || strings.Contains(name, filter) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// GetDeploymentByRepo get deployment in the given namespace with repo name
func GetDeploymentByRepo(client kubernetes.Interface, ns string, repoName string) (*appsv1.Deployment, error) {
	deps, err := client.AppsV1().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deps.Items {
		if strings.HasPrefix(d.Name, repoName) {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("no deployment found for repository name '%s'", repoName)
}

// IsDeploymentRunning returns whether this deployment is running
func IsDeploymentRunning(client kubernetes.Interface, name, namespace string) (bool, error) {
	options := metav1.GetOptions{}

	d, err := client.ExtensionsV1beta1().Deployments(namespace).Get(name, options)
	if err != nil {
		return false, err
	}

	if d.Status.ReadyReplicas > 0 {
		return true, nil
	}
	return false, nil
}

// WaitForAllDeploymentsToBeReady waits for the pods of all deployment to become ready in the given namespace
func WaitForAllDeploymentsToBeReady(client kubernetes.Interface, namespace string, timeoutPerDeploy time.Duration) error {
	deployList, err := client.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	if deployList == nil || len(deployList.Items) == 0 {
		return fmt.Errorf("no deployments found in namespace %s", namespace)
	}

	for _, d := range deployList.Items {
		err = WaitForDeploymentToBeReady(client, d.Name, namespace, timeoutPerDeploy)
		if err != nil {
			log.Logger().Warnf("deployment %s failed to become ready in namespace %s", d.Name, namespace)
		}
	}
	return nil
}

// WaitForDeploymentToBeCreatedAndReady waits for the pods of a deployment created and to become ready
func WaitForDeploymentToBeCreatedAndReady(client kubernetes.Interface, name, namespace string, timeoutPerDeploy time.Duration) error {

	options := metav1.ListOptions{FieldSelector: fmt.Sprintf("metadata.name=%s", name)}

	w, err := client.AppsV1().Deployments(namespace).Watch(options)
	if err != nil {
		return err
	}
	defer w.Stop()

	condition := func(event watch.Event) (bool, error) {
		running, _ := IsDeploymentRunning(client, name, namespace)
		return running, nil
	}
	ctx, _ := context.WithTimeout(context.Background(), timeoutPerDeploy)
	_, err = tools_watch.UntilWithoutRetry(ctx, w, condition)

	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("deployment %s never became ready", name)
	}
	return nil
}

// WaitForDeploymentToBeReady waits for the pods of a deployment to become ready
func WaitForDeploymentToBeReady(client kubernetes.Interface, name, namespace string, timeout time.Duration) error {
	d, err := client.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return err
	}

	// Skip watching if the deployment is ready
	if d.Status.Replicas != d.Status.ReadyReplicas {
		options := metav1.ListOptions{LabelSelector: selector.String()}
		w, err := client.CoreV1().Pods(namespace).Watch(options)

		if err != nil {
			return err
		}
		defer w.Stop()

		condition := func(event watch.Event) (bool, error) {
			pod := event.Object.(*v1.Pod)
			return IsPodReady(pod), nil
		}

		ctx, _ := context.WithTimeout(context.Background(), timeout)
		_, err = tools_watch.UntilWithoutRetry(ctx, w, condition)

		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("deployment %s never became ready", name)
		}
	}

	return nil
}

// DeploymentPodCount returns pod counts of deployment
func DeploymentPodCount(client kubernetes.Interface, name, namespace string) (int, error) {
	pods, err := GetDeploymentPods(client, name, namespace)
	if err == nil {
		return len(pods), err
	}
	return 0, err
}

// GetDeploymentPods returns pods of deployment
func GetDeploymentPods(client kubernetes.Interface, name, namespace string) ([]v1.Pod, error) {
	d, err := client.ExtensionsV1beta1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, err
	}

	options := metav1.ListOptions{LabelSelector: selector.String()}

	pods, err := client.CoreV1().Pods(namespace).List(options)

	return pods.Items, err
}
