package v1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// Release represents a single version of an app that has been released
type Release struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ReleaseSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ReleaseStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ReleaseList is a list of Release resources
type ReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Release `json:"items"`
}

// ReleaseSpec is the specification of the Release
type ReleaseSpec struct {
	Name              string             `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	Version           string             `json:"version,omitempty"  protobuf:"bytes,2,opt,name=version"`
	GitHTTPURL        string             `json:"gitHttpUrl,omitempty"  protobuf:"bytes,3,opt,name=gitHttpUrl"`
	GitCloneURL       string             `json:"gitCloneUrl,omitempty"  protobuf:"bytes,4,opt,name=gitCloneUrl"`
	Commits           []CommitSummary    `json:"commits,omitempty" protobuf:"bytes,5,opt,name=commits"`
	Issues            []IssueSummary     `json:"issues,omitempty" protobuf:"bytes,6,opt,name=issues"`
	PullRequests      []IssueSummary     `json:"pullRequests,omitempty" protobuf:"bytes,7,opt,name=pullRequests"`
	DependencyUpdates []DependencyUpdate `json:"dependencyUpdates,omitempty" protobuf:"bytes,11,opt,name=dependencyUpdates"`
	ReleaseNotesURL   string             `json:"releaseNotesURL,omitempty" protobuf:"bytes,8,opt,name=releaseNotesURL"`
	GitRepository     string             `json:"gitRepository,omitempty" protobuf:"bytes,9,opt,name=gitRepository"`
	GitOwner          string             `json:"gitOwner,omitempty" protobuf:"bytes,10,opt,name=gitOwner"`
}

// ReleaseStatus is the status of a release
type ReleaseStatus struct {
	Status ReleaseStatusType `json:"status,omitempty"  protobuf:"bytes,1,opt,name=status"`
}

// IssueSummary is the summary of an issue
type IssueSummary struct {
	ID                string        `json:"id,omitempty"  protobuf:"bytes,1,opt,name=id"`
	URL               string        `json:"url,omitempty"  protobuf:"bytes,2,opt,name=url"`
	Title             string        `json:"title,omitempty"  protobuf:"bytes,3,opt,name=title"`
	Body              string        `json:"body,omitempty"  protobuf:"bytes,4,opt,name=body"`
	State             string        `json:"state,omitempty"  protobuf:"bytes,5,opt,name=state"`
	Message           string        `json:"message,omitempty"  protobuf:"bytes,6,opt,name=message"`
	User              *UserDetails  `json:"user,omitempty"  protobuf:"bytes,7,opt,name=user"`
	Assignees         []UserDetails `json:"assignees,omitempty"  protobuf:"bytes,8,opt,name=assignees"`
	ClosedBy          *UserDetails  `json:"closedBy,omitempty"  protobuf:"bytes,9,opt,name=closedBy"`
	CreationTimestamp *metav1.Time  `json:"creationTimestamp,omitempty" protobuf:"bytes,10,opt,name=creationTimestamp"`
	Labels            []IssueLabel  `json:"labels,omitempty" protobuf:"bytes,11,opt,name=labels"`
}

type IssueLabel struct {
	URL   string `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	Name  string `json:"url,omitempty"  protobuf:"bytes,2,opt,name=url"`
	Color string `json:"color,omitempty"  protobuf:"bytes,3,opt,name=color"`
}

// CommitSummary is the summary of a commit
type CommitSummary struct {
	Message   string       `json:"message,omitempty"  protobuf:"bytes,1,opt,name=message"`
	SHA       string       `json:"sha,omitempty"  protobuf:"bytes,2,opt,name=sha"`
	URL       string       `json:"url,omitempty"  protobuf:"bytes,3,opt,name=url"`
	Author    *UserDetails `json:"author,omitempty"  protobuf:"bytes,4,opt,name=author"`
	Committer *UserDetails `json:"committer,omitempty"  protobuf:"bytes,5,opt,name=committer"`
	Branch    string       `json:"branch,omitempty"  protobuf:"bytes,6,opt,name=branch"`
	IssueIDs  []string     `json:"issueIds,omitempty"  protobuf:"bytes,7,opt,name=issueIds"`
}

// ReleaseStatusType is the status of a release; usually deployed or failed at completion
type ReleaseStatusType string

const (
	// ReleaseStatusTypeNone an activity step has not started yet
	ReleaseStatusTypeNone ReleaseStatusType = ""
	// ReleaseStatusTypePending the release is pending
	ReleaseStatusTypePending ReleaseStatusType = "Pending"
	// ReleaseStatusTypeDeployed a release has been deployed
	ReleaseStatusTypeDeployed ReleaseStatusType = "Deployed"
	// ReleaseStatusTypeFailed release failed
	ReleaseStatusTypeFailed ReleaseStatusType = "Failed"
)

// IsClosed returns true if this issue is closed or fixed
func (i *IssueSummary) IsClosed() bool {
	lower := strings.ToLower(i.State)
	return strings.HasPrefix(lower, "clos") || strings.HasPrefix(lower, "fix")
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// GitService represents a git provider so we can map the host name to a kinda of git service
type GitService struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec GitServiceSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// GitServiceSpec is the specification of an GitService
type GitServiceSpec struct {
	GitKind string `json:"gitKind,omitempty" protobuf:"bytes,1,opt,name=gitKind"`
	URL     string `json:"url,omitempty" protobuf:"bytes,2,opt,name=host"`
	Name    string `json:"name,omitempty" protobuf:"bytes,3,opt,name=host"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitServiceList is a list of GitService resources
type GitServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []GitService `json:"items"`
}

//DependencyUpdate describes an dependency update message from the commit log
type DependencyUpdate struct {
	DependencyUpdateDetails `json:",inline"`
	Paths                   []DependencyUpdatePath `json:"paths,omitempty"`
}

// DependencyUpdatePath is the path of a dependency update
type DependencyUpdatePath []DependencyUpdateDetails

// DependencyUpdateDetails are the details of a dependency update
type DependencyUpdateDetails struct {
	Host               string `json:"host"`
	Owner              string `json:"owner"`
	Repo               string `json:"repo"`
	Component          string `json:"component, omitempty"`
	URL                string `json:"url"`
	FromVersion        string `json:"fromVersion"`
	FromReleaseHTMLURL string `json:"fromReleaseHTMLURL"`
	FromReleaseName    string `json:"fromReleaseName"`
	ToVersion          string `json:"toVersion"`
	ToReleaseHTMLURL   string `json:"toReleaseHTMLURL"`
	ToReleaseName      string `json:"toReleaseName"`
}

func (d *DependencyUpdateDetails) String() string {
	return fmt.Sprintf("%s/%s/%s:%s", d.Host, d.Owner, d.Repo, d.Component)
}
