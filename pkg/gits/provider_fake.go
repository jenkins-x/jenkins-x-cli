package gits

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const jenkinsWebhookPath = "/jenkins-webhook/"

type FakeProviderType int

const (
	GitHub FakeProviderType = iota
	Gitlab
	Gitea
	BitbucketCloud
	BitbucketServer
	Gerrit
	Fake
)

type CommitStatus string

const (
	CommitStatusPending CommitStatus = "pending"
	CommitSatusSuccess               = "success"
	CommitStatusError                = "error"
	CommitStatusFailure              = "failure"
)

var (
	// PullRequestOpen is the state a pull request is in when it is open
	PullRequestOpen = "open"
)

type FakeCommit struct {
	Commit *GitCommit
	Status CommitStatus
}

type FakePullRequest struct {
	PullRequest *GitPullRequest
	Commits     []*FakeCommit
	Comment     string
}

type FakeIssue struct {
	Issue   *GitIssue
	Comment string
}

type FakeRepository struct {
	Owner              string
	GitRepo            *GitRepository
	PullRequests       map[int]*FakePullRequest
	Issues             map[int]*FakeIssue
	Commits            []*FakeCommit
	issueCount         int
	Releases           map[string]*GitRelease
	PullRequestCounter int
	BaseDir            string
	CloneDir           string
	Projects           []GitProject
	WikiEnabled        bool
}

type FakeProvider struct {
	Server auth.AuthServer
	User   auth.UserAuth

	Organizations            []GitOrganisation
	Repositories             map[string][]*FakeRepository
	ForkedRepositories       map[string][]*FakeRepository
	Type                     FakeProviderType
	Users                    []*GitUser
	WebHooks                 []*GitWebHookArguments
	Gitter                   Gitter
	CreateRepositoryAddFiles func(dir string) error
}

func (f *FakeProvider) ListOrganisations() ([]GitOrganisation, error) {
	return f.Organizations, nil
}

func (f *FakeProvider) ListRepositories(org string) ([]*GitRepository, error) {
	repos, ok := f.Repositories[org]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", org)
	}
	gitRepos := []*GitRepository{}
	for _, repo := range repos {
		gitRepos = append(gitRepos, repo.GitRepo)
	}
	return gitRepos, nil
}

func (f *FakeProvider) CreateRepository(org string, name string, private bool) (*GitRepository, error) {
	r, err := NewFakeRepository(org, name, f.CreateRepositoryAddFiles, f.Gitter)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	r.GitRepo.Private = private
	f.Repositories[org] = append(f.Repositories[org], r)
	return r.GitRepo, nil
}

func (f *FakeProvider) GetRepository(org string, name string) (*GitRepository, error) {
	repos, ok := f.Repositories[org]
	if ok {
		for _, repo := range repos {
			if repo.GitRepo.Name == name {
				return repo.GitRepo, nil
			}
		}
	} else {
		repos, ok := f.ForkedRepositories[org]
		if ok {
			for _, repo := range repos {
				if repo.GitRepo.Name == name {
					return repo.GitRepo, nil
				}
			}
		} else {
			return nil, fmt.Errorf("organization '%s' not found", org)
		}
	}
	return nil, fmt.Errorf("repository '%s' not found within the organization '%s'", name, org)
}

func (f *FakeProvider) DeleteRepository(org string, name string) error {
	for i, repo := range f.Repositories[org] {
		if repo.GitRepo.Name == name {
			f.Repositories[org] = append(f.Repositories[org][:i], f.Repositories[org][i+1:]...)
			return nil
		}
	}
	// Now look to see if there is a fork
	return fmt.Errorf("repository '%s' not found within the organization '%s'", name, org)
}

func (f *FakeProvider) ForkRepository(originalOrg string, name string, destinationOrg string) (*GitRepository, error) {
	for _, repo := range f.Repositories[originalOrg] {
		if repo.GitRepo.Name == name {
			if destinationOrg == "" {
				destinationOrg = f.User.Username
			}
			data, err := json.Marshal(repo)
			if err != nil {
				return nil, errors.Wrapf(err, "copying %+v", data)
			}
			fork := FakeRepository{}
			err = json.Unmarshal(data, &fork)
			if err != nil {
				return nil, errors.Wrapf(err, "copying %+v", data)
			}
			fork.Owner = destinationOrg
			fork.GitRepo.Organisation = destinationOrg
			fork.GitRepo.Fork = true
			fork.GitRepo.URL = fmt.Sprintf("https://fake.git/%s/%s.git", destinationOrg, name)
			fork.GitRepo.HTMLURL = fmt.Sprintf("https://fake.git/%s/%s", destinationOrg, name)
			fork.GitRepo.Project = destinationOrg
			if fork.BaseDir != "" {
				fork.CloneDir = filepath.Join(fork.BaseDir, destinationOrg)
				err := util.CopyDir(repo.CloneDir, fork.CloneDir, true)
				if err != nil {
					return nil, errors.WithStack(err)
				}
				fork.GitRepo.CloneURL = fmt.Sprintf("file://%s", fork.CloneDir)
			} else {
				fork.GitRepo.CloneURL = fmt.Sprintf("https://fake.git/%s/%s.git", destinationOrg, name)
			}
			f.ForkedRepositories[destinationOrg] = append(f.ForkedRepositories[destinationOrg], &fork)
			return fork.GitRepo, nil
		}
	}
	return nil, fmt.Errorf("repository '%s' not found within the organization '%s'", name, originalOrg)
}

func (f *FakeProvider) RenameRepository(org string, name string, newName string) (*GitRepository, error) {
	for _, repo := range f.Repositories[org] {
		if repo.GitRepo.Name == name {
			repo.GitRepo.Name = newName
			return repo.GitRepo, nil
		}
	}
	for _, repo := range f.ForkedRepositories[org] {
		if repo.GitRepo.Name == name {
			repo.GitRepo.Name = newName
			return repo.GitRepo, nil
		}
	}
	return nil, fmt.Errorf("repository '%s' not found within the organization '%s'", name, org)
}

func (f *FakeProvider) ValidateRepositoryName(org string, name string) error {
	for _, repo := range f.Repositories[org] {
		if repo.GitRepo.Name == name {
			return nil
		}
	}
	return fmt.Errorf("repository '%s' not found within the organization '%s'", name, org)
}

func (f *FakeProvider) CreatePullRequest(data *GitPullRequestArguments) (*GitPullRequest, error) {
	org := data.GitRepository.Organisation
	repoName := data.GitRepository.Name
	repos, ok := f.Repositories[org]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", org)
	}

	var repo *FakeRepository
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			repo = r
		}
	}
	if repo == nil {
		return nil, fmt.Errorf("repository '%s' not found", repoName)
	}

	repo.issueCount += 1
	number := repo.issueCount
	labels := make([]*Label, 0)

	for _, l := range data.Labels {
		labelName := l
		labels = append(labels, &Label{
			ID:          nil,
			URL:         nil,
			Name:        &labelName,
			Color:       nil,
			Description: nil,
			Default:     nil,
		})
	}
	pr := &GitPullRequest{
		URL: fmt.Sprintf("https://fake.git/%s/%s/pulls/%d", org, repoName, number),
		Author: &GitUser{
			URL:       "",
			Login:     "",
			Name:      "",
			Email:     "",
			AvatarURL: "",
		},
		Owner:          org,
		Repo:           repoName,
		Number:         &number,
		Mergeable:      nil,
		Merged:         nil,
		HeadRef:        &data.Head,
		State:          &PullRequestOpen,
		StatusesURL:    nil,
		IssueURL:       nil,
		DiffURL:        nil,
		MergeCommitSHA: nil,
		ClosedAt:       nil,
		MergedAt:       nil,
		Labels:         labels,
		LastCommitSha:  "",
		Title:          data.Title,
		Body:           data.Body,
	}

	fakePr := &FakePullRequest{
		PullRequest: pr,
		Comment:     data.Title,
	}
	// If there is a change in the SHA then create a fake PR
	if data.Head != data.Base {
		fakePr.Commits = []*FakeCommit{
			{
				Status: CommitStatusPending,
				Commit: &GitCommit{
					URL:     fmt.Sprintf("https://fake.git/%s/%s/commits/%s", org, repoName, data.Head),
					SHA:     data.Head,
					Message: data.Title,
				},
			},
		}
	}
	repo.PullRequests[number] = fakePr
	return pr, nil
}

// UpdatePullRequest updates the pull request number with the new data
func (f *FakeProvider) UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error) {
	return nil, errors.Errorf("Not yet implemented for fake")
}

func (f *FakeProvider) UpdatePullRequestStatus(pr *GitPullRequest) error {
	owner := pr.Owner
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("no repositories for owner '%s'", owner)
	}
	repoName := pr.Repo
	number := *pr.Number
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			prFound, ok := r.PullRequests[number]
			merged := true
			if !ok {
				// PR not found, assume it was already merged
				sha := r.Commits[0].Commit.SHA // let's pretend the last commit in the repo is the merge commit sha
				pr.MergeCommitSHA = &sha
				pr.Merged = &merged
				return nil
			}
			// PR found, check if it's merged but does not have a merge commit, then set it
			if prFound.PullRequest.Merged != nil && *prFound.PullRequest.Merged &&
				(prFound.PullRequest.MergeCommitSHA == nil || len(*prFound.PullRequest.MergeCommitSHA) == 0) {
				pr.MergeCommitSHA = &prFound.PullRequest.LastCommitSha
				return nil
			}

			// PR is there, and it's not merged, no action required
			return nil
		}
	}
	return fmt.Errorf("no repository '%s' found for owner '%s'", repoName, owner)
}

func (f *FakeProvider) GetPullRequest(owner string, repo *GitRepository, number int) (*GitPullRequest, error) {
	repos, ok := f.Repositories[owner]
	if !ok {
		return nil, fmt.Errorf("no repositories found for '%s'", owner)
	}
	repoName := repo.Name
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			pr, ok := r.PullRequests[number]
			if !ok {
				return nil, fmt.Errorf("pull request with id '%d' not found", number)
			}
			return pr.PullRequest, nil
		}
	}

	return nil, fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error) {
	answer := make([]*GitPullRequest, 0)
	repos, ok := f.Repositories[owner]
	if !ok {
		return nil, fmt.Errorf("no repositories found for '%s'", owner)
	}
	for _, r := range repos {
		if r.GitRepo.Name == repo {
			for _, pr := range r.PullRequests {
				if util.DereferenceString(pr.PullRequest.State) == PullRequestOpen {
					answer = append(answer, pr.PullRequest)
				}
			}
		}
	}
	return answer, nil
}

func (f *FakeProvider) GetPullRequestCommits(owner string, repo *GitRepository, number int) ([]*GitCommit, error) {
	repos, ok := f.Repositories[owner]
	if !ok {
		return nil, fmt.Errorf("no repositories found for '%s'", owner)
	}
	repoName := repo.Name
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			pr, ok := r.PullRequests[number]
			if !ok {
				return nil, fmt.Errorf("pull request with id '%d' not found", number)
			}
			commits := []*GitCommit{}
			for _, commit := range pr.Commits {
				commits = append(commits, commit.Commit)
			}
			return commits, nil
		}
	}
	return nil, fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) PullRequestLastCommitStatus(pr *GitPullRequest) (string, error) {
	owner := pr.Owner
	repos, ok := f.Repositories[owner]
	if !ok {
		return "", fmt.Errorf("no repositories found for '%s'", owner)
	}
	repoName := pr.Repo
	number := *pr.Number
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			pr, ok := r.PullRequests[number]
			if !ok {
				return "", fmt.Errorf("pull request with id '%d' not found", number)
			}
			len := len(pr.Commits)
			if len < 1 {
				return "", errors.New("pull request has no commits")
			}
			lastCommit := pr.Commits[len-1]
			return string(lastCommit.Status), nil
		}
	}
	return "", fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) ListCommitStatus(org string, repoName string, sha string) ([]*GitRepoStatus, error) {
	repos, ok := f.Repositories[org]
	if !ok {
		return nil, fmt.Errorf("no repositories found for '%s'", org)
	}
	var repo *FakeRepository
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			repo = r
		}
	}

	if repo == nil {
		return nil, fmt.Errorf("repository with name '%s' not found", repoName)
	}

	answer := []*GitRepoStatus{}
	for _, commit := range repo.Commits {
		if commit.Commit.SHA == sha {
			status := &GitRepoStatus{
				ID:          commit.Commit.SHA,
				URL:         commit.Commit.URL,
				State:       string(commit.Status),
				Description: commit.Commit.Message,
			}
			answer = append(answer, status)
		}
	}
	return answer, nil
}

func (f *FakeProvider) UpdateCommitStatus(org string, repo string, sha string, status *GitRepoStatus) (*GitRepoStatus, error) {
	repoStatus, err := f.ListCommitStatus(org, repo, sha)
	if err != nil {
		return &GitRepoStatus{}, err
	}
	updated := false
	for i, s := range repoStatus {
		if s.ID == status.ID {
			repoStatus[i] = status
			updated = true
		}
	}
	if !updated {
		repoStatus = append(repoStatus, status)
	}
	return status, nil

}

func (f *FakeProvider) MergePullRequest(pr *GitPullRequest, message string) error {
	owner := pr.Owner
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("no repositories found for '%s'", owner)
	}
	repoName := pr.Repo
	number := *pr.Number
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			fakePR, ok := r.PullRequests[number]
			if !ok {
				return fmt.Errorf("pull request with id '%d' not found", number)
			}
			// make sure the commit goes to the repo
			l := len(fakePR.Commits)
			lastCommit := fakePR.Commits[l-1]
			if len(r.Commits) == 0 {
				r.Commits = append(r.Commits, lastCommit)
			} else {
				r.Commits[len(r.Commits)-1] = lastCommit
			}
			delete(r.PullRequests, number)
			return nil
		}
	}
	return fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) CreateWebHook(data *GitWebHookArguments) error {
	f.WebHooks = append(f.WebHooks, data)
	return nil
}

func (p *FakeProvider) ListWebHooks(owner string, repo string) ([]*GitWebHookArguments, error) {
	return p.WebHooks, nil
}

func (p *FakeProvider) UpdateWebHook(data *GitWebHookArguments) error {
	return fmt.Errorf("not implemented!")
}

func (f *FakeProvider) IsGitHub() bool {
	return f.Type == GitHub
}

func (f *FakeProvider) IsGitea() bool {
	return f.Type == Gitea
}

func (f *FakeProvider) IsBitbucketCloud() bool {
	return f.Type == BitbucketCloud
}

func (f *FakeProvider) IsBitbucketServer() bool {
	return f.Type == BitbucketServer
}

func (f *FakeProvider) IsGerrit() bool {
	return f.Type == Gerrit
}

func (f *FakeProvider) Kind() string {
	switch f.Type {
	case GitHub:
		return KindGitHub
	case Gitlab:
		return KindGitlab
	case Gitea:
		return KindGitea
	case BitbucketCloud:
		return KindBitBucketCloud
	case BitbucketServer:
		return KindBitBucketServer
	case Fake:
		return KindGitFake
	default:
		return KindUnknown
	}
}

func (f *FakeProvider) GetIssue(org string, name string, number int) (*GitIssue, error) {
	repos, ok := f.Repositories[org]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", org)
	}
	for _, repo := range repos {
		if repo.GitRepo.Name == name {
			issue, ok := repo.Issues[number]
			if !ok {
				return nil, fmt.Errorf("no issue found with ID '%d'", number)
			}
			return issue.Issue, nil
		}
	}
	return nil, fmt.Errorf("no issue found with name '%s'", name)
}

func (f *FakeProvider) IssueURL(org string, name string, number int, isPull bool) string {
	serverPrefix := f.Server.URL
	if !strings.HasPrefix(serverPrefix, "https://") {
		serverPrefix = "https://" + serverPrefix
	}
	path := "issues"
	if isPull {
		path = "pull"
	}
	url := util.UrlJoin(serverPrefix, org, name, path, strconv.Itoa(number))
	return url
}

func (f *FakeProvider) SearchIssues(org string, name string, query string) ([]*GitIssue, error) {
	repos, ok := f.Repositories[org]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", org)
	}
	for _, repo := range repos {
		if repo.GitRepo.Name == name {
			answer := []*GitIssue{}
			for _, issue := range repo.Issues {
				if query == "" || query == util.DereferenceString(issue.Issue.State) {
					answer = append(answer, issue.Issue)
				}

			}
			return answer, nil
		}
	}
	return nil, fmt.Errorf("repository with name '%s' not found", name)
}

func (f *FakeProvider) SearchIssuesClosedSince(org string, name string, t time.Time) ([]*GitIssue, error) {
	issues, err := f.SearchIssues(org, name, "")
	if err != nil {
		return nil, err
	}

	answer := []*GitIssue{}
	for _, issue := range issues {
		closedTime := issue.ClosedAt
		if closedTime.After(t) {
			answer = append(answer, issue)
		}
	}
	return answer, nil
}

func (f *FakeProvider) CreateIssue(owner string, repoName string, issue *GitIssue) (*GitIssue, error) {
	repos, ok := f.Repositories[owner]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", owner)
	}
	for _, repo := range repos {
		if repo.GitRepo.Name == repoName {
			repo.issueCount += 1
			number := repo.issueCount
			issue.Number = &number
			newIssue := &FakeIssue{
				Issue:   issue,
				Comment: "",
			}
			repo.Issues[number] = newIssue
			return issue, nil
		}
	}
	return nil, fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) HasIssues() bool {
	return true
}

func (f *FakeProvider) AddPRComment(pr *GitPullRequest, comment string) error {
	owner := pr.Owner
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("no repositories found for '%s'", owner)
	}
	repoName := pr.Repo
	number := *pr.Number
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			pr, ok := r.PullRequests[number]
			if !ok {
				return fmt.Errorf("pull request with id '%d' not found", number)
			}
			pr.Comment = comment
			return nil
		}
	}
	return fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) CreateIssueComment(owner string, repoName string, number int, comment string) error {
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("no repositories found for '%s'", owner)
	}
	for _, r := range repos {
		if r.GitRepo.Name == repoName {
			issue, ok := r.Issues[number]
			if !ok {
				return fmt.Errorf("issue with id '%d' not found", number)
			}
			issue.Comment = comment
			return nil
		}
	}
	return fmt.Errorf("repository with name '%s' not found", repoName)
}

func (f *FakeProvider) UpdateRelease(owner string, repoName string, tag string, releaseInfo *GitRelease) error {
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("organization '%s' not found", owner)
	}

	for _, repo := range repos {
		if repo.GitRepo.Name == repoName {
			repo.Releases[tag] = releaseInfo
			return nil
		}
	}
	return fmt.Errorf("repository with name '%s' not found", repoName)
}

// UpdateReleaseStatus updates the status (release/prerelease) of a release
func (f *FakeProvider) UpdateReleaseStatus(owner string, repoName string, tag string, releaseInfo *GitRelease) error {
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("organization '%s' not found", owner)
	}

	for _, repo := range repos {
		if repo.GitRepo.Name == repoName {
			repo.Releases[tag] = releaseInfo
			return nil
		}
	}
	return fmt.Errorf("repository with name '%s' not found", repoName)
}
func (f *FakeProvider) ListReleases(org string, name string) ([]*GitRelease, error) {
	repos, ok := f.Repositories[org]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", org)
	}

	for _, repo := range repos {
		if repo.GitRepo.Name == name {
			releases := []*GitRelease{}
			for _, release := range repo.Releases {
				releases = append(releases, release)
			}
			return releases, nil
		}
	}
	return nil, fmt.Errorf("repository with name '%s' not found", name)
}

// GetRelease returns the release info for the org, repository name and tag, or nil if no release is found
func (f *FakeProvider) GetRelease(org string, name string, tag string) (*GitRelease, error) {
	releases, err := f.ListReleases(org, name)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	for _, release := range releases {
		if release.TagName == tag {
			return release, nil
		}
	}
	return nil, nil
}

func (f *FakeProvider) JenkinsWebHookPath(gitURL string, secret string) string {
	return jenkinsWebhookPath
}

func (f *FakeProvider) Label() string {
	return f.Server.Label()
}

func (f *FakeProvider) ServerURL() string {
	return f.Server.URL
}

func (f *FakeProvider) BranchArchiveURL(org string, name string, branch string) string {
	return util.UrlJoin(f.ServerURL(), org, name, "archive", branch+".zip")
}

func (f *FakeProvider) CurrentUsername() string {
	return f.User.Username
}

func (f *FakeProvider) UserAuth() auth.UserAuth {
	return f.User
}

func (f *FakeProvider) UserInfo(username string) *GitUser {
	for _, user := range f.Users {
		if user.Login == username {
			return user
		}
	}
	return nil
}

func (f *FakeProvider) AddCollaborator(user string, organisation string, repo string) error {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for git fake. Please add user: %v as a collaborator to this project.", user)
	return nil
}

func (f *FakeProvider) ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for git fake.")
	return []*github.RepositoryInvitation{}, &github.Response{}, nil
}

func (f *FakeProvider) AcceptInvitation(ID int64) (*github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for git fake.")
	return &github.Response{}, nil
}

// GetContent gets the content
func (f *FakeProvider) GetContent(org string, name string, path string, ref string) (*GitFileContent, error) {
	return nil, nil
}

// ShouldForkForPullReques treturns true if we should create a personal fork of this repository
// before creating a pull request
func (f *FakeProvider) ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool {
	// Simple algorithm to always ask for a fork if the username is not the same as the CurrentUsername
	return originalOwner != f.CurrentUsername()
}

// GetProjects returns all the git projects in owner/repo
func (f *FakeProvider) GetProjects(owner string, repo string) ([]GitProject, error) {
	if repos, ok := f.Repositories[owner]; ok {
		for _, r := range repos {
			if r.Name() == repo {
				return r.Projects, nil
			}
		}
	}
	return nil, nil
}

//ConfigureFeatures sets specific features as enabled or disabled for owner/repo
func (f *FakeProvider) ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error) {
	if repos, ok := f.Repositories[owner]; ok {
		for _, r := range repos {
			if r.Name() == repo {
				if issues != nil {
					r.GitRepo.HasIssues = util.DereferenceBool(issues)
				}
				if projects != nil {
					r.GitRepo.HasProjects = util.DereferenceBool(projects)
				}
				if wikis != nil {
					r.GitRepo.HasWiki = util.DereferenceBool(wikis)
				}
			}
			return r.GitRepo, nil
		}
	}
	return nil, errors.Errorf("unable to find %s/%s", owner, repo)
}

// IsWikiEnabled returns true if a wiki is enabled for owner/repo
func (f *FakeProvider) IsWikiEnabled(owner string, repo string) (bool, error) {
	if repos, ok := f.Repositories[owner]; ok {
		for _, r := range repos {
			if r.Name() == repo {
				return r.WikiEnabled, nil
			}
		}
	}
	return false, nil
}

func (r *FakeRepository) String() string {
	return r.Owner + "/" + r.Name()
}

func (r *FakeRepository) Name() string {
	return r.GitRepo.Name
}

// NewFakeRepository creates a new fake repository
func NewFakeRepository(owner string, repoName string, addFiles func(dir string) error, gitter Gitter) (*FakeRepository, error) {
	repo := FakeRepository{
		Owner: owner,
		GitRepo: &GitRepository{
			Name:         repoName,
			CloneURL:     "https://fake.git/" + owner + "/" + repoName + ".git",
			HTMLURL:      "https://fake.git/" + owner + "/" + repoName,
			URL:          "https://fake.git/" + owner + "/" + repoName + ".git",
			Scheme:       "https",
			Host:         "fake.git",
			Organisation: owner,
		},
		PullRequests: map[int]*FakePullRequest{},
		Commits:      []*FakeCommit{},
		Releases:     make(map[string]*GitRelease),
	}
	if addFiles != nil && gitter != nil {
		dir, err := ioutil.TempDir("", "")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		cloneDir := filepath.Join(dir, owner)
		err = os.MkdirAll(cloneDir, 0755)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		err = gitter.Init(cloneDir)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		err = gitter.SetRemoteURL(cloneDir, "origin", repo.GitRepo.CloneURL)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		err = addFiles(cloneDir)
		if err != nil {
			return nil, errors.Wrapf(err, "adding files to %s", dir)
		}
		err = gitter.Add(cloneDir, "-A")
		if err != nil {
			return nil, errors.WithStack(err)
		}

		err = gitter.CommitDir(cloneDir, "Initial Commit")

		if err != nil {
			return nil, errors.WithStack(err)
		}
		// Now we need to detach ourselves from master to allow others to push
		err = gitter.Checkout(cloneDir, "--detach")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		repo.BaseDir = dir
		repo.CloneDir = cloneDir
		repo.GitRepo.CloneURL = fmt.Sprintf("file://%s", cloneDir)
	}
	return &repo, nil
}

// NewFakeRepository creates a new fake repository
func NewFakeProvider(repositories ...*FakeRepository) *FakeProvider {
	provider := &FakeProvider{
		Repositories:       map[string][]*FakeRepository{},
		ForkedRepositories: map[string][]*FakeRepository{},
		Server: auth.AuthServer{
			URL: FakeGitURL,
		},
	}
	for _, repo := range repositories {
		owner := repo.Owner
		if provider.User.Username == "" {
			provider.User.Username = owner
		}
		if provider.User.ApiToken == "" {
			provider.User.ApiToken = "fake"
		}
		if owner == "" {
			log.Logger().Warnf("Missing owner for Repository %s", repo.Name())
		}
		s := append(provider.Repositories[owner], repo)
		provider.Repositories[owner] = s
	}
	return provider
}

// ListCommits returns the list of commits in the master brach only (TODO: read opt param to apply to other branches)
func (f *FakeProvider) ListCommits(owner, name string, opt *ListCommitsArguments) ([]*GitCommit, error) {
	repos, ok := f.Repositories[owner]
	if !ok {
		return nil, fmt.Errorf("organization '%s' not found", owner)
	}
	var repo *FakeRepository
	for _, r := range repos {
		if r.GitRepo.Name == name {
			repo = r
			break
		}
	}
	if repo == nil {
		return nil, fmt.Errorf("repository with name '%s' not found", name)
	}

	commits := []*GitCommit{}
	for _, c := range repo.Commits {
		commits = append(commits, c.Commit)
	}
	return commits, nil

}

// AddLabelsToIssue adds labels to an issue
func (f *FakeProvider) AddLabelsToIssue(owner, repo string, number int, labels []string) error {
	repos, ok := f.Repositories[owner]
	if !ok {
		return fmt.Errorf("no repositories found for '%s'", owner)
	}
	for _, r := range repos {

		if r.GitRepo.Name == repo {
			for _, pr := range r.PullRequests {
				if util.DereferenceInt(pr.PullRequest.Number) == number {
					ls := make([]*Label, 0)
					for _, l := range labels {
						labelName := l
						ls = append(ls, &Label{
							Name: &labelName,
						})
					}
					pr.PullRequest.Labels = ls
					break
				}
			}
			break
		}
	}
	return nil
}

// GetLatestRelease fetches the latest release from the git provider for org and name
func (f *FakeProvider) GetLatestRelease(org string, name string) (*GitRelease, error) {
	releases, err := f.ListReleases(org, name)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if len(releases) == 0 {
		return nil, errors.Errorf("%s/%s has no releases", org, name)
	}
	return releases[len(releases)-1], nil
}

// UploadReleaseAsset will upload an asset to org/repo to a release with id, giving it a name, it will return the release asset from the git provider
func (f *FakeProvider) UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error) {
	return nil, nil
}

// GetBranch returns the branch information for an owner/repo, including the commit at the tip
func (f *FakeProvider) GetBranch(owner string, repo string, branch string) (*GitBranch, error) {
	return nil, nil
}
