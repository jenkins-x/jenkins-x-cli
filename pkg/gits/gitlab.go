package gits

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	errors2 "github.com/pkg/errors"

	"github.com/google/go-github/github"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/xanzy/go-gitlab"
)

type GitlabProvider struct {
	Username string
	Client   *gitlab.Client
	Context  context.Context

	Server auth.AuthServer
	User   auth.UserAuth
	Git    Gitter
}

func NewGitlabProvider(server *auth.AuthServer, user *auth.UserAuth, git Gitter) (GitProvider, error) {
	u := server.URL
	c := gitlab.NewClient(nil, user.ApiToken)
	if !IsGitLabServerURL(u) {
		if err := c.SetBaseURL(u); err != nil {
			return nil, err
		}
	}
	return WithGitlabClient(server, user, c, git)
}

func IsGitLabServerURL(u string) bool {
	u = strings.TrimSuffix(u, "/")
	return u == "" || u == "https://gitlab.com" || u == "http://gitlab.com"
}

// Used by unit tests to inject a mocked client
func WithGitlabClient(server *auth.AuthServer, user *auth.UserAuth, client *gitlab.Client, git Gitter) (GitProvider, error) {
	provider := &GitlabProvider{
		Server:   *server,
		User:     *user,
		Username: user.Username,
		Client:   client,
		Git:      git,
	}
	return provider, nil
}

func (g *GitlabProvider) ListRepositories(org string) ([]*GitRepository, error) {
	result, _, err := getRepositories(g.Client, g.Username, org, "")
	if err != nil {
		return nil, err
	}

	var repos []*GitRepository
	for _, p := range result {
		repos = append(repos, fromGitlabProject(p))
	}
	return repos, nil
}

func (g *GitlabProvider) ListReleases(org string, name string) ([]*GitRelease, error) {
	answer := []*GitRelease{}
	// TODO
	return answer, nil
}

// GetRelease returns the release info for the org, repo name and tag
func (g *GitlabProvider) GetRelease(org string, name string, tag string) (*GitRelease, error) {
	return nil, nil
}

func getRepositories(g *gitlab.Client, username string, org string, searchFilter string) ([]*gitlab.Project, *gitlab.Response, error) {
	// TODO: handle the case of more than "pageSize" results, similarly to ListOpenPullRequests().
	gitlabSearchFilter := gitlab.String(searchFilter)
	listOpts := gitlab.ListOptions{PerPage: pageSize}
	listProjectOpts := &gitlab.ListProjectsOptions{
		Owned:       gitlab.Bool(true),
		Search:      gitlabSearchFilter,
		ListOptions: listOpts,
	}

	if org != "" {
		projects, resp, err := g.Groups.ListGroupProjects(org, &gitlab.ListGroupProjectsOptions{Search: gitlabSearchFilter, ListOptions: listOpts})
		if err != nil {
			return g.Projects.ListUserProjects(org, listProjectOpts)
		}
		return projects, resp, err

	}
	return g.Projects.ListUserProjects(username, listProjectOpts)
}

func GetOwnerNamespaceID(g *gitlab.Client, owner string) (int, error) {
	n := &gitlab.ListNamespacesOptions{
		Search: &owner,
	}

	namespaces, _, err := g.Namespaces.ListNamespaces(n)
	if err != nil {
		return -1, err
	}

	for _, v := range namespaces {
		if v.FullPath == owner {
			return v.ID, nil
		}
	}

	return -1, fmt.Errorf("no namespace found for owner %s", owner)
}

func fromGitlabProject(p *gitlab.Project) *GitRepository {
	org := ""
	if p.Namespace != nil {
		org = p.Namespace.Name
	}
	return &GitRepository{
		Organisation: org,
		Project:      org,
		Name:         p.Name,
		HTMLURL:      p.WebURL,
		SSHURL:       p.SSHURLToRepo,
		CloneURL:     p.HTTPURLToRepo,
		Fork:         p.ForkedFromProject != nil,
	}
}

func (g *GitlabProvider) CreateRepository(org string, name string, private bool) (*GitRepository, error) {
	visibility := gitlab.PublicVisibility
	if private {
		visibility = gitlab.PrivateVisibility
	}

	namespaceID, err := GetOwnerNamespaceID(g.Client, owner(org, g.Username))
	if err != nil {
		return nil, err
	}

	p := &gitlab.CreateProjectOptions{
		Name:        &name,
		Visibility:  &visibility,
		NamespaceID: &namespaceID,
	}

	project, _, err := g.Client.Projects.CreateProject(p)
	if err != nil {
		return nil, err
	}
	return fromGitlabProject(project), nil
}

func owner(org, username string) string {
	if org == "" {
		return username
	}
	return org
}

func (g *GitlabProvider) GetRepository(org, name string) (*GitRepository, error) {
	pid, err := g.projectId(org, g.Username, name)
	if err != nil {
		return nil, err
	}
	project, response, err := g.Client.Projects.GetProject(pid, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("request: %s failed due to: %s", response.Request.URL, err)
	}
	return fromGitlabProject(project), nil
}

func (g *GitlabProvider) ListOrganisations() ([]GitOrganisation, error) {
	groups, _, err := g.Client.Groups.ListGroups(nil)
	if err != nil {
		return nil, err
	}

	var organizations []GitOrganisation
	for _, v := range groups {
		organizations = append(organizations, GitOrganisation{v.Path})
	}
	return organizations, nil
}

func (g *GitlabProvider) projectId(org, username, name string) (string, error) {
	repos, _, err := getRepositories(g.Client, username, org, name)
	if err != nil {
		return "", err
	}

	for _, repo := range repos {
		if repo.Name == name {
			return strconv.Itoa(repo.ID), nil
		}
	}
	return "", fmt.Errorf("no repository found with name %s", name)
}

func (g *GitlabProvider) DeleteRepository(org, name string) error {
	pid, err := g.projectId(org, g.Username, name)
	if err != nil {
		return err
	}

	_, err = g.Client.Projects.DeleteProject(pid)
	if err != nil {
		return fmt.Errorf("failed to delete repository %s due to: %s", pid, err)
	}
	return err
}

func (g *GitlabProvider) ForkRepository(originalOrg, name, destinationOrg string) (*GitRepository, error) {
	pid, err := g.projectId(originalOrg, g.Username, name)
	if err != nil {
		return nil, err
	}
	project, _, err := g.Client.Projects.ForkProject(pid, nil, nil)
	if err != nil {
		return nil, err
	}

	return fromGitlabProject(project), nil
}

func (g *GitlabProvider) RenameRepository(org, name, newName string) (*GitRepository, error) {
	pid, err := g.projectId(org, g.Username, name)
	if err != nil {
		return nil, err
	}
	options := &gitlab.EditProjectOptions{
		Name: &newName,
	}

	project, _, err := g.Client.Projects.EditProject(pid, options)
	if err != nil {
		return nil, err
	}
	return fromGitlabProject(project), nil
}

func (g *GitlabProvider) ValidateRepositoryName(org, name string) error {
	pid, err := g.projectId(org, g.Username, name)
	if err == nil {
		return fmt.Errorf("repository %s already exists", pid)
	}
	return nil
}

func (g *GitlabProvider) CreatePullRequest(data *GitPullRequestArguments) (*GitPullRequest, error) {
	owner := data.GitRepository.Organisation
	repo := data.GitRepository.Name
	title := data.Title
	body := data.Body
	head := data.Head
	base := data.Base

	o := &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		Description:  &body,
		SourceBranch: &head,
		TargetBranch: &base,
	}

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return nil, err
	}
	mr, _, err := g.Client.MergeRequests.CreateMergeRequest(pid, o)
	if err != nil {
		return nil, err
	}

	return fromMergeRequest(mr, owner, repo), nil
}

// UpdatePullRequest updates pull request with number using data
func (g *GitlabProvider) UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error) {
	owner := data.GitRepository.Organisation
	repo := data.GitRepository.Name
	title := data.Title
	body := data.Body
	base := data.Base

	o := &gitlab.UpdateMergeRequestOptions{
		Title:        &title,
		Description:  &body,
		TargetBranch: &base,
	}

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return nil, err
	}
	mr, resp, err := g.Client.MergeRequests.UpdateMergeRequest(pid, number, o)
	if err != nil {
		if resp != nil && resp.Body != nil {
			data, err2 := ioutil.ReadAll(resp.Body)
			if err2 == nil && len(data) > 0 {
				return nil, errors2.Wrapf(err, "response: %s", string(data))
			}
		}
		return nil, err
	}

	return &GitPullRequest{
		URL:    mr.WebURL,
		Owner:  owner,
		Repo:   repo,
		Number: &mr.IID,
	}, nil
}

func fromMergeRequest(mr *gitlab.MergeRequest, owner, repo string) *GitPullRequest {
	merged := false
	if mr.MergedAt != nil {
		merged = true
	}
	return &GitPullRequest{
		Author: &GitUser{
			Login: mr.Author.Username,
		},
		URL:            mr.WebURL,
		Owner:          owner,
		Repo:           repo,
		Number:         &mr.IID,
		State:          &mr.State,
		Title:          mr.Title,
		Body:           mr.Description,
		MergeCommitSHA: &mr.MergeCommitSHA,
		Merged:         &merged,
		LastCommitSha:  mr.SHA,
		MergedAt:       mr.MergedAt,
		ClosedAt:       mr.ClosedAt,
	}
}

func (g *GitlabProvider) UpdatePullRequestStatus(pr *GitPullRequest) error {
	owner := pr.Owner
	repo := pr.Repo

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return err
	}
	mr, _, err := g.Client.MergeRequests.GetMergeRequest(pid, *pr.Number, nil, nil)
	if err != nil {
		return err
	}

	*pr = *fromMergeRequest(mr, owner, repo)
	return nil
}

// GetPullRequest gets a PR
func (g *GitlabProvider) GetPullRequest(owner string, repo *GitRepository, number int) (*GitPullRequest, error) {
	pr := &GitPullRequest{
		Owner:  owner,
		Repo:   repo.Name,
		Number: &number,
	}
	err := g.UpdatePullRequestStatus(pr)

	return pr, err
}

// ListOpenPullRequests lists the open pull requests
func (g *GitlabProvider) ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error) {
	gitlabOpen := "opened"
	opt := &gitlab.ListMergeRequestsOptions{
		State: &gitlabOpen,
		ListOptions: gitlab.ListOptions{
			Page:    0,
			PerPage: pageSize,
		},
	}
	answer := []*GitPullRequest{}
	for {
		prs, _, err := g.Client.MergeRequests.ListMergeRequests(opt)
		if err != nil {
			return answer, err
		}
		for _, pr := range prs {
			answer = append(answer, fromMergeRequest(pr, owner, repo))
		}
		if len(prs) < pageSize || len(prs) == 0 {
			break
		}
		opt.Page++
	}
	return answer, nil
}

// GetPullRequestCommits gets the PR commits
func (g *GitlabProvider) GetPullRequestCommits(owner string, repository *GitRepository, number int) ([]*GitCommit, error) {
	repo := repository.Name
	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return nil, err
	}
	commits, _, err := g.Client.MergeRequests.GetMergeRequestCommits(pid, number, nil)

	if err != nil {
		return nil, err
	}

	answer := []*GitCommit{}

	for _, commit := range commits {
		if commit == nil {
			continue
		}
		summary := &GitCommit{
			Message: commit.Message,
			SHA:     commit.ID,
			Author: &GitUser{
				Email: commit.AuthorEmail,
			},
		}
		answer = append(answer, summary)
	}

	return answer, nil
}

func (g *GitlabProvider) PullRequestLastCommitStatus(pr *GitPullRequest) (string, error) {
	owner := pr.Owner
	repo := pr.Repo

	ref := pr.LastCommitSha
	if ref == "" {
		return "", fmt.Errorf("missing String for LastCommitSha %#v", pr)
	}

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return "", err
	}
	c, _, err := g.Client.Commits.GetCommitStatuses(pid, ref, nil)
	if err != nil {
		return "", err
	}

	for _, result := range c {
		if result.Status != "" {
			return result.Status, nil
		}
	}
	return "", fmt.Errorf("could not find a status for repository %s with ref %s", pid, ref)
}

func (g *GitlabProvider) ListCommitStatus(org string, repo string, sha string) ([]*GitRepoStatus, error) {
	pid, err := g.projectId(org, g.Username, repo)
	if err != nil {
		return nil, err
	}
	c, _, err := g.Client.Commits.GetCommitStatuses(pid, sha, nil)
	if err != nil {
		return nil, err
	}

	var statuses []*GitRepoStatus

	for _, result := range c {
		statuses = append(statuses, fromCommitStatus(result))
	}

	return statuses, nil
}

// UpdateCommitStatus updates the commit status
func (g *GitlabProvider) UpdateCommitStatus(owner string, repo string, sha string, status *GitRepoStatus) (*GitRepoStatus, error) {
	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return nil, err
	}
	statusOptions := &gitlab.SetCommitStatusOptions{
		State:       gitlab.BuildStateValue(status.State),
		Name:        &status.Context,
		Context:     &status.Context,
		Description: &status.Description,
		TargetURL:   &status.TargetURL,
	}
	c, _, err := g.Client.Commits.SetCommitStatus(pid, sha, statusOptions, nil)
	if err != nil {
		return nil, err
	}
	return &GitRepoStatus{
		ID:          strconv.Itoa(c.ID),
		Description: c.Description,
		State:       c.Status,
		Context:     c.Name,
		TargetURL:   c.TargetURL,
	}, err
}

func fromCommitStatus(status *gitlab.CommitStatus) *GitRepoStatus {
	return &GitRepoStatus{
		ID:          string(status.ID),
		URL:         status.TargetURL,
		State:       status.Status,
		Description: status.Description,
	}
}

func (g *GitlabProvider) MergePullRequest(pr *GitPullRequest, message string) error {
	pid, err := g.projectId(pr.Owner, g.Username, pr.Repo)
	if err != nil {
		return err
	}

	opt := &gitlab.AcceptMergeRequestOptions{MergeCommitMessage: &message}

	_, _, err = g.Client.MergeRequests.AcceptMergeRequest(pid, *pr.Number, opt)
	return err
}

func (g *GitlabProvider) CreateWebHook(data *GitWebHookArguments) error {
	pid, err := g.projectId(data.Owner, g.Username, data.Repo.Name)
	if err != nil {
		return nil
	}

	flag := true
	owner := owner(data.Owner, g.Username)
	webhookURL := util.UrlJoin(data.URL, owner, data.Repo.Name)
	opt := &gitlab.AddProjectHookOptions{
		URL:                 &webhookURL,
		Token:               &data.Secret,
		PushEvents:          &flag,
		MergeRequestsEvents: &flag,
		IssuesEvents:        &flag,
		NoteEvents:          &flag,
	}
	_, _, err = g.Client.Projects.AddProjectHook(pid, opt)
	return err
}

// ListWebHooks lists the webhooks
func (g *GitlabProvider) ListWebHooks(owner string, repo string) ([]*GitWebHookArguments, error) {
	answer := []*GitWebHookArguments{}
	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return answer, err
	}
	opt := &gitlab.ListProjectHooksOptions{}
	hooks, _, err := g.Client.Projects.ListProjectHooks(pid, opt, nil)
	for _, hook := range hooks {
		answer = append(answer, gitLabToGitHook(owner, repo, hook))
	}
	return answer, err
}

func gitLabToGitHook(owner string, repo string, hook *gitlab.ProjectHook) *GitWebHookArguments {
	return &GitWebHookArguments{
		ID:    int64(hook.ID),
		Owner: owner,
		Repo: &GitRepository{
			Organisation: owner,
			Name:         repo,
		},
		URL: hook.URL,
	}
}

func (g *GitlabProvider) UpdateWebHook(data *GitWebHookArguments) error {
	pid, err := g.projectId(data.Owner, g.Username, data.Repo.Name)
	if err != nil {
		return nil
	}
	flag := true
	owner := owner(data.Owner, g.Username)
	webhookURL := util.UrlJoin(data.URL, owner, data.Repo.Name)
	opt := &gitlab.EditProjectHookOptions{
		URL:                 &webhookURL,
		Token:               &data.Secret,
		PushEvents:          &flag,
		MergeRequestsEvents: &flag,
		IssuesEvents:        &flag,
		NoteEvents:          &flag,
	}
	_, _, err = g.Client.Projects.EditProjectHook(pid, int(data.ID), opt)
	return err
}

func (g *GitlabProvider) SearchIssues(org, repo, query string) ([]*GitIssue, error) {
	opt := &gitlab.ListProjectIssuesOptions{Search: &query}
	return g.searchIssuesWithOptions(org, repo, opt)
}

func (g *GitlabProvider) SearchIssuesClosedSince(org string, repo string, t time.Time) ([]*GitIssue, error) {
	closed := "closed"
	opt := &gitlab.ListProjectIssuesOptions{State: &closed}
	issues, err := g.searchIssuesWithOptions(org, repo, opt)
	if err != nil {
		return issues, err
	}
	return FilterIssuesClosedSince(issues, t), nil
}

func (g *GitlabProvider) searchIssuesWithOptions(org string, repo string, opt *gitlab.ListProjectIssuesOptions) ([]*GitIssue, error) {
	pid, err := g.projectId(org, g.Username, repo)
	if err != nil {
		return nil, err
	}
	issues, _, err := g.Client.Issues.ListProjectIssues(pid, opt)
	if err != nil {
		return nil, err
	}
	return fromGitlabIssues(issues, owner(org, g.Username), repo), nil
}

func (g *GitlabProvider) GetIssue(org, repo string, number int) (*GitIssue, error) {
	owner := owner(org, g.Username)
	pid, err := g.projectId(org, g.Username, repo)
	if err != nil {
		return nil, err
	}
	issue, _, err := g.Client.Issues.GetIssue(pid, number)
	if err != nil {
		return nil, err
	}
	return fromGitlabIssue(issue, owner, repo), nil
}

func (g *GitlabProvider) CreateIssue(owner string, repo string, issue *GitIssue) (*GitIssue, error) {
	labels := []string{}
	for _, label := range issue.Labels {
		name := label.Name
		if name != "" {
			labels = append(labels, name)
		}
	}

	gitlabLabels := gitlab.Labels(labels)
	opt := &gitlab.CreateIssueOptions{
		Title:       &issue.Title,
		Description: &issue.Body,
		Labels:      &gitlabLabels,
	}

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return nil, err
	}
	gitlabIssue, _, err := g.Client.Issues.CreateIssue(pid, opt)
	if err != nil {
		return nil, err
	}

	return fromGitlabIssue(gitlabIssue, owner, repo), nil
}

func fromGitlabIssues(issues []*gitlab.Issue, owner, repo string) []*GitIssue {
	var result []*GitIssue

	for _, v := range issues {
		result = append(result, fromGitlabIssue(v, owner, repo))
	}
	return result
}

func fromGitlabIssue(issue *gitlab.Issue, owner, repo string) *GitIssue {
	var labels []GitLabel
	for _, v := range issue.Labels {
		labels = append(labels, GitLabel{Name: v})
	}

	return &GitIssue{
		Number:    &issue.IID,
		URL:       issue.WebURL,
		Owner:     owner,
		Repo:      repo,
		Title:     issue.Title,
		Body:      issue.Description,
		Labels:    labels,
		CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt,
		ClosedAt:  issue.ClosedAt,
	}
}

func (g *GitlabProvider) AddPRComment(pr *GitPullRequest, comment string) error {
	owner := pr.Owner
	repo := pr.Repo

	opt := &gitlab.CreateMergeRequestNoteOptions{Body: &comment}

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return nil
	}
	_, _, err = g.Client.Notes.CreateMergeRequestNote(pid, *pr.Number, opt)
	return err
}

func (g *GitlabProvider) CreateIssueComment(owner string, repo string, number int, comment string) error {
	opt := &gitlab.CreateIssueNoteOptions{Body: &comment}

	pid, err := g.projectId(owner, g.Username, repo)
	if err != nil {
		return err
	}
	_, _, err = g.Client.Notes.CreateIssueNote(pid, number, opt)
	return err
}

func (g *GitlabProvider) HasIssues() bool {
	return true
}

func (g *GitlabProvider) IsGitHub() bool {
	return false
}

func (g *GitlabProvider) IsGitea() bool {
	return false
}

func (g *GitlabProvider) IsBitbucketCloud() bool {
	return false
}

func (g *GitlabProvider) IsBitbucketServer() bool {
	return false
}

func (g *GitlabProvider) IsGerrit() bool {
	return false
}

func (g *GitlabProvider) Kind() string {
	return "gitlab"
}

func (g *GitlabProvider) JenkinsWebHookPath(gitURL string, secret string) string {
	return "/project"
}

func (g *GitlabProvider) Label() string {
	return g.Server.Label()
}

func (g *GitlabProvider) ServerURL() string {
	return g.Server.URL
}

func (g *GitlabProvider) BranchArchiveURL(org string, name string, branch string) string {
	return util.UrlJoin(g.ServerURL(), org, name, "-/archive", branch, name+"-"+branch+".zip")
}

func (g *GitlabProvider) CurrentUsername() string {
	return g.Username
}

func (g *GitlabProvider) UserAuth() auth.UserAuth {
	return g.User
}

func (g *GitlabProvider) UserInfo(username string) *GitUser {
	users, _, err := g.Client.Users.ListUsers(&gitlab.ListUsersOptions{Username: &username})

	if err != nil || len(users) == 0 {
		return nil
	}

	user := users[0]

	return &GitUser{
		Login:     username,
		URL:       user.WebsiteURL,
		AvatarURL: user.AvatarURL,
		Name:      user.Name,
		Email:     user.Email,
	}
}

func (g *GitlabProvider) UpdateRelease(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	return nil
}

// UpdateReleaseStatus is not supported for this git provider
func (g *GitlabProvider) UpdateReleaseStatus(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	return nil
}

// IssueURL returns the URL of the issue
func (g *GitlabProvider) IssueURL(org string, name string, number int, isPull bool) string {
	return ""
}

// AddCollaborator adds a collaborator
func (g *GitlabProvider) AddCollaborator(user string, organisation string, repo string) error {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for gitlab. Please add user: %v as a collaborator to this project.", user)
	return nil
}

// ListInvitations lists pending invites
func (g *GitlabProvider) ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for gitlab.")
	return []*github.RepositoryInvitation{}, &github.Response{}, nil
}

// AcceptInvitation accepts an invitation
func (g *GitlabProvider) AcceptInvitation(ID int64) (*github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for gitlab.")
	return &github.Response{}, nil
}

// GetContent returns the content of a file
func (g *GitlabProvider) GetContent(org string, name string, path string, ref string) (*GitFileContent, error) {
	return nil, fmt.Errorf("Getting content not supported on gitlab")
}

// ShouldForkForPullReques treturns true if we should create a personal fork of this repository
// before creating a pull request
func (g *GitlabProvider) ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool {
	// return originalOwner != username
	// TODO assuming forking doesn't work yet?
	return false
}

// GitlabAccessTokenURL returns the URL to click on to generate a personal access token for the Git provider
func GitlabAccessTokenURL(url string) string {
	return util.UrlJoin(url, "/profile/personal_access_tokens")
}

// ListCommits lists the commits for the specified repo and owner
func (g *GitlabProvider) ListCommits(owner, repo string, opt *ListCommitsArguments) ([]*GitCommit, error) {
	return nil, fmt.Errorf("Listing commits not supported on gitlab")
}

// AddLabelsToIssue adds labels to issues or pullrequests
func (g *GitlabProvider) AddLabelsToIssue(owner, repo string, number int, labels []string) error {
	log.Logger().Warnf("Adding labels not supported on gitlab yet for repo %s/%s issue %d labels %v", owner, repo, number, labels)
	return nil
}

// GetLatestRelease fetches the latest release from the git provider for org and name
func (g *GitlabProvider) GetLatestRelease(org string, name string) (*GitRelease, error) {
	// TODO
	return nil, nil
}

// UploadReleaseAsset will upload an asset to org/repo to a release with id, giving it a name, it will return the release asset from the git provider
func (g *GitlabProvider) UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error) {
	return nil, nil
}

// GetBranch returns the branch information for an owner/repo, including the commit at the tip
func (g *GitlabProvider) GetBranch(owner string, repo string, branch string) (*GitBranch, error) {
	return nil, nil
}

// GetProjects returns all the git projects in owner/repo
func (g *GitlabProvider) GetProjects(owner string, repo string) ([]GitProject, error) {
	return nil, nil
}

//ConfigureFeatures sets specific features as enabled or disabled for owner/repo
func (g *GitlabProvider) ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error) {
	return nil, nil
}

// IsWikiEnabled returns true if a wiki is enabled for owner/repo
func (g *GitlabProvider) IsWikiEnabled(owner string, repo string) (bool, error) {
	return false, nil
}
