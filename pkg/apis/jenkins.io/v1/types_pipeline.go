package v1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	LabelProvider      = "provider"
	LabelOwner         = "owner"
	LabelRepository    = "repository"
	LabelBranch        = "branch"
	LabelBuild         = "build"
	LabelLastCommitSha = "lastCommitSha"
	LabelContext       = "context"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// PipelineActivity represents pipeline activity for a particular run of a pipeline
type PipelineActivity struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   PipelineActivitySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status PipelineActivityStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// PipelineActivitySpec is the specification of the pipeline activity
type PipelineActivitySpec struct {
	Pipeline              string                 `json:"pipeline,omitempty" protobuf:"bytes,1,opt,name=pipeline"`
	Build                 string                 `json:"build,omitempty" protobuf:"bytes,2,opt,name=build"`
	Version               string                 `json:"version,omitempty" protobuf:"bytes,3,opt,name=version"`
	Status                ActivityStatusType     `json:"status,omitempty" protobuf:"bytes,4,opt,name=status"`
	StartedTimestamp      *metav1.Time           `json:"startedTimestamp,omitempty" protobuf:"bytes,5,opt,name=startedTimestamp"`
	CompletedTimestamp    *metav1.Time           `json:"completedTimestamp,omitempty" protobuf:"bytes,6,opt,name=completedTimestamp"`
	Steps                 []PipelineActivityStep `json:"steps,omitempty" protobuf:"bytes,7,opt,name=steps"`
	BuildURL              string                 `json:"buildUrl,omitempty" protobuf:"bytes,8,opt,name=buildUrl"`
	BuildLogsURL          string                 `json:"buildLogsUrl,omitempty" protobuf:"bytes,9,opt,name=buildLogsUrl"`
	GitURL                string                 `json:"gitUrl,omitempty" protobuf:"bytes,10,opt,name=gitUrl"`
	GitRepository         string                 `json:"gitRepository,omitempty" protobuf:"bytes,11,opt,name=gitRepository"`
	GitOwner              string                 `json:"gitOwner,omitempty" protobuf:"bytes,12,opt,name=gitOwner"`
	GitBranch             string                 `json:"gitBranch,omitempty" protobuf:"bytes,13,opt,name=gitBranch"`
	Author                string                 `json:"author,omitempty" protobuf:"bytes,14,opt,name=author"`
	PullTitle             string                 `json:"pullTitle,omitempty" protobuf:"bytes,15,opt,name=pullTitle"`
	ReleaseNotesURL       string                 `json:"releaseNotesURL,omitempty" protobuf:"bytes,16,opt,name=releaseNotesURL"`
	LastCommitSHA         string                 `json:"lastCommitSHA,omitempty" protobuf:"bytes,17,opt,name=lastCommitSHA"`
	LastCommitMessage     string                 `json:"lastCommitMessage,omitempty" protobuf:"bytes,18,opt,name=lastCommitMessage"`
	LastCommitURL         string                 `json:"lastCommitURL,omitempty" protobuf:"bytes,19,opt,name=lastCommitURL"`
	Workflow              string                 `json:"workflow,omitempty" protobuf:"bytes,20,opt,name=workflow"`
	WorkflowStatus        ActivityStatusType     `json:"workflowStatus,omitempty" protobuf:"bytes,21,opt,name=workflowStatus"`
	WorkflowMessage       string                 `json:"workflowMessage,omitempty" protobuf:"bytes,22,opt,name=workflowMessage"`
	PostExtensions        []ExtensionExecution   `json:"postExtensions,omitempty" protobuf:"bytes,23,opt,name=postExtensions"`
	Attachments           []Attachment           `json:"attachments,omitempty" protobuf:"bytes,24,opt,name=attachments"`
	BatchPipelineActivity BatchPipelineActivity  `json:"batchPipelineActivity,omitempty" protobuf:"bytes,25,opt,name=batchPipelineActivity"`
	Context               string                 `json:"context,omitempty" protobuf:"bytes,26,opt,name=context"`
	BaseSHA               string                 `json:"baseSHA,omitempty" protobuf:"bytes,27,opt,name=baseSHA"`
}

// BatchPipelineActivity contains information about a batch build, used by both the batch build and its comprising PRs for linking them together
type BatchPipelineActivity struct {
	BatchBuildNumber       string            `json:"batchBuildNumber,omitempty" protobuf:"bytes,1,opt,name=batchBuildNumber"`
	BatchBranchName        string            `json:"batchBranchName,omitempty" protobuf:"bytes,2,opt,name=batchBranchName"`
	ComprisingPulLRequests []PullRequestInfo `json:"pullRequestInfo,omitempty" protobuf:"bytes,3,opt,name=pullRequestInfo"`
}

// PullRequestInfo contains information about a PR included in a batch, like its PR number, the last build number, and SHA
type PullRequestInfo struct {
	PullRequestNumber string `json:"pullRequestNumber,omitempty" protobuf:"bytes,1,opt,name=pullRequestNumber"`
	// LastBuildNumberForCommit is the number of the last successful build of this PR outside of a batch
	LastBuildNumberForCommit string `json:"lastBuildNumberForCommit,omitempty" protobuf:"bytes,2,opt,name=lastBuildNumberForCommit"`
	// LastBuildSHA is the commit SHA in the last successful build of this PR outside of a batch.
	LastBuildSHA string `json:"lastBuildSHA,omitempty" protobuf:"bytes,3,opt,name=lastBuildSHA"`
}

// PipelineActivityStep represents a step in a pipeline activity
type PipelineActivityStep struct {
	Kind    ActivityStepKindType `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	Stage   *StageActivityStep   `json:"stage,omitempty" protobuf:"bytes,2,opt,name=stage"`
	Promote *PromoteActivityStep `json:"promote,omitempty" protobuf:"bytes,3,opt,name=promote"`
	Preview *PreviewActivityStep `json:"preview,omitempty" protobuf:"bytes,4,opt,name=preview"`
}

// CoreActivityStep is a base step included in Stages of a pipeline or other kinds of step
type CoreActivityStep struct {
	Name               string             `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Description        string             `json:"description,omitempty" protobuf:"bytes,2,opt,name=description"`
	Status             ActivityStatusType `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
	StartedTimestamp   *metav1.Time       `json:"startedTimestamp,omitempty" protobuf:"bytes,4,opt,name=startedTimestamp"`
	CompletedTimestamp *metav1.Time       `json:"completedTimestamp,omitempty" protobuf:"bytes,5,opt,name=completedTimestamp"`
}

// StageActivityStep represents a stage of zero to more sub steps in a jenkins pipeline
type StageActivityStep struct {
	CoreActivityStep `json:",inline"`

	Steps []CoreActivityStep `json:"steps,omitempty" protobuf:"bytes,1,opt,name=steps"`
}

// PreviewActivityStep is the step of creating a preview environment as part of a Pull Request pipeline
type PreviewActivityStep struct {
	CoreActivityStep `json:",inline"`

	Environment    string `json:"environment,omitempty" protobuf:"bytes,1,opt,name=environment"`
	PullRequestURL string `json:"pullRequestURL,omitempty" protobuf:"bytes,2,opt,name=pullRequestURL"`
	ApplicationURL string `json:"applicationURL,omitempty" protobuf:"bytes,3,opt,name=applicationURL"`
}

// PromoteActivityStep is the step of promoting a version of an application to an environment
type PromoteActivityStep struct {
	CoreActivityStep `json:",inline"`

	Environment    string                  `json:"environment,omitempty" protobuf:"bytes,1,opt,name=environment"`
	PullRequest    *PromotePullRequestStep `json:"pullRequest,omitempty" protobuf:"bytes,2,opt,name=pullRequest"`
	Update         *PromoteUpdateStep      `json:"update,omitempty" protobuf:"bytes,3,opt,name=update"`
	ApplicationURL string                  `json:"applicationURL,omitempty" protobuf:"bytes,4,opt,name=environment"`
}

// GitStatus the status of a git commit in terms of CI/CD
type GitStatus struct {
	URL    string `json:"url,omitempty" protobuf:"bytes,1,opt,name=url"`
	Status string `json:"status,omitempty" protobuf:"bytes,2,opt,name=status"`
}

// PromotePullRequestStep is the step for promoting a version to an environment by raising a Pull Request on the
// git repository of the environment
type PromotePullRequestStep struct {
	CoreActivityStep `json:",inline"`

	PullRequestURL string `json:"pullRequestURL,omitempty" protobuf:"bytes,1,opt,name=pullRequestURL"`
	MergeCommitSHA string `json:"mergeCommitSHA,omitempty" protobuf:"bytes,2,opt,name=mergeCommitSHA"`
}

// PromoteUpdateStep is the step for updating a promotion after the Pull Request merges to master
type PromoteUpdateStep struct {
	CoreActivityStep `json:",inline"`

	Statuses []GitStatus `json:"statuses,omitempty" protobuf:"bytes,1,opt,name=statuses"`
}

// PipelineActivityStatus is the status for an Environment resource
type PipelineActivityStatus struct {
	Version string `json:"version,omitempty"  protobuf:"bytes,1,opt,name=version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineActivityList is a list of PipelineActivity resources
type PipelineActivityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PipelineActivity `json:"items"`
}

// ActivityStepKindType is a kind of step
type ActivityStepKindType string

const (
	// ActivityStepKindTypeNone no kind yet
	ActivityStepKindTypeNone ActivityStepKindType = ""
	// ActivityStepKindTypeStage a group of low level steps
	ActivityStepKindTypeStage ActivityStepKindType = "Stage"
	// ActivityStepKindTypePreview a promote activity
	ActivityStepKindTypePreview ActivityStepKindType = "Preview"
	// ActivityStepKindTypePromote a promote activity
	ActivityStepKindTypePromote ActivityStepKindType = "Promote"
)

// ActivityStatusType is the status of an activity; usually succeeded or failed/error on completion
type ActivityStatusType string

const (
	// ActivityStatusTypeNone an activity step has not started yet
	ActivityStatusTypeNone ActivityStatusType = ""
	// ActivityStatusTypePending an activity step is waiting to start
	ActivityStatusTypePending ActivityStatusType = "Pending"
	// ActivityStatusTypeRunning an activity is running
	ActivityStatusTypeRunning ActivityStatusType = "Running"
	// ActivityStatusTypeSucceeded an activity completed successfully
	ActivityStatusTypeSucceeded ActivityStatusType = "Succeeded"
	// ActivityStatusTypeFailed an activity failed
	ActivityStatusTypeFailed ActivityStatusType = "Failed"
	// ActivityStatusTypeWaitingForApproval an activity is waiting for approval
	ActivityStatusTypeWaitingForApproval ActivityStatusType = "WaitingForApproval"
	// ActivityStatusTypeError there is some error with an activity
	ActivityStatusTypeError ActivityStatusType = "Error"
	// ActivityStatusTypeAborted if the workflow was aborted
	ActivityStatusTypeAborted ActivityStatusType = "Aborted"
	// ActivityStatusTypeNotExecuted if the workflow was not executed
	ActivityStatusTypeNotExecuted ActivityStatusType = "NotExecuted"
)

type Attachment struct {
	Name string   `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	URLs []string `json:"urls,omitempty"  protobuf:"bytes,2,opt,name=urls"`
}

// IsTerminated returns true if this activity has stopped executing
func (s ActivityStatusType) IsTerminated() bool {
	return s == ActivityStatusTypeSucceeded || s == ActivityStatusTypeFailed || s == ActivityStatusTypeError || s == ActivityStatusTypeAborted
}

func (s ActivityStatusType) String() string {
	return string(s)
}

// RepositoryName returns the repository name for the given pipeline
func (p *PipelineActivity) RepositoryName() string {
	repoName := p.Spec.GitRepository
	pipelineName := p.Spec.Pipeline

	paths := strings.Split(pipelineName, "/")
	if repoName == "" && len(paths) > 1 {
		repoName = paths[len(paths)-2]
		p.Spec.GitRepository = repoName
	}
	return repoName
}

// RepositoryOwner returns the repository name for the given pipeline
func (p *PipelineActivity) RepositoryOwner() string {
	repoOwner := p.Spec.GitOwner
	pipelineName := p.Spec.Pipeline

	paths := strings.SplitN(pipelineName, "/", 2)
	if repoOwner == "" && len(paths) > 1 {
		repoOwner = paths[0]
		p.Spec.GitOwner = repoOwner
	}
	return repoOwner
}

// BranchName returns the name of the branch for the pipeline
func (p *PipelineActivity) BranchName() string {
	pipelineName := p.Spec.Pipeline
	if pipelineName == "" {
		return ""
	}
	paths := strings.Split(pipelineName, "/")
	branch := paths[len(paths)-1]
	p.Spec.GitBranch = branch
	return branch
}

// StagesByStatus returns a map with statuses as keys and lists of stage names with that status as the values.
func (p *PipelineActivity) StagesByStatus() map[ActivityStatusType][]string {
	statusMap := make(map[ActivityStatusType][]string)

	for _, stage := range p.Spec.Steps {
		if stage.Kind == ActivityStepKindTypeStage && stage.Stage != nil {
			if _, exists := statusMap[stage.Stage.Status]; !exists {
				statusMap[stage.Stage.Status] = []string{}
			}
			statusMap[stage.Stage.Status] = append(statusMap[stage.Stage.Status], stage.Stage.Name)
		}
	}

	return statusMap
}
