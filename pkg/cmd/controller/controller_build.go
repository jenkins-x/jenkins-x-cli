package controller

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/step/git/credentials"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/logs"
	"k8s.io/apimachinery/pkg/fields"

	"github.com/jenkins-x/jx/pkg/collector"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/tekton"
	"github.com/jenkins-x/jx/pkg/tekton/metapipeline"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"k8s.io/client-go/kubernetes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/jenkins-x/jx/pkg/kube"
)

const (
	defaultTargetURLTemplate = "{{ .BaseURL }}/teams/{{ .Namespace }}/projects/{{ .Owner }}/{{ .Repository }}/{{ .Branch }}/{{ .Build }}"
)

// ControllerBuildOptions are the flags for the commands
type ControllerBuildOptions struct {
	ControllerOptions

	Namespace           string
	InitGitCredentials  bool
	GitReporting        bool
	TargetURLTemplate   string
	FailIfNoGitProvider bool
	JobURLBase          string

	EnvironmentCache *kube.EnvironmentNamespaceCache

	DryRun bool

	// private fields added for easier testing
	gitHubProvider gits.GitProvider
}

// LongTermStorageLogWriter is an implementation of logs.LogWriter that saves the obtained log lines
// and sends them to a Collector when the channel is closed
type LongTermStorageLogWriter struct {
	data       []byte
	kubeClient kubernetes.Interface
	logMasker  *kube.LogMasker
}

// WriteLog will receive a logs.LogLine value and append its bytes to the LongTermStorageLogWriter stored bytes
func (w *LongTermStorageLogWriter) WriteLog(logLine logs.LogLine, lch chan<- logs.LogLine) error {
	lch <- logLine
	return nil
}

// StreamLog will receive a logs channel and an errors channel which the logs producer will send
// it will mask the lines marked as ShouldMask then it will append the line's bytes to the already stored ones
func (w *LongTermStorageLogWriter) StreamLog(lch <-chan logs.LogLine, ech <-chan error) error {
	for {
		select {
		case l, ok := <-lch:
			if !ok {
				return nil
			}
			if w.logMasker != nil && l.ShouldMask {
				l.Line = w.logMasker.MaskLog(l.Line)
			}
			line := []byte(l.Line)
			line = append(line, '\n')
			w.data = append(w.data, line...)
		case err := <-ech:
			return err
		}
	}
}

// BytesLimit defines the limit of bytes to be used to fetch the logs from the kube API
// defaulted to 0 for this implementation
func (w *LongTermStorageLogWriter) BytesLimit() int {
	//We are not limiting bytes with this writer
	return 0
}

// NewCmdControllerBuild creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdControllerBuild(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ControllerBuildOptions{
		ControllerOptions: ControllerOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Runs the build controller",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		Aliases: []string{"builds"},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to watch or defaults to the current namespace")
	cmd.Flags().BoolVarP(&options.InitGitCredentials, "git-credentials", "", false, "If enable then lets run the 'jx step git credentials' step to initialise git credentials")
	cmd.Flags().BoolVarP(&options.FailIfNoGitProvider, "fail-on-git-provider-error", "", false, "If enable then lets terminate quickly if we cannot create a git provider")

	// optional git reporting flags
	cmd.Flags().StringVarP(&options.TargetURLTemplate, "target-url-template", "", "", "The Go template for generating the target URL of pipeline logs/views if git reporting is enabled. If unspecified, a default will be used based on `--job-url-base`.")
	cmd.Flags().BoolVarP(&options.GitReporting, "git-reporting", "", false, "If enabled then lets report pipeline success/failures to the git provider. Note this is purely tactical until we can do this natively inside tekton")
	cmd.Flags().StringVarP(&options.JobURLBase, "job-url-base", "", "", "The base URL, such as 'https://dashboard.jenkins-x.live', for generating the target URL for pipeline logs if git reporting is enabled.")
	return cmd
}

// Run implements this command
func (o *ControllerBuildOptions) Run() error {
	apisClient, err := o.ApiExtensionsClient()
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

	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	tektonClient, _, err := o.TektonClient()
	if err != nil {
		return err
	}

	if !o.GitReporting {
		if strings.ToLower(os.Getenv("GIT_REPORTING")) == "true" {
			o.GitReporting = true
		}
	}
	if o.GitReporting {
		if o.TargetURLTemplate == "" {
			o.TargetURLTemplate = os.Getenv("TARGET_URL_TEMPLATE")
		}
		if o.TargetURLTemplate == "" {
			o.TargetURLTemplate = defaultTargetURLTemplate
		}
	}

	ns := o.Namespace
	if ns == "" {
		ns = devNs
	}

	o.EnvironmentCache = kube.CreateEnvironmentCache(jxClient, ns)

	if o.InitGitCredentials {
		err = o.InitGitConfigAndUser()
		if err != nil {
			return err
		}
	}

	err = o.ensureSourceRepositoryHasLabels(jxClient, ns)
	if err != nil {
		log.Logger().Warnf("failed to label the legacy SourceRepository resources: %s", err)
	}

	err = o.ensurePipelineActivityHasLabels(jxClient, ns)
	if err != nil {
		log.Logger().Warnf("failed to label the legacy PipelineActivity resources: %s", err)
	}

	pod := &corev1.Pod{}
	log.Logger().Infof("Watching for Pods in namespace %s", util.ColorInfo(ns))
	listWatch := cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "pods", ns, fields.Everything())
	kube.SortListWatchByName(listWatch)
	_, controller := cache.NewInformer(
		listWatch,
		pod,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onPipelinePod(obj, kubeClient, jxClient, tektonClient, ns)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onPipelinePod(newObj, kubeClient, jxClient, tektonClient, ns)
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

func (o *ControllerBuildOptions) handleStandalonePod(pod *corev1.Pod, kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string) {
	labels := pod.Labels
	if labels != nil {
		buildName := labels[builds.LabelBuildName]
		if buildName == "" {
			buildName = labels[builds.LabelOldBuildName]
		}
		if buildName == "" {
			buildName = labels[builds.LabelPipelineRunName]
		}
		if buildName != "" {
			log.Logger().Debugf("Found build pod %s", pod.Name)

			activities := jxClient.JenkinsV1().PipelineActivities(ns)
			key := o.createPromoteStepActivityKey(buildName, pod)
			if key != nil {
				name := ""
				err := util.Retry(time.Second*20, func() error {
					a, created, err := key.GetOrCreate(jxClient, ns)
					if err != nil {
						operation := "update"
						if created {
							operation = "create"
						}
						log.Logger().Warnf("Failed to %s PipelineActivities for build %s: %s", operation, buildName, err)
						return err
					}
					if o.updatePipelineActivity(kubeClient, ns, a, buildName, pod) {
						log.Logger().Debugf("updating PipelineActivity %s from handleStandalonePod()", a.Name)
						_, err := activities.PatchUpdate(a)
						if err != nil {
							log.Logger().Warnf("Failed to update PipelineActivity %s due to: %s", a.Name, err.Error())
							name = a.Name
							return err
						}
					}
					return nil
				})
				if err != nil {
					log.Logger().Warnf("Failed to update PipelineActivities %s: %s", name, err)
				}
			}
		}
	}

}

func (o *ControllerBuildOptions) onPipelinePod(obj interface{}, kubeClient kubernetes.Interface, jxClient versioned.Interface, tektonClient tektonclient.Interface, ns string) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		log.Logger().Infof("Object is not a Pod %#v", obj)
		return
	}
	if pod != nil {
		if pod.Labels[pipeline.GroupName+pipeline.PipelineRunLabelKey] != "" {
			if pod.Labels[syntax.LabelStageName] != "" {
				prName := pod.Labels[pipeline.GroupName+pipeline.PipelineRunLabelKey]
				pr, err := tektonClient.TektonV1alpha1().PipelineRuns(ns).Get(prName, metav1.GetOptions{})
				if err != nil {
					log.Logger().Warnf("Error getting PipelineRun for name %s: %s", prName, err)
					return
				}
				// Get the Pod for this PipelineRun
				podList, err := kubeClient.CoreV1().Pods(ns).List(metav1.ListOptions{
					LabelSelector: builds.LabelPipelineRunName + "=" + prName,
				})
				if err != nil {
					log.Logger().Warnf("Error getting PodList for PipelineRun %s: %s", prName, err)
					return
				}

				structure, err := tekton.StructureForPipelineRun(jxClient, ns, pr)
				if err != nil {
					log.Logger().Warnf("Error getting PipelineStructure for PipelineRun %s: %s", prName, err)
					return
				}
				pri, err := tekton.CreatePipelineRunInfo(prName, podList, structure, pr)
				if err != nil {
					log.Logger().Warnf("Error creating PipelineRunInfo for PipelineRun %s: %s", prName, err)
					return
				}
				if pri == nil {
					log.Logger().Warnf("No PipelineRunInfo created for PipelineRun %s: %s", prName, err)
					return
				}

				log.Logger().Debugf("Found pipeline run %s", pri.Name)

				activities := jxClient.JenkinsV1().PipelineActivities(ns)
				key := o.createPromoteStepActivityKeyFromRun(pri)
				if key != nil {
					name := ""
					err := util.Retry(time.Second*20, func() error {
						a, created, err := key.GetOrCreate(jxClient, ns)
						if err != nil {
							operation := "update"
							if created {
								operation = "create"
							}
							log.Logger().Warnf("Failed to %s PipelineActivities for build %s: %s", operation, pri.Name, err)
							return err
						}
						if o.updatePipelineActivityForRun(kubeClient, ns, a, pri, pod) {
							log.Logger().Debugf("updating PipelineActivity %s from updatePipelineActivityForRun()", a.Name)
							_, err := activities.PatchUpdate(a)
							if err != nil {
								log.Logger().Warnf("Failed to update PipelineActivity %s due to: %s", a.Name, err.Error())
								name = a.Name
								return err
							}
						}
						return nil
					})
					if err != nil {
						log.Logger().Warnf("Failed to update PipelineActivities %s: %s", name, err)
					}
				}
			} else {
				o.handleStandalonePod(pod, kubeClient, jxClient, ns)
			}
		}
	}
}

// createPromoteStepActivityKey deduces the pipeline metadata from the build pod
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
			Context:           buildInfo.Context,
		},
	}
}

// completeBuildSourceInfo sets the PR author and PR title from GitHub in the given PA
// If the PA is a branch build it then sets the commit author and last commit message
func (o *ControllerBuildOptions) completeBuildSourceInfo(activity *v1.PipelineActivity) error {

	log.Logger().Infof("[BuildInfo] Completing build info for PipelineActivity=%s", activity.Name)

	gitInfo, err := gits.ParseGitURL(activity.Spec.GitURL)
	if err != nil {
		return err
	}
	if activity.Spec.Author != "" {
		// info already set, save some GH requests
		return nil
	}

	// get a git API client
	provider, err := o.getGithubProvider(gitInfo)
	if err != nil {
		if o.FailIfNoGitProvider {
			log.Logger().Fatalf("could not create git provider: %s", err.Error())
		}
		return err
	}

	// extract (org, repo, commit) or (org, repo, #PR) from key
	if strings.HasPrefix(strings.ToUpper(activity.Spec.GitBranch), "PR-") {
		// this is a PR build
		n := strings.Replace(strings.ToUpper(activity.Spec.GitBranch), "PR-", "", -1)
		prNumber, err := strconv.Atoi(n)
		if err != nil {
			return err
		}
		pr, e := provider.GetPullRequest(gitInfo.Organisation, gitInfo, prNumber)
		if e != nil {
			return err
		}
		if pr.Author != nil {
			activity.Spec.Author = pr.Author.Login
		}
		activity.Spec.PullTitle = pr.Title
		log.Logger().Infof("[BuildInfo] PipelineActivity set with author=%s and PR title=%s", activity.Spec.Author, activity.Spec.PullTitle)
	} else {
		// this is a branch build
		gitCommits, e := provider.ListCommits(gitInfo.Organisation, gitInfo.Name, &gits.ListCommitsArguments{
			SHA:     activity.Spec.GitBranch,
			Page:    1,
			PerPage: 1,
		})
		if e != nil {
			return e
		}
		if len(gitCommits) > 0 {
			if gitCommits[0] != nil && gitCommits[0].Author != nil {
				activity.Spec.Author = gitCommits[0].Author.Login
				activity.Spec.LastCommitMessage = gitCommits[0].Message
			}
		}
		log.Logger().Infof("[BuildInfo] PipelineActivity set with author=%s and last message", activity.Spec.Author)
	}
	return nil
}

func (o *ControllerBuildOptions) getGithubProvider(gitInfo *gits.GitRepository) (gits.GitProvider, error) {
	// this internal provider is only used during tests
	if o.gitHubProvider != nil {
		return o.gitHubProvider, nil
	}

	// production code always goes this way
	server, userAuth, err := o.GetPipelineGitAuthForRepo(gitInfo)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, fmt.Errorf("no pipeline git auth could be found")
	}
	if userAuth == nil || userAuth.IsInvalid() {
		return nil, fmt.Errorf("no pipeline git user auth could be found")
	}
	gitProvider, err := gits.CreateProvider(server, userAuth, nil)
	if err != nil {
		return nil, err
	}
	return gitProvider, nil
}

// createPromoteStepActivityKeyFromRun deduces the pipeline metadata from the pipeline run info
func (o *ControllerBuildOptions) createPromoteStepActivityKeyFromRun(pri *tekton.PipelineRunInfo) *kube.PromoteStepActivityKey {
	if pri.GitURL == "" || pri.GitInfo == nil {
		return nil
	}
	return &kube.PromoteStepActivityKey{
		PipelineActivityKey: kube.PipelineActivityKey{
			Name:              pri.Name,
			Pipeline:          pri.Pipeline,
			Build:             pri.Build,
			LastCommitSHA:     pri.LastCommitSHA,
			LastCommitMessage: pri.LastCommitMessage,
			LastCommitURL:     pri.LastCommitURL,
			GitInfo:           pri.GitInfo,
			Context:           pri.Context,
		},
	}
}

func (o *ControllerBuildOptions) updatePipelineActivity(kubeClient kubernetes.Interface, ns string, activity *v1.PipelineActivity, buildName string, pod *corev1.Pod) bool {
	originYaml := toYamlString(activity)
	_, containerStatuses, _ := kube.GetContainersWithStatusAndIsInit(pod)
	containersTerminated := len(containerStatuses) > 0
	for _, c := range containerStatuses {
		name := strings.Replace(strings.TrimPrefix(c.Name, "build-step-"), "-", " ", -1)
		name = strings.Replace(strings.TrimPrefix(c.Name, "step-"), "-", " ", -1)
		title := strings.Title(name)
		_, stage, _ := kube.GetOrCreateStage(activity, title)

		running := c.State.Running
		terminated := c.State.Terminated

		var startedAt metav1.Time
		var finishedAt metav1.Time
		if terminated != nil {
			startedAt = terminated.StartedAt
			finishedAt = terminated.FinishedAt

			if !finishedAt.IsZero() {
				stage.CompletedTimestamp = &finishedAt
			}
		}
		if startedAt.IsZero() && running != nil {
			startedAt = running.StartedAt
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
			containersTerminated = false
		}
	}
	spec := &activity.Spec
	var biggestFinishedAt metav1.Time

	allCompleted := true
	failed := false
	running := true
	for i := range spec.Steps {
		step := &spec.Steps[i]
		stage := step.Stage
		if stage != nil {
			stageFinished := spec.Status.IsTerminated()
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
				switch stage.Status {
				case v1.ActivityStatusTypeSucceeded, v1.ActivityStatusTypeNotExecuted:
					// stage did not fail
				default:
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

	if !allCompleted && containersTerminated {
		allCompleted = true
	}
	if allCompleted && (spec.Status == v1.ActivityStatusTypeFailed || spec.Status == v1.ActivityStatusTypeSucceeded) {
		if failed {
			spec.Status = v1.ActivityStatusTypeFailed
		} else {
			spec.Status = v1.ActivityStatusTypeSucceeded
		}
		if !biggestFinishedAt.IsZero() {
			spec.CompletedTimestamp = &biggestFinishedAt
		}

		// log that the build completed
		logJobCompletedState(activity, nil)

		// lets ensure we overwrite any canonical jenkins build URL thats generated automatically
		if spec.BuildLogsURL == "" || !strings.Contains(spec.BuildLogsURL, pod.Name) {
			log.Logger().Debugf("Storing build logs for %s", activity.Name)
			podInterface := kubeClient.CoreV1().Pods(ns)

			envName := kube.LabelValueDevEnvironment
			devEnv := o.EnvironmentCache.Item(envName)
			location := v1.StorageLocation{}
			settings := &devEnv.Spec.TeamSettings
			if devEnv == nil {
				log.Logger().Warnf("No Environment %s found", envName)
			} else {
				location = settings.StorageLocationOrDefault(kube.ClassificationLogs)
			}
			if location.IsEmpty() {
				location.GitURL = activity.Spec.GitURL
				if location.GitURL == "" {
					log.Logger().Warnf("No GitURL on PipelineActivity %s", activity.Name)
				}
			}
			masker, err := kube.NewLogMasker(kubeClient, ns)
			if err != nil {
				log.Logger().Warnf("Failed to create LogMasker in namespace %s: %s", ns, err.Error())
			}
			logURL, err := o.generateBuildLogURL(podInterface, ns, activity, buildName, pod, location, settings, o.InitGitCredentials, masker)
			if err != nil {
				log.Logger().Warnf("%s", err)
			}
			if logURL != "" {
				spec.BuildLogsURL = logURL
			}
		}
	} else {
		if running {
			spec.Status = v1.ActivityStatusTypeRunning
		} else {
			spec.Status = v1.ActivityStatusTypePending
		}
	}

	if spec.Author == "" && !o.DryRun {
		err := o.completeBuildSourceInfo(activity)
		if err != nil {
			log.Logger().Warnf("Error completing build information: %s", err)
		}
	}

	// lets compare YAML in case we modify arrays in place on a copy (such as the steps) and don't detect we changed things
	newYaml := toYamlString(activity)
	return originYaml != newYaml
}

func (o *ControllerBuildOptions) updatePipelineActivityForRun(kubeClient kubernetes.Interface, ns string, activity *v1.PipelineActivity, pri *tekton.PipelineRunInfo, pod *corev1.Pod) bool {
	originYaml := toYamlString(activity)
	for _, stage := range pri.Stages {
		updateForStage(stage, activity)
	}

	spec := &activity.Spec
	var biggestFinishedAt metav1.Time

	allStagesCompleted := true
	failed := false
	running := false
	for i := range spec.Steps {
		step := &spec.Steps[i]
		stage := step.Stage
		if stage != nil {
			stageFinished := spec.Status.IsTerminated()
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
				switch stage.Status {
				case v1.ActivityStatusTypeSucceeded, v1.ActivityStatusTypeNotExecuted:
					// stage did not fail
				default:
					failed = true
				}
			} else {
				allStagesCompleted = false
			}
			if stage.Status == v1.ActivityStatusTypeRunning {
				running = true
			}
			if stage.Status == v1.ActivityStatusTypeRunning || stage.Status == v1.ActivityStatusTypePending {
				allStagesCompleted = false
			}
		}
	}

	// allStagesCompleted means all stages ran and completed. !running && failed means there are no currently running
	// stages (i.e., they're all either successful, failed, or pending), and at least one of them failed. This is the
	// best way we've got to determine that an earlier stage has failed and later stages never actually launched.
	if allStagesCompleted || (!running && failed) {
		if failed {
			spec.Status = v1.ActivityStatusTypeFailed
			// Mark any Pending stages as not executed
			for i := range spec.Steps {
				step := &spec.Steps[i]
				stage := step.Stage
				if stage != nil {
					if stage.Status == v1.ActivityStatusTypePending {
						stage.Status = v1.ActivityStatusTypeNotExecuted
					}
				}
			}
		} else if len(spec.Steps) == 1 && spec.Steps[0].Stage.Name == strings.NewReplacer("-", " ").Replace(metapipeline.MetaPipelineStageName) {
			// If there's one stage, it's passed, and it's the metapipeline, assume we're still running.
			spec.Status = v1.ActivityStatusTypeRunning
		} else {
			spec.Status = v1.ActivityStatusTypeSucceeded
		}

		// Only complete the job if it's failed, or if it's finished and the PRI we're looking at is _not_ the metapipeline
		if spec.Status == v1.ActivityStatusTypeFailed || (spec.Status.IsTerminated() && pri.Type != tekton.MetaPipeline.String()) {
			if !biggestFinishedAt.IsZero() {
				spec.CompletedTimestamp = &biggestFinishedAt
			}

			// log that the build completed
			logJobCompletedState(activity, pri)

			// TODO: This will need to be reworked for per-step logs, so leaving alone as part of metapipeline work
			// lets ensure we overwrite any canonical jenkins build URL thats generated automatically
			if spec.BuildLogsURL == "" && !o.DryRun {
				podInterface := kubeClient.CoreV1().Pods(ns)

				envName := kube.LabelValueDevEnvironment
				devEnv := o.EnvironmentCache.Item(envName)
				location := v1.StorageLocation{}
				settings := &devEnv.Spec.TeamSettings
				if devEnv == nil {
					log.Logger().Warnf("No Environment %s found", envName)
				} else {
					location = settings.StorageLocationOrDefault(kube.ClassificationLogs)
				}
				if location.IsEmpty() {
					location.GitURL = activity.Spec.GitURL
					if location.GitURL == "" {
						log.Logger().Warnf("No GitURL on PipelineActivity %s", activity.Name)
					}
				}

				masker, err := kube.NewLogMasker(kubeClient, ns)
				if err != nil {
					log.Logger().Warnf("Failed to create LogMasker in namespace %s: %s", ns, err.Error())
				}

				logURL, err := o.generateBuildLogURL(podInterface, ns, activity, pri.PipelineRun, pod, location, settings, o.InitGitCredentials, masker)
				if err != nil {
					log.Logger().Warnf("%s", err)
				}
				if logURL != "" {
					spec.BuildLogsURL = logURL
				}
			}
		}
	} else {
		if running {
			spec.Status = v1.ActivityStatusTypeRunning
		} else {
			spec.Status = v1.ActivityStatusTypePending
		}
	}

	if spec.Author == "" && !o.DryRun {
		err := o.completeBuildSourceInfo(activity)
		if err != nil {
			log.Logger().Warnf("Error completing build information: %s", err)
		}
	}

	// Set the base SHA if present in the run info
	if pri.BaseSHA != "" && spec.BaseSHA != pri.BaseSHA {
		spec.BaseSHA = pri.BaseSHA
	}

	// TODO this is a tactical approach until we move all the reporting of tekton pipelines into tekton outputs
	o.reportStatus(kubeClient, ns, activity, pri, pod)

	// lets compare YAML in case we modify arrays in place on a copy (such as the steps) and don't detect we changed things
	newYaml := toYamlString(activity)
	return originYaml != newYaml
}

func updateForStage(si *tekton.StageInfo, a *v1.PipelineActivity) {
	_, stage, _ := kube.GetOrCreateStage(a, si.GetStageNameIncludingParents())
	containersTerminated := false

	if si.Pod != nil {
		var stageSteps []v1.CoreActivityStep
		pod := si.Pod
		_, containerStatuses, _ := kube.GetContainersWithStatusAndIsInit(pod)
		containersTerminated = len(containerStatuses) > 0
		for i, container := range containerStatuses {
			title := getStepTitle(container.Name)
			step, _ := kube.GetStepValueFromStage(stage, title)
			running := container.State.Running
			terminated := container.State.Terminated

			var startedAt metav1.Time
			var finishedAt metav1.Time
			if startedAt.IsZero() {
				startedAt = determineStepStartTime(i, running, terminated, stageSteps)
			}
			if terminated != nil {
				finishedAt = terminated.FinishedAt
				if !finishedAt.IsZero() {
					step.CompletedTimestamp = &finishedAt
				}
			}

			if !startedAt.IsZero() {
				step.StartedTimestamp = &startedAt
			}
			step.Description = createStepDescription(container.Name, pod)

			if terminated != nil {
				if terminated.ExitCode == 0 {
					if didPreviousStepFail(i, stageSteps) {
						step.Status = v1.ActivityStatusTypeNotExecuted
					} else {
						step.Status = v1.ActivityStatusTypeSucceeded
					}
				} else {
					step.Status = v1.ActivityStatusTypeFailed
				}
			} else {
				if running != nil && isStepRunning(i, stageSteps) {
					step.Status = v1.ActivityStatusTypeRunning
				} else {
					step.Status = v1.ActivityStatusTypePending
				}
				containersTerminated = false
			}
			stageSteps = append(stageSteps, step)
		}
		stage.Steps = stageSteps
	}

	for _, nested := range si.Parallel {
		updateForStage(nested, a)
	}

	for _, nested := range si.Stages {
		updateForStage(nested, a)
	}

	var biggestFinishedAt metav1.Time

	childStageNames := si.GetFullChildStageNames(false)

	if len(childStageNames) > 0 {
		var childStages []*v1.StageActivityStep
		for _, s := range a.Spec.Steps {
			if s.Stage != nil {
				for _, c := range childStageNames {
					if s.Stage.Name == c {
						childStages = append(childStages, s.Stage)
					}
				}
			}
		}

		childrenCompleted := true
		childrenFailed := false
		childrenRunning := true

		for _, child := range childStages {
			childFinished := child.Status.IsTerminated()
			if child.StartedTimestamp != nil && stage.StartedTimestamp == nil {
				stage.StartedTimestamp = child.StartedTimestamp
			}
			if child.CompletedTimestamp != nil {
				t := child.CompletedTimestamp
				if !t.IsZero() {
					childFinished = true
					if biggestFinishedAt.IsZero() || t.After(biggestFinishedAt.Time) {
						biggestFinishedAt = *t
					}
				}
			}
			if childFinished {
				if child.Status != v1.ActivityStatusTypeSucceeded {
					childrenFailed = true
				}
			} else {
				childrenCompleted = false
			}
			if child.Status == v1.ActivityStatusTypeRunning {
				childrenRunning = true
			}
			if child.Status == v1.ActivityStatusTypeRunning || child.Status == v1.ActivityStatusTypePending {
				childrenCompleted = false
			}
		}

		if childrenCompleted {
			if childrenFailed {
				stage.Status = v1.ActivityStatusTypeFailed
			} else {
				stage.Status = v1.ActivityStatusTypeSucceeded
			}
			if !biggestFinishedAt.IsZero() {
				stage.CompletedTimestamp = &biggestFinishedAt
			}
		} else {
			if childrenRunning {
				stage.Status = v1.ActivityStatusTypeRunning
			} else {
				stage.Status = v1.ActivityStatusTypePending
			}
		}
	} else {
		allCompleted := false
		failed := false
		running := si.Pod != nil
		for i := range stage.Steps {
			step := &stage.Steps[i]
			if stage != nil {
				stepFinished := step.Status.IsTerminated()
				if step.StartedTimestamp != nil && stage.StartedTimestamp == nil {
					stage.StartedTimestamp = step.StartedTimestamp
				}
				if step.CompletedTimestamp != nil {
					t := step.CompletedTimestamp
					if !t.IsZero() {
						stepFinished = true
						if biggestFinishedAt.IsZero() || t.After(biggestFinishedAt.Time) {
							biggestFinishedAt = *t
						}
					}
				}
				if stepFinished {
					if step.Status != v1.ActivityStatusTypeSucceeded {
						failed = true
					}
				} else {
					allCompleted = false
				}
				if step.Status == v1.ActivityStatusTypeRunning {
					running = true
				}
				if step.Status == v1.ActivityStatusTypeRunning || step.Status == v1.ActivityStatusTypePending {
					allCompleted = false
				}
			}
		}

		if !allCompleted && containersTerminated {
			allCompleted = true
		}
		if allCompleted {
			if failed {
				stage.Status = v1.ActivityStatusTypeFailed
			} else {
				stage.Status = v1.ActivityStatusTypeSucceeded
			}
			if !biggestFinishedAt.IsZero() {
				stage.CompletedTimestamp = &biggestFinishedAt
			}
		} else {
			if running {
				stage.Status = v1.ActivityStatusTypeRunning
			} else {
				stage.Status = v1.ActivityStatusTypePending
			}
		}
	}
}

// didPreviousStepFail checks if the step before the given index failed. This is used to mark not-actually-executed steps
// correctly.
func didPreviousStepFail(index int, stageSteps []v1.CoreActivityStep) bool {
	if len(stageSteps) > 0 {
		previousStep := stageSteps[index-1]
		return previousStep.CompletedTimestamp != nil && previousStep.Status != v1.ActivityStatusTypeSucceeded
	}
	return false
}

// isStepRunning checks if the step at the given index is actually running its command, not just waiting for the previous
// step to finish. The step is running if either there are no steps yet in the stageSteps slice, meaning this is the first
// step we're adding, or if the step before it in stageSteps has completed.
func isStepRunning(index int, stageSteps []v1.CoreActivityStep) bool {
	if len(stageSteps) > 0 {
		previousStep := stageSteps[index-1]
		return previousStep.CompletedTimestamp != nil
	}
	return true
}

// determineStepStartTime checks to see if there's a step before this one. If so, it returns the time at which that step
// finished. Otherwise, it checks to see if the current step is running or finished and returns the appropriate start time.
// This is to work around the fact that Tekton steps all have the same start time, since all containers in the pod start
// at once, even though the actual command doesn't get run until the previous step finishes.
func determineStepStartTime(index int, running *corev1.ContainerStateRunning, terminated *corev1.ContainerStateTerminated, stageSteps []v1.CoreActivityStep) metav1.Time {
	var startedAt metav1.Time
	if len(stageSteps) > 0 {
		previousStep := stageSteps[index-1]
		if previousStep.CompletedTimestamp != nil {
			startedAt = *previousStep.CompletedTimestamp
		}
	} else {
		if running != nil {
			startedAt = running.StartedAt
		} else if terminated != nil {
			startedAt = terminated.StartedAt
		}
	}
	return startedAt
}

// getStepTitle translates the step container's name into the title for the step used in PipelineActivity
func getStepTitle(containerName string) string {
	name := strings.Replace(strings.TrimPrefix(containerName, "build-step-"), "-", " ", -1)
	name = strings.Replace(strings.TrimPrefix(containerName, "step-"), "-", " ", -1)
	return strings.Title(name)
}

// toYamlString returns the YAML string or error when marshalling the given resource
func toYamlString(resource interface{}) string {
	data, err := yaml.Marshal(resource)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// generates the build log URL and returns the URL
func (o *ControllerBuildOptions) generateBuildLogURL(podInterface typedcorev1.PodInterface, ns string, activity *v1.PipelineActivity, buildName string, pod *corev1.Pod, location v1.StorageLocation, settings *v1.TeamSettings, initGitCredentials bool, logMasker *kube.LogMasker) (string, error) {

	log.Logger().Debugf("Collecting logs for %s to location %s", activity.Name, location.Description())
	coll, err := collector.NewCollector(location, o.Git())
	if err != nil {
		return "", errors.Wrapf(err, "could not create Collector for pod %s in namespace %s with settings %#v", pod.Name, ns, settings)
	}

	owner := activity.RepositoryOwner()
	repository := activity.RepositoryName()
	branch := activity.BranchName()
	buildNumber := activity.Spec.Build
	if buildNumber == "" {
		buildNumber = "1"
	}

	pathDir := filepath.Join("jenkins-x", "logs", owner, repository, branch)
	fileName := filepath.Join(pathDir, buildNumber+".log")

	var clientErrs []error
	kubeClient, err := o.KubeClient()
	clientErrs = append(clientErrs, err)
	tektonClient, _, err := o.TektonClient()
	clientErrs = append(clientErrs, err)
	jx, _, err := o.JXClient()
	clientErrs = append(clientErrs, err)

	err = util.CombineErrors(clientErrs...)
	if err != nil {
		return "", errors.Wrap(err, "there was a problem obtaining one of the clients")
	}

	var logWriter logs.LogWriter
	w := LongTermStorageLogWriter{
		data:       []byte{},
		kubeClient: kubeClient,
		logMasker:  logMasker,
	}
	logWriter = &w

	tektonLogger := logs.TektonLogger{
		JXClient:     jx,
		KubeClient:   kubeClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter:    logWriter,
	}

	log.Logger().Debugf("Capturing running build logs for %s", activity.Name)
	err = tektonLogger.GetRunningBuildLogs(activity, buildName, false)
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem getting logs for build %s", buildName)
	}

	if initGitCredentials {
		gc := &credentials.StepGitCredentialsOptions{}
		copy := *o.CommonOptions
		gc.CommonOptions = &copy
		gc.BatchMode = true
		log.Logger().Info("running: jx step git credentials")
		err = gc.Run()
		if err != nil {
			return "", errors.Wrapf(err, "Failed to setup git credentials")
		}
	}

	log.Logger().Infof("storing logs for activity %s into storage at %s", activity.Name, fileName)
	answer, err := coll.CollectData(w.data, fileName)
	if err != nil {
		log.Logger().Errorf("failed to store logs for activity %s into storage at %s: %s", activity.Name, fileName, err.Error())
		return answer, err
	}
	log.Logger().Infof("stored logs for activity %s into storage at %s", activity.Name, fileName)
	return answer, nil
}

// ensurePipelineActivityHasLabels older versions of controller build did not add labels properly
// so lets enrich PipelineActivity on startup
func (o *ControllerBuildOptions) ensurePipelineActivityHasLabels(jxClient versioned.Interface, ns string) error {
	activities := jxClient.JenkinsV1().PipelineActivities(ns)
	actList, err := activities.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, act := range actList.Items {
		updated := false
		if act.Labels == nil {
			act.Labels = map[string]string{}
		}
		provider := kube.ToProviderName(act.Spec.GitURL)
		owner := act.RepositoryOwner()
		repository := act.RepositoryName()
		branch := act.BranchName()
		build := act.Spec.Build

		if act.Labels[v1.LabelProvider] != provider && provider != "" {
			act.Labels[v1.LabelProvider] = provider
			updated = true
		}
		if act.Labels[v1.LabelOwner] != owner && owner != "" {
			act.Labels[v1.LabelOwner] = owner
			updated = true
		}
		if act.Labels[v1.LabelRepository] != repository && repository != "" {
			act.Labels[v1.LabelRepository] = repository
			updated = true
		}
		if act.Labels[v1.LabelBranch] != branch && branch != "" {
			act.Labels[v1.LabelBranch] = branch
			updated = true
		}
		if act.Labels[v1.LabelBuild] != build && build != "" {
			act.Labels[v1.LabelBuild] = build
			updated = true
		}
		if updated {
			err = o.Retry(3, time.Second*3, func() error {
				resource, err := activities.Get(act.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				resource.Labels = act.Labels
				_, err = activities.Update(resource)
				return err
			})
			if err != nil {
				return errors.Wrapf(err, "failed to modify labels on PipelineActivity %s", act.Name)
			}
			log.Logger().Infof("updated labels on PipelineActivity %s", util.ColorInfo(act.Name))
		}
	}
	return nil
}

func (o *ControllerBuildOptions) ensureSourceRepositoryHasLabels(jxClient versioned.Interface, ns string) error {
	sourceRepositoryInterface := jxClient.JenkinsV1().SourceRepositories(ns)
	srList, err := sourceRepositoryInterface.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, sr := range srList.Items {
		updated := false
		if sr.Labels == nil {
			sr.Labels = map[string]string{}
		}
		provider := kube.ToProviderName(sr.Spec.Provider)
		owner := sr.Spec.Org
		repository := sr.Spec.Repo

		if sr.Labels[v1.LabelProvider] != provider && provider != "" {
			sr.Labels[v1.LabelProvider] = provider
			updated = true
		}
		if sr.Labels[v1.LabelOwner] != owner && owner != "" {
			sr.Labels[v1.LabelOwner] = owner
			updated = true
		}
		if sr.Labels[v1.LabelRepository] != repository && repository != "" {
			sr.Labels[v1.LabelRepository] = repository
			updated = true
		}
		if updated {
			err = o.Retry(3, time.Second*3, func() error {
				resource, err := sourceRepositoryInterface.Get(sr.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				resource.Labels = sr.Labels
				_, err = sourceRepositoryInterface.Update(resource)
				return err
			})
			if err != nil {
				return errors.Wrapf(err, "failed to modify labels on SourceRepository %s", sr.Name)
			}
			log.Logger().Infof("updated labels on SourceRepository %s", util.ColorInfo(sr.Name))
		}
	}
	return nil
}

func (o *ControllerBuildOptions) reportStatus(kubeClient kubernetes.Interface, ns string, activity *v1.PipelineActivity, pri *tekton.PipelineRunInfo, pod *corev1.Pod) {
	if !o.GitReporting {
		return
	}

	sha := activity.Spec.LastCommitSHA
	if sha == "" {
		sha = pri.LastCommitSHA
		if sha == "" && activity.Labels != nil {
			sha = activity.Labels[v1.LabelLastCommitSha]
		}
		activity.Spec.LastCommitSHA = sha
	}
	owner := activity.Spec.GitOwner
	repo := activity.Spec.GitRepository
	gitURL := activity.Spec.GitURL
	activityStatus := activity.Spec.Status
	statusInfo := toScmStatusDescriptionRunningStages(activity)

	fields := map[string]interface{}{
		"name":        activity.Name,
		"status":      activityStatus,
		"gitOwner":    owner,
		"gitRepo":     repo,
		"gitSHA":      sha,
		"gitURL":      gitURL,
		"gitBranch":   activity.Spec.GitBranch,
		"gitStatus":   statusInfo.scmStatus,
		"buildNumber": activity.Spec.Build,
		"duration":    util.DurationString(activity.Spec.StartedTimestamp, activity.Spec.CompletedTimestamp),
	}
	if gitURL == "" {
		log.Logger().WithFields(fields).Debugf("Cannot report pipeline %s as we have no git SHA", activity.Name)
		return

	}
	if sha == "" {
		log.Logger().WithFields(fields).Debugf("Cannot report pipeline %s as we have no git SHA", activity.Name)
		return
	}
	if owner == "" {
		log.Logger().WithFields(fields).Debugf("Cannot report pipeline %s as we have no git Owner", activity.Name)
		return
	}
	if repo == "" {
		log.Logger().WithFields(fields).Debugf("Cannot report pipeline %s as we have no git repository name", activity.Name)
		return
	}

	if activity.Annotations == nil {
		activity.Annotations = map[string]string{}
	}
	if statusInfo.scmStatus == "" {
		return
	}

	switch activity.Annotations[kube.AnnotationGitReportState] {
	// already completed - avoid reporting again if a promotion happens after a PR has merged and the pipeline updates status
	case string(v1.ActivityStatusTypeSucceeded), string(v1.ActivityStatusTypeAborted), string(v1.ActivityStatusTypeFailed):
		return
	}

	// Check if state and running stages haven't changed and return if they haven't
	if activity.Annotations[kube.AnnotationGitReportState] == string(activityStatus) &&
		activity.Annotations[kube.AnnotationGitReportRunningStages] == statusInfo.runningStages {
		return
	}

	activity.Annotations[kube.AnnotationGitReportState] = string(activityStatus)
	activity.Annotations[kube.AnnotationGitReportRunningStages] = statusInfo.runningStages

	pipelineContext := pri.Context
	if pipelineContext == "" {
		pipelineContext = "jenkins-x"
	}

	gitRepoStatus := &gits.GitRepoStatus{
		State:       statusInfo.scmStatus,
		Context:     pipelineContext,
		Description: statusInfo.description,
	}
	targetURL := CreateReportTargetURL(o.TargetURLTemplate, ReportParams{
		Owner:      owner,
		Repository: repo,
		Branch:     activity.Spec.GitBranch,
		Build:      activity.Spec.Build,
		Context:    pipelineContext,
		BaseURL:    strings.TrimRight(o.JobURLBase, "/"),
		Namespace:  ns,
	})
	if strings.HasPrefix(targetURL, "http://") || strings.HasPrefix(targetURL, "https://") {
		gitRepoStatus.TargetURL = targetURL
	}

	gitProvider, err := o.GitProviderForURL(gitURL, "git provider")
	if err != nil {
		log.Logger().WithFields(fields).WithError(err).Warnf("failed to create git provider")
		return
	}

	_, err = gitProvider.UpdateCommitStatus(owner, repo, sha, gitRepoStatus)
	if err != nil {
		log.Logger().WithFields(fields).WithError(err).Warnf("failed to report git status")
	} else {
		log.Logger().WithFields(fields).Info("reported git status")
	}
}

// ReportParams contains the parameters for target URL templates
type ReportParams struct {
	BaseURL, Owner, Repository, Branch, Build, Context, Namespace string
}

// CreateReportTargetURL creates the target URL for pipeline results/logs from a template
func CreateReportTargetURL(templateText string, params ReportParams) string {
	templateData, err := util.ToObjectMap(params)
	if err != nil {
		log.Logger().WithError(err).Warnf("failed to convert git ReportParams to a map for %#v", params)
		return ""
	}

	funcMap := helm.NewFunctionMap()
	tmpl, err := template.New("target_url.tmpl").Option("missingkey=error").Funcs(funcMap).Parse(templateText)
	if err != nil {
		log.Logger().WithError(err).Warnf("failed to parse git ReportsParam template: %s", templateText)
		return ""
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, templateData)
	if err != nil {
		log.Logger().WithError(err).Warnf("failed to evaluate git ReportsParam template: %s due to: %s", templateText, err.Error())
		return ""
	}
	return buf.String()
}

type reportStatusInfo struct {
	scmStatus     string
	description   string
	runningStages string
}

func toScmStatusDescriptionRunningStages(activity *v1.PipelineActivity) reportStatusInfo {
	info := reportStatusInfo{
		description:   "",
		runningStages: "",
		scmStatus:     "",
	}
	switch activity.Spec.Status {
	case v1.ActivityStatusTypeSucceeded:
		info.scmStatus = "success"
		info.description = "Pipeline successful"
	case v1.ActivityStatusTypeRunning, v1.ActivityStatusTypePending:
		info.scmStatus = "pending"
		info.description = "Pipeline running"
	case v1.ActivityStatusTypeError:
		info.scmStatus = "error"
		info.description = "Error executing pipeline"
	default:
		info.scmStatus = "failure"
		info.description = "Pipeline failed"
	}
	stagesByStatus := activity.StagesByStatus()

	if len(stagesByStatus[v1.ActivityStatusTypeRunning]) > 0 {
		info.runningStages = strings.Join(stagesByStatus[v1.ActivityStatusTypeRunning], ",")
		info.description = fmt.Sprintf("Pipeline running stage(s): %s", strings.Join(stagesByStatus[v1.ActivityStatusTypeRunning], ", "))
		if len(info.description) > 63 {
			info.description = info.description[:59] + "..."
		}
	}
	return info
}

// createStepDescription uses the spec of the container to return a description
func createStepDescription(containerName string, pod *corev1.Pod) string {
	containers, _, isInit := kube.GetContainersWithStatusAndIsInit(pod)

	for _, c := range containers {
		_, args := kube.GetCommandAndArgs(&c, isInit)
		if c.Name == containerName && len(args) > 0 {
			if args[0] == "-url" && len(args) > 1 {
				return args[1]
			}
		}
	}
	return ""
}

func logJobCompletedState(activity *v1.PipelineActivity, pri *tekton.PipelineRunInfo) {
	// log that the build completed
	var gitProviderUrl string
	if activity.Spec.GitURL != "" {
		gitInfo, err := gits.ParseGitURL(activity.Spec.GitURL)
		if err != nil {
			log.Logger().Warnf("unable to parse %s as git url, %v", activity.Spec.GitURL, err)
		}
		gitProviderUrl = gitInfo.ProviderURL()
	}

	var prNumber string
	// extract (org, repo, commit) or (org, repo, #PR) from key
	if strings.HasPrefix(strings.ToUpper(activity.Spec.GitBranch), "PR-") {
		// this is a PR build
		prNumber = strings.Replace(strings.ToUpper(activity.Spec.GitBranch), "PR-", "", -1)
	}

	stages := make([]map[string]interface{}, 0)

	for _, s := range activity.Spec.Steps {
		if s.Kind == v1.ActivityStepKindTypeStage {
			steps := make([]map[string]interface{}, 0)
			for _, st := range s.Stage.Steps {
				step := map[string]interface{}{
					"name":     st.Name,
					"status":   st.Status,
					"duration": util.DurationString(st.StartedTimestamp, st.CompletedTimestamp),
				}
				steps = append(steps, step)
			}
			stage := map[string]interface{}{
				"name":     s.Stage.Name,
				"status":   s.Stage.Status,
				"duration": util.DurationString(s.Stage.StartedTimestamp, s.Stage.CompletedTimestamp),
				"steps":    steps,
			}
			stages = append(stages, stage)
		}
	}

	fields := map[string]interface{}{
		"name":              activity.Name,
		"status":            activity.Spec.Status,
		"gitOwner":          activity.Spec.GitOwner,
		"gitRepo":           activity.Spec.GitRepository,
		"gitProviderUrl":    gitProviderUrl,
		"gitBranch":         activity.Spec.GitBranch,
		"buildNumber":       activity.Spec.Build,
		"pullRequestNumber": prNumber,
		"duration":          util.DurationString(activity.Spec.StartedTimestamp, activity.Spec.CompletedTimestamp),
		"stages":            stages,
	}
	if pri != nil {
		fields["pipelineRunInfo"] = pri.Name
		fields["pipelineRunType"] = pri.Type
	}
	log.Logger().WithFields(fields).Infof("Build %s %s", activity.Name, activity.Spec.Status)
}
