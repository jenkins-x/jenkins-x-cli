package gits

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	gerrit "github.com/andygrunwald/go-gerrit"
	"github.com/google/go-github/github"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
)

type GerritProvider struct {
	Client   *gerrit.Client
	Username string
	Context  context.Context

	Server auth.AuthServer
	User   auth.UserAuth
	Git    Gitter
}

func NewGerritProvider(server *auth.AuthServer, user *auth.UserAuth, git Gitter) (GitProvider, error) {
	ctx := context.Background()

	provider := GerritProvider{
		Server:   *server,
		User:     *user,
		Context:  ctx,
		Username: user.Username,
		Git:      git,
	}

	client, err := gerrit.NewClient(server.URL, nil)
	if err != nil {
		return nil, err
	}
	client.Authentication.SetBasicAuth(user.Username, user.ApiToken)
	provider.Client = client

	return &provider, nil
}

// We have to do this because url.Escape is not idempotent, so we unescape the URL
// to ensure it's not encoded, then we re-encode it.
func buildEncodedProjectName(org, name string) string {
	var fullName string

	if org != "" {
		fullName = fmt.Sprintf("%s/%s", org, name)
	} else {
		fullName = fmt.Sprintf("%s", name)
	}

	fullNamePathUnescaped, err := url.PathUnescape(fullName)
	if err != nil {
		return ""
	}
	fullNamePathEscaped := url.PathEscape(fullNamePathUnescaped)

	return fullNamePathEscaped
}

func (p *GerritProvider) projectInfoToGitRepository(project *gerrit.ProjectInfo) *GitRepository {
	return &GitRepository{
		Name:     project.Name,
		CloneURL: fmt.Sprintf("%s/%s", p.Server.URL, project.Name),
		SSHURL:   fmt.Sprintf("%s:%s", p.Server.URL, project.Name),
	}
}

func (p *GerritProvider) ListRepositories(org string) ([]*GitRepository, error) {
	options := &gerrit.ProjectOptions{
		Description: true,
		Prefix:      url.PathEscape(org),
	}

	gerritProjects, _, err := p.Client.Projects.ListProjects(options)
	if err != nil {
		return nil, err
	}

	repos := []*GitRepository{}

	for name, project := range *gerritProjects {
		project.Name = name
		repo := p.projectInfoToGitRepository(&project)

		repos = append(repos, repo)
	}

	return repos, nil
}

func (p *GerritProvider) CreateRepository(org string, name string, private bool) (*GitRepository, error) {
	input := &gerrit.ProjectInput{
		SubmitType:      "INHERIT",
		Description:     "Created automatically by Jenkins X.",
		PermissionsOnly: private,
	}

	fullNamePathEscaped := buildEncodedProjectName(org, name)
	project, _, err := p.Client.Projects.CreateProject(fullNamePathEscaped, input)
	if err != nil {
		return nil, err
	}

	repo := p.projectInfoToGitRepository(project)
	return repo, nil
}

func (p *GerritProvider) GetRepository(org string, name string) (*GitRepository, error) {
	fullName := buildEncodedProjectName(org, name)

	project, _, err := p.Client.Projects.GetProject(fullName)
	if err != nil {
		return nil, err
	}
	return p.projectInfoToGitRepository(project), nil
}

func (p *GerritProvider) DeleteRepository(org string, name string) error {
	return nil
}

func (p *GerritProvider) ForkRepository(originalOrg string, name string, destinationOrg string) (*GitRepository, error) {
	return nil, nil
}

func (p *GerritProvider) RenameRepository(org string, name string, newName string) (*GitRepository, error) {
	return nil, nil
}

func (p *GerritProvider) ValidateRepositoryName(org string, name string) error {
	return nil
}

func (p *GerritProvider) CreatePullRequest(data *GitPullRequestArguments) (*GitPullRequest, error) {
	return nil, nil
}

// UpdatePullRequest updates pull request with number using data
func (p *GerritProvider) UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error) {
	return nil, errors.Errorf("Not yet implemented for gerrit")
}

func (p *GerritProvider) UpdatePullRequestStatus(pr *GitPullRequest) error {
	return nil
}

func (p *GerritProvider) GetPullRequest(owner string, repo *GitRepository, number int) (*GitPullRequest, error) {
	return nil, nil
}

// ListOpenPullRequests lists the open pull requests
func (p *GerritProvider) ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error) {
	return nil, nil
}

func (p *GerritProvider) GetPullRequestCommits(owner string, repo *GitRepository, number int) ([]*GitCommit, error) {
	return nil, nil
}

func (p *GerritProvider) PullRequestLastCommitStatus(pr *GitPullRequest) (string, error) {
	return "", nil
}

func (p *GerritProvider) ListCommitStatus(org string, repo string, sha string) ([]*GitRepoStatus, error) {
	return nil, nil
}

// UpdateCommitStatus updates the status of a specified commit in a specified repo.
func (p *GerritProvider) UpdateCommitStatus(org, repo, sha string, status *GitRepoStatus) (*GitRepoStatus, error) {
	return nil, nil
}

func (p *GerritProvider) MergePullRequest(pr *GitPullRequest, message string) error {
	return nil
}

func (p *GerritProvider) CreateWebHook(data *GitWebHookArguments) error {
	return nil
}

// UpdateWebHook update a webhook with the data specified.
func (p *GerritProvider) UpdateWebHook(data *GitWebHookArguments) error {
	return nil
}

// ListWebHooks lists all webhooks for the specified repo.
func (p *GerritProvider) ListWebHooks(org, repo string) ([]*GitWebHookArguments, error) {
	return nil, nil
}

// ListOrganisations lists all organizations the configured user has access to.
func (p *GerritProvider) ListOrganisations() ([]GitOrganisation, error) {
	return nil, nil
}

func (p *GerritProvider) IsGitHub() bool {
	return false
}

func (p *GerritProvider) IsGitea() bool {
	return false
}

func (p *GerritProvider) IsBitbucketCloud() bool {
	return false
}

func (p *GerritProvider) IsBitbucketServer() bool {
	return false
}

func (p *GerritProvider) IsGerrit() bool {
	return true
}

func (p *GerritProvider) Kind() string {
	return "gerrit"
}

func (p *GerritProvider) GetIssue(org string, name string, number int) (*GitIssue, error) {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return nil, nil
}

func (p *GerritProvider) IssueURL(org string, name string, number int, isPull bool) string {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return ""
}

func (p *GerritProvider) SearchIssues(org string, name string, query string) ([]*GitIssue, error) {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return nil, nil
}

func (p *GerritProvider) SearchIssuesClosedSince(org string, name string, t time.Time) ([]*GitIssue, error) {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return nil, nil
}

func (p *GerritProvider) CreateIssue(owner string, repo string, issue *GitIssue) (*GitIssue, error) {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return nil, nil
}

func (p *GerritProvider) HasIssues() bool {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return false
}

func (p *GerritProvider) AddPRComment(pr *GitPullRequest, comment string) error {
	return nil
}

func (p *GerritProvider) CreateIssueComment(owner string, repo string, number int, comment string) error {
	log.Logger().Warn("Gerrit does not support issue tracking")
	return nil
}

func (p *GerritProvider) UpdateRelease(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	return nil
}

// UpdateReleaseStatus is not supported for this git provider
func (p *GerritProvider) UpdateReleaseStatus(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	return nil
}

func (p *GerritProvider) ListReleases(org string, name string) ([]*GitRelease, error) {
	return nil, nil
}

// GetRelease returns the release info for org, repo name and tag
func (p *GerritProvider) GetRelease(org string, name string, tag string) (*GitRelease, error) {
	return nil, nil
}

func (p *GerritProvider) JenkinsWebHookPath(gitURL string, secret string) string {
	return ""
}

func (p *GerritProvider) Label() string {
	return ""
}

func (p *GerritProvider) ServerURL() string {
	return ""
}

func (p *GerritProvider) BranchArchiveURL(org string, name string, branch string) string {
	return ""
}

func (p *GerritProvider) CurrentUsername() string {
	return ""
}

func (p *GerritProvider) UserAuth() auth.UserAuth {
	return auth.UserAuth{}
}

func (p *GerritProvider) UserInfo(username string) *GitUser {
	return nil
}

func (p *GerritProvider) AddCollaborator(user string, organisation string, repo string) error {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for gerrit. Please add user: %v as a collaborator to this project.", user)
	return nil
}

func (p *GerritProvider) ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for gerrit.")
	return []*github.RepositoryInvitation{}, &github.Response{}, nil
}

func (p *GerritProvider) AcceptInvitation(ID int64) (*github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for gerrit.")
	return &github.Response{}, nil
}

func (p *GerritProvider) GetContent(org string, name string, path string, ref string) (*GitFileContent, error) {
	return nil, fmt.Errorf("Getting content not supported on gerrit")
}

// ShouldForkForPullReques treturns true if we should create a personal fork of this repository
// before creating a pull request
func (p *GerritProvider) ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool {
	return originalOwner != username
}

func (p *GerritProvider) ListCommits(owner, repo string, opt *ListCommitsArguments) ([]*GitCommit, error) {
	return nil, fmt.Errorf("Listing commits not supported on gerrit")
}

// AddLabelsToIssue adds labels to issues or pullrequests
func (p *GerritProvider) AddLabelsToIssue(owner, repo string, number int, labels []string) error {
	return fmt.Errorf("Getting content not supported on gerrit")
}

// GetLatestRelease fetches the latest release from the git provider for org and name
func (p *GerritProvider) GetLatestRelease(org string, name string) (*GitRelease, error) {
	return nil, nil
}

// UploadReleaseAsset will upload an asset to org/repo to a release with id, giving it a name, it will return the release asset from the git provider
func (p *GerritProvider) UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error) {
	return nil, nil
}

// GetBranch returns the branch information for an owner/repo, including the commit at the tip
func (p *GerritProvider) GetBranch(owner string, repo string, branch string) (*GitBranch, error) {
	return nil, nil
}

// GetProjects returns all the git projects in owner/repo
func (p *GerritProvider) GetProjects(owner string, repo string) ([]GitProject, error) {
	return nil, nil
}

//ConfigureFeatures sets specific features as enabled or disabled for owner/repo
func (p *GerritProvider) ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error) {
	return nil, nil
}

// IsWikiEnabled returns true if a wiki is enabled for owner/repo
func (p *GerritProvider) IsWikiEnabled(owner string, repo string) (bool, error) {
	return false, nil
}
