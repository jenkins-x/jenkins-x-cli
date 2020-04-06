package gits

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/google/go-github/github"
	"github.com/mitchellh/mapstructure"

	bitbucket "github.com/gfleury/go-bitbucket-v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// pageLimit is used for the page size for API responses
const pageLimit = 25

// BitbucketServerProvider implements GitProvider interface for a bitbucket server
type BitbucketServerProvider struct {
	Client   *bitbucket.APIClient
	Username string
	Context  context.Context

	Server auth.AuthServer
	User   auth.UserAuth
	Git    Gitter
}

type projectsPage struct {
	Size          int                 `json:"size"`
	Limit         int                 `json:"limit"`
	Start         int                 `json:"start"`
	NextPageStart int                 `json:"nextPageStart"`
	IsLastPage    bool                `json:"isLastPage"`
	Values        []bitbucket.Project `json:"values"`
}

type commitsPage struct {
	Size          int                `json:"size"`
	Limit         int                `json:"limit"`
	Start         int                `json:"start"`
	NextPageStart int                `json:"nextPageStart"`
	IsLastPage    bool               `json:"isLastPage"`
	Values        []bitbucket.Commit `json:"values"`
}

type buildStatusesPage struct {
	Size       int                     `json:"size"`
	Limit      int                     `json:"limit"`
	Start      int                     `json:"start"`
	IsLastPage bool                    `json:"isLastPage"`
	Values     []bitbucket.BuildStatus `json:"values"`
}

type reposPage struct {
	Size          int                    `json:"size"`
	Limit         int                    `json:"limit"`
	Start         int                    `json:"start"`
	NextPageStart int                    `json:"nextPageStart"`
	IsLastPage    bool                   `json:"isLastPage"`
	Values        []bitbucket.Repository `json:"values"`
}

type pullRequestPage struct {
	Size          int                     `json:"size"`
	Limit         int                     `json:"limit"`
	Start         int                     `json:"start"`
	NextPageStart int                     `json:"nextPageStart"`
	IsLastPage    bool                    `json:"isLastPage"`
	Values        []bitbucket.PullRequest `json:"values"`
}

type webHooksPage struct {
	Size          int       `json:"size"`
	Limit         int       `json:"limit"`
	Start         int       `json:"start"`
	NextPageStart int       `json:"nextPageStart"`
	IsLastPage    bool      `json:"isLastPage"`
	Values        []webHook `json:"values"`
}

type webHook struct {
	ID            int64                  `json:"id"`
	Name          string                 `json:"name"`
	CreatedDate   int64                  `json:"createdDate"`
	UpdatedDate   int64                  `json:"updatedDate"`
	Events        []string               `json:"events"`
	Configuration map[string]interface{} `json:"configuration"`
	URL           string                 `json:"url"`
	Active        bool                   `json:"active"`
}

func NewBitbucketServerProvider(server *auth.AuthServer, user *auth.UserAuth, git Gitter) (GitProvider, error) {
	ctx := context.Background()
	apiKeyAuthContext := context.WithValue(ctx, bitbucket.ContextAccessToken, user.ApiToken)

	provider := BitbucketServerProvider{
		Server:   *server,
		User:     *user,
		Username: user.Username,
		Context:  apiKeyAuthContext,
		Git:      git,
	}

	cfg := bitbucket.NewConfiguration(server.URL + "/rest")
	provider.Client = bitbucket.NewAPIClient(apiKeyAuthContext, cfg)

	return &provider, nil
}

func BitbucketServerRepositoryToGitRepository(bRepo bitbucket.Repository) *GitRepository {
	var sshURL string
	var httpCloneURL string
	for _, link := range bRepo.Links.Clone {
		if link.Name == "ssh" {
			sshURL = link.Href
		}
	}
	isFork := false

	if httpCloneURL == "" {
		cloneLinks := bRepo.Links.Clone

		for _, link := range cloneLinks {
			if link.Name == "http" {
				httpCloneURL = link.Href
				if !strings.HasSuffix(httpCloneURL, ".git") {
					httpCloneURL += ".git"
				}
			}
		}
	}
	if httpCloneURL == "" {
		httpCloneURL = sshURL
	}

	htmlURL := bRepo.Links.Self[0].Href
	return &GitRepository{
		Name:         bRepo.Name,
		HTMLURL:      htmlURL,
		CloneURL:     httpCloneURL,
		SSHURL:       sshURL,
		URL:          htmlURL,
		Fork:         isFork,
		Private:      !bRepo.Public,
		Organisation: strings.ToLower(bRepo.Project.Key),
		Project:      bRepo.Project.Key,
	}
}

func (b *BitbucketServerProvider) GetRepository(org string, name string) (*GitRepository, error) {
	var repo bitbucket.Repository
	apiResponse, err := b.Client.DefaultApi.GetRepository(org, name)

	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &repo)
	if err != nil {
		return nil, err
	}

	return BitbucketServerRepositoryToGitRepository(repo), nil
}

func (b *BitbucketServerProvider) ListOrganisations() ([]GitOrganisation, error) {
	var orgsPage projectsPage
	orgsList := []GitOrganisation{}
	paginationOptions := make(map[string]interface{})

	paginationOptions["start"] = 0
	paginationOptions["limit"] = pageLimit
	for {
		apiResponse, err := b.Client.DefaultApi.GetProjects(paginationOptions)
		if err != nil {
			return nil, err
		}

		err = mapstructure.Decode(apiResponse.Values, &orgsPage)
		if err != nil {
			return nil, err
		}

		for _, project := range orgsPage.Values {
			orgsList = append(orgsList, GitOrganisation{Login: project.Key})
		}

		if orgsPage.IsLastPage {
			break
		}
		if !b.moveToNextPage(paginationOptions, orgsPage.NextPageStart) {
			break
		}
	}

	return orgsList, nil
}

func (b *BitbucketServerProvider) ListRepositories(org string) ([]*GitRepository, error) {
	var reposPage reposPage
	repos := []*GitRepository{}
	paginationOptions := make(map[string]interface{})

	paginationOptions["start"] = 0
	paginationOptions["limit"] = pageLimit

	for {
		apiResponse, err := b.Client.DefaultApi.GetRepositoriesWithOptions(org, paginationOptions)
		if err != nil {
			return nil, err
		}

		err = mapstructure.Decode(apiResponse.Values, &reposPage)
		if err != nil {
			return nil, err
		}

		for _, bRepo := range reposPage.Values {
			repos = append(repos, BitbucketServerRepositoryToGitRepository(bRepo))
		}

		if reposPage.IsLastPage {
			break
		}
		if !b.moveToNextPage(paginationOptions, reposPage.NextPageStart) {
			break
		}
	}

	return repos, nil
}

func (b *BitbucketServerProvider) CreateRepository(org, name string, private bool) (*GitRepository, error) {
	var repo bitbucket.Repository

	repoRequest := map[string]interface{}{
		"name":   name,
		"public": !private,
	}

	requestBody, err := json.Marshal(repoRequest)
	if err != nil {
		return nil, err
	}

	apiResponse, err := b.Client.DefaultApi.CreateRepositoryWithOptions(org, requestBody, []string{"application/json"})
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &repo)
	if err != nil {
		return nil, err
	}

	return BitbucketServerRepositoryToGitRepository(repo), nil
}

func (b *BitbucketServerProvider) DeleteRepository(org, name string) error {
	_, err := b.Client.DefaultApi.DeleteRepository(org, name)

	return err
}

func (b *BitbucketServerProvider) RenameRepository(org, name, newName string) (*GitRepository, error) {
	var repo bitbucket.Repository
	var options = map[string]interface{}{
		"name": newName,
	}

	requestBody, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}

	apiResponse, err := b.Client.DefaultApi.UpdateRepositoryWithOptions(org, name, requestBody, []string{"application/json"})
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &repo)
	if err != nil {
		return nil, err
	}

	return BitbucketServerRepositoryToGitRepository(repo), nil
}

func (b *BitbucketServerProvider) ValidateRepositoryName(org, name string) error {
	apiResponse, err := b.Client.DefaultApi.GetRepository(org, name)

	if apiResponse != nil && apiResponse.Response.StatusCode == 404 {
		return nil
	}

	if err == nil {
		return fmt.Errorf("repository %s/%s already exists", b.Username, name)
	}

	return err
}

func (b *BitbucketServerProvider) ForkRepository(originalOrg, name, destinationOrg string) (*GitRepository, error) {
	var repo bitbucket.Repository
	var apiResponse *bitbucket.APIResponse
	var options = map[string]interface{}{}

	if destinationOrg != "" {
		options["project"] = map[string]interface{}{
			"key": destinationOrg,
		}
	}

	requestBody, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}

	_, err = b.Client.DefaultApi.ForkRepository(originalOrg, name, requestBody, []string{"application/json"})
	if err != nil {
		return nil, err
	}

	// Wait up to 1 minute for the fork to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)

		if destinationOrg == "" {
			apiResponse, err = b.Client.DefaultApi.GetUserRepository(b.CurrentUsername(), name)
		} else {
			apiResponse, err = b.Client.DefaultApi.GetRepository(destinationOrg, name)
		}

		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &repo)
	if err != nil {
		return nil, err
	}

	return BitbucketServerRepositoryToGitRepository(repo), nil
}

func (b *BitbucketServerProvider) CreatePullRequest(data *GitPullRequestArguments) (*GitPullRequest, error) {
	var bPullRequest, bPR bitbucket.PullRequest
	var options = map[string]interface{}{
		"title":       data.Title,
		"description": data.Body,
		"state":       "OPEN",
		"open":        true,
		"closed":      false,
		"fromRef": map[string]interface{}{
			"id": data.Head,
			"repository": map[string]interface{}{
				"slug": data.GitRepository.Name,
				"project": map[string]interface{}{
					"key": data.GitRepository.Project,
				},
			},
		},
		"toRef": map[string]interface{}{
			"id": data.Base,
			"repository": map[string]interface{}{
				"slug": data.GitRepository.Name,
				"project": map[string]interface{}{
					"key": data.GitRepository.Project,
				},
			},
		},
	}

	requestBody, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}

	apiResponse, err := b.Client.DefaultApi.CreatePullRequestWithOptions(data.GitRepository.Project, data.GitRepository.Name, requestBody)
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &bPullRequest)
	if err != nil {
		return nil, err
	}

	// Wait up to 1 minute for the pull request to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(2 * time.Second)

		apiResponse, err = b.Client.DefaultApi.GetPullRequest(data.GitRepository.Project, data.GitRepository.Name, bPullRequest.ID)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &bPR)
	if err != nil {
		return nil, err
	}

	return &GitPullRequest{
		URL:    bPR.Links.Self[0].Href,
		Owner:  bPR.Author.User.Name,
		Repo:   bPR.ToRef.Repository.Name,
		Number: &bPR.ID,
		State:  &bPR.State,
	}, nil
}

// UpdatePullRequest updates pull request number with data
func (b *BitbucketServerProvider) UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error) {
	return nil, errors.Errorf("Not yet implemented for bitbucket")
}

func parseBitBucketServerURL(URL string) (string, string) {
	var projectKey, repoName, subString string
	var projectsIndex, reposIndex, repoEndIndex int

	if strings.HasSuffix(URL, ".git") {
		subString = strings.TrimSuffix(URL, ".git")
		reposIndex = strings.LastIndex(subString, "/")
		repoName = subString[reposIndex+1:]

		subString = strings.TrimSuffix(subString, "/"+repoName)
		projectsIndex = strings.LastIndex(subString, "/")
		projectKey = subString[projectsIndex+1:]

	} else {
		projectsIndex = strings.Index(URL, "projects/")
		subString = URL[projectsIndex+9:]
		projectKey = subString[:strings.Index(subString, "/")]

		reposIndex = strings.Index(subString, "repos/")
		subString = subString[reposIndex+6:]

		repoEndIndex = strings.Index(subString, "/")

		if repoEndIndex == -1 {
			repoName = subString
		} else {
			repoName = subString[:repoEndIndex]
		}
	}

	return projectKey, repoName
}

func getMergeCommitSHAFromPRActivity(prActivity map[string]interface{}) *string {
	var activity []map[string]interface{}
	var mergeCommit map[string]interface{}

	mapstructure.Decode(prActivity["values"], &activity)
	mapstructure.Decode(activity[0]["commit"], &mergeCommit)
	commitSHA := mergeCommit["id"].(string)

	return &commitSHA
}

func getLastCommitSHAFromPRCommits(prCommits map[string]interface{}) string {
	return getLastCommitFromPRCommits(prCommits).ID
}

func getLastCommitFromPRCommits(prCommits map[string]interface{}) *bitbucket.Commit {
	var commits []bitbucket.Commit
	mapstructure.Decode(prCommits["values"], &commits)
	return &commits[0]
}

func (b *BitbucketServerProvider) UpdatePullRequestStatus(pr *GitPullRequest) error {
	var bitbucketPR bitbucket.PullRequest
	var prCommits, prActivity map[string]interface{}

	prID := *pr.Number
	projectKey, repo := parseBitBucketServerURL(pr.URL)
	apiResponse, err := b.Client.DefaultApi.GetPullRequest(projectKey, repo, prID)
	if err != nil {
		return err
	}

	err = mapstructure.Decode(apiResponse.Values, &bitbucketPR)
	if err != nil {
		return err
	}

	pr.State = &bitbucketPR.State
	pr.Title = bitbucketPR.Title
	pr.Body = bitbucketPR.Description
	pr.Author = &GitUser{
		Login: bitbucketPR.Author.User.Name,
	}

	if bitbucketPR.State == "MERGED" {
		merged := true
		pr.Merged = &merged
		apiResponse, err := b.Client.DefaultApi.GetPullRequestActivity(projectKey, repo, prID)
		if err != nil {
			return err
		}

		mapstructure.Decode(apiResponse.Values, &prActivity)
		pr.MergeCommitSHA = getMergeCommitSHAFromPRActivity(prActivity)
	}
	diffURL := bitbucketPR.Links.Self[0].Href + "/diff"
	pr.DiffURL = &diffURL

	apiResponse, err = b.Client.DefaultApi.GetPullRequestCommits(projectKey, repo, prID)
	if err != nil {
		return err
	}
	mapstructure.Decode(apiResponse.Values, &prCommits)
	pr.LastCommitSha = getLastCommitSHAFromPRCommits(prCommits)

	return nil
}

func (b *BitbucketServerProvider) GetPullRequest(owner string, repo *GitRepository, number int) (*GitPullRequest, error) {
	var bPR bitbucket.PullRequest

	apiResponse, err := b.Client.DefaultApi.GetPullRequest(repo.Project, repo.Name, number)
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(apiResponse.Values, &bPR)
	if err != nil {
		return nil, err
	}

	answer := b.toPullRequest(bPR)
	return answer, nil
}

func (b *BitbucketServerProvider) toPullRequest(bPR bitbucket.PullRequest) *GitPullRequest {
	author := &GitUser{
		URL:   bPR.Author.User.Links.Self[0].Href,
		Login: bPR.Author.User.Slug,
		Name:  bPR.Author.User.Name,
		Email: bPR.Author.User.EmailAddress,
	}
	answer := &GitPullRequest{
		URL:           bPR.Links.Self[0].Href,
		Owner:         bPR.Author.User.Name,
		Repo:          bPR.ToRef.Repository.Name,
		Number:        &bPR.ID,
		State:         &bPR.State,
		Author:        author,
		LastCommitSha: bPR.FromRef.LatestCommit,
	}
	return answer
}

// ListOpenPullRequests lists the open pull requests
func (b *BitbucketServerProvider) ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error) {
	answer := []*GitPullRequest{}
	var pullRequests pullRequestPage

	paginationOptions := make(map[string]interface{})

	paginationOptions["start"] = 0
	paginationOptions["limit"] = pageLimit

	// TODO how to pass in the owner and repo and status? these are total guesses
	paginationOptions["owner"] = owner
	paginationOptions["repo"] = repo
	paginationOptions["state"] = "open"

	for {
		apiResponse, err := b.Client.DefaultApi.GetPullRequests(paginationOptions)
		if err != nil {
			return nil, err
		}

		err = mapstructure.Decode(apiResponse.Values, &pullRequests)
		if err != nil {
			return nil, err
		}

		for _, pr := range pullRequests.Values {
			answer = append(answer, b.toPullRequest(pr))
		}

		if pullRequests.IsLastPage {
			break
		}
		if !b.moveToNextPage(paginationOptions, pullRequests.NextPageStart) {
			break
		}
	}
	return answer, nil
}

// moveToNextPage returns true if we should move to the next page
func (b *BitbucketServerProvider) moveToNextPage(paginationOptions map[string]interface{}, nextPage int) bool {
	lastStartValue := paginationOptions["start"]
	lastStart, _ := lastStartValue.(int)
	if lastStart < 0 {
		lastStart = 0
	}
	if nextPage < 0 {
		return false
	}
	if nextPage <= lastStart {
		return false
	}
	paginationOptions["start"] = nextPage
	return true
}

func convertBitBucketCommitToGitCommit(bCommit *bitbucket.Commit, repo *GitRepository) *GitCommit {
	return &GitCommit{
		SHA:     bCommit.ID,
		Message: bCommit.Message,
		Author: &GitUser{
			Login: bCommit.Author.Name,
			Name:  bCommit.Author.DisplayName,
			Email: bCommit.Author.EmailAddress,
		},
		URL: repo.URL + "/commits/" + bCommit.ID,
		Committer: &GitUser{
			Login: bCommit.Committer.Name,
			Name:  bCommit.Committer.DisplayName,
			Email: bCommit.Committer.EmailAddress,
		},
	}
}

func (b *BitbucketServerProvider) GetPullRequestCommits(owner string, repository *GitRepository, number int) ([]*GitCommit, error) {
	var commitsPage commitsPage
	commits := []*GitCommit{}
	paginationOptions := make(map[string]interface{})

	paginationOptions["start"] = 0
	paginationOptions["limit"] = pageLimit
	for {
		apiResponse, err := b.Client.DefaultApi.GetPullRequestCommitsWithOptions(repository.Project, repository.Name, number, paginationOptions)
		if err != nil {
			return nil, err
		}

		err = mapstructure.Decode(apiResponse.Values, &commitsPage)
		if err != nil {
			return nil, err
		}

		for _, commit := range commitsPage.Values {
			commits = append(commits, convertBitBucketCommitToGitCommit(&commit, repository))
		}

		if commitsPage.IsLastPage {
			break
		}
		if !b.moveToNextPage(paginationOptions, commitsPage.NextPageStart) {
			break
		}
	}

	return commits, nil
}

func (b *BitbucketServerProvider) PullRequestLastCommitStatus(pr *GitPullRequest) (string, error) {
	var prCommits map[string]interface{}
	var buildStatusesPage buildStatusesPage

	projectKey, repo := parseBitBucketServerURL(pr.URL)
	apiResponse, err := b.Client.DefaultApi.GetPullRequestCommits(projectKey, repo, *pr.Number)
	if err != nil {
		return "", err
	}
	mapstructure.Decode(apiResponse.Values, &prCommits)
	lastCommit := getLastCommitFromPRCommits(prCommits)
	lastCommitSha := lastCommit.ID

	apiResponse, err = b.Client.DefaultApi.GetCommitBuildStatuses(lastCommitSha)
	if err != nil {
		return "", err
	}

	mapstructure.Decode(apiResponse.Values, &buildStatusesPage)
	if buildStatusesPage.Size == 0 {
		return "success", nil
	}

	for _, buildStatus := range buildStatusesPage.Values {
		if time.Unix(buildStatus.DateAdded, 0).After(time.Unix(lastCommit.CommitterTimestamp, 0)) {
			// var from BitBucketCloudProvider
			return stateMap[buildStatus.State], nil
		}
	}

	return "success", nil
}

func (b *BitbucketServerProvider) ListCommitStatus(org, repo, sha string) ([]*GitRepoStatus, error) {
	var buildStatusesPage buildStatusesPage
	statuses := []*GitRepoStatus{}

	for {
		apiResponse, err := b.Client.DefaultApi.GetCommitBuildStatuses(sha)
		if err != nil {
			return nil, err
		}

		mapstructure.Decode(apiResponse.Values, &buildStatusesPage)

		for _, buildStatus := range buildStatusesPage.Values {
			statuses = append(statuses, convertBitBucketBuildStatusToGitStatus(&buildStatus))
		}

		if buildStatusesPage.IsLastPage {
			break
		}
	}

	return statuses, nil
}

func (b *BitbucketServerProvider) UpdateCommitStatus(org string, repo string, sha string, status *GitRepoStatus) (*GitRepoStatus, error) {
	return &GitRepoStatus{}, errors.New("TODO")
}

func convertBitBucketBuildStatusToGitStatus(buildStatus *bitbucket.BuildStatus) *GitRepoStatus {
	return &GitRepoStatus{
		ID:  buildStatus.Key,
		URL: buildStatus.Url,
		// var from BitBucketCloudProvider
		State:       stateMap[buildStatus.State],
		TargetURL:   buildStatus.Url,
		Description: buildStatus.Description,
	}
}

func (b *BitbucketServerProvider) MergePullRequest(pr *GitPullRequest, message string) error {
	var currentPR bitbucket.PullRequest
	projectKey, repo := parseBitBucketServerURL(pr.URL)
	queryParams := map[string]interface{}{}

	apiResponse, err := b.Client.DefaultApi.GetPullRequest(projectKey, repo, *pr.Number)
	if err != nil {
		return err
	}

	mapstructure.Decode(apiResponse.Values, &currentPR)
	queryParams["version"] = currentPR.Version

	var options = map[string]interface{}{
		"message": message,
	}

	requestBody, err := json.Marshal(options)
	if err != nil {
		return err
	}

	apiResponse, err = b.Client.DefaultApi.Merge(projectKey, repo, *pr.Number, queryParams, requestBody, []string{"application/json"})
	if err != nil {
		return err
	}

	return nil
}

func (b *BitbucketServerProvider) parseWebHookURL(data *GitWebHookArguments) (string, string, error) {
	repoURL := data.Repo.URL
	owner := data.Repo.Organisation
	repoName := data.Repo.Name
	if repoURL == "" {
		repository, err := b.GetRepository(owner, repoName)
		if err != nil {
			return "", "", errors.Wrapf(err, "failed to find repository %s/%s in server %s", owner, repoName, b.Server.URL)
		}
		repoURL = repository.URL
		if repoURL == "" {
			repoURL = repository.HTMLURL
		}
		if repoURL == "" {
			return "", "", errors.Wrapf(err, "repository %s/%s on server %s has no URL", owner, repoName, b.Server.URL)
		}
	}
	projectKey, repo := parseBitBucketServerURL(repoURL)
	return projectKey, repo, nil
}

// CreateWebHook adds a new webhook to a git repository
func (b *BitbucketServerProvider) CreateWebHook(data *GitWebHookArguments) error {
	projectKey, repo, err := b.parseWebHookURL(data)
	if err != nil {
		return err
	}

	if data.URL == "" {
		return errors.New("missing property URL")
	}

	hooks, err := b.ListWebHooks(projectKey, repo)
	if err != nil {
		return errors.Wrapf(err, "error querying webhooks on %s/%s\n", projectKey, repo)
	}
	for _, hook := range hooks {
		if data.URL == hook.URL {
			log.Logger().Warnf("Already has a webhook registered for %s", data.URL)
			return nil
		}
	}

	var options = map[string]interface{}{
		"url":    data.URL,
		"name":   "Jenkins X Web Hook",
		"active": true,
		"events": []string{"repo:refs_changed", "repo:modified", "repo:forked", "repo:comment:added", "repo:comment:edited", "repo:comment:deleted", "pr:opened", "pr:reviewer:approved", "pr:reviewer:unapproved", "pr:reviewer:needs_work", "pr:merged", "pr:declined", "pr:deleted", "pr:comment:added", "pr:comment:edited", "pr:comment:deleted"},
	}

	if data.Secret != "" {
		options["configuration"] = map[string]interface{}{
			"secret": data.Secret,
		}
	}

	requestBody, err := json.Marshal(options)
	if err != nil {
		return errors.Wrap(err, "failed to JSON encode webhook request body for creation")
	}

	_, err = b.Client.DefaultApi.CreateWebhook(projectKey, repo, requestBody, []string{"application/json"})

	if err != nil {
		return errors.Wrapf(err, "create webhook request failed on %s/%s", projectKey, repo)
	}
	return nil
}

// ListWebHooks lists all of the webhooks on a given git repository
func (b *BitbucketServerProvider) ListWebHooks(owner string, repo string) ([]*GitWebHookArguments, error) {
	var webHooksPage webHooksPage
	var webHooks []*GitWebHookArguments

	paginationOptions := make(map[string]interface{})
	paginationOptions["start"] = 0
	paginationOptions["limit"] = pageLimit

	for {
		apiResponse, err := b.Client.DefaultApi.FindWebhooks(owner, repo, paginationOptions)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to list webhooks on repository %s/%s", owner, repo)
		}

		err = mapstructure.Decode(apiResponse.Values, &webHooksPage)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode response from list webhooks")
		}

		for _, wh := range webHooksPage.Values {
			secret := ""
			if cfg, ok := wh.Configuration["secret"].(string); ok {
				secret = cfg
			}

			webHooks = append(webHooks, &GitWebHookArguments{
				ID:     wh.ID,
				Owner:  owner,
				Repo:   nil,
				URL:    wh.URL,
				Secret: secret,
			})
		}

		if webHooksPage.IsLastPage {
			break
		}
		if !b.moveToNextPage(paginationOptions, webHooksPage.NextPageStart) {
			break
		}
	}

	return webHooks, nil
}

// UpdateWebHook is used to update a webhook on a git repository.  It is best to pass in the webhook ID.
func (b *BitbucketServerProvider) UpdateWebHook(data *GitWebHookArguments) error {
	projectKey, repo, err := b.parseWebHookURL(data)
	if err != nil {
		return err
	}

	if data.URL == "" {
		return errors.New("missing property URL")
	}

	dataID := data.ID
	if dataID == 0 && data.ExistingURL != "" {
		hooks, err := b.ListWebHooks(projectKey, repo)
		if err != nil {
			log.Logger().Errorf("Error querying webhooks on %s/%s: %s", projectKey, repo, err)
		}
		for _, hook := range hooks {
			if data.ExistingURL == hook.URL {
				log.Logger().Warnf("Found existing webhook for url %s", data.ExistingURL)
				dataID = hook.ID
			}
		}
	}
	if dataID == 0 {
		log.Logger().Warn("No webhooks found to update")
		return nil
	}
	id := int32(dataID)
	if int64(id) != dataID {
		return errors.Errorf("Failed to update webhook with ID = %d due to int32 conversion failure", dataID)
	}

	var options = map[string]interface{}{
		"url":    data.URL,
		"name":   "Jenkins X Web Hook",
		"active": true,
		"events": []string{"repo:refs_changed", "repo:modified", "repo:forked", "repo:comment:added", "repo:comment:edited", "repo:comment:deleted", "pr:opened", "pr:reviewer:approved", "pr:reviewer:unapproved", "pr:reviewer:needs_work", "pr:merged", "pr:declined", "pr:deleted", "pr:comment:added", "pr:comment:edited", "pr:comment:deleted"},
	}

	if data.Secret != "" {
		options["configuration"] = map[string]interface{}{
			"secret": data.Secret,
		}
	}

	requestBody, err := json.Marshal(options)
	if err != nil {
		return errors.Wrap(err, "failed to JSON encode webhook request body for update")
	}

	log.Logger().Infof("Updating Bitbucket server webhook for %s/%s for url %s", util.ColorInfo(projectKey), util.ColorInfo(repo), util.ColorInfo(data.URL))
	_, err = b.Client.DefaultApi.UpdateWebhook(projectKey, repo, id, requestBody, []string{"application/json"})

	if err != nil {
		return errors.Wrapf(err, "failed to update webhook on %s/%s", projectKey, repo)
	}
	return nil
}

func (b *BitbucketServerProvider) SearchIssues(org string, name string, query string) ([]*GitIssue, error) {

	gitIssues := []*GitIssue{}

	log.Logger().Warn("Searching issues on bitbucket server is not supported at this moment")

	return gitIssues, nil
}

func (b *BitbucketServerProvider) SearchIssuesClosedSince(org string, name string, t time.Time) ([]*GitIssue, error) {
	issues, err := b.SearchIssues(org, name, "")
	if err != nil {
		return issues, err
	}
	return FilterIssuesClosedSince(issues, t), nil
}

func (b *BitbucketServerProvider) GetIssue(org string, name string, number int) (*GitIssue, error) {

	log.Logger().Warn("Finding an issue on bitbucket server is not supported at this moment")
	return &GitIssue{}, nil
}

func (b *BitbucketServerProvider) IssueURL(org string, name string, number int, isPull bool) string {
	serverPrefix := b.Server.URL
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

func (b *BitbucketServerProvider) CreateIssue(owner string, repo string, issue *GitIssue) (*GitIssue, error) {

	log.Logger().Warn("Creating an issue on bitbucket server is not suuported at this moment")
	return &GitIssue{}, nil
}

func (b *BitbucketServerProvider) AddPRComment(pr *GitPullRequest, comment string) error {

	if pr.Number == nil {
		return fmt.Errorf("Missing Number for GitPullRequest %#v", pr)
	}
	n := *pr.Number

	prComment := `{
		"text": "` + comment + `"
	}`
	_, err := b.Client.DefaultApi.CreateComment_1(pr.Owner, pr.Repo, n, prComment, []string{"application/json"})
	return err
}

func (b *BitbucketServerProvider) CreateIssueComment(owner string, repo string, number int, comment string) error {
	log.Logger().Warn("Bitbucket Server doesn't support adding issue comments via the REST API")
	return nil
}

func (b *BitbucketServerProvider) HasIssues() bool {
	return true
}

func (b *BitbucketServerProvider) IsGitHub() bool {
	return false
}

func (b *BitbucketServerProvider) IsGitea() bool {
	return false
}

func (b *BitbucketServerProvider) IsBitbucketCloud() bool {
	return false
}

func (b *BitbucketServerProvider) IsBitbucketServer() bool {
	return true
}

func (b *BitbucketServerProvider) IsGerrit() bool {
	return false
}

func (b *BitbucketServerProvider) Kind() string {
	return "bitbucketserver"
}

// Exposed by Jenkins plugin; this one is for https://wiki.jenkins.io/display/JENKINS/BitBucket+Plugin
func (b *BitbucketServerProvider) JenkinsWebHookPath(gitURL string, secret string) string {
	return "/bitbucket-scmsource-hook/notify?server_url=" + url.QueryEscape(b.Server.URL)
}

func (b *BitbucketServerProvider) Label() string {
	return b.Server.Label()
}

func (b *BitbucketServerProvider) ServerURL() string {
	return b.Server.URL
}

func (b *BitbucketServerProvider) BranchArchiveURL(org string, name string, branch string) string {
	return util.UrlJoin(b.ServerURL(), "rest/api/1.0/projects", org, "repos", name, "archive?format=zip&at="+branch)
}

func (b *BitbucketServerProvider) CurrentUsername() string {
	return b.Username
}

func (b *BitbucketServerProvider) UserAuth() auth.UserAuth {
	return b.User
}

func (b *BitbucketServerProvider) UserInfo(username string) *GitUser {
	var user bitbucket.UserWithLinks
	apiResponse, err := b.Client.DefaultApi.GetUser(username)
	if err != nil {
		log.Logger().Error("Unable to fetch user info for " + username + " due to " + err.Error())
		return nil
	}
	err = mapstructure.Decode(apiResponse.Values, &user)

	return &GitUser{
		Login: username,
		Name:  user.DisplayName,
		Email: user.EmailAddress,
		URL:   user.Links.Self[0].Href,
	}
}

func (b *BitbucketServerProvider) UpdateRelease(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	log.Logger().Warn("Bitbucket Server doesn't support releases")
	return nil
}

// UpdateReleaseStatus is not supported for this git provider
func (b *BitbucketServerProvider) UpdateReleaseStatus(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	log.Logger().Warn("Bitbucket Server doesn't support releases")
	return nil
}

func (b *BitbucketServerProvider) ListReleases(org string, name string) ([]*GitRelease, error) {
	answer := []*GitRelease{}
	log.Logger().Warn("Bitbucket Server doesn't support releases")
	return answer, nil
}

// GetRelease is unsupported on bitbucket as releases are not supported
func (b *BitbucketServerProvider) GetRelease(org string, name string, tag string) (*GitRelease, error) {
	log.Logger().Warn("Bitbucket Cloud doesn't support releases")
	return nil, nil
}

func (b *BitbucketServerProvider) AddCollaborator(user string, organisation string, repo string) error {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for bitbucket. Please add user: %v as a collaborator to this project.", user)
	return nil
}

func (b *BitbucketServerProvider) ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for bitbucket.")
	return []*github.RepositoryInvitation{}, &github.Response{}, nil
}

func (b *BitbucketServerProvider) AcceptInvitation(ID int64) (*github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for bitbucket.")
	return &github.Response{}, nil
}

func (b *BitbucketServerProvider) GetContent(org string, name string, path string, ref string) (*GitFileContent, error) {
	return nil, fmt.Errorf("Getting content not supported on bitbucket")
}

// ShouldForkForPullReques treturns true if we should create a personal fork of this repository
// before creating a pull request
func (b *BitbucketServerProvider) ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool {
	// return originalOwner != username
	// TODO assuming forking doesn't work yet?
	return false
}

func BitBucketServerAccessTokenURL(url string) string {
	// TODO with github we can default the scopes/flags we need on a token via adding
	// ?scopes=repo,read:user,user:email,write:repo_hook
	//
	// is there a way to do that for bitbucket?
	return util.UrlJoin(url, "/plugins/servlet/access-tokens/manage")
}

// ListCommits lists the commits for the specified repo and owner
func (b *BitbucketServerProvider) ListCommits(owner, repo string, opt *ListCommitsArguments) ([]*GitCommit, error) {
	return nil, fmt.Errorf("Listing commits not supported on bitbucket")
}

// AddLabelsToIssue adds labels to issues or pullrequests
func (b *BitbucketServerProvider) AddLabelsToIssue(owner, repo string, number int, labels []string) error {
	log.Logger().Warnf("Adding labels not supported on bitbucket server yet for repo %s/%s issue %d labels %v", owner, repo, number, labels)
	return nil
}

// GetLatestRelease fetches the latest release from the git provider for org and name
func (b *BitbucketServerProvider) GetLatestRelease(org string, name string) (*GitRelease, error) {
	return nil, nil
}

// UploadReleaseAsset will upload an asset to org/repo to a release with id, giving it a name, it will return the release asset from the git provider
func (b *BitbucketServerProvider) UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error) {
	return nil, nil
}

// GetBranch returns the branch information for an owner/repo, including the commit at the tip
func (b *BitbucketServerProvider) GetBranch(owner string, repo string, branch string) (*GitBranch, error) {
	return nil, nil
}

// GetProjects returns all the git projects in owner/repo
func (b *BitbucketServerProvider) GetProjects(owner string, repo string) ([]GitProject, error) {
	return nil, nil
}

//ConfigureFeatures sets specific features as enabled or disabled for owner/repo
func (b *BitbucketServerProvider) ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error) {
	return nil, nil
}

// IsWikiEnabled returns true if a wiki is enabled for owner/repo
func (b *BitbucketServerProvider) IsWikiEnabled(owner string, repo string) (bool, error) {
	return false, nil
}
