package gits

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/go-github/github"
	"github.com/jenkins-x/jx/pkg/auth"
	gitcfg "gopkg.in/src-d/go-git.v4/config"
)

// OrganisationLister returns a slice of GitOrganisation
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/gits OrganisationLister -o mocks/organisation_lister.go
type OrganisationLister interface {
	ListOrganisations() ([]GitOrganisation, error)
}

// OrganisationChecker verifies if an user is member of an organization
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/gits OrganisationChecker -o mocks/organisation_checker.go
type OrganisationChecker interface {
	IsUserInOrganisation(user string, organisation string) (bool, error)
}

// GitProvider is the interface for abstracting use of different git provider APIs
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/gits GitProvider -o mocks/git_provider.go
type GitProvider interface {
	OrganisationLister

	ListRepositories(org string) ([]*GitRepository, error)

	CreateRepository(org string, name string, private bool) (*GitRepository, error)

	GetRepository(org string, name string) (*GitRepository, error)

	DeleteRepository(org string, name string) error

	ForkRepository(originalOrg string, name string, destinationOrg string) (*GitRepository, error)

	RenameRepository(org string, name string, newName string) (*GitRepository, error)

	ValidateRepositoryName(org string, name string) error

	CreatePullRequest(data *GitPullRequestArguments) (*GitPullRequest, error)

	UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error)

	UpdatePullRequestStatus(pr *GitPullRequest) error

	AddLabelsToIssue(owner, repo string, number int, labels []string) error

	GetPullRequest(owner string, repo *GitRepository, number int) (*GitPullRequest, error)

	ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error)

	GetPullRequestCommits(owner string, repo *GitRepository, number int) ([]*GitCommit, error)

	PullRequestLastCommitStatus(pr *GitPullRequest) (string, error)

	ListCommitStatus(org string, repo string, sha string) ([]*GitRepoStatus, error)

	ListCommits(owner string, repo string, opt *ListCommitsArguments) ([]*GitCommit, error)

	UpdateCommitStatus(org string, repo string, sha string, status *GitRepoStatus) (*GitRepoStatus, error)

	MergePullRequest(pr *GitPullRequest, message string) error

	CreateWebHook(data *GitWebHookArguments) error

	ListWebHooks(org string, repo string) ([]*GitWebHookArguments, error)

	UpdateWebHook(data *GitWebHookArguments) error

	IsGitHub() bool

	IsGitea() bool

	IsBitbucketCloud() bool

	IsBitbucketServer() bool

	IsGerrit() bool

	Kind() string

	GetIssue(org string, name string, number int) (*GitIssue, error)

	IssueURL(org string, name string, number int, isPull bool) string

	SearchIssues(org string, name string, state string) ([]*GitIssue, error)

	SearchIssuesClosedSince(org string, name string, t time.Time) ([]*GitIssue, error)

	CreateIssue(owner string, repo string, issue *GitIssue) (*GitIssue, error)

	HasIssues() bool

	AddPRComment(pr *GitPullRequest, comment string) error

	CreateIssueComment(owner string, repo string, number int, comment string) error

	UpdateRelease(owner string, repo string, tag string, releaseInfo *GitRelease) error

	UpdateReleaseStatus(owner string, repo string, tag string, releaseInfo *GitRelease) error

	ListReleases(org string, name string) ([]*GitRelease, error)

	GetRelease(org string, name string, tag string) (*GitRelease, error)

	UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error)

	GetLatestRelease(org string, name string) (*GitRelease, error)

	GetContent(org string, name string, path string, ref string) (*GitFileContent, error)

	// returns the path relative to the Jenkins URL to trigger webhooks on this kind of repository
	//

	// e.g. for GitHub its /github-webhook/
	// other examples include:
	//
	// * gitlab: /gitlab/notify_commit
	// https://github.com/elvanja/jenkins-gitlab-hook-plugin#notify-commit-hook
	//
	// * git plugin
	// /git/notifyCommit?url=
	// http://kohsuke.org/2011/12/01/polling-must-die-triggering-jenkins-builds-from-a-git-hook/
	//
	// * gitea
	// /gitea-webhook/post
	//
	// * generic webhook
	// /generic-webhook-trigger/invoke?token=abc123
	// https://wiki.jenkins.io/display/JENKINS/Generic+Webhook+Trigger+Plugin

	JenkinsWebHookPath(gitURL string, secret string) string

	// Label returns the Git service label or name
	Label() string

	// ServerURL returns the Git server URL
	ServerURL() string

	// BranchArchiveURL returns a URL to the ZIP archive for the git branch
	BranchArchiveURL(org string, name string, branch string) string

	// Returns the current username
	CurrentUsername() string

	// Returns the current user auth
	UserAuth() auth.UserAuth

	// Returns user info, if possible
	UserInfo(username string) *GitUser

	AddCollaborator(string, string, string) error
	// TODO Refactor to remove bespoke types when we implement another provider
	ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error)
	// TODO Refactor to remove bespoke types when we implement another provider
	AcceptInvitation(int64) (*github.Response, error)

	// ShouldForkForPullRequest returns true if we should create a personal fork of this repository
	// before creating a pull request
	ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool

	GetBranch(owner string, repo string, branch string) (*GitBranch, error)

	GetProjects(owner string, repo string) ([]GitProject, error)

	IsWikiEnabled(owner string, repo string) (bool, error)

	ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error)
}

// Gitter defines common git actions used by Jenkins X via git cli
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/gits Gitter -o mocks/gitter.go
type Gitter interface {
	Config(dir string, args ...string) error
	FindGitConfigDir(dir string) (string, string, error)
	PrintCreateRepositoryGenerateAccessToken(server *auth.AuthServer, username string, o io.Writer)

	Status(dir string) error
	Server(dir string) (string, error)
	Info(dir string) (*GitRepository, error)
	IsFork(dir string) (bool, error)
	Version() (string, error)
	RepoName(org, repoName string) string

	Username(dir string) (string, error)
	SetUsername(dir string, username string) error
	Email(dir string) (string, error)
	SetEmail(dir string, email string) error
	GetAuthorEmailForCommit(dir string, sha string) (string, error)

	Init(dir string) error
	Clone(url string, directory string) error
	CloneBare(dir string, url string) error
	PushMirror(dir string, url string) error

	// ShallowCloneBranch TODO not sure if this method works any more - consider using ShallowClone(dir, url, branch, "")
	ShallowCloneBranch(url string, branch string, directory string) error
	ShallowClone(dir string, url string, commitish string, pullRequest string) error
	FetchUnshallow(dir string) error
	IsShallow(dir string) (bool, error)
	Push(dir string, remote string, force bool, refspec ...string) error
	PushMaster(dir string) error
	PushTag(dir string, tag string) error
	// CreateAuthenticatedURL adds username and password into the specified git URL.
	CreateAuthenticatedURL(url string, userAuth *auth.UserAuth) (string, error)
	ForcePushBranch(dir string, localBranch string, remoteBranch string) error
	CloneOrPull(url string, directory string) error
	Pull(dir string) error
	PullRemoteBranches(dir string) error
	PullUpstream(dir string) error

	// ResetToUpstream resets the given branch to the upstream version
	ResetToUpstream(dir string, branch string) error

	AddRemote(dir string, name string, url string) error
	SetRemoteURL(dir string, name string, gitURL string) error
	UpdateRemote(dir, url string) error
	DiscoverRemoteGitURL(gitConf string) (string, error)
	DiscoverUpstreamGitURL(gitConf string) (string, error)
	RemoteBranches(dir string) ([]string, error)
	RemoteBranchNames(dir string, prefix string) ([]string, error)
	RemoteMergedBranchNames(dir string, prefix string) ([]string, error)
	GetRemoteUrl(config *gitcfg.Config, name string) string
	RemoteUpdate(dir string) error
	LocalBranches(dir string) ([]string, error)
	Remotes(dir string) ([]string, error)

	Branch(dir string) (string, error)
	CreateBranchFrom(dir string, branchName string, startPoint string) error
	CreateBranch(dir string, branch string) error
	CheckoutRemoteBranch(dir string, branch string) error
	Checkout(dir string, branch string) error
	CheckoutCommitFiles(dir string, commit string, files []string) error
	CheckoutOrphan(dir string, branch string) error
	ConvertToValidBranchName(name string) string
	FetchBranch(dir string, repo string, refspec ...string) error
	FetchBranchShallow(dir string, repo string, refspec ...string) error
	FetchBranchUnshallow(dir string, repo string, refspec ...string) error
	Merge(dir string, commitish string) error
	MergeTheirs(dir string, commitish string) error
	Reset(dir string, commitish string, hard bool) error
	RebaseTheirs(dir string, upstream string, branch string, skipEmpty bool) error
	CherryPick(dir string, commitish string) error
	CherryPickTheirs(dir string, commitish string) error
	CherryPickTheirsKeepRedundantCommits(dir string, commitish string) error

	StashPush(dir string) error
	StashPop(dir string) error

	Remove(dir, fileName string) error
	RemoveForce(dir, fileName string) error
	CleanForce(dir, fileName string) error
	Add(dir string, args ...string) error

	CommitIfChanges(dir string, message string) error
	CommitDir(dir string, message string) error
	AddCommit(dir string, msg string) error
	AddCommitFiles(dir string, msg string, files []string) error
	HasChanges(dir string) (bool, error)
	HasFileChanged(dir string, fileName string) (bool, error)
	Diff(dir string) (string, error)
	ListChangedFilesFromBranch(dir string, branch string) (string, error)
	LoadFileFromBranch(dir string, branch string, file string) (string, error)

	GetLatestCommitMessage(dir string) (string, error)
	GetCommitPointedToByPreviousTag(dir string) (string, string, error)
	GetCommitPointedToByLatestTag(dir string) (string, string, error)
	GetCommitPointedToByTag(dir string, tag string) (string, error)
	FetchTags(dir string) error
	FetchRemoteTags(dir string, repo string) error
	Tags(dir string) ([]string, error)
	FilterTags(dir string, filter string) ([]string, error)
	CreateTag(dir string, tag string, msg string) error
	GetLatestCommitSha(dir string) (string, error)
	GetFirstCommitSha(dir string) (string, error)
	GetCommits(dir string, start string, end string) ([]GitCommit, error)
	RevParse(dir string, rev string) (string, error)
	GetCommitsNotOnAnyRemote(dir string, branch string) ([]GitCommit, error)
	Describe(dir string, contains bool, commitish string, abbrev string, fallback bool) (string, string, error)
	IsAncestor(dir string, possibleAncestor string, commitish string) (bool, error)

	GetRevisionBeforeDate(dir string, t time.Time) (string, error)
	GetRevisionBeforeDateText(dir string, dateText string) (string, error)
	DeleteRemoteBranch(dir string, remoteName string, branch string) error
	DeleteLocalBranch(dir string, branch string) error

	SetUpstreamTo(dir string, branch string) error

	WriteRepoAttributes(dir string, contents string) error
	ReadRepoAttributes(dir string) (string, error)
}

// PullRequestDetails is the details for creating a pull request
type PullRequestDetails struct {
	Message    string
	BranchName string
	Title      string
	Labels     []string
}

func (p *PullRequestDetails) String() string {
	return fmt.Sprintf("Branch Name: %s; Title: %s; Message: %s", p.BranchName, p.Title, p.Message)
}
