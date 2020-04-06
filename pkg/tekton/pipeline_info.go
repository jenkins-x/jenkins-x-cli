package tekton

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	tektonv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	knativeapis "knative.dev/pkg/apis"
)

// PipelineRunInfo provides information on a PipelineRun and its stages for use in getting logs and populating activity
type PipelineRunInfo struct {
	Name              string
	Organisation      string
	Repository        string
	Branch            string
	Context           string
	Build             string
	BuildNumber       int
	Pipeline          string
	PipelineRun       string
	LastCommitSHA     string
	BaseSHA           string
	LastCommitMessage string
	LastCommitURL     string
	GitURL            string
	GitInfo           *gits.GitRepository
	Stages            []*StageInfo
	Type              string
	CreatedTime       time.Time
}

// StageInfo provides information on a particular stage, including its pod info or info on its nested stages
type StageInfo struct {
	// TODO: For now, we're not including git info - we're going to assume we have the same git info for the whole
	// pipeline.
	Name string

	// These fields will populated for all non-parent stages
	PodName        string
	Task           string
	TaskRun        string
	FirstStepImage string
	CreatedTime    time.Time
	Pod            *corev1.Pod

	// These fields will only be populated for appropriate parent stages
	Parallel []*StageInfo
	Stages   []*StageInfo

	// This field will be non-empty if this is a nested stage, containing a list of  the names of all its parent stages with the top-level parent first
	Parents []string
}

// GetStageNameIncludingParents constructs a full stage name including its parents, if they exist.
func (si *StageInfo) GetStageNameIncludingParents() string {
	if si.Name != "" {
		return strings.NewReplacer("-", " ").Replace(strings.Join(append(si.Parents, si.Name), " / "))
	}
	return si.PodName
}

// PipelineRunInfoFilter allows specifying criteria on which to filter a list of PipelineRunInfos
type PipelineRunInfoFilter struct {
	Owner      string
	Repository string
	Branch     string
	Build      string
	Filter     string
	Pending    bool
	Context    string
}

// GetBuild gets the build identifier
func (pri PipelineRunInfo) GetBuild() string {
	return pri.Build
}

// GetOrderedTaskStages gets all the stages in this pipeline which actually contain a Task, in rough execution order
// TODO: Handle parallelism better, where execution is not a straight line.
func (pri *PipelineRunInfo) GetOrderedTaskStages() []*StageInfo {
	var stages []*StageInfo

	for _, n := range pri.Stages {
		stages = append(stages, n.getOrderedTaskStagesForStage()...)
	}

	return stages
}

func (si *StageInfo) getOrderedTaskStagesForStage() []*StageInfo {
	// If this is a Task Stage, not a parent Stage, return itself
	if si.Task != "" {
		return []*StageInfo{si}
	}

	var stages []*StageInfo

	if len(si.Stages) > 0 {
		for _, n := range si.Stages {
			stages = append(stages, n.getOrderedTaskStagesForStage()...)
		}
	}

	if len(si.Parallel) > 0 {
		for _, n := range si.Parallel {
			stages = append(stages, n.getOrderedTaskStagesForStage()...)
		}
	}

	return stages
}

// CreatePipelineRunInfo looks up the PipelineRun for a given name and creates the PipelineRunInfo for it
func CreatePipelineRunInfo(prName string, podList *corev1.PodList, ps *v1.PipelineStructure, pr *tektonv1alpha1.PipelineRun) (*PipelineRunInfo, error) {
	branch := ""
	lastCommitSha := ""
	lastCommitMessage := ""
	lastCommitURL := ""
	owner := ""
	repo := ""
	build := ""
	pullRefs := ""
	pullBaseSha := ""
	pullPullSha := ""
	shaFromGitInit := ""
	shaRegexp, err := regexp.Compile("\b[a-z0-9]{40}\b")
	if err != nil {
		log.Logger().Warnf("Failed to compile regexp because %s", err)
	}
	gitURL := ""

	if pr == nil {
		return nil, errors.New(fmt.Sprintf("PipelineRun %s cannot be found", prName))
	}

	pipelineType := BuildPipeline

	if strings.HasPrefix(pr.Name, MetaPipeline.String()+"-") {
		pipelineType = MetaPipeline
	}

	pri := &PipelineRunInfo{
		Name:        possiblyUniquePipelineResourceName(pr.Labels[LabelOwner], pr.Labels[LabelRepo], pr.Labels[LabelBranch], pr.Labels[LabelContext], pr.Labels[LabelType], false) + "-" + pr.Labels[LabelBuild],
		PipelineRun: pr.Name,
		Pipeline:    pr.Spec.PipelineRef.Name,
		Type:        pipelineType.String(),
		CreatedTime: pr.CreationTimestamp.Time,
	}

	var pod *corev1.Pod

	prStatus := pr.Status.GetCondition(knativeapis.ConditionSucceeded)
	if err := pri.SetPodsForPipelineRun(podList, ps); err != nil {
		return nil, errors.Wrapf(err, "Failure populating stages and pods for PipelineRun %s", prName)
	}

	pod = pri.FindFirstStagePod()

	if pod == nil {
		if prStatus != nil && prStatus.Status == corev1.ConditionUnknown {
			return nil, errors.New(fmt.Sprintf("Couldn't find a Stage with steps for PipelineRun %s", prName))
		}
		// Just return nil if the pipeline run is completed and its pods have been GCed
		return nil, nil
	}

	if pod.Labels != nil {
		pri.Context = pod.Labels[LabelContext]
	}
	containers, _, isInit := kube.GetContainersWithStatusAndIsInit(pod)
	for _, container := range containers {
		// We historically used the git source step automatically injected by Tekton to get the git URL and the sha or
		// branch being built, but that is no longer going to always be accurate due to our bespoke git merge step
		// handling checkout/merging of PR branches into the target branch.
		// The Prow/Lighthouse provided environment variables are a better source of truth, but we preserve this logic
		// for now in case of edge cases, as a fallback.
		if strings.HasPrefix(container.Name, "build-step-git-source") || strings.HasPrefix(container.Name, "step-git-source") {
			_, args := kube.GetCommandAndArgs(&container, isInit)
			for i := 0; i <= len(args)-2; i += 2 {
				key := args[i]
				value := args[i+1]

				switch key {
				case "-url":
					gitURL = value
				case "-revision":
					if shaRegexp.MatchString(value) {
						shaFromGitInit = value
					} else {
						branch = value
					}
				}
			}
		}
		for _, v := range container.Env {
			if v.Value == "" {
				continue
			}
			// PULL_PULL_SHA is set by Prow/Lighthouse with the HEAD SHA of the PR being built, or the HEAD SHA of the
			// first PR in a batch. It will be empty for non-PR builds.
			if v.Name == "PULL_PULL_SHA" {
				pullPullSha = v.Value
			}
			// PULL_BASE_SHA is set by Prow/Lighthouse with the target SHA for a PR build, or with just the HEAD SHA for
			// master. It is always set.
			if v.Name == "PULL_BASE_SHA" {
				pullBaseSha = v.Value
			}
			// BRANCH_NAME is set by Prow/Lighthouse with the branch name being built - either PR-123 or master in almost
			// all cases. It is always set.
			if v.Name == util.EnvVarBranchName {
				branch = v.Value
			}
			// REPO_OWNER is set by Prow/Lighthouse with the org or user the repo is under on the SCM provider. It is
			// always set.
			if v.Name == "REPO_OWNER" {
				owner = v.Value
			}
			// REPO_NAME is set by Prow/Lighthouse with the repo name. It is always set.
			if v.Name == "REPO_NAME" {
				repo = v.Value
			}
			// Deprecated - this is only set for static masters.
			if v.Name == "JX_BUILD_NUMBER" {
				build = v.Value
			}
			// SOURCE_URL is set by Prow/Lighthouse with the clone URL for the repo. It is always set.
			if v.Name == "SOURCE_URL" && gitURL == "" {
				gitURL = v.Value
			}
			// PULL_REFS is set by Prow/Lighthouse with a comma-separated list of colon-delimited pairs of branch:ref
			// involved in the build. For PRs, it will be "master:...,1:...", for batch builds, it will be "master:...,1:...,2:...",
			// and for release builds, it will be "master:...". It is always set.
			if v.Name == "PULL_REFS" && pullRefs == "" {
				pullRefs = v.Value
			}
		}
		if branch == "" {
			for _, v := range container.Env {
				// PULL_BASE_REF is set by Prow/Lighthouse to the branch or ref name the PR is targeting, like "master".
				// It is only set for PR and batch builds.
				if v.Name == "PULL_BASE_REF" {
					build = v.Value
				}
			}
		}
		if build == "" {
			for _, v := range container.Env {
				// BUILD_NUMBER is set by the metapipeline. This used to also look at BUILD_ID, but we don't set that
				// any more.
				if v.Name == "BUILD_NUMBER" {
					build = v.Value
				}
			}
		}
	}

	if pullBaseSha != "" {
		pri.BaseSHA = pullBaseSha
	}

	if pullPullSha != "" {
		lastCommitSha = pullPullSha
	}

	// If we have the PULL_REFS env var, fall back on it for the lastCommitSha and base sha.
	if pullRefs != "" {
		splitRefs := strings.Split(pullRefs, ",")
		// If there's at least one entry, use the first entry for the base sha.
		if len(splitRefs) > 0 {
			if pri.BaseSHA == "" {
				pri.BaseSHA = shaFromPullRefEntry(splitRefs[0])
			}
		}
		// If there are at least two entries, use the second entry for the lastCommitSha
		if len(splitRefs) > 1 {
			if lastCommitSha == "" {
				// If we don't have a lastCommitSha, use the second ref
				lastCommitSha = shaFromPullRefEntry(splitRefs[1])
			}
		}
	}

	// Fall back on git checkout if for some reason the last commit sha wasn't set based on env vars.
	if lastCommitSha == "" {
		lastCommitSha = shaFromGitInit
	}

	if build == "" {
		build = builds.GetBuildNumberFromLabels(pr.Labels)
	}
	if build == "" {
		build = "1"
	}
	buildNumber, err := strconv.Atoi(build)
	if err != nil {
		buildNumber = 1
	}

	pri.Build = build
	pri.BuildNumber = buildNumber
	pri.Branch = branch
	if gitURL != "" {
		gitInfo, err := gits.ParseGitURL(gitURL)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to parse Git URL %s", gitURL)
		}
		if owner == "" {
			owner = gitInfo.Organisation
		}
		if repo == "" {
			repo = gitInfo.Name
		}
		pri.GitInfo = gitInfo
		pri.Pipeline = owner + "/" + repo + "/" + branch
		pri.Name = owner + "-" + repo + "-" + branch + "-" + build
		pri.Organisation = owner
		pri.Repository = repo
		pri.GitURL = gitURL
		pri.LastCommitMessage = lastCommitMessage
		pri.LastCommitSHA = lastCommitSha
		pri.LastCommitURL = lastCommitURL
	}
	return pri, nil
}

// shaFromPullRefEntry returns the sha from "some-ref-name-or-pr-number:01234567890abcdef...", returning an empty string
// if the entry isn't in that format.
func shaFromPullRefEntry(refEntry string) string {
	idx := strings.LastIndex(refEntry, ":")
	if idx > 0 {
		return refEntry[idx+1:]
	}
	return ""
}

// SetPodsForPipelineRun populates the pods for all stages within its PipelineRunInfo
func (pri *PipelineRunInfo) SetPodsForPipelineRun(podList *corev1.PodList, ps *v1.PipelineStructure) error {
	if pri.PipelineRun == "" {
		return errors.New("No PipelineRun specified")
	}

	if ps == nil {
		return errors.New(fmt.Sprintf("Could not find PipelineStructure for PipelineRun %s", pri.PipelineRun))
	}

	pscs := ps.GetAllStagesAndChildren()

	var firstTaskStage *StageInfo

	for _, psc := range pscs {
		pri.Stages = append(pri.Stages, stageAndChildrenToStageInfo(psc, []string{}))
	}

	for _, si := range pri.Stages {
		if firstTaskStage == nil {
			firstTaskStage = si
		}
		if err := si.SetPodsForStageInfo(podList, pri.PipelineRun); err != nil {
			return errors.Wrapf(err, "Couldn't populate Pods for Stages")
		}
	}

	return nil
}

// SetPodsForStageInfo populates the pods for a particular stage and/or its children
func (si *StageInfo) SetPodsForStageInfo(podList *corev1.PodList, prName string) error {
	var podListItems []corev1.Pod

	for _, p := range podList.Items {
		if p.Labels[syntax.LabelStageName] == syntax.MangleToRfc1035Label(si.Name, "") && p.Labels[pipeline.GroupName+pipeline.PipelineRunLabelKey] == prName {
			podListItems = append(podListItems, p)
		}
	}

	if si.Task != "" {
		if len(podListItems) == 0 {
			// TODO: Probably the pod just hasn't started yet, so return nil
			return nil
		}
		if len(podListItems) > 1 {
			return errors.New(fmt.Sprintf("Too many Pods (%d) found for PipelineRun %s and Stage %s", len(podListItems), prName, si.Name))
		}
		pod := podListItems[0]
		si.PodName = pod.Name
		si.Task = pod.Labels[builds.LabelTaskName]
		si.TaskRun = pod.Labels[builds.LabelTaskRunName]
		si.Pod = &pod
		si.CreatedTime = pod.CreationTimestamp.Time
		containers, _, isInit := kube.GetContainersWithStatusAndIsInit(&pod)
		if isInit && len(containers) > 2 {
			si.FirstStepImage = containers[2].Image
		} else if !isInit && len(containers) > 1 {
			si.FirstStepImage = containers[1].Image
		}
	} else if len(si.Stages) > 0 {
		for _, child := range si.Stages {
			if err := child.SetPodsForStageInfo(podList, prName); err != nil {
				return err
			}
		}
	} else if len(si.Parallel) > 0 {
		for _, child := range si.Parallel {
			if err := child.SetPodsForStageInfo(podList, prName); err != nil {
				return err
			}
		}
	}

	return nil
}

// FindFirstStagePod finds the first stage in this pipeline run to have a pod, and then returns its pod
func (pri *PipelineRunInfo) FindFirstStagePod() *corev1.Pod {
	for _, s := range pri.Stages {
		found := s.findTaskStageInfo()
		if found != nil {
			return found.Pod
		}
	}
	return nil
}

// findTaskStageInfo gets the first stage that should actually have a pod created for it
func (si *StageInfo) findTaskStageInfo() *StageInfo {
	if si.Task != "" {
		return si
	}
	for _, s := range si.Parallel {
		child := s.findTaskStageInfo()
		if child != nil {
			return child
		}
	}
	for _, s := range si.Stages {
		child := s.findTaskStageInfo()
		if child != nil {
			return child
		}
	}

	return nil
}

// GetFullChildStageNames gets the fully qualified (i.e., with parents appended) names of each stage underneath this one.
func (si *StageInfo) GetFullChildStageNames(includeSelf bool) []string {
	if si.Task != "" && includeSelf {
		return []string{si.GetStageNameIncludingParents()}
	}

	var names []string
	for _, n := range si.Parallel {
		names = append(names, n.GetFullChildStageNames(true)...)
	}
	for _, n := range si.Stages {
		names = append(names, n.GetFullChildStageNames(true)...)
	}

	return names
}

func stageAndChildrenToStageInfo(psc *v1.PipelineStageAndChildren, parents []string) *StageInfo {
	si := &StageInfo{
		Name:    psc.Stage.Name,
		Parents: parents,
	}
	if psc.Stage.TaskRef != nil {
		si.Task = *psc.Stage.TaskRef
	}

	for _, s := range psc.Stages {
		si.Stages = append(si.Stages, stageAndChildrenToStageInfo(&s, append(parents, psc.Stage.Name)))
	}

	for _, s := range psc.Parallel {
		si.Parallel = append(si.Parallel, stageAndChildrenToStageInfo(&s, append(parents, psc.Stage.Name)))
	}

	return si
}

// PipelineRunMatches returns true if the pipeline run info matches the filter
func (o *PipelineRunInfoFilter) PipelineRunMatches(info *PipelineRunInfo) bool {
	if o.Owner != "" && o.Owner != info.Organisation {
		return false
	}
	if o.Repository != "" && o.Repository != info.Repository {
		return false
	}
	if o.Branch != "" && strings.ToLower(o.Branch) != strings.ToLower(info.Branch) {
		return false
	}
	if o.Build != "" && o.Build != info.Build {
		return false
	}
	if o.Context != "" && o.Context != info.Context {
		return false
	}
	if o.Filter != "" && !strings.Contains(info.Name, o.Filter) {
		return false
	}
	if o.Pending {
		status := info.Status()
		if status != "Pending" && status != "Running" {
			return false
		}
	}
	return true
}

// BuildNumber returns the integer build number filter if specified
func (o *PipelineRunInfoFilter) BuildNumber() int {
	text := o.Build
	if text != "" {
		answer, err := strconv.Atoi(text)
		if err != nil {
			return answer
		}
	}
	return 0
}

// MatchesPipeline returns true if this build info matches the given pipeline
func (pri *PipelineRunInfo) MatchesPipeline(activity *v1.PipelineActivity) bool {
	d := kube.CreatePipelineDetails(activity)
	if d == nil {
		return false
	}
	return d.GitOwner == pri.Organisation && d.GitRepository == pri.Repository && d.Build == pri.Build && strings.ToLower(d.BranchName) == strings.ToLower(pri.Branch) && d.Context == pri.Context
}

// Status returns the build status
func (pri *PipelineRunInfo) Status() string {
	pod := pri.FindFirstStagePod()
	if pod == nil {
		return "No Pod"
	}
	return string(pod.Status.Phase)
}

// ToBuildPodInfo converts the object into a BuildPodInfo so it can be easily filtered
func (pri PipelineRunInfo) ToBuildPodInfo() *builds.BuildPodInfo {
	answer := &builds.BuildPodInfo{
		Name:              pri.Name,
		Organisation:      pri.Organisation,
		Repository:        pri.Repository,
		Branch:            pri.Branch,
		Build:             pri.Build,
		BuildNumber:       pri.BuildNumber,
		Context:           pri.Context,
		Pipeline:          pri.Pipeline,
		LastCommitSHA:     pri.LastCommitSHA,
		LastCommitURL:     pri.LastCommitURL,
		LastCommitMessage: pri.LastCommitMessage,
		GitInfo:           pri.GitInfo,
	}
	pod := pri.FindFirstStagePod()
	if pod != nil {
		answer.Pod = pod
		answer.PodName = pod.Name
		containers := pod.Spec.Containers
		if len(containers) > 0 {
			answer.FirstStepImage = containers[0].Image
		}
		answer.CreatedTime = pod.CreationTimestamp.Time
	}
	return answer
}

// PipelineRunInfoOrder allows sorting of a slice of PipelineRunInfos
type PipelineRunInfoOrder []*PipelineRunInfo

func (a PipelineRunInfoOrder) Len() int      { return len(a) }
func (a PipelineRunInfoOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PipelineRunInfoOrder) Less(i, j int) bool {
	b1 := a[i]
	b2 := a[j]
	if b1.Organisation != b2.Organisation {
		return b1.Organisation < b2.Organisation
	}
	if b1.Repository != b2.Repository {
		return b1.Repository < b2.Repository
	}
	if b1.Branch != b2.Branch {
		return b1.Branch < b2.Branch
	}
	return b1.BuildNumber > b2.BuildNumber
}

// SortPipelineRunInfos sorts a slice of PipelineRunInfos by their org, repo, branch, and build number
func SortPipelineRunInfos(pris []*PipelineRunInfo) {
	sort.Sort(PipelineRunInfoOrder(pris))
}
