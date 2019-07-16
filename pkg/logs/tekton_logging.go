package logs

import (
	"fmt"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/tekton"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	v1alpha12 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LogWriter is an interface that can be implemented to define different ways to stream / write logs
// it's the implementer's responsibility to route those logs through the corresponding medium
type LogWriter interface {
	WriteLog(line string) error
	StreamLog(ns string, pod *corev1.Pod, containerName *corev1.Container) error
}

// GetTektonPipelinesWithActivePipelineActivity returns list of all PipelineActivities with corresponding Tekton PipelineRuns ordered by the PipelineRun creation timestamp and a map to obtain its reference once a name has been selected
func GetTektonPipelinesWithActivePipelineActivity(jxClient versioned.Interface, tektonClient tektonclient.Interface, ns string, filters []string, context string) ([]string, map[string]*v1.PipelineActivity, error) {
	labelsFilter := strings.Join(filters, ",")
	paList, err := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{
		LabelSelector: labelsFilter,
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "there was a problem getting the PipelineActivities")
	}

	paMap := make(map[string]*v1.PipelineActivity)
	for _, pa := range paList.Items {
		p := pa
		paMap[createPipelineActivityName(p.Labels, p.Spec.Build)] = &p
	}

	// This is a temporary solution until we add the "context" label to PipelineActivities
	labelsFilter = strings.Replace(labelsFilter, "repository=", "repo=", 1)
	if context != "" {
		labelsFilter = modifyFilterForPipelineRun(labelsFilter, context)
	}
	tektonPRs, _ := tektonClient.TektonV1alpha1().PipelineRuns(ns).List(metav1.ListOptions{
		LabelSelector: labelsFilter,
	})

	sort.Slice(tektonPRs.Items, func(i, j int) bool {
		return tektonPRs.Items[i].CreationTimestamp.After(tektonPRs.Items[j].CreationTimestamp.Time)
	})

	var names []string
	for _, pr := range tektonPRs.Items {
		prBuildNumber := pr.Labels[v1.LabelBuild]
		if prBuildNumber == "" {
			prBuildNumber = findLegacyPipelineRunBuildNumber(&pr)
		}
		paName := createPipelineActivityName(pr.Labels, prBuildNumber)
		if _, exists := paMap[paName]; exists {
			nameWithContext := fmt.Sprintf("%s %s", paName, pr.Labels["context"])
			replacePipelineActivityNameWithEnrichedName(paMap, paName, nameWithContext)
			names = append(names, nameWithContext)
		}
	}

	return names, paMap, nil
}

func modifyFilterForPipelineRun(labelsFilter string, context string) string {
	contextFilter := fmt.Sprintf("context=%s", context)
	if labelsFilter == "" {
		return contextFilter
	}
	return fmt.Sprintf("%s,%s", labelsFilter, contextFilter)
}

func createPipelineActivityName(labels map[string]string, buildNumber string) string {
	repository := labels[v1.LabelRepository]
	// The label is called "repo" in the PipelineRun CRD and "repository" in the PipelineActivity CRD
	if repository == "" {
		repository = labels["repo"]
	}
	return strings.ToLower(fmt.Sprintf("%s/%s/%s #%s", labels[v1.LabelOwner], repository, labels[v1.LabelBranch], buildNumber))
}

func replacePipelineActivityNameWithEnrichedName(paMap map[string]*v1.PipelineActivity, formerName string, newName string) {
	paMap[newName] = paMap[formerName]
	delete(paMap, formerName)
}

func findLegacyPipelineRunBuildNumber(pipelineRun *v1alpha12.PipelineRun) string {
	var buildNumber string
	for _, p := range pipelineRun.Spec.Params {
		if p.Name == "build_id" {
			buildNumber = p.Value
		}
	}
	return buildNumber
}

func getPipelineRunNamesForActivity(pa *v1.PipelineActivity, tektonClient tektonclient.Interface) ([]string, error) {
	filters := []string{
		fmt.Sprintf("%s=%s", v1.LabelOwner, pa.Spec.GitOwner),
		fmt.Sprintf("%s=%s", v1.LabelRepository, pa.Spec.GitRepository),
		fmt.Sprintf("%s=%s", v1.LabelBranch, pa.Spec.GitBranch),
	}

	tektonPRs, err := tektonClient.TektonV1alpha1().PipelineRuns(pa.Namespace).List(metav1.ListOptions{
		LabelSelector: strings.Join(filters, ","),
	})
	if err != nil {
		return nil, err
	}
	// For legacy purposes, look for the old "repo" label as well.
	if len(tektonPRs.Items) == 0 {
		tektonPRs, err = tektonClient.TektonV1alpha1().PipelineRuns(pa.Namespace).List(metav1.ListOptions{
			LabelSelector: strings.Replace(strings.Join(filters, ","), "repository=", "repo=", 1),
		})
		if err != nil {
			return nil, err
		}
	}
	var names []string
	for _, pr := range tektonPRs.Items {
		buildNumber := pr.Labels[tekton.LabelBuild]
		if buildNumber == "" {
			buildNumber = findLegacyPipelineRunBuildNumber(&pr)
		}
		if buildNumber == pa.Spec.Build {
			names = append(names, pr.Name)
		}
	}

	return names, nil
}

// GetRunningBuildLogs obtains the logs of the provided PipelineActivity and streams the running build pods' logs using the provided LogWriter
func GetRunningBuildLogs(pa *v1.PipelineActivity, buildName string, kubeClient kubernetes.Interface, tektonClient tektonclient.Interface, writer LogWriter) error {
	pipelineRunNames, err := getPipelineRunNamesForActivity(pa, tektonClient)
	if err != nil {
		return errors.Wrapf(err, "failed to get PipelineRun names for activity %s in namespace %s", pa.Name, pa.Namespace)
	}
	pipelineRunsLogged := make(map[string]bool)
	foundLogs := false

	for len(pipelineRunNames) > len(pipelineRunsLogged) {
		pods, err := builds.GetBuildPods(kubeClient, pa.Namespace)
		if err != nil {
			return errors.Wrapf(err, "failed to get build pods in namespace %s", pa.Namespace)
		}

		sort.Slice(pods, func(i, j int) bool {
			return pods[i].CreationTimestamp.Before(&pods[j].CreationTimestamp)
		})

		runsSeenForPods := make(map[string]bool)
		// This method will be executed by both the CLI and the UI, we don't know if the UI has color enabled, so we are using a local instance instead of the global one
		c := color.New(color.FgGreen)
		c.EnableColor()
		for _, pod := range pods {
			stageName := pod.Labels["jenkins.io/task-stage-name"]
			pipelineRun := pod.Labels[builds.LabelPipelineRunName]
			_, seen := pipelineRunsLogged[pipelineRun]

			params := builds.CreateBuildPodInfo(pod)
			if !seen && params.Organisation == pa.Spec.GitOwner && params.Repository == pa.Spec.GitRepository &&
				strings.ToLower(params.Branch) == strings.ToLower(pa.Spec.GitBranch) && params.Build == pa.Spec.Build {
				runsSeenForPods[pipelineRun] = true
				foundLogs = true
				containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)
				for i, ic := range containers {
					pod, err = waitForContainerToStart(kubeClient, pa.Namespace, pod, i, writer)
					err = writer.WriteLog(fmt.Sprintf("Showing logs for build %v stage %s and container %s\n", c.Sprintf(buildName), c.Sprintf(stageName), c.Sprintf(ic.Name)))
					if err != nil {
						return errors.Wrapf(err, "there was a problem writing a single line into the logs writer")
					}
					err = writer.StreamLog(pa.Namespace, pod, &ic)
					if err != nil {
						return errors.Wrapf(err, "there was a problem writing into the stream writer")
					}
				}
			}
		}
		pipelineRunNames, err = getPipelineRunNamesForActivity(pa, tektonClient)
		if err != nil {
			return errors.Wrapf(err, "failed to get PipelineRun names for activity %s in namespace %s", pa.Name, pa.Namespace)
		}
		for k, v := range runsSeenForPods {
			pipelineRunsLogged[k] = v
		}
		if !foundLogs {
			break
		}
	}
	if !foundLogs {
		return errors.New("the build pods for this build have been garbage collected and the log was not found in the long term storage bucket")
	}

	return nil
}

func waitForContainerToStart(kubeClient kubernetes.Interface, ns string, pod *corev1.Pod, idx int, logWriter LogWriter) (*corev1.Pod, error) {
	if pod.Status.Phase == corev1.PodFailed {
		log.Logger().Warnf("pod %s has failed", pod.Name)
		return pod, nil
	}
	if kube.HasContainerStarted(pod, idx) {
		return pod, nil
	}
	containerName := ""
	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)
	if idx < len(containers) {
		containerName = containers[idx].Name
	}
	// This method will be executed by both the CLI and the UI, we don't know if the UI has color enabled, so we are using a local instance instead of the global one
	c := color.New(color.FgGreen)
	c.EnableColor()
	if err := logWriter.WriteLog(fmt.Sprintf("waiting for pod %s container %s to start...\n", c.Sprintf(pod.Name), c.Sprintf(containerName))); err != nil {
		log.Logger().Warn("There was a problem writing a single line into the writeFN")
	}
	for {
		time.Sleep(time.Second)

		p, err := kubeClient.CoreV1().Pods(ns).Get(pod.Name, metav1.GetOptions{})
		if err != nil {
			return p, errors.Wrapf(err, "failed to load pod %s", pod.Name)
		}
		if kube.HasContainerStarted(p, idx) {
			return p, nil
		}
	}
}

// StreamPipelinePersistentLogs reads logs from the provided bucket URL and writes them using the provided LogWriter
func StreamPipelinePersistentLogs(logWriter LogWriter, logsURL string) error {
	//TODO: This should be changed in the future when other bucket providers are supported
	urlParts := strings.Split(logsURL, "://")
	var scheme string
	if len(urlParts) > 1 {
		scheme = urlParts[0]
	}
	var logBytes []byte
	var err error
	switch scheme {
	case "gs":
		logBytes, err = gke.DownloadFileFromBucket(logsURL)
	case "http":
		fallthrough
	case "https":
		return logWriter.WriteLog("The build pods for this build have been garbage collected and long term storage bucket configuration wasn't found for this environment")
	default:
		return logWriter.WriteLog(fmt.Sprintf("The provided logsURL scheme is not supported: %s", scheme))
	}

	if err != nil {
		return errors.Wrapf(err, "there was a problem obtaining the log file %s", logsURL)
	}
	return logWriter.WriteLog(string(logBytes))
}
