package opts

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// WaitForReadyPodForDeployment waits for a pod of a deployment to be ready
func (o *CommonOptions) WaitForReadyPodForDeployment(c kubernetes.Interface, ns string, name string, names []string, readyOnly bool) (string, error) {
	deployment, err := c.AppsV1().Deployments(ns).Get(name, metav1.GetOptions{})
	if err != nil || deployment == nil {
		return "", util.InvalidArg(name, names)
	}
	selector := deployment.Spec.Selector
	if selector == nil {
		return "", fmt.Errorf("No selector defined on Deployment %s in namespace %s", name, ns)
	}
	labels := selector.MatchLabels
	if labels == nil {
		return "", fmt.Errorf("No MatchLabels defined on the Selector of Deployment %s in namespace %s", name, ns)
	}
	return o.WaitForReadyPodForSelectorLabels(c, ns, labels, readyOnly)
}

// WaitForReadyPodForSelectorLabels waits for a pod selected by the given labels to be ready
func (o *CommonOptions) WaitForReadyPodForSelectorLabels(c kubernetes.Interface, ns string, labels map[string]string, readyOnly bool) (string, error) {
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: labels})
	if err != nil {
		return "", err
	}
	return o.WaitForReadyPodForSelector(c, ns, selector, readyOnly)
}

// WaitForReadyPodForSelector waits for a pod to which the selector applies to be ready
func (o *CommonOptions) WaitForReadyPodForSelector(c kubernetes.Interface, ns string, selector labels.Selector, readyOnly bool) (string, error) {
	log.Logger().Warnf("Waiting for a running pod in namespace %s with labels %v", ns, selector.String())
	lastPod := ""
	for {
		pods, err := c.CoreV1().Pods(ns).List(metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return "", err
		}
		name := ""
		loggedContainerIdx := -1
		var latestPod *corev1.Pod
		lastTime := time.Time{}
		for _, pod := range pods.Items {
			phase := pod.Status.Phase
			if phase == corev1.PodRunning || phase == corev1.PodPending {
				if !readyOnly {
					created := pod.CreationTimestamp
					if name == "" || created.After(lastTime) {
						lastTime = created.Time
						name = pod.Name
						latestPod = &pod
					}
				}
			}
		}
		if latestPod != nil && name != "" {
			if name != lastPod {
				lastPod = name
				loggedContainerIdx = -1
				log.Logger().Warnf("Found newest pod: %s", name)
			}
			if kube.IsPodReady(latestPod) {
				return name, nil
			}

			_, containerStatuses, _ := kube.GetContainersWithStatusAndIsInit(latestPod)
			for idx, ic := range containerStatuses {
				if isContainerStarted(&ic.State) && idx > loggedContainerIdx {
					loggedContainerIdx = idx
					containerName := ic.Name
					log.Logger().Warnf("Container on pod: %s is: %s", name, containerName)
					err = o.TailLogs(ns, name, containerName)
					if err != nil {
						break
					}
				}
			}
		}
		// TODO replace with a watch flavour
		time.Sleep(time.Second)
	}
}

func isContainerStarted(state *corev1.ContainerState) bool {
	if state == nil {
		return false
	}
	if state.Running != nil {
		return !state.Running.StartedAt.IsZero()
	}
	if state != nil && state.Terminated != nil {
		return !state.Terminated.StartedAt.IsZero()
	}
	return false
}
