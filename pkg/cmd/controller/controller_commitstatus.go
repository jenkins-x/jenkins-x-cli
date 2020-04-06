package controller

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/prow/config"

	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/prow"

	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx/pkg/extensions"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/builds"

	corev1 "k8s.io/api/core/v1"

	jenkinsv1client "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"

	"k8s.io/client-go/tools/cache"

	"github.com/jenkins-x/jx/pkg/log"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/spf13/cobra"
)

// ControllerCommitStatusOptions the options for the controller
type ControllerCommitStatusOptions struct {
	ControllerOptions
}

// NewCmdControllerCommitStatus creates a command object for the "create" command
func NewCmdControllerCommitStatus(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ControllerCommitStatusOptions{
		ControllerOptions: ControllerOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "commitstatus",
		Short: "Updates commit status",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

// Run implements this command
func (o *ControllerCommitStatusOptions) Run() error {
	// Always run in batch mode as a controller is never run interactively
	o.BatchMode = true

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	kubeClient, _, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	err = kube.RegisterCommitStatusCRD(apisClient)
	if err != nil {
		return err
	}
	err = kube.RegisterPipelineActivityCRD(apisClient)
	if err != nil {
		return err
	}

	commitstatusListWatch := cache.NewListWatchFromClient(jxClient.JenkinsV1().RESTClient(), "commitstatuses", ns, fields.Everything())
	kube.SortListWatchByName(commitstatusListWatch)
	_, commitstatusController := cache.NewInformer(
		commitstatusListWatch,
		&jenkinsv1.CommitStatus{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onCommitStatusObj(obj, jxClient, ns)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onCommitStatusObj(newObj, jxClient, ns)
			},
			DeleteFunc: func(obj interface{}) {

			},
		},
	)
	stop := make(chan struct{})
	go commitstatusController.Run(stop)

	podListWatch := cache.NewListWatchFromClient(kubeClient.CoreV1().RESTClient(), "pods", ns, fields.Everything())
	kube.SortListWatchByName(podListWatch)
	_, podWatch := cache.NewInformer(
		podListWatch,
		&corev1.Pod{},
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onPodObj(obj, jxClient, kubeClient, ns)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onPodObj(newObj, jxClient, kubeClient, ns)
			},
			DeleteFunc: func(obj interface{}) {

			},
		},
	)
	stop = make(chan struct{})
	podWatch.Run(stop)

	if err != nil {
		return err
	}
	return nil
}

func (o *ControllerCommitStatusOptions) onCommitStatusObj(obj interface{}, jxClient jenkinsv1client.Interface, ns string) {
	check, ok := obj.(*jenkinsv1.CommitStatus)
	if !ok {
		log.Logger().Fatalf("commit status controller: unexpected type %v", obj)
	} else {
		err := o.onCommitStatus(check, jxClient, ns)
		if err != nil {
			log.Logger().Fatalf("commit status controller: %v", err)
		}
	}
}

func (o *ControllerCommitStatusOptions) onCommitStatus(check *jenkinsv1.CommitStatus, jxClient jenkinsv1client.Interface, ns string) error {
	groupedBySha := make(map[string][]jenkinsv1.CommitStatusDetails, 0)
	for _, v := range check.Spec.Items {
		if _, ok := groupedBySha[v.Commit.SHA]; !ok {
			groupedBySha[v.Commit.SHA] = make([]jenkinsv1.CommitStatusDetails, 0)
		}
		groupedBySha[v.Commit.SHA] = append(groupedBySha[v.Commit.SHA], v)
	}
	for _, vs := range groupedBySha {
		var last jenkinsv1.CommitStatusDetails
		for _, v := range vs {
			lastBuildNumber, err := strconv.Atoi(getBuildNumber(last.PipelineActivity.Name))
			if err != nil {
				return err
			}
			buildNumber, err := strconv.Atoi(getBuildNumber(v.PipelineActivity.Name))
			if err != nil {
				return err
			}
			if lastBuildNumber < buildNumber {
				last = v
			}
		}
		err := o.update(&last, jxClient, ns)
		if err != nil {
			gitProvider, gitRepoInfo, err1 := o.getGitProvider(last.Commit.GitURL)
			if err1 != nil {
				return err1
			}
			_, err1 = extensions.NotifyCommitStatus(last.Commit, "error", "", "Internal Error performing commit status updates", "", last.Context, gitProvider, gitRepoInfo)
			if err1 != nil {
				return err
			}
			return err
		}
	}
	return nil
}

func (o *ControllerCommitStatusOptions) onPodObj(obj interface{}, jxClient jenkinsv1client.Interface, kubeClient kubernetes.Interface, ns string) {
	check, ok := obj.(*corev1.Pod)
	if !ok {
		log.Logger().Fatalf("pod watcher: unexpected type %v", obj)
	} else {
		err := o.onPod(check, jxClient, kubeClient, ns)
		if err != nil {
			log.Logger().Fatalf("pod watcher: %v", err)
		}
	}
}

func (o *ControllerCommitStatusOptions) onPod(pod *corev1.Pod, jxClient jenkinsv1client.Interface, kubeClient kubernetes.Interface, ns string) error {
	if pod != nil {
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
				org := ""
				repo := ""
				pullRequest := ""
				pullPullSha := ""
				pullBaseSha := ""
				buildNumber := ""
				jxBuildNumber := ""
				buildId := ""
				sourceUrl := ""
				branch := ""

				containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)
				for _, container := range containers {
					for _, e := range container.Env {
						switch e.Name {
						case "REPO_OWNER":
							org = e.Value
						case "REPO_NAME":
							repo = e.Value
						case "PULL_NUMBER":
							pullRequest = fmt.Sprintf("PR-%s", e.Value)
						case "PULL_PULL_SHA":
							pullPullSha = e.Value
						case "PULL_BASE_SHA":
							pullBaseSha = e.Value
						case "JX_BUILD_NUMBER":
							jxBuildNumber = e.Value
						case "BUILD_NUMBER":
							buildNumber = e.Value
						case "BUILD_ID":
							buildId = e.Value
						case "SOURCE_URL":
							sourceUrl = e.Value
						case "PULL_BASE_REF":
							branch = e.Value
						}
					}
				}

				sha := pullBaseSha
				if pullRequest == "PR-" {
					pullRequest = ""
				} else {
					sha = pullPullSha
					branch = pullRequest
				}

				// if BUILD_ID is set, use it, otherwise if JX_BUILD_NUMBER is set, use it, otherwise use BUILD_NUMBER
				if jxBuildNumber != "" {
					buildNumber = jxBuildNumber
				}
				if buildId != "" {
					buildNumber = buildId
				}

				pipelineActName := naming.ToValidName(fmt.Sprintf("%s-%s-%s-%s", org, repo, branch, buildNumber))

				// PLM TODO This is a bit of hack, we need a working build controller
				// Try to add the lastCommitSha and gitUrl to the PipelineActivity
				act, err := jxClient.JenkinsV1().PipelineActivities(ns).Get(pipelineActName, metav1.GetOptions{})
				if err != nil {
					// An error just means the activity doesn't exist yet
					log.Logger().Debugf("pod watcher: Unable to find PipelineActivity for %s", pipelineActName)
				} else {
					act.Spec.LastCommitSHA = sha
					act.Spec.GitURL = sourceUrl
					act.Spec.GitOwner = org
					log.Logger().Debugf("pod watcher: Adding lastCommitSha: %s and gitUrl: %s to %s", act.Spec.LastCommitSHA, act.Spec.GitURL, pipelineActName)
					_, err := jxClient.JenkinsV1().PipelineActivities(ns).PatchUpdate(act)
					if err != nil {
						// We can safely return this error as it will just get logged
						return err
					}
				}
				if org != "" && repo != "" && buildNumber != "" && (pullBaseSha != "" || pullPullSha != "") {
					log.Logger().Debugf("pod watcher: build pod: %s, org: %s, repo: %s, buildNumber: %s, pullBaseSha: %s, pullPullSha: %s, pullRequest: %s, sourceUrl: %s", pod.Name, org, repo, buildNumber, pullBaseSha, pullPullSha, pullRequest, sourceUrl)
					if sha == "" {
						log.Logger().Warnf("pod watcher: No sha on %s, not upserting commit status", pod.Name)
					} else {
						prow := prow.Options{
							KubeClient: kubeClient,
							NS:         ns,
						}
						prowConfig, _, err := prow.GetProwConfig()
						if err != nil {
							return errors.Wrap(err, "getting prow config")
						}
						contexts, err := config.GetBranchProtectionContexts(org, repo, prowConfig)
						if err != nil {
							return err
						}
						log.Logger().Debugf("pod watcher: Using contexts %v", contexts)

						for _, ctx := range contexts {
							if pullRequest != "" {
								name := naming.ToValidName(fmt.Sprintf("%s-%s-%s-%s", org, repo, branch, ctx))

								err = o.UpsertCommitStatusCheck(name, pipelineActName, sourceUrl, sha, pullRequest, ctx, pod.Status.Phase, jxClient, ns)
								if err != nil {
									return err
								}
							}
						}
					}
				}
			}

		}

	}
	return nil
}

func (o *ControllerCommitStatusOptions) UpsertCommitStatusCheck(name string, pipelineActName string, url string, sha string, pullRequest string, context string, phase corev1.PodPhase, jxClient jenkinsv1client.Interface, ns string) error {
	if name != "" {

		status, err := jxClient.JenkinsV1().CommitStatuses(ns).Get(name, metav1.GetOptions{})
		create := false
		insert := false
		actRef := jenkinsv1.ResourceReference{}
		if err != nil {
			create = true
		} else {
			log.Logger().Infof("pod watcher: commit status already exists for %s", name)
		}
		// Create the activity reference
		act, err := jxClient.JenkinsV1().PipelineActivities(ns).Get(pipelineActName, metav1.GetOptions{})
		if err == nil {
			actRef.Name = act.Name
			actRef.Kind = act.Kind
			actRef.UID = act.UID
			actRef.APIVersion = act.APIVersion
		}

		possibleStatusDetails := make([]int, 0)
		for i, v := range status.Spec.Items {
			if v.Commit.SHA == sha && v.PipelineActivity.Name == pipelineActName {
				possibleStatusDetails = append(possibleStatusDetails, i)
			}
		}
		statusDetails := jenkinsv1.CommitStatusDetails{}
		log.Logger().Debugf("pod watcher: Discovered possible status details %v", possibleStatusDetails)
		if len(possibleStatusDetails) == 1 {
			log.Logger().Debugf("CommitStatus %s for pipeline %s already exists", name, pipelineActName)
		} else if len(possibleStatusDetails) == 0 {
			insert = true
		} else {
			return fmt.Errorf("More than %d status detail for sha %s, should 1 or 0, found %v", len(possibleStatusDetails), sha, possibleStatusDetails)
		}

		if create || insert {
			// This is not the same pipeline activity the status was created for,
			// or there is no existing status, so we make a new one
			statusDetails = jenkinsv1.CommitStatusDetails{
				Checked: false,
				Commit: jenkinsv1.CommitStatusCommitReference{
					GitURL:      url,
					PullRequest: pullRequest,
					SHA:         sha,
				},
				PipelineActivity: actRef,
				Context:          context,
			}
		}
		if create {
			log.Logger().Infof("pod watcher: Creating commit status for pipeline activity %s", pipelineActName)
			status = &jenkinsv1.CommitStatus{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"lastCommitSha": sha,
					},
				},
				Spec: jenkinsv1.CommitStatusSpec{
					Items: []jenkinsv1.CommitStatusDetails{
						statusDetails,
					},
				},
			}
			_, err := jxClient.JenkinsV1().CommitStatuses(ns).Create(status)
			if err != nil {
				return err
			}

		} else if insert {
			status.Spec.Items = append(status.Spec.Items, statusDetails)
			log.Logger().Infof("pod watcher: Adding commit status for pipeline activity %s", pipelineActName)
			_, err := jxClient.JenkinsV1().CommitStatuses(ns).PatchUpdate(status)
			if err != nil {
				return err
			}
		} else {
			log.Logger().Debugf("pod watcher: Not updating or creating pipeline activity %s", pipelineActName)
		}
	} else {
		errors.New("commit status controller: Must supply name")
	}
	return nil
}

func (o *ControllerCommitStatusOptions) update(statusDetails *jenkinsv1.CommitStatusDetails, jxClient jenkinsv1client.Interface, ns string) error {
	gitProvider, gitRepoInfo, err := o.getGitProvider(statusDetails.Commit.GitURL)
	if err != nil {
		return err
	}
	pass := false
	if statusDetails.Checked {
		var commentBuilder strings.Builder
		pass = true
		for _, c := range statusDetails.Items {
			if !c.Pass {
				pass = false
				fmt.Fprintf(&commentBuilder, "%s | %s | %s | TODO | `/test this`\n", c.Name, c.Description, statusDetails.Commit.SHA)
			}
		}
		if pass {
			_, err := extensions.NotifyCommitStatus(statusDetails.Commit, "success", "", "Completed successfully", "", statusDetails.Context, gitProvider, gitRepoInfo)
			if err != nil {
				return err
			}
		} else {
			comment := fmt.Sprintf(
				"The following commit statusDetails checks **failed**, say `/retest` to rerun them all:\n"+
					"\n"+
					"Name | Description | Commit | Details | Rerun command\n"+
					"--- | --- | --- | --- | --- \n"+
					"%s\n"+
					"<details>\n"+
					"\n"+
					"Instructions for interacting with me using PR comments are available [here](https://git.k8s.io/community/contributors/guide/pull-requests.md).  If you have questions or suggestions related to my behavior, please file an issue against the [kubernetes/test-infra](https://github.com/kubernetes/test-infra/issues/new?title=Prow%%20issue:) repository. I understand the commands that are listed [here](https://go.k8s.io/bot-commands).\n"+
					"</details>", commentBuilder.String())
			_, err := extensions.NotifyCommitStatus(statusDetails.Commit, "failure", "", fmt.Sprintf("%s failed", statusDetails.Context), comment, statusDetails.Context, gitProvider, gitRepoInfo)
			if err != nil {
				return err
			}
		}
	} else {
		_, err = extensions.NotifyCommitStatus(statusDetails.Commit, "pending", "", fmt.Sprintf("Waiting for %s to complete", statusDetails.Context), "", statusDetails.Context, gitProvider, gitRepoInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *ControllerCommitStatusOptions) getGitProvider(url string) (gits.GitProvider, *gits.GitRepository, error) {
	// TODO This is an epic hack to get the git stuff working
	gitInfo, err := gits.ParseGitURL(url)
	if err != nil {
		return nil, nil, err
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return nil, nil, err
	}
	gitKind, err := o.GitServerKind(gitInfo)
	if err != nil {
		return nil, nil, err
	}
	for _, server := range authConfigSvc.Config().Servers {
		if server.Kind == gitKind && len(server.Users) >= 1 {
			// Just grab the first user for now
			username := server.Users[0].Username
			apiToken := server.Users[0].ApiToken
			err = os.Setenv("GIT_USERNAME", username)
			if err != nil {
				return nil, nil, err
			}
			err = os.Setenv("GIT_API_TOKEN", apiToken)
			if err != nil {
				return nil, nil, err
			}
			break
		}
	}
	return o.CreateGitProviderForURLWithoutKind(url)
}

func getBuildNumber(pipelineActName string) string {
	if pipelineActName == "" {
		return "-1"
	}
	pipelineParts := strings.Split(pipelineActName, "-")
	if len(pipelineParts) > 3 {
		return pipelineParts[len(pipelineParts)-1]
	} else {
		return ""
	}

}
