package kube

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	typev1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PipelineActivityKey struct {
	Name              string
	Pipeline          string
	Build             string
	Version           string
	BuildURL          string
	BuildLogsURL      string
	ReleaseNotesURL   string
	LastCommitSHA     string
	LastCommitMessage string
	LastCommitURL     string
	GitInfo           *gits.GitRepository
	PullRefs          map[string]string
	Context           string
}

func (k *PipelineActivityKey) IsValid() bool {
	return len(k.Name) > 0
}

func (k *PipelineActivityKey) isBatchBuild() bool {
	return len(k.PullRefs) > 1
}

func (k *PipelineActivityKey) isPRBuild() bool {
	return len(k.PullRefs) == 1
}

type PromoteStepActivityKey struct {
	PipelineActivityKey

	Environment    string
	ApplicationURL string
}

type PromotePullRequestFn func(*v1.PipelineActivity, *v1.PipelineActivityStep, *v1.PromoteActivityStep, *v1.PromotePullRequestStep) error
type PromoteUpdateFn func(*v1.PipelineActivity, *v1.PipelineActivityStep, *v1.PromoteActivityStep, *v1.PromoteUpdateStep) error

type PipelineDetails struct {
	GitOwner      string
	GitRepository string
	BranchName    string
	Pipeline      string
	Build         string
	Context       string
}

// PipelineID is an identifier for a Pipeline.
// A pipeline is typically identified by its owner, repository, and branch with the ID field taking the form
// `<owner>/>repository>/<branch>`
type PipelineID struct {
	ID   string
	Name string
}

// NewPipelineIDFromString creates a new PipelineID, given a pre-built string identifier.
// The string identifier is expected to follow the format `<owner>/>repository>/<branch>`, though this isn't actually
// validated/mandated here.
func NewPipelineIDFromString(id string) PipelineID {
	sanitisedName := strings.Replace(strings.ToLower(id), "/", "-", -1)
	sanitisedName = strings.Replace(sanitisedName, "_", "-", -1)
	pID := PipelineID{
		ID: id,
		//TODO: disabling the encoding of the name, as it doesn't work for some upper case values. Upshot is conflicts on org/repo/branch that differ only in case.
		//See https://github.com/jenkins-x/jx/issues/2551
		//Name: util.EncodeKubernetesName(strings.Replace(id, "/", "-", -1)),
		Name: sanitisedName,
	}
	return pID
}

// NewPipelineID creates a new PipelineID for a given owner, repository, and branch.
func NewPipelineID(owner string, repository string, branch string) PipelineID {
	return NewPipelineIDFromString(fmt.Sprintf("%s/%s/%s", owner, repository, branch))
}

// GetActivityName Builds a Kubernetes-friendly (i.e. a-z,-,. only) name for a specific activity, within a pipeline.
func (p *PipelineID) GetActivityName(activity string) string {
	return fmt.Sprintf("%s-%s", p.Name, activity)
}

// CreatePipelineDetails creates a PipelineDetails object populated from the activity
func CreatePipelineDetails(activity *v1.PipelineActivity) *PipelineDetails {
	spec := &activity.Spec
	repoOwner := spec.GitOwner
	repoName := spec.GitRepository
	buildNumber := spec.Build
	branchName := ""
	context := spec.Context
	pipeline := spec.Pipeline
	if pipeline != "" {
		paths := strings.Split(pipeline, "/")
		if len(paths) > 2 {
			if repoOwner == "" {
				repoOwner = paths[0]
			}
			if repoName == "" {
				repoName = paths[1]
			}
			branchName = paths[2]
		}
	}
	if branchName == "" {
		branchName = "master"
	}
	if pipeline == "" && (repoName != "" && repoOwner != "") {
		pipeline = repoOwner + "/" + repoName + "/" + branchName
	}
	return &PipelineDetails{
		GitOwner:      repoOwner,
		GitRepository: repoName,
		BranchName:    branchName,
		Pipeline:      pipeline,
		Build:         buildNumber,
		Context:       context,
	}
}

// GenerateBuildNumber generates a new build number for the given pipeline
func GenerateBuildNumber(activities typev1.PipelineActivityInterface, pipelines []*v1.PipelineActivity, pn PipelineID) (string, *v1.PipelineActivity, error) {
	buildCounter := 0
	for _, pipeline := range pipelines {
		if strings.EqualFold(pipeline.Spec.Pipeline, pn.ID) {
			b := pipeline.Spec.Build
			if b != "" {
				bi, err := strconv.Atoi(b)
				if err == nil {
					if bi > buildCounter {
						buildCounter = bi
					}
				}
			}
		}
	}
	buildCounter++
	build := strconv.Itoa(buildCounter)
	name := pn.GetActivityName(build)

	k := &PipelineActivityKey{
		Name:     name,
		Pipeline: pn.ID,
		Build:    build,
	}
	a := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PipelineActivitySpec{},
	}
	spec := &a.Spec
	updateActivitySpec(k, spec)

	answer, err := activities.Create(a)
	if err != nil {
		return "", nil, err
	}
	return build, answer, nil
}

// GetOrCreate gets or creates the pipeline activity
func (k *PipelineActivityKey) GetOrCreate(jxClient versioned.Interface, ns string) (*v1.PipelineActivity, bool, error) {
	name := naming.ToValidName(k.Name)
	create := false
	defaultActivity := &v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.PipelineActivitySpec{},
	}
	activitiesClient := jxClient.JenkinsV1().PipelineActivities(ns)

	if activitiesClient == nil {
		log.Logger().Errorf("No PipelineActivities client available")
		return defaultActivity, create, fmt.Errorf("no PipelineActivities client available")
	}
	a, err := activitiesClient.Get(name, metav1.GetOptions{})
	if err != nil {
		create = true
		a = defaultActivity
	}

	if k.isBatchBuild() {
		//If it's bigger than 1, it can only be a batch build
		err = k.addBatchBuildData(activitiesClient, a)
		if err != nil {
			return defaultActivity, create, errors.Wrap(err, "there was a problem adding batch build data")
		}
	}

	oldSpec := a.Spec
	oldLabels := a.Labels

	updateActivity(k, a)

	if k.isPRBuild() {
		err = k.reconcileBatchBuildIndividualPR(activitiesClient, a)
		if err != nil {
			return defaultActivity, create, errors.Wrap(err, "there was a problem reconciling batch build data")
		}
	}

	if create {
		answer, err := activitiesClient.Create(a)
		return answer, true, err
	} else {
		if !reflect.DeepEqual(&a.Spec, &oldSpec) || !reflect.DeepEqual(a.Labels, oldLabels) {
			answer, err := activitiesClient.PatchUpdate(a)
			if err != nil {
				return answer, false, err
			}
			answer, err = activitiesClient.Get(name, metav1.GetOptions{})
			return answer, false, err
		}
		return a, false, nil
	}
}

func (k *PipelineActivityKey) reconcileBatchBuildIndividualPR(activitiesClient typev1.PipelineActivityInterface, currentActivity *v1.PipelineActivity) error {
	log.Logger().Info("Checking if batch build reconciling is needed")
	activities, err := k.findMatchingPipelineActivitiesWithSameCommitSHA(activitiesClient, currentActivity)
	if err != nil {
		return errors.Wrap(err, "there was a problem listing all activities to reconcile a batch build")
	}

	if len(activities.Items) == 0 {
		log.Logger().Infof("No past executions with the same lastCommitSha found - reconciliation not needed")
		return nil
	}

	currentBuildNumber, err := strconv.Atoi(k.Build)
	if err != nil {
		return errors.Wrapf(err, "error parsing the current build number for PipelineActivity %s", currentActivity.Name)
	}

	//It only makes sense to look for the build before this one
	previousBuildNumber := currentBuildNumber - 1
	for _, v := range activities.Items {
		if v.Spec.BatchPipelineActivity.BatchBuildNumber != "" {
			if buildNumber, err := strconv.Atoi(v.Spec.Build); err == nil {
				//Check if it's the previous build, then we can update the PRs and the batch build
				if previousBuildNumber == buildNumber {
					log.Logger().Infof("Found an earlier PipelineActivity for %s and equal lastCommitSha with batch information", currentActivity.Labels[v1.LabelBranch])
					currentActivity.Spec.BatchPipelineActivity.BatchBuildNumber = v.Spec.BatchPipelineActivity.BatchBuildNumber
					return updateBatchBuildComprisingPRs(activitiesClient, currentActivity, v.Spec.BatchPipelineActivity.BatchBuildNumber, &v)
				}
			}
		}
	}
	return nil
}

func (k *PipelineActivityKey) findMatchingPipelineActivitiesWithSameCommitSHA(activitiesClient typev1.PipelineActivityInterface, currentActivity *v1.PipelineActivity) (*v1.PipelineActivityList, error) {
	//Create a selector for other runs of this PR with the same last commit SHA
	lastCommitSHARequirement, err := labels.NewRequirement("lastCommitSha", selection.In, []string{currentActivity.Labels[v1.LabelLastCommitSha]})
	if err != nil {
		return nil, err
	}

	branchRequirement, err := labels.NewRequirement("branch", selection.In, []string{currentActivity.Labels[v1.LabelBranch]})
	if err != nil {
		return nil, err
	}
	labelSelector := labels.NewSelector().Add(*lastCommitSHARequirement).Add(*branchRequirement)

	gitOwnerSelector := fields.OneTermEqualSelector("spec.gitOwner", currentActivity.Spec.GitOwner)
	gitRepoSelector := fields.OneTermEqualSelector("spec.gitRepository", currentActivity.Spec.GitRepository)
	fieldSelector := fields.AndSelectors(gitOwnerSelector, gitRepoSelector)

	return ListSelectedPipelineActivities(activitiesClient, labelSelector, fieldSelector)
}

func updateBatchBuildComprisingPRs(activitiesClient typev1.PipelineActivityInterface, currentActivity *v1.PipelineActivity, batchBuild string, previousActivityForPR *v1.PipelineActivity) error {
	labels := currentActivity.Labels
	paName := fmt.Sprintf("%s-%s-batch-%s", currentActivity.Spec.GitOwner, currentActivity.Spec.GitRepository, batchBuild)
	log.Logger().Infof("Looking for batch pipeline activity with name %s", paName)
	batchPipelineActivity, err := activitiesClient.Get(paName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "there was a problem getting the PipelineActivity %s", paName)
	}

	if batchPipelineActivity == nil {
		log.Logger().Warnf("No batch pipeline found for reconciliation")
		return nil
	}

	for i := range batchPipelineActivity.Spec.BatchPipelineActivity.ComprisingPulLRequests {
		pr := &batchPipelineActivity.Spec.BatchPipelineActivity.ComprisingPulLRequests[i]
		if pr.PullRequestNumber == labels[v1.LabelBranch] {
			log.Logger().Infof("Updating the build reference for %s in %s with build number %s", pr.PullRequestNumber, paName, currentActivity.Spec.Build)
			pr.LastBuildNumberForCommit = currentActivity.Spec.Build
			break
		}
	}

	_, err = activitiesClient.Update(batchPipelineActivity)
	if err != nil {
		return errors.Wrapf(err, "there was a problem updating the batch PipelineActivity %s", paName)
	}

	log.Logger().Infof("Removing stale batch build information from %s", previousActivityForPR.Name)
	previousActivityForPR.Spec.BatchPipelineActivity.BatchBuildNumber = ""
	_, err = activitiesClient.Update(previousActivityForPR)
	if err != nil {
		return errors.Wrapf(err, "there was a problem updating the PipelineActivity %s", previousActivityForPR.Name)
	}

	return nil
}

func (k *PipelineActivityKey) addBatchBuildData(activitiesClient typev1.PipelineActivityInterface, currentActivity *v1.PipelineActivity) error {
	var prInfos []v1.PullRequestInfo
	for prNumber, sha := range k.PullRefs {
		//Get the build number of the PR based on the SHA
		listOptions := metav1.ListOptions{}
		selector := fmt.Sprintf("lastCommitSha in (%s), branch in (PR-%s)", sha, prNumber)
		listOptions.LabelSelector = selector
		list, err := activitiesClient.List(listOptions)
		if err != nil {
			return errors.Wrapf(err, "there was a problem listing all PipelineActivities for PR-%s with lastCommitSha %s", prNumber, sha)
		}

		// Only proceed if there are existing PipelineActivitys for this commit/branch - it's entirely possible they've
		// all been GCed since they last ran. If there are none, just continue to the next commit/branch.
		if len(list.Items) > 0 {
			//Select the first one as the latest build for this SHA, then iterate the rest
			selectedPipeline := list.Items[0]
			list.Items = list.Items[1:]
			for _, i := range list.Items {
				if i.Spec.Build != "" && selectedPipeline.Spec.Build != "" {
					ib, err := strconv.Atoi(i.Spec.Build)
					if err != nil {
						log.Logger().Errorf("%s", err)
					}
					cb, err := strconv.Atoi(selectedPipeline.Spec.Build)
					if err != nil {
						log.Logger().Errorf("%s", err)
					}
					if ib > cb {
						selectedPipeline = i
					}
				}
			}

			//Update the selected PR's PipelineActivity with the batch info
			selectedPipeline.Spec.BatchPipelineActivity = v1.BatchPipelineActivity{
				BatchBranchName:  currentActivity.Labels[v1.LabelBranch],
				BatchBuildNumber: k.Build,
			}

			_, err = activitiesClient.Update(&selectedPipeline)
			if err != nil {
				return errors.Wrap(err, "there was a problem updating the PR's PipelineActivity")
			}
			//Add this PR's PipelineActivity info to the pull requests array of the batch build's PipelineActivity
			prInfos = append(prInfos, v1.PullRequestInfo{
				PullRequestNumber:        selectedPipeline.Labels[v1.LabelBranch],
				LastBuildNumberForCommit: selectedPipeline.Spec.Build,
				LastBuildSHA:             sha,
			})
		}
	}
	currentActivity.Spec.BatchPipelineActivity = v1.BatchPipelineActivity{
		ComprisingPulLRequests: prInfos,
	}

	return nil
}

// GitOwner returns the git owner (person / organisation) or blank string if it cannot be found
func (k *PipelineActivityKey) GitOwner() string {
	if k.GitInfo != nil {
		return k.GitInfo.Organisation
	}
	pipeline := k.Pipeline
	if pipeline == "" {
		return ""
	}
	paths := strings.Split(pipeline, "/")
	if len(paths) > 1 {
		return paths[0]
	}
	return ""
}

// GitRepository returns the git repository name or blank string if it cannot be found
func (k *PipelineActivityKey) GitRepository() string {
	if k.GitInfo != nil {
		return k.GitInfo.Name
	}
	pipeline := k.Pipeline
	if pipeline == "" {
		return ""
	}
	paths := strings.Split(pipeline, "/")
	if len(paths) > 1 {
		return paths[len(paths)-2]
	}
	return ""
}

// GitURL returns the git URL or blank string if it cannot be found
func (k *PipelineActivityKey) GitURL() string {
	if k.GitInfo != nil {
		return k.GitInfo.URL
	}
	return ""
}

func updateActivity(k *PipelineActivityKey, activity *v1.PipelineActivity) {
	if activity.Labels == nil {
		activity.Labels = make(map[string]string, 4)
	}

	updateActivitySpec(k, &activity.Spec)

	activity.Labels[v1.LabelProvider] = ToProviderName(activity.Spec.GitURL)
	activity.Labels[v1.LabelOwner] = activity.RepositoryOwner()
	activity.Labels[v1.LabelRepository] = activity.RepositoryName()
	activity.Labels[v1.LabelBranch] = activity.BranchName()
	if activity.Spec.Context != "" {
		activity.Labels[v1.LabelContext] = activity.Spec.Context
	}
	if k.isPRBuild() {
		for _, v := range k.PullRefs {
			activity.Labels[v1.LabelLastCommitSha] = v
		}
	}
	buildNumber := activity.Spec.Build
	if buildNumber != "" {
		activity.Labels[v1.LabelBuild] = buildNumber
	}

	for k, v := range activity.Labels {
		activity.Labels[k] = naming.ToValidValue(v)
	}
}

func updateActivitySpec(k *PipelineActivityKey, spec *v1.PipelineActivitySpec) {
	if k.Pipeline != "" && spec.Pipeline == "" {
		spec.Pipeline = k.Pipeline
	}
	if k.Build != "" && spec.Build == "" {
		spec.Build = k.Build
	}
	if k.BuildURL != "" && spec.BuildURL == "" {
		spec.BuildURL = k.BuildURL
	}
	if k.BuildLogsURL != "" && spec.BuildLogsURL == "" {
		spec.BuildLogsURL = k.BuildLogsURL
	}
	if k.ReleaseNotesURL != "" && spec.ReleaseNotesURL == "" {
		spec.ReleaseNotesURL = k.ReleaseNotesURL
	}
	if k.LastCommitSHA != "" && spec.LastCommitSHA == "" {
		spec.LastCommitSHA = k.LastCommitSHA
	}
	if k.LastCommitMessage != "" && spec.LastCommitMessage == "" {
		spec.LastCommitMessage = k.LastCommitMessage
	}
	if k.LastCommitURL != "" && spec.LastCommitURL == "" {
		spec.LastCommitURL = k.LastCommitURL
	}
	if k.Version != "" && spec.Version == "" {
		spec.Version = k.Version
	}
	if k.Context != "" && spec.Context == "" {
		spec.Context = k.Context
	}
	gi := k.GitInfo
	if gi != nil {
		if gi.URL != "" && spec.GitURL == "" {
			spec.GitURL = gi.URL
		}
		if gi.Organisation != "" && spec.GitOwner == "" {
			spec.GitOwner = gi.Organisation
		}
		if gi.Name != "" && spec.GitRepository == "" {
			spec.GitRepository = gi.Name
		}
	}
}

// GetOrCreatePreview gets or creates the Preview step for the key
func (k *PromoteStepActivityKey) GetOrCreatePreview(jxClient versioned.Interface, ns string) (*v1.PipelineActivity, *v1.PipelineActivityStep, *v1.PreviewActivityStep, bool, error) {
	a, _, err := k.GetOrCreate(jxClient, ns)
	if err != nil {
		return nil, nil, nil, false, err
	}
	spec := &a.Spec
	for _, step := range spec.Steps {
		if k.matchesPreview(&step) {
			return a, &step, step.Preview, false, nil
		}
	}
	// if there is no initial release Stage lets add one
	if len(spec.Steps) == 0 {
		endTime := time.Now()
		startTime := endTime.Add(-1 * time.Minute)

		spec.Steps = append(spec.Steps, v1.PipelineActivityStep{
			Kind: v1.ActivityStepKindTypeStage,
			Stage: &v1.StageActivityStep{
				CoreActivityStep: v1.CoreActivityStep{
					StartedTimestamp: &metav1.Time{
						Time: startTime,
					},
					CompletedTimestamp: &metav1.Time{
						Time: endTime,
					},
					Status: v1.ActivityStatusTypeSucceeded,
					Name:   "Release",
				},
			},
		})
	}
	// lets add a new step
	preview := &v1.PreviewActivityStep{
		CoreActivityStep: v1.CoreActivityStep{
			StartedTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
		Environment: k.Environment,
	}
	step := v1.PipelineActivityStep{
		Kind:    v1.ActivityStepKindTypePreview,
		Preview: preview,
	}
	spec.Steps = append(spec.Steps, step)
	return a, &spec.Steps[len(spec.Steps)-1], preview, true, nil
}

// GetOrCreateStage gets or creates the stage for the given name
func GetOrCreateStage(a *v1.PipelineActivity, stageName string) (*v1.PipelineActivityStep, *v1.StageActivityStep, bool) {
	for i := range a.Spec.Steps {
		step := &a.Spec.Steps[i]
		stage := step.Stage
		if stage != nil && stage.Name == stageName {
			return step, step.Stage, false
		}
	}

	stage := &v1.StageActivityStep{
		CoreActivityStep: v1.CoreActivityStep{
			Name: stageName,
		},
	}
	a.Spec.Steps = append(a.Spec.Steps, v1.PipelineActivityStep{
		Kind:  v1.ActivityStepKindTypeStage,
		Stage: stage,
	})
	step := &a.Spec.Steps[len(a.Spec.Steps)-1]
	return step, step.Stage, true
}

// GetStepValueFromStage gets the value for the step for the given name in the given stage, if it already exists. If
// it doesn't exist, it will create a new step for that name instead.
func GetStepValueFromStage(stage *v1.StageActivityStep, stepName string) (v1.CoreActivityStep, bool) {
	for i := range stage.Steps {
		step := &stage.Steps[i]
		if step != nil && step.Name == stepName {
			return *step, false
		}
	}

	step := v1.CoreActivityStep{
		Name: stepName,
	}
	return step, true
}

// GetOrCreatePromote gets or creates the Promote step for the key
func (k *PromoteStepActivityKey) GetOrCreatePromote(jxClient versioned.Interface, ns string) (*v1.PipelineActivity, *v1.PipelineActivityStep, *v1.PromoteActivityStep, bool, error) {
	a, _, err := k.GetOrCreate(jxClient, ns)
	if err != nil {
		return nil, nil, nil, false, err
	}
	spec := &a.Spec
	for _, step := range spec.Steps {
		if k.matchesPromote(&step) {
			return a, &step, step.Promote, false, nil
		}
	}
	// if there is no initial release Stage lets add one
	if len(spec.Steps) == 0 {
		endTime := time.Now()
		startTime := endTime.Add(-1 * time.Minute)

		spec.Steps = append(spec.Steps, v1.PipelineActivityStep{
			Kind: v1.ActivityStepKindTypeStage,
			Stage: &v1.StageActivityStep{
				CoreActivityStep: v1.CoreActivityStep{
					StartedTimestamp: &metav1.Time{
						Time: startTime,
					},
					CompletedTimestamp: &metav1.Time{
						Time: endTime,
					},
					Status: v1.ActivityStatusTypeSucceeded,
					Name:   "Release",
				},
			},
		})
	}
	// lets add a new step
	promote := &v1.PromoteActivityStep{
		CoreActivityStep: v1.CoreActivityStep{
			StartedTimestamp: &metav1.Time{
				Time: time.Now(),
			},
		},
		Environment: k.Environment,
	}
	step := v1.PipelineActivityStep{
		Kind:    v1.ActivityStepKindTypePromote,
		Promote: promote,
	}
	spec.Steps = append(spec.Steps, step)
	return a, &spec.Steps[len(spec.Steps)-1], promote, true, nil
}

// GetOrCreatePromotePullRequest gets or creates the PromotePullRequest for the key
func (k *PromoteStepActivityKey) GetOrCreatePromotePullRequest(jxClient versioned.Interface, ns string) (*v1.PipelineActivity, *v1.PipelineActivityStep, *v1.PromoteActivityStep, *v1.PromotePullRequestStep, bool, error) {
	a, s, p, created, err := k.GetOrCreatePromote(jxClient, ns)
	if err != nil {
		return nil, nil, nil, nil, created, err
	}
	if p.PullRequest == nil {
		created = true
		p.PullRequest = &v1.PromotePullRequestStep{
			CoreActivityStep: v1.CoreActivityStep{
				StartedTimestamp: &metav1.Time{
					Time: time.Now(),
				},
			},
		}
	}
	return a, s, p, p.PullRequest, created, err
}

// GetOrCreatePromoteUpdate gets or creates the Promote for the key
func (k *PromoteStepActivityKey) GetOrCreatePromoteUpdate(jxClient versioned.Interface, ns string) (*v1.PipelineActivity, *v1.PipelineActivityStep, *v1.PromoteActivityStep, *v1.PromoteUpdateStep, bool, error) {
	a, s, p, created, err := k.GetOrCreatePromote(jxClient, ns)
	if err != nil {
		return nil, nil, nil, nil, created, err
	}

	// lets check the PR is completed
	if p.PullRequest != nil {
		if p.PullRequest.Status == v1.ActivityStatusTypeNone {
			p.PullRequest.Status = v1.ActivityStatusTypeSucceeded
		}
	}
	if p.Update == nil {
		created = true
		p.Update = &v1.PromoteUpdateStep{
			CoreActivityStep: v1.CoreActivityStep{
				StartedTimestamp: &metav1.Time{
					Time: time.Now(),
				},
			},
		}
	}
	return a, s, p, p.Update, created, err
}

//OnPromotePullRequest updates activities on a Promote PR
func (k *PromoteStepActivityKey) OnPromotePullRequest(kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string, fn PromotePullRequestFn) error {
	if !k.IsValid() {
		return nil
	}
	activities := jxClient.JenkinsV1().PipelineActivities(ns)
	if activities == nil {
		log.Logger().Warn("Warning: no PipelineActivities client available!")
		return nil
	}
	a, s, ps, p, added, err := k.GetOrCreatePromotePullRequest(jxClient, ns)
	if err != nil {
		return err
	}
	p1 := asYaml(a)
	err = fn(a, s, ps, p)
	if err != nil {
		return err
	}
	if ok, _ := IsTektonEnabled(kubeClient, ns); ok && p.Status != v1.ActivityStatusTypeRunning && p.Status != v1.ActivityStatusTypeSucceeded {
		a.Spec.Status = p.Status
	}

	p2 := asYaml(a)

	if added || p1 == "" || p1 != p2 {
		_, err = activities.PatchUpdate(a)
	}
	return err
}

//OnPromoteUpdate updates activities on a Promote Update
func (k *PromoteStepActivityKey) OnPromoteUpdate(kubeClient kubernetes.Interface, jxClient versioned.Interface, ns string, fn PromoteUpdateFn) error {
	if !k.IsValid() {
		return nil
	}
	activities := jxClient.JenkinsV1().PipelineActivities(ns)
	if activities == nil {
		log.Logger().Warn("Warning: no PipelineActivities client available!")
		return nil
	}
	a, s, ps, p, added, err := k.GetOrCreatePromoteUpdate(jxClient, ns)
	if err != nil {
		return err
	}
	p1 := asYaml(a)
	if k.ApplicationURL != "" {
		ps.ApplicationURL = k.ApplicationURL
	}
	err = fn(a, s, ps, p)
	if err != nil {
		return err
	}
	if k.ApplicationURL != "" {
		ps.ApplicationURL = k.ApplicationURL
	}

	if ok, _ := IsTektonEnabled(kubeClient, ns); ok && p.Status != v1.ActivityStatusTypeRunning {
		a.Spec.Status = p.Status
	}
	p2 := asYaml(a)

	if added || p1 == "" || p1 != p2 {
		_, err = activities.PatchUpdate(a)
	}
	return err
}

// ListSelectedPipelineActivities retrieves the PipelineActivities instances matching the specified label and field selectors. Selectors can be empty or nil.
func ListSelectedPipelineActivities(activitiesClient typev1.PipelineActivityInterface, labelSelector fmt.Stringer, fieldSelector fields.Selector) (*v1.PipelineActivityList, error) {
	log.Logger().Debugf("looking for PipelineActivities with label selector %v and field selector %v", labelSelector, fieldSelector)

	listOptions := metav1.ListOptions{}
	if labelSelector != nil {
		listOptions.LabelSelector = labelSelector.String()
	}

	// Field selectors cannot directly be applied to the list query for custom CRDs - https://github.com/kubernetes/kubernetes/issues/51046
	// We just apply the label selectors and apply the field selection client side
	pipelineActivityList, err := activitiesClient.List(listOptions)
	if err != nil {
		return nil, err
	}

	if fieldSelector == nil {
		return pipelineActivityList, nil
	}

	var matchedItems []v1.PipelineActivity
	for _, pipelineActivity := range pipelineActivityList.Items {
		fieldMap, err := newFieldMap(pipelineActivity)
		if err != nil {
			return nil, errors.Wrap(err, "unable to convert struct to map")
		}
		if fieldSelector.Matches(fieldMap) {
			matchedItems = append(matchedItems, pipelineActivity)
		}
	}

	pipelineActivityList.Items = matchedItems
	return pipelineActivityList, nil
}

func asYaml(activity *v1.PipelineActivity) string {
	data, err := yaml.Marshal(activity)
	if err == nil {
		return string(data)
	}
	log.Logger().Warnf("Failed to marshal PipelineActivity to YAML %s: %s", activity.Name, err)
	return ""
}

func (k *PromoteStepActivityKey) matchesPreview(step *v1.PipelineActivityStep) bool {
	s := step.Preview
	return s != nil && s.Environment == k.Environment
}

func (k *PromoteStepActivityKey) matchesPromote(step *v1.PipelineActivityStep) bool {
	s := step.Promote
	return s != nil && s.Environment == k.Environment
}
