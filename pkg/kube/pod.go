package kube

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	tools_watch "k8s.io/client-go/tools/watch"
)

// credit https://github.com/kubernetes/kubernetes/blob/8719b4a/pkg/api/v1/pod/util.go
// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *v1.Pod) bool {
	phase := pod.Status.Phase
	if phase != v1.PodRunning || pod.DeletionTimestamp != nil {
		return false
	}
	return IsPodReadyConditionTrue(pod.Status)
}

// IsPodCompleted returns true if a pod is completed (succeeded or failed); false otherwise.
func IsPodCompleted(pod *v1.Pod) bool {
	phase := pod.Status.Phase
	if phase == v1.PodSucceeded || phase == v1.PodFailed {
		return true
	}
	return false
}

// IsPodSucceeded returns true if a pod is succeeded
func IsPodSucceeded(pod *v1.Pod) bool {
	phase := pod.Status.Phase
	if phase == v1.PodSucceeded {
		return true
	}
	return false
}

// credit https://github.com/kubernetes/kubernetes/blob/8719b4a/pkg/api/v1/pod/util.go
// IsPodReady retruns true if a pod is ready; false otherwise.
func IsPodReadyConditionTrue(status v1.PodStatus) bool {
	condition := GetPodReadyCondition(status)
	return condition != nil && condition.Status == v1.ConditionTrue
}

func PodStatus(pod *v1.Pod) string {
	if pod.DeletionTimestamp != nil {
		return "Terminating"
	}
	phase := pod.Status.Phase
	if IsPodReady(pod) {
		return "Ready"
	}
	return string(phase)
}

// credit https://github.com/kubernetes/kubernetes/blob/8719b4a/pkg/api/v1/pod/util.go
// Extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func GetPodReadyCondition(status v1.PodStatus) *v1.PodCondition {
	_, condition := GetPodCondition(&status, v1.PodReady)
	return condition
}

// credit https://github.com/kubernetes/kubernetes/blob/8719b4a/pkg/api/v1/pod/util.go
// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetPodCondition(status *v1.PodStatus, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

// GetCurrentPod returns the current pod the code is running in or nil if it cannot be deduced
func GetCurrentPod(kubeClient kubernetes.Interface, ns string) (*v1.Pod, error) {
	name := os.Getenv("HOSTNAME")
	if name == "" {
		return nil, nil
	}
	name = naming.ToValidName(name)
	return kubeClient.CoreV1().Pods(ns).Get(name, meta_v1.GetOptions{})
}

func waitForPodSelector(client kubernetes.Interface, namespace string, options meta_v1.ListOptions,
	timeout time.Duration, condition func(event watch.Event) (bool, error)) error {
	w, err := client.CoreV1().Pods(namespace).Watch(options)
	if err != nil {
		return err
	}
	defer w.Stop()

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	_, err = tools_watch.UntilWithoutRetry(ctx, w, condition)

	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("pod %s never became ready", options.String())
	}
	return nil
}

// HasContainerStarted returns true if the given Container has started running
func HasContainerStarted(pod *v1.Pod, idx int) bool {
	if pod == nil {
		return false
	}
	_, statuses, _ := GetContainersWithStatusAndIsInit(pod)
	if idx >= len(statuses) {
		return false
	}
	ic := statuses[idx]
	if ic.State.Running != nil || ic.State.Terminated != nil {
		return true
	}
	return false
}

// waits for the pod to become ready using the pod name
func WaitForPodNameToBeReady(client kubernetes.Interface, namespace string, name string, timeout time.Duration) error {
	options := meta_v1.ListOptions{
		// TODO
		//FieldSelector: fields.OneTermEqualSelector(api.ObjectNameField, name).String(),
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	}
	condition := func(event watch.Event) (bool, error) {
		pod := event.Object.(*v1.Pod)

		return IsPodReady(pod), nil
	}
	return waitForPodSelector(client, namespace, options, timeout, condition)
}

// WaitForPodNameToBeComplete waits for the pod to complete (succeed or fail) using the pod name
func WaitForPodNameToBeComplete(client kubernetes.Interface, namespace string, name string,
	timeout time.Duration) error {
	options := meta_v1.ListOptions{
		// TODO
		//FieldSelector: fields.OneTermEqualSelector(api.ObjectNameField, name).String(),
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name).String(),
	}
	condition := func(event watch.Event) (bool, error) {
		pod := event.Object.(*v1.Pod)

		return IsPodCompleted(pod), nil
	}
	return waitForPodSelector(client, namespace, options, timeout, condition)
}

func GetPodNames(client kubernetes.Interface, ns string, filter string) ([]string, error) {
	names := []string{}
	list, err := client.CoreV1().Pods(ns).List(meta_v1.ListOptions{})
	if err != nil {
		return names, fmt.Errorf("Failed to load Pods %s", err)
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

func GetPods(client kubernetes.Interface, ns string, filter string) ([]string, map[string]*v1.Pod, error) {
	names := []string{}
	m := map[string]*v1.Pod{}
	list, err := client.CoreV1().Pods(ns).List(meta_v1.ListOptions{})
	if err != nil {
		return names, m, fmt.Errorf("Failed to load Pods %s", err)
	}
	for _, d := range list.Items {
		c := d
		name := d.Name
		m[name] = &c
		if filter == "" || strings.Contains(name, filter) && d.DeletionTimestamp == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, m, nil
}

func GetPodsWithLabels(client kubernetes.Interface, ns string, selector string) ([]string, map[string]*v1.Pod, error) {
	names := []string{}
	m := map[string]*v1.Pod{}
	list, err := client.CoreV1().Pods(ns).List(meta_v1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return names, m, fmt.Errorf("Failed to load Pods %s", err)
	}
	for _, d := range list.Items {
		c := d
		name := d.Name
		m[name] = &c
		if d.DeletionTimestamp == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, m, nil
}

// GetDevPodNames returns the users dev pod names. If username is blank, all devpod names will be returned
func GetDevPodNames(client kubernetes.Interface, ns string, username string) ([]string, map[string]*v1.Pod, error) {
	names := []string{}
	m := map[string]*v1.Pod{}
	listOptions := meta_v1.ListOptions{}
	if username != "" {
		listOptions.LabelSelector = LabelDevPodUsername + "=" + username
	} else {
		listOptions.LabelSelector = LabelDevPodName
	}
	list, err := client.CoreV1().Pods(ns).List(listOptions)
	if err != nil {
		return names, m, fmt.Errorf("Failed to load Pods %s", err)
	}
	for _, d := range list.Items {
		c := d
		name := d.Name
		m[name] = &c
		names = append(names, name)
	}
	sort.Strings(names)
	return names, m, nil
}

// GetPodRestars returns the number of restarts of a POD
func GetPodRestarts(pod *v1.Pod) int32 {
	var restarts int32
	statuses := pod.Status.ContainerStatuses
	if len(statuses) == 0 {
		return restarts
	}
	for _, status := range statuses {
		restarts += status.RestartCount
	}
	return restarts
}

// GetContainersWithStatusAndIsInit gets the containers in the pod, either init containers or non-init depending on whether
// non-init containers are present, and a flag as to whether this list of containers are init containers or not.
func GetContainersWithStatusAndIsInit(pod *v1.Pod) ([]v1.Container, []v1.ContainerStatus, bool) {
	isInit := true
	allContainers := pod.Spec.InitContainers
	statuses := pod.Status.InitContainerStatuses
	containers := pod.Spec.Containers

	if len(containers) > 1 && len(pod.Status.ContainerStatuses) == len(containers) {
		isInit = false
		// Add the non-init containers
		// If there's a "nop" container at the end, the pod was created with before Tekton 0.5.x, so trim off the no-op container at the end of the list.
		if containers[len(containers)-1].Name == "nop" {
			allContainers = append(allContainers, containers[:len(containers)-1]...)
		} else {
			allContainers = append(allContainers, containers...)
		}
		// Since status ordering is unpredictable, don't trim here - we'll be sorting/filtering below anyway.
		statuses = append(statuses, pod.Status.ContainerStatuses...)
	}

	var sortedStatuses []v1.ContainerStatus
	for _, c := range allContainers {
		for _, cs := range statuses {
			if cs.Name == c.Name {
				sortedStatuses = append(sortedStatuses, cs)
				break
			}
		}
	}
	return allContainers, sortedStatuses, isInit
}
