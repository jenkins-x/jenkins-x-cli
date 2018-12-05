package cmd

import (
	"fmt"
	"github.com/jenkins-x/jx/pkg/gits"
	"io"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/jenkins-x/jx/pkg/kube"
)

// ControllerBuildOptions are the flags for the commands
type ControllerBuildOptions struct {
	ControllerOptions

	Namespace string

	EnvironmentCache *kube.EnvironmentNamespaceCache
}

// NewCmdControllerBuild creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdControllerBuild(f Factory, in terminal.FileReader, out terminal.FileWriter, errOut io.Writer) *cobra.Command {
	options := &ControllerBuildOptions{
		ControllerOptions: ControllerOptions{
			CommonOptions: CommonOptions{
				Factory: f,
				In:      in,
				Out:     out,
				Err:     errOut,
			},
		},
	}

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Runs the build controller",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
		Aliases: []string{"builds"},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to watch or defaults to the current namespace")
	return cmd
}

// Run implements this command
func (o *ControllerBuildOptions) Run() error {
	apisClient, err := o.CreateApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterPipelineActivityCRD(apisClient)
	if err != nil {
		return err
	}

	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	kubeClient, _, err := o.KubeClient()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if ns == "" {
		ns = devNs
	}

	o.EnvironmentCache = kube.CreateEnvironmentCache(jxClient, ns)

	pod := &corev1.Pod{}
	log.Infof("Watching for Knative build pods in namespace %s\n", util.ColorInfo(ns))
	listWatch := cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "pods", ns, fields.Everything())
	kube.SortListWatchByName(listWatch)
	_, controller := cache.NewInformer(
		listWatch,
		pod,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onPod(obj, kubeClient, jxClient, ns)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onPod(newObj, kubeClient, jxClient, ns)
			},
			DeleteFunc: func(obj interface{}) {
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}

func (o *ControllerBuildOptions) onPod(obj interface{}, kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		log.Infof("Object is not a Pod %#v\n", obj)
		return
	}
	if pod != nil {
		labels := pod.Labels
		if labels != nil {
			buildName := labels[builds.LabelBuildName]
			if buildName == "" {
				buildName = labels[builds.LabelOldBuildName]
			}
			if buildName != "" {
				//log.Infof("Found build pod %s\n", pod.Name)

				activities := jxClient.JenkinsV1().PipelineActivities(ns)
				key := o.createPromoteStepActivityKey(buildName, pod)
				if key != nil {
					name := ""
					err := util.Retry(time.Second*20, func() error {
						a, created, err := key.GetOrCreate(activities)
						if err != nil {
							operation := "update"
							if created {
								operation = "create"
							}
							log.Warnf("Failed to %s PipelineActivities for build %s: %s\n", operation, buildName, err)
						}
						if o.updatePipelineActivity(kubeClient, ns, a, buildName, pod) {
							_, err := activities.Update(a)
							if err != nil {
								name = a.Name
								return err
							}
						}
						return nil
					})
					if err != nil {
						log.Warnf("Failed to update PipelineActivities%s: %s\n", name, err)
					}
				}
			}
		}
	}
}

// createPromoteStepActivityKey deduces the pipeline metadata from the Knative build pod
func (o *ControllerBuildOptions) createPromoteStepActivityKey(buildName string, pod *corev1.Pod) *kube.PromoteStepActivityKey {

	buildInfo := builds.CreateBuildPodInfo(pod)
	if buildInfo.GitURL == "" || buildInfo.GitInfo == nil {
		return nil
	}
	return &kube.PromoteStepActivityKey{
		PipelineActivityKey: kube.PipelineActivityKey{
			Name:              buildInfo.Name,
			Pipeline:          buildInfo.Pipeline,
			Build:             buildInfo.Build,
			LastCommitSHA:     buildInfo.LastCommitSHA,
			LastCommitMessage: buildInfo.LastCommitMessage,
			LastCommitURL:     buildInfo.LastCommitURL,
			GitInfo:           buildInfo.GitInfo,
		},
	}
}

func (o *ControllerBuildOptions) updatePipelineActivity(kubeClient kubernetes.Interface, ns string, activity *v1.PipelineActivity, buildName string, pod *corev1.Pod) bool {
	copy := *activity
	for _, c := range pod.Status.InitContainerStatuses {
		name := strings.Replace(strings.TrimPrefix(c.Name, "build-step-"), "-", " ", -1)
		title := strings.Title(name)
		_, stage, _ := kube.GetOrCreateStage(activity, title)

		running := c.State.Running
		terminated := c.State.Terminated

		var startedAt metav1.Time
		var finishedAt metav1.Time
		if running != nil {
			startedAt = running.StartedAt
		} else if terminated != nil {
			startedAt = terminated.StartedAt
			finishedAt = terminated.FinishedAt

			if !finishedAt.IsZero() {
				stage.CompletedTimestamp = &finishedAt
			}
		}
		if !startedAt.IsZero() {
			stage.StartedTimestamp = &startedAt
		}
		stage.Description = createStepDescription(c.Name, pod)

		if terminated != nil {
			if terminated.ExitCode == 0 {
				stage.Status = v1.ActivityStatusTypeSucceeded
			} else {
				stage.Status = v1.ActivityStatusTypeFailed
			}
		} else {
			if running != nil {
				stage.Status = v1.ActivityStatusTypeRunning
			} else {
				stage.Status = v1.ActivityStatusTypePending
			}
		}
	}
	spec := &activity.Spec
	var biggestFinishedAt metav1.Time

	allCompleted := true
	failed := false
	running := true
	for _, step := range spec.Steps {
		stage := step.Stage
		if stage != nil {
			stageFinished := false
			if stage.StartedTimestamp != nil && spec.StartedTimestamp == nil {
				spec.StartedTimestamp = stage.StartedTimestamp
			}
			if stage.CompletedTimestamp != nil {
				t := stage.CompletedTimestamp
				if !t.IsZero() {
					stageFinished = true
					if biggestFinishedAt.IsZero() || t.After(biggestFinishedAt.Time) {
						biggestFinishedAt = *t
					}
				}
			}
			if stageFinished {
				if stage.Status != v1.ActivityStatusTypeSucceeded {
					failed = true
				}
			} else {
				allCompleted = false
			}
			if stage.Status == v1.ActivityStatusTypeRunning {
				running = true
			}
			if stage.Status == v1.ActivityStatusTypeRunning || stage.Status == v1.ActivityStatusTypePending {
				allCompleted = false
			}
		}
	}
	if allCompleted {
		if failed {
			spec.Status = v1.ActivityStatusTypeFailed
		} else {
			spec.Status = v1.ActivityStatusTypeSucceeded
		}
		if !biggestFinishedAt.IsZero() {
			spec.CompletedTimestamp = &biggestFinishedAt
		}
		// lets ensure we overwrite any canonical jenkins build URL thats generated automatically
		if spec.BuildLogsURL == "" || !strings.Contains(spec.BuildLogsURL, pod.Name) {
			podInterface := kubeClient.CoreV1().Pods(ns)

			envName := kube.LabelValueDevEnvironment
			devEnv := o.EnvironmentCache.Item(envName)
			var location  *v1.StorageLocation
			if devEnv == nil {
				log.Warnf("No Environment %s found\n", envName)
			} else {
				location = devEnv.Spec.TeamSettings.StorageLocation(kube.ClassificationLogs)
			}
			if location == nil {
				location = &v1.StorageLocation{}
			}
			if location.IsEmpty() {
				location.GitURL = activity.Spec.GitURL
				if location.GitURL == "" {
					log.Warnf("No GitURL on PipelineActivity %s\n", activity.Name)
				}
			}
			spec.BuildLogsURL = generateBuildLogURL(podInterface, ns, activity, buildName, pod, location)
		}
	} else {
		if running {
			spec.Status = v1.ActivityStatusTypeRunning
		} else {
			spec.Status = v1.ActivityStatusTypePending
		}
	}
	return !reflect.DeepEqual(&copy, activity)
}

// generates the build log URL and returns the URL
func generateBuildLogURL(podInterface typedcorev1.PodInterface, ns string, activity *v1.PipelineActivity, buildName string, pod *corev1.Pod, location *v1.StorageLocation) string {
	data, err := builds.GetBuildLogsForPod(podInterface, pod)
	if err != nil {
		// probably due to not being available yet
		//log.Warnf("Failed to get build log for pod %s in namespace %s: %s\n", pod.Name, ns, err)
		return ""
	}

	//log.Infof("got build log for pod: %s PipelineActivity: %s with bytes: %d\n", pod.Name, activity.Name, len(data))
	
	sourceURL := location.GitURL
	if sourceURL == "" {
		// TODO handle http URLs too
		return ""
	}
	gitClient := gits.NewGitCLI()
	ghPagesDir, err := cloneGitHubPagesBranchToTempDir(sourceURL, gitClient)
	if err != err {
		log.Warnf("Failed to git clone gh-pages branch for %s: %s\n", sourceURL, err)
		return ""
	}

	owner := activity.Spec.GitOwner
	repository := activity.RepositoryName()
	branch := activity.BranchName()
	buildNumber := activity.Spec.Build
	if buildNumber == "" {
		buildNumber = "1"
	}

	pathDir := filepath.Join("jenkins-x", "logs", owner, repository, branch)
	outDir := filepath.Join(ghPagesDir, pathDir)
	err = os.MkdirAll(outDir, util.DefaultWritePermissions)
	if err != nil {
		log.Warnf("Failed to write create dir for log file %s: %s\n", outDir, err)
		return ""
	}

	fileName := filepath.Join(pathDir, buildNumber+".log")
	outFile := filepath.Join(ghPagesDir, fileName)
	err = ioutil.WriteFile(outFile, data, util.DefaultWritePermissions)
	if err != nil {
		log.Warnf("Failed to write log file %s: %s\n", outFile, err)
		return ""
	}

	err = gitClient.Add(ghPagesDir, pathDir)
	if err != nil {
		log.Warnf("Failed to add to gh-pages repo dir %s: %s\n", pathDir, err)
		return ""
	}
	err = gitClient.CommitIfChanges(ghPagesDir, fmt.Sprintf("Publishing log for Pipeline %s", activity.Name))
	if err != nil {
		log.Warnf("Failed to commit gh-pages repo dir %s: %s\n", ghPagesDir, err)
		return ""
	}
	err = gitClient.Push(ghPagesDir)
	if err != nil {
		log.Warnf("Failed to push gh-pages repo dir %s: %s\n", ghPagesDir, err)
		return ""
	}

	// TODO only github supported for now! Lets switch to Provider
	return fmt.Sprintf("https://%s.github.io/%s/%s", owner, repository, fileName)
}

// createStepDescription uses the spec of the init container to return a description
func createStepDescription(initContainerName string, pod *corev1.Pod) string {
	for _, c := range pod.Spec.InitContainers {
		args := c.Args
		if c.Name == initContainerName && len(args) > 0 {
			if args[0] == "-url" && len(args) > 1 {
				return args[1]
			}
		}
	}
	return ""
}

// DigitSuffix outputs digital suffix
func DigitSuffix(text string) string {
	answer := ""
	for {
		l := len(text)
		if l == 0 {
			return answer
		}
		lastChar := text[l-1:]
		for _, rune := range lastChar {
			if !unicode.IsDigit(rune) {
				return answer
			}
			break
		}
		answer = lastChar + answer
		text = text[0 : l-1]
	}
}
