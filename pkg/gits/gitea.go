package gits

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	errors2 "github.com/pkg/errors"

	"code.gitea.io/sdk/gitea"
	"github.com/google/go-github/github"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

type GiteaProvider struct {
	Username string
	Client   *gitea.Client

	Server auth.AuthServer
	User   auth.UserAuth
	Git    Gitter
}

func NewGiteaProvider(server *auth.AuthServer, user *auth.UserAuth, git Gitter) (GitProvider, error) {
	client := gitea.NewClient(server.URL, user.ApiToken)

	provider := GiteaProvider{
		Client:   client,
		Server:   *server,
		User:     *user,
		Username: user.Username,
		Git:      git,
	}

	return &provider, nil
}

func (p *GiteaProvider) ListOrganisations() ([]GitOrganisation, error) {
	answer := []GitOrganisation{}
	orgs, err := p.Client.ListMyOrgs()
	if err != nil {
		return answer, err
	}

	for _, org := range orgs {
		name := org.UserName
		if name != "" {
			o := GitOrganisation{
				Login: name,
			}
			answer = append(answer, o)
		}
	}
	return answer, nil
}

func (p *GiteaProvider) ListRepositories(org string) ([]*GitRepository, error) {
	answer := []*GitRepository{}
	if org == "" {
		repos, err := p.Client.ListMyRepos()
		if err != nil {
			return answer, err
		}
		for _, repo := range repos {
			answer = append(answer, toGiteaRepo(repo.Name, repo))
		}
		return answer, nil
	}
	repos, err := p.Client.ListOrgRepos(org)
	if err != nil {
		return answer, err
	}
	for _, repo := range repos {
		answer = append(answer, toGiteaRepo(repo.Name, repo))
	}
	return answer, nil
}

func (p *GiteaProvider) ListReleases(org string, name string) ([]*GitRelease, error) {
	owner := org
	if owner == "" {
		owner = p.Username
	}
	answer := []*GitRelease{}
	repos, err := p.Client.ListReleases(owner, name)
	if err != nil {
		return answer, err
	}
	for _, repo := range repos {
		answer = append(answer, toGiteaRelease(org, name, repo))
	}
	return answer, nil
}

// GetRelease returns the release info for org, repo name and tag
func (p *GiteaProvider) GetRelease(org string, name string, tag string) (*GitRelease, error) {
	releases, err := p.ListReleases(org, name)
	if err != nil {
		return nil, errors2.WithStack(err)
	}
	for _, release := range releases {
		if release.TagName == tag {
			return release, nil
		}
	}
	return nil, nil
}

func toGiteaRelease(org string, name string, release *gitea.Release) *GitRelease {
	totalDownloadCount := 0
	assets := make([]GitReleaseAsset, 0)
	for _, asset := range release.Attachments {
		totalDownloadCount = totalDownloadCount + int(asset.DownloadCount)
		assets = append(assets, GitReleaseAsset{
			Name:               asset.Name,
			BrowserDownloadURL: asset.DownloadURL,
		})
	}
	return &GitRelease{
		Name:          release.Title,
		TagName:       release.TagName,
		Body:          release.Note,
		URL:           release.URL,
		HTMLURL:       release.URL,
		DownloadCount: totalDownloadCount,
		Assets:        &assets,
	}
}

func (p *GiteaProvider) CreateRepository(org string, name string, private bool) (*GitRepository, error) {
	options := gitea.CreateRepoOption{
		Name:    name,
		Private: private,
	}
	repo, err := p.Client.CreateRepo(options)
	if err != nil {
		return nil, fmt.Errorf("Failed to create repository %s/%s due to: %s", org, name, err)
	}
	return toGiteaRepo(name, repo), nil
}

func (p *GiteaProvider) GetRepository(org string, name string) (*GitRepository, error) {
	repo, err := p.Client.GetRepo(org, name)
	if err != nil {
		return nil, fmt.Errorf("Failed to get repository %s/%s due to: %s", org, name, err)
	}
	return toGiteaRepo(name, repo), nil
}

func (p *GiteaProvider) DeleteRepository(org string, name string) error {
	owner := org
	if owner == "" {
		owner = p.Username
	}
	err := p.Client.DeleteRepo(owner, name)
	if err != nil {
		return fmt.Errorf("Failed to delete repository %s/%s due to: %s", owner, name, err)
	}
	return err
}

func toGiteaRepo(name string, repo *gitea.Repository) *GitRepository {
	return &GitRepository{
		Name:             name,
		AllowMergeCommit: true,
		CloneURL:         repo.CloneURL,
		HTMLURL:          repo.HTMLURL,
		SSHURL:           repo.SSHURL,
		Fork:             repo.Fork,
	}
}

func (p *GiteaProvider) ForkRepository(originalOrg string, name string, destinationOrg string) (*GitRepository, error) {
	repoConfig := gitea.CreateForkOption{
		Organization: &destinationOrg,
	}
	repo, err := p.Client.CreateFork(originalOrg, name, repoConfig)
	if err != nil {
		msg := ""
		if destinationOrg != "" {
			msg = fmt.Sprintf(" to %s", destinationOrg)
		}
		owner := destinationOrg
		if owner == "" {
			owner = p.Username
		}
		if strings.Contains(err.Error(), "try again later") {
			log.Logger().Warnf("Waiting for the fork of %s/%s to appear...", owner, name)
			// lets wait for the fork to occur...
			start := time.Now()
			deadline := start.Add(time.Minute)
			for {
				time.Sleep(5 * time.Second)
				repo, err = p.Client.GetRepo(owner, name)
				if repo != nil && err == nil {
					break
				}
				t := time.Now()
				if t.After(deadline) {
					return nil, fmt.Errorf("Gave up waiting for Repository %s/%s to appear: %s", owner, name, err)
				}
			}
		} else {
			return nil, fmt.Errorf("Failed to fork repository %s/%s%s due to: %s", originalOrg, name, msg, err)
		}
	}
	return toGiteaRepo(name, repo), nil
}

func (p *GiteaProvider) CreateWebHook(data *GitWebHookArguments) error {
	owner := data.Owner
	if owner == "" {
		owner = p.Username
	}
	repo := data.Repo.Name
	if repo == "" {
		return fmt.Errorf("Missing property Repo")
	}
	webhookUrl := data.URL
	if repo == "" {
		return fmt.Errorf("Missing property URL")
	}
	hooks, err := p.Client.ListRepoHooks(owner, repo)
	if err != nil {
		return err
	}
	for _, hook := range hooks {
		s := hook.Config["url"]
		if s == webhookUrl {
			log.Logger().Warnf("Already has a webhook registered for %s", webhookUrl)
			return nil
		}
	}
	config := map[string]string{
		"url":          webhookUrl,
		"content_type": "json",
	}
	if data.Secret != "" {
		config["secret"] = data.Secret
	}
	hook := gitea.CreateHookOption{
		Type:   "gitea",
		Config: config,
		Events: []string{"create", "push", "pull_request"},
		Active: true,
	}
	log.Logger().Infof("Creating Gitea webhook for %s/%s for url %s", util.ColorInfo(owner), util.ColorInfo(repo), util.ColorInfo(webhookUrl))
	_, err = p.Client.CreateRepoHook(owner, repo, hook)
	if err != nil {
		return fmt.Errorf("Failed to create webhook for %s/%s with %#v due to: %s", owner, repo, hook, err)
	}
	return err
}

func (p *GiteaProvider) ListWebHooks(owner string, repo string) ([]*GitWebHookArguments, error) {
	webHooks := []*GitWebHookArguments{}
	return webHooks, fmt.Errorf("not implemented!")
}

func (p *GiteaProvider) UpdateWebHook(data *GitWebHookArguments) error {
	return fmt.Errorf("not implemented!")
}

func (p *GiteaProvider) CreatePullRequest(data *GitPullRequestArguments) (*GitPullRequest, error) {
	owner := data.GitRepository.Organisation
	repo := data.GitRepository.Name
	title := data.Title
	body := data.Body
	head := data.Head
	base := data.Base
	config := gitea.CreatePullRequestOption{}
	if title != "" {
		config.Title = title
	}
	if body != "" {
		config.Body = body
	}
	if head != "" {
		config.Head = head
	}
	if base != "" {
		config.Base = base
	}
	pr, err := p.Client.CreatePullRequest(owner, repo, config)
	if err != nil {
		return nil, err
	}
	id := int(pr.Index)
	answer := &GitPullRequest{
		URL:    pr.HTMLURL,
		Number: &id,
		Owner:  data.GitRepository.Organisation,
		Repo:   data.GitRepository.Name,
	}
	if pr.Head != nil {
		answer.LastCommitSha = pr.Head.Sha
	}
	return answer, nil
}

// UpdatePullRequest updates pull request with number using data
func (p *GiteaProvider) UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error) {
	return nil, errors2.Errorf("Not yet implemented for gitea")
}

func (p *GiteaProvider) UpdatePullRequestStatus(pr *GitPullRequest) error {
	if pr.Number == nil {
		return fmt.Errorf("Missing Number for GitPullRequest %#v", pr)
	}
	n := *pr.Number
	result, err := p.Client.GetPullRequest(pr.Owner, pr.Repo, int64(n))
	if err != nil {
		return fmt.Errorf("Could not find pull request for %s/%s #%d: %s", pr.Owner, pr.Repo, n, err)
	}
	p.updatePullRequest(pr, result)
	return nil
}

// updatePullRequest updates the pr with the data from Gitea
func (p *GiteaProvider) updatePullRequest(pr *GitPullRequest, source *gitea.PullRequest) {
	pr.Author = &GitUser{
		Login: source.Poster.UserName,
	}
	merged := source.HasMerged
	pr.Merged = &merged
	pr.Mergeable = &source.Mergeable
	pr.MergedAt = source.Merged
	pr.MergeCommitSHA = source.MergedCommitID
	pr.Title = source.Title
	pr.Body = source.Body
	stateText := string(source.State)
	pr.State = &stateText
	head := source.Head
	if head != nil {
		pr.LastCommitSha = head.Sha
	} else {
		pr.LastCommitSha = ""
	}
	/*
		TODO

		pr.ClosedAt = source.Closed
		pr.StatusesURL = source.StatusesURL
		pr.IssueURL = source.IssueURL
		pr.DiffURL = source.DiffURL
	*/
}

func (p *GiteaProvider) toPullRequest(owner string, repo string, pr *gitea.PullRequest) *GitPullRequest {
	id := int(pr.Index)
	answer := &GitPullRequest{
		URL:    pr.URL,
		Owner:  owner,
		Repo:   repo,
		Number: &id,
	}
	p.updatePullRequest(answer, pr)
	return answer
}

// ListOpenPullRequests lists the open pull requests
func (p *GiteaProvider) ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error) {
	opt := gitea.ListPullRequestsOptions{}
	answer := []*GitPullRequest{}
	for {
		prs, err := p.Client.ListRepoPullRequests(owner, repo, opt)
		if err != nil {
			return answer, err
		}
		for _, pr := range prs {
			answer = append(answer, p.toPullRequest(owner, repo, pr))
		}
		if len(prs) < pageSize || len(prs) == 0 {
			break
		}
		opt.Page += 1
	}
	return answer, nil
}

func (p *GiteaProvider) GetPullRequest(owner string, repo *GitRepository, number int) (*GitPullRequest, error) {
	pr := &GitPullRequest{
		Owner:  owner,
		Repo:   repo.Name,
		Number: &number,
	}
	err := p.UpdatePullRequestStatus(pr)
	return pr, err
}

func (p *GiteaProvider) GetPullRequestCommits(owner string, repository *GitRepository, number int) ([]*GitCommit, error) {
	answer := []*GitCommit{}

	// TODO there does not seem to be any way to get a diff of commits
	// unless maybe checking out the repo (do we have access to a local copy?)
	// there is a pr.Base and pr.Head that might be able to compare to get
	// commits somehow, but does not look like anything through the api

	return answer, nil
}

func (p *GiteaProvider) GetIssue(org string, name string, number int) (*GitIssue, error) {
	i, err := p.Client.GetIssue(org, name, int64(number))
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return nil, nil
		}
		return nil, err
	}
	return p.fromGiteaIssue(org, name, i)
}

func (p *GiteaProvider) IssueURL(org string, name string, number int, isPull bool) string {
	serverPrefix := p.Server.URL
	if strings.Index(serverPrefix, "://") < 0 {
		serverPrefix = "https://" + serverPrefix
	}
	path := "issues"
	if isPull {
		path = "pull"
	}
	url := util.UrlJoin(serverPrefix, org, name, path, strconv.Itoa(number))
	return url
}

func (p *GiteaProvider) SearchIssues(org string, name string, filter string) ([]*GitIssue, error) {
	opts := gitea.ListIssueOption{}
	// TODO apply the filter?
	return p.searchIssuesWithOptions(org, name, opts)
}

func (p *GiteaProvider) SearchIssuesClosedSince(org string, name string, t time.Time) ([]*GitIssue, error) {
	opts := gitea.ListIssueOption{}
	issues, err := p.searchIssuesWithOptions(org, name, opts)
	if err != nil {
		return issues, err
	}
	return FilterIssuesClosedSince(issues, t), nil
}

func (p *GiteaProvider) searchIssuesWithOptions(org string, name string, opts gitea.ListIssueOption) ([]*GitIssue, error) {
	opts.Page = 0
	answer := []*GitIssue{}
	issues, err := p.Client.ListRepoIssues(org, name, opts)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return answer, nil
		}
		return answer, err
	}
	for _, issue := range issues {
		i, err := p.fromGiteaIssue(org, name, issue)
		if err != nil {
			return answer, err
		}
		answer = append(answer, i)
	}
	return answer, nil
}

func (p *GiteaProvider) fromGiteaIssue(org string, name string, i *gitea.Issue) (*GitIssue, error) {
	state := string(i.State)
	labels := []GitLabel{}
	for _, label := range i.Labels {
		labels = append(labels, toGiteaLabel(label))
	}
	assignees := []GitUser{}
	assignee := i.Assignee
	if assignee != nil {
		assignees = append(assignees, *toGiteaUser(assignee))
	}
	number := int(i.ID)
	return &GitIssue{
		Number:        &number,
		URL:           p.IssueURL(org, name, number, false),
		State:         &state,
		Title:         i.Title,
		Body:          i.Body,
		IsPullRequest: i.PullRequest != nil,
		Labels:        labels,
		User:          toGiteaUser(i.Poster),
		Assignees:     assignees,
		CreatedAt:     &i.Created,
		UpdatedAt:     &i.Updated,
		ClosedAt:      i.Closed,
	}, nil
}

func (p *GiteaProvider) CreateIssue(owner string, repo string, issue *GitIssue) (*GitIssue, error) {
	config := gitea.CreateIssueOption{
		Title: issue.Title,
		Body:  issue.Body,
	}
	i, err := p.Client.CreateIssue(owner, repo, config)
	if err != nil {
		return nil, err
	}
	return p.fromGiteaIssue(owner, repo, i)
}

func toGiteaLabel(label *gitea.Label) GitLabel {
	return GitLabel{
		Name:  label.Name,
		Color: label.Color,
		URL:   label.URL,
	}
}

func toGiteaUser(user *gitea.User) *GitUser {
	return &GitUser{
		Login:     user.UserName,
		Name:      user.FullName,
		Email:     user.Email,
		AvatarURL: user.AvatarURL,
	}
}

func (p *GiteaProvider) MergePullRequest(pr *GitPullRequest, message string) error {
	if pr.Number == nil {
		return fmt.Errorf("Missing Number for GitPullRequest %#v", pr)
	}
	n := *pr.Number
	return p.Client.MergePullRequest(pr.Owner, pr.Repo, int64(n))
}

func (p *GiteaProvider) PullRequestLastCommitStatus(pr *GitPullRequest) (string, error) {
	ref := pr.LastCommitSha
	if ref == "" {
		return "", fmt.Errorf("Missing String for LastCommitSha %#v", pr)
	}
	results, err := p.Client.ListStatuses(pr.Owner, pr.Repo, ref, gitea.ListStatusesOption{})
	if err != nil {
		return "", err
	}
	for _, result := range results {
		text := string(result.State)
		if text != "" {
			return text, nil
		}
	}
	return "", fmt.Errorf("Could not find a status for repository %s/%s with ref %s", pr.Owner, pr.Repo, ref)
}

func (p *GiteaProvider) AddPRComment(pr *GitPullRequest, comment string) error {
	if pr.Number == nil {
		return fmt.Errorf("Missing Number for GitPullRequest %#v", pr)
	}
	n := *pr.Number
	prComment := gitea.CreateIssueCommentOption{
		Body: asText(&comment),
	}
	_, err := p.Client.CreateIssueComment(pr.Owner, pr.Repo, int64(n), prComment)
	return err
}

func (p *GiteaProvider) CreateIssueComment(owner string, repo string, number int, comment string) error {
	issueComment := gitea.CreateIssueCommentOption{
		Body: comment,
	}
	_, err := p.Client.CreateIssueComment(owner, repo, int64(number), issueComment)
	if err != nil {
		return err
	}
	return nil
}

func (p *GiteaProvider) ListCommitStatus(org string, repo string, sha string) ([]*GitRepoStatus, error) {
	answer := []*GitRepoStatus{}
	results, err := p.Client.ListStatuses(org, repo, sha, gitea.ListStatusesOption{})
	if err != nil {
		return answer, fmt.Errorf("Could not find a status for repository %s/%s with ref %s", org, repo, sha)
	}
	for _, result := range results {
		status := &GitRepoStatus{
			ID:          string(result.ID),
			Context:     result.Context,
			URL:         result.URL,
			TargetURL:   result.TargetURL,
			State:       string(result.State),
			Description: result.Description,
		}
		answer = append(answer, status)
	}
	return answer, nil
}

func (b *GiteaProvider) UpdateCommitStatus(org string, repo string, sha string, status *GitRepoStatus) (*GitRepoStatus, error) {
	return &GitRepoStatus{}, errors.New("TODO")
}

func (p *GiteaProvider) RenameRepository(org string, name string, newName string) (*GitRepository, error) {
	return nil, fmt.Errorf("Rename of repositories is not supported for Gitea")
}

func (p *GiteaProvider) ValidateRepositoryName(org string, name string) error {
	_, err := p.Client.GetRepo(org, name)
	if err == nil {
		return fmt.Errorf("Repository %s already exists", p.Git.RepoName(org, name))
	}
	if strings.Contains(err.Error(), "404") {
		return nil
	}
	return err
}

func (p *GiteaProvider) UpdateRelease(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	var release *gitea.Release
	releases, err := p.Client.ListReleases(owner, repo)
	found := false
	for _, rel := range releases {
		if rel.TagName == tag {
			release = rel
			found = true
			break
		}
	}
	flag := false

	// lets populate the release
	if !found {
		createRelease := gitea.CreateReleaseOption{
			TagName:      releaseInfo.TagName,
			Title:        releaseInfo.Name,
			Note:         releaseInfo.Body,
			IsDraft:      flag,
			IsPrerelease: flag,
		}
		_, err = p.Client.CreateRelease(owner, repo, createRelease)
		return err
	} else {
		editRelease := gitea.EditReleaseOption{
			TagName:      release.TagName,
			Title:        release.Title,
			Note:         release.Note,
			IsDraft:      &flag,
			IsPrerelease: &flag,
		}
		if editRelease.Title == "" && releaseInfo.Name != "" {
			editRelease.Title = releaseInfo.Name
		}
		if editRelease.TagName == "" && releaseInfo.TagName != "" {
			editRelease.TagName = releaseInfo.TagName
		}
		if editRelease.Note == "" && releaseInfo.Body != "" {
			editRelease.Note = releaseInfo.Body
		}
		r2, err := p.Client.EditRelease(owner, repo, release.ID, editRelease)
		if err != nil {
			return err
		}
		if r2 != nil {
			releaseInfo.URL = r2.URL
		}
	}
	return err
}

// UpdateReleaseStatus updates the state (release/prerelease) of a release
func (p *GiteaProvider) UpdateReleaseStatus(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	var release *gitea.Release
	releases, err := p.Client.ListReleases(owner, repo)
	found := false
	for _, rel := range releases {
		if rel.TagName == tag {
			release = rel
			found = true
			break
		}
	}
	flag := false

	if found {
		editRelease := gitea.EditReleaseOption{
			TagName:      release.TagName,
			Title:        release.Title,
			Note:         release.Note,
			IsDraft:      &flag,
			IsPrerelease: &flag,
		}

		if editRelease.IsPrerelease != &releaseInfo.PreRelease {
			editRelease.IsPrerelease = &releaseInfo.PreRelease
		}
		_, err := p.Client.EditRelease(owner, repo, release.ID, editRelease)
		if err != nil {
			return err
		}
	}
	return err
}

func (p *GiteaProvider) HasIssues() bool {
	return true
}

func (p *GiteaProvider) IsGitHub() bool {
	return false
}

func (p *GiteaProvider) IsGitea() bool {
	return true
}

func (p *GiteaProvider) IsBitbucketCloud() bool {
	return false
}

func (p *GiteaProvider) IsBitbucketServer() bool {
	return false
}

func (p *GiteaProvider) IsGerrit() bool {
	return false
}

func (p *GiteaProvider) Kind() string {
	return "gitea"
}

func (p *GiteaProvider) JenkinsWebHookPath(gitURL string, secret string) string {
	return "/gitea-webhook/post"
}

func GiteaAccessTokenURL(url string) string {
	return util.UrlJoin(url, "/user/settings/applications")
}

func (p *GiteaProvider) Label() string {
	return p.Server.Label()
}

func (p *GiteaProvider) ServerURL() string {
	return p.Server.URL
}

func (p *GiteaProvider) BranchArchiveURL(org string, name string, branch string) string {
	return util.UrlJoin(p.ServerURL(), org, name, "archive", branch+".zip")
}

func (p *GiteaProvider) UserAuth() auth.UserAuth {
	return p.User
}

func (p *GiteaProvider) CurrentUsername() string {
	return p.Username
}

func (p *GiteaProvider) UserInfo(username string) *GitUser {
	user, err := p.Client.GetUserInfo(username)

	if err != nil {
		return nil
	}

	return &GitUser{
		Login:     username,
		Name:      user.FullName,
		AvatarURL: user.AvatarURL,
		Email:     user.Email,
		// TODO figure the Gitea user url
		URL: p.Server.URL + "/" + username,
	}
}

func (p *GiteaProvider) AddCollaborator(user string, organisation string, repo string) error {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for Gitea. Please add user: %v as a collaborator to this project.", user)
	return nil
}

func (p *GiteaProvider) ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for Gitea.")
	return []*github.RepositoryInvitation{}, &github.Response{}, nil
}

func (p *GiteaProvider) AcceptInvitation(ID int64) (*github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for Gitea.")
	return &github.Response{}, nil
}

func (p *GiteaProvider) GetContent(org string, name string, path string, ref string) (*GitFileContent, error) {
	return nil, fmt.Errorf("Getting content not supported on gitea")
}

// ShouldForkForPullReques treturns true if we should create a personal fork of this repository
// before creating a pull request
func (p *GiteaProvider) ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool {
	return originalOwner != username
}

func (p *GiteaProvider) ListCommits(owner, repo string, opt *ListCommitsArguments) ([]*GitCommit, error) {
	return nil, fmt.Errorf("Listing commits not supported on gitea")
}

// AddLabelsToIssue adds labels to issues or pullrequests
func (p *GiteaProvider) AddLabelsToIssue(owner, repo string, number int, labels []string) error {
	return fmt.Errorf("Getting content not supported on gitea")
}

// GetLatestRelease fetches the latest release from the git provider for org and name
func (p *GiteaProvider) GetLatestRelease(org string, name string) (*GitRelease, error) {
	releases, err := p.Client.ListReleases(org, name)
	if err != nil {
		return nil, errors2.Wrapf(err, "getting releases for %s/%s", org, name)
	}
	return toGiteaRelease(org, name, releases[0]), nil
}

// UploadReleaseAsset will upload an asset to org/repo to a release with id, giving it a name, it will return the release asset from the git provider
func (p *GiteaProvider) UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error) {
	return nil, nil
}

// GetBranch returns the branch information for an owner/repo, including the commit at the tip
func (p *GiteaProvider) GetBranch(owner string, repo string, branch string) (*GitBranch, error) {
	return nil, nil
}

// GetProjects returns all the git projects in owner/repo
func (p *GiteaProvider) GetProjects(owner string, repo string) ([]GitProject, error) {
	return nil, nil
}

//ConfigureFeatures sets specific features as enabled or disabled for owner/repo
func (p *GiteaProvider) ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error) {
	return nil, nil
}

// IsWikiEnabled returns true if a wiki is enabled for owner/repo
func (p *GiteaProvider) IsWikiEnabled(owner string, repo string) (bool, error) {
	return false, nil
}
