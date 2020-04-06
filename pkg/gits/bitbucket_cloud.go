package gits

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/google/go-github/github"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	bitbucket "github.com/wbrefvem/go-bitbucket"
)

// BitbucketCloudProvider implements GitProvider interface for bitbucket.org
type BitbucketCloudProvider struct {
	Client   *bitbucket.APIClient
	Username string
	Context  context.Context

	Server auth.AuthServer
	User   auth.UserAuth
	Git    Gitter
}

var stateMap = map[string]string{
	"SUCCESSFUL": "success",
	"FAILED":     "failure",
	"INPROGRESS": "in-progress",
	"STOPPED":    "stopped",
}

func NewBitbucketCloudProvider(server *auth.AuthServer, user *auth.UserAuth, git Gitter) (GitProvider, error) {
	ctx := context.Background()

	basicAuth := bitbucket.BasicAuth{
		UserName: user.Username,
		Password: user.ApiToken,
	}
	basicAuthContext := context.WithValue(ctx, bitbucket.ContextBasicAuth, basicAuth)

	provider := BitbucketCloudProvider{
		Server:   *server,
		User:     *user,
		Username: user.Username,
		Context:  basicAuthContext,
		Git:      git,
	}

	cfg := bitbucket.NewConfiguration()
	provider.Client = bitbucket.NewAPIClient(cfg)

	return &provider, nil
}

func (b *BitbucketCloudProvider) ListOrganisations() ([]GitOrganisation, error) {

	teams := []GitOrganisation{}

	var results bitbucket.PaginatedTeams
	var err error

	// Pagination is gross.
	for {
		if results.Next == "" {
			results, _, err = b.Client.TeamsApi.TeamsGet(b.Context, map[string]interface{}{"role": "member"})
		} else {
			results, _, err = b.Client.PagingApi.TeamsPageGet(b.Context, results.Next)
		}

		if err != nil {
			return nil, err
		}

		for _, team := range results.Values {
			teams = append(teams, GitOrganisation{Login: team.Username})
		}

		if results.Next == "" {
			break
		}
	}

	return teams, nil
}

func BitbucketRepositoryToGitRepository(bRepo bitbucket.Repository) *GitRepository {
	var sshURL string
	var httpCloneURL string
	for _, link := range bRepo.Links.Clone {
		if link.Name == "ssh" {
			sshURL = link.Href
		}
	}
	isFork := false
	if bRepo.Parent != nil {
		isFork = true
	}
	if httpCloneURL == "" {
		httpCloneURL = bRepo.Links.Html.Href
		if !strings.HasSuffix(httpCloneURL, ".git") {
			httpCloneURL += ".git"
		}
	}
	if httpCloneURL == "" {
		httpCloneURL = sshURL
	}
	return &GitRepository{
		Name:         bRepo.Name,
		HTMLURL:      bRepo.Links.Html.Href,
		CloneURL:     httpCloneURL,
		SSHURL:       sshURL,
		Language:     bRepo.Language,
		Fork:         isFork,
		Organisation: bRepo.Owner.Username,
		Project:      bRepo.Project.Name,
		Private:      bRepo.IsPrivate,
	}
}

func (b *BitbucketCloudProvider) ListRepositories(org string) ([]*GitRepository, error) {

	repos := []*GitRepository{}

	var results bitbucket.PaginatedRepositories
	var err error

	for {
		if results.Next == "" {
			results, _, err = b.Client.RepositoriesApi.RepositoriesUsernameGet(b.Context, org, nil)
		} else {
			results, _, err = b.Client.PagingApi.RepositoriesPageGet(b.Context, results.Next)
		}

		if err != nil {
			return nil, err
		}

		for _, repo := range results.Values {
			repos = append(repos, BitbucketRepositoryToGitRepository(repo))
		}

		if results.Next == "" {
			break
		}
	}

	return repos, nil
}

func (b *BitbucketCloudProvider) CreateRepository(
	org string,
	name string,
	private bool,
) (*GitRepository, error) {

	options := map[string]interface{}{}
	options["body"] = bitbucket.Repository{
		IsPrivate: private,
		Scm:       "git",
	}

	result, _, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugPost(
		b.Context,
		org,
		name,
		options,
	)

	if err != nil {
		return nil, err
	}

	return BitbucketRepositoryToGitRepository(result), nil
}

func (b *BitbucketCloudProvider) GetRepository(
	org string,
	name string,
) (*GitRepository, error) {

	repo, _, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugGet(
		b.Context,
		org,
		name,
	)

	if err != nil {
		return nil, err
	}

	return BitbucketRepositoryToGitRepository(repo), nil
}

func (b *BitbucketCloudProvider) DeleteRepository(org string, name string) error {

	_, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugDelete(
		b.Context,
		org,
		name,
		nil,
	)

	if err != nil {
		return err
	}

	return nil
}

func (b *BitbucketCloudProvider) ForkRepository(
	originalOrg string,
	name string,
	destinationOrg string,
) (*GitRepository, error) {
	options := map[string]interface{}{
		"body": map[string]interface{}{},
	}
	repo, _, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugForksPost(
		b.Context,
		originalOrg,
		name,
		options,
	)

	if err != nil {
		return nil, err
	}

	_, _, err = b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugForksGet(
		b.Context,
		originalOrg,
		repo.Name,
	)

	// Fork isn't ready
	if err != nil {

		// Wait up to 1 minute for the fork to be ready
		for i := 0; i < 30; i++ {
			_, _, err = b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugForksGet(
				b.Context,
				originalOrg,
				repo.Name,
			)

			if err == nil {
				break
			}

			time.Sleep(2 * time.Second)
		}
	}

	return BitbucketRepositoryToGitRepository(repo), nil
}

func (b *BitbucketCloudProvider) RenameRepository(
	org string,
	name string,
	newName string,
) (*GitRepository, error) {

	var options = map[string]interface{}{
		"name": newName,
	}

	repo, _, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugPut(
		b.Context,
		org,
		name,
		options,
	)

	if err != nil {
		return nil, err
	}

	return BitbucketRepositoryToGitRepository(repo), nil
}

func (b *BitbucketCloudProvider) ValidateRepositoryName(org string, name string) error {

	_, r, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugGet(
		b.Context,
		org,
		name,
	)

	if r != nil && r.StatusCode == 404 {
		return nil
	}

	if err == nil {
		return fmt.Errorf("repository %s/%s already exists", org, name)
	}

	return err
}

func (b *BitbucketCloudProvider) CreatePullRequest(
	data *GitPullRequestArguments,
) (*GitPullRequest, error) {

	if data.GitRepository.Organisation == "" {
		data.GitRepository.Organisation = b.Username
	}
	head := bitbucket.PullrequestEndpointBranch{Name: data.Head}
	sourceFullName := fmt.Sprintf("%s/%s", data.GitRepository.Organisation, data.GitRepository.Name)
	sourceRepo := bitbucket.Repository{FullName: sourceFullName}
	source := bitbucket.PullrequestEndpoint{
		Repository: &sourceRepo,
		Branch:     &head,
	}

	base := bitbucket.PullrequestEndpointBranch{Name: data.Base}
	destination := bitbucket.PullrequestEndpoint{
		Branch: &base,
	}

	bPullrequest := bitbucket.Pullrequest{
		Source:      &source,
		Destination: &destination,
		Title:       data.Title,
	}

	var options = map[string]interface{}{
		"body": bPullrequest,
	}

	pr, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPost(
		b.Context,
		data.GitRepository.Organisation,
		data.GitRepository.Name,
		options,
	)

	if err != nil {
		return nil, err
	}

	_, _, err = b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdGet(
		b.Context,
		data.GitRepository.Organisation,
		data.GitRepository.Name,
		pr.Id,
	)

	if err != nil {
		// Wait up to 1 minute for the PR to be ready.
		for i := 0; i < 30; i++ {
			_, _, err = b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdGet(
				b.Context,
				data.GitRepository.Organisation,
				data.GitRepository.Name,
				pr.Id,
			)

			if err == nil {
				break
			}

			time.Sleep(2 * time.Second)
		}
	}

	i := int(pr.Id)
	prID := &i

	newPR := &GitPullRequest{
		URL:    pr.Links.Html.Href,
		Author: b.UserInfo(pr.Author.Username),
		Owner:  strings.Split(pr.Destination.Repository.FullName, "/")[0],
		Repo:   pr.Destination.Repository.Name,
		Number: prID,
		State:  &pr.State,
	}

	return newPR, nil
}

// UpdatePullRequest updates pull request number with data
func (b *BitbucketCloudProvider) UpdatePullRequest(data *GitPullRequestArguments, number int) (*GitPullRequest, error) {
	return nil, errors.Errorf("Not yet implemented for bitbucket")
}

func (b *BitbucketCloudProvider) UpdatePullRequestStatus(pr *GitPullRequest) error {

	prID := int32(*pr.Number)
	bitbucketPR, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdGet(
		b.Context,
		pr.Owner,
		pr.Repo,
		prID,
	)

	if err != nil {
		return err
	}

	pr.State = &bitbucketPR.State
	pr.Title = bitbucketPR.Title
	pr.Body = bitbucketPR.Summary.Raw
	pr.Author = b.UserInfo(bitbucketPR.Author.Username)

	if bitbucketPR.MergeCommit != nil {
		pr.MergeCommitSHA = &bitbucketPR.MergeCommit.Hash
	}
	pr.DiffURL = &bitbucketPR.Links.Diff.Href

	if bitbucketPR.State == "MERGED" {
		merged := true
		pr.Merged = &merged
	}

	commits, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdCommitsGet(
		b.Context,
		pr.Owner,
		strconv.FormatInt(int64(prID), 10),
		pr.Repo,
	)

	if err != nil {
		return err
	}

	values := commits["values"].([]interface{})
	commit := values[0].(map[string]interface{})

	pr.LastCommitSha = commit["hash"].(string)

	return nil
}

// GetPullRequest returns the PR
func (b *BitbucketCloudProvider) GetPullRequest(owner string, repoInfo *GitRepository, number int) (*GitPullRequest, error) {
	repo := repoInfo.Name
	pr, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdGet(
		b.Context,
		owner,
		repo,
		int32(number),
	)

	if err != nil {
		return nil, err
	}

	return b.toPullRequest(pr, number), nil
}

func (b *BitbucketCloudProvider) toPullRequest(pr bitbucket.Pullrequest, number int) *GitPullRequest {
	author := &GitUser{
		Login:     pr.Author.Username,
		Name:      pr.Author.DisplayName,
		URL:       pr.Author.Links.Self.Href,
		AvatarURL: pr.Author.Links.Avatar.Href,
		// Note that Email is resolved using the GitUserResolver
	}
	answer := &GitPullRequest{
		URL:    pr.Links.Html.Href,
		Owner:  strings.Split(pr.Destination.Repository.FullName, "/")[0],
		Repo:   pr.Destination.Repository.Name,
		Number: &number,
		State:  &pr.State,
		Author: author,
	}
	return answer
}

func (b *BitbucketCloudProvider) GetPullRequestCommits(owner string, repository *GitRepository, number int) ([]*GitCommit, error) {
	repo := repository.Name
	answer := []*GitCommit{}

	// for some reason the 2nd parameter is the PR id, seems like an inconsistency/bug in the api
	commits, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdCommitsGet(b.Context, owner, strconv.Itoa(number), repo)
	if err != nil {
		return answer, err
	}

	commitVals, ok := commits["values"]
	if !ok {
		return answer, fmt.Errorf("No value key for %s/%s/%d", owner, repo, number)
	}

	commitValues, ok := commitVals.([]interface{})
	if !ok {
		return answer, fmt.Errorf("No commitValues for %s/%s/%d", owner, repo, number)
	}

	rawEmailMatcher, _ := regexp.Compile("[^<]*<([^>]+)>")

	for _, data := range commitValues {
		if data == nil {
			continue
		}

		comm, ok := data.(map[string]interface{})
		if !ok {
			log.Logger().Warn(fmt.Sprintf("Unexpected data structure for GetPullRequestCommits values from PR %s/%s/%d", owner, repo, number))
			continue
		}

		shaVal, ok := comm["hash"]
		if !ok {
			continue
		}

		sha, ok := shaVal.(string)
		if !ok {
			log.Logger().Warn(fmt.Sprintf("Unexpected data structure for GetPullRequestCommits hash from PR %s/%s/%d", owner, repo, number))
			continue
		}

		commit, _, err := b.Client.CommitsApi.RepositoriesUsernameRepoSlugCommitNodeGet(b.Context, owner, sha, repo)
		if err != nil {
			return answer, err
		}

		// update the login and email
		login := ""
		email := ""
		if commit.Author != nil {
			// commit.Author is the actual Bitbucket user
			if commit.Author.User != nil {
				login = commit.Author.User.Username
			}
			// Author.MessageLines contains the Git commit author in the form: User <email@example.com>
			email = rawEmailMatcher.ReplaceAllString(commit.Author.Raw, "$1")
		}

		summary := &GitCommit{
			Message: commit.Message,
			URL:     "", // Commit model no longer provides links.
			SHA:     commit.Hash,
			Author: &GitUser{
				Login: login,
				Email: email,
			},
		}

		answer = append(answer, summary)
	}
	return answer, nil
}

func (b *BitbucketCloudProvider) PullRequestLastCommitStatus(pr *GitPullRequest) (string, error) {

	latestCommitStatus := bitbucket.Commitstatus{}

	var result bitbucket.PaginatedCommitstatuses
	var err error

	for {
		if result.Next == "" {
			result, _, err = b.Client.CommitstatusesApi.RepositoriesUsernameRepoSlugCommitNodeStatusesGet(
				b.Context,
				pr.Owner,
				pr.LastCommitSha,
				pr.Repo,
			)
		} else {
			result, _, err = b.Client.PagingApi.CommitstatusesPageGet(b.Context, result.Next)
		}

		if err != nil {
			return "", err
		}

		// Our first time building, so return "success"
		if result.Size == 0 {
			return "success", nil
		}

		for _, status := range result.Values {
			if status.CreatedOn.After(latestCommitStatus.CreatedOn) {
				latestCommitStatus = status
			}
		}

		if result.Next == "" {
			break
		}
	}

	return stateMap[latestCommitStatus.State], nil
}

// ListOpenPullRequests lists the open pull requests
func (b *BitbucketCloudProvider) ListOpenPullRequests(owner string, repo string) ([]*GitPullRequest, error) {
	answer := []*GitPullRequest{}

	var results bitbucket.PaginatedPullrequests
	var err error

	if owner == "" {
		owner = b.Username
	}
	repoResource, _, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugGet(
		b.Context,
		owner,
		repo,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find repository %s/%s", owner, repo)
	}

	repoSlug := repoResource.Uuid
	if repoSlug == "" {
		return nil, fmt.Errorf("failed to find slug for repository %s/%s", owner, repo)
	}
	for {
		if results.Next == "" {
			results, _, err = b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsGet(b.Context, owner, repoSlug, nil)
		} else {
			results, _, err = b.Client.PagingApi.PullrequestsPageGet(b.Context, results.Next)
		}

		if err != nil {
			return nil, err
		}

		for _, pr := range results.Values {
			if pr.Author != nil && pr.Author.Username == b.Username {
				answer = append(answer, b.toPullRequest(pr, int(pr.Id)))
			}
		}

		if results.Next == "" {
			break
		}
	}
	return answer, nil
}

func (b *BitbucketCloudProvider) ListCommitStatus(org string, repo string, sha string) ([]*GitRepoStatus, error) {

	statuses := []*GitRepoStatus{}

	var result bitbucket.PaginatedCommitstatuses
	var err error

	for {
		if result.Next == "" {
			result, _, err = b.Client.CommitstatusesApi.RepositoriesUsernameRepoSlugCommitNodeStatusesGet(
				b.Context,
				org,
				sha,
				repo,
			)
		} else {
			result, _, err = b.Client.PagingApi.CommitstatusesPageGet(b.Context, result.Next)
		}

		if err != nil {
			return nil, err
		}

		for _, status := range result.Values {

			if err != nil {
				return nil, err
			}

			newStatus := &GitRepoStatus{
				ID:          status.Key,
				URL:         status.Links.Commit.Href,
				State:       stateMap[status.State],
				TargetURL:   status.Links.Self.Href,
				Description: status.Description,
			}
			statuses = append(statuses, newStatus)
		}

		if result.Next == "" {
			break
		}
	}
	return statuses, nil
}

func (b *BitbucketCloudProvider) UpdateCommitStatus(org string, repo string, sha string, status *GitRepoStatus) (*GitRepoStatus, error) {
	return &GitRepoStatus{}, errors.New("TODO")
}

func (b *BitbucketCloudProvider) MergePullRequest(pr *GitPullRequest, message string) error {

	options := map[string]interface{}{
		"body": bitbucket.PullrequestMergeParameters{
			Message: message,
		},
	}

	_, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdMergePost(
		b.Context,
		pr.Owner,
		strconv.FormatInt(int64(*pr.Number), 10),
		pr.Repo,
		options,
	)

	if err != nil {
		return err
	}

	return nil
}

func (b *BitbucketCloudProvider) CreateWebHook(data *GitWebHookArguments) error {

	options := map[string]interface{}{
		"body": map[string]interface{}{
			"url":    data.URL,
			"active": true,
			"events": []string{
				"repo:push",
				"pullrequest:created",
				"pullrequest:updated",
				"pullrequest:fulfilled",
				"pullrequest:rejected",
			},
			"description": "Jenkins X Web Hook",
		},
	}

	_, _, err := b.Client.RepositoriesApi.RepositoriesUsernameRepoSlugHooksPost(
		b.Context,
		data.Repo.Organisation,
		data.Repo.Name,
		options,
	)

	if err != nil {
		return err
	}
	return nil
}

// ListWebHooks lists the webhooks
func (b *BitbucketCloudProvider) ListWebHooks(owner string, repo string) ([]*GitWebHookArguments, error) {
	webHooks := []*GitWebHookArguments{}
	return webHooks, fmt.Errorf("not implemented!")
}

// UpdateWebHook updates the webhook
func (b *BitbucketCloudProvider) UpdateWebHook(data *GitWebHookArguments) error {
	return fmt.Errorf("not implemented!")
}

func BitbucketIssueToGitIssue(bIssue bitbucket.Issue) *GitIssue {
	id := int(bIssue.Id)
	ownerAndRepo := strings.Split(bIssue.Repository.FullName, "/")
	owner := ownerAndRepo[0]

	var assignee GitUser

	if bIssue.Assignee != nil {
		assignee = GitUser{
			URL:   bIssue.Assignee.Links.Self.Href,
			Login: bIssue.Assignee.Username,
			Name:  bIssue.Assignee.DisplayName,
		}
	}
	gitIssue := &GitIssue{
		URL:       bIssue.Links.Self.Href,
		Owner:     owner,
		Repo:      bIssue.Repository.Name,
		Number:    &id,
		Title:     bIssue.Title,
		Body:      bIssue.Content.Markup,
		State:     &bIssue.State,
		IssueURL:  &bIssue.Links.Html.Href,
		CreatedAt: &bIssue.CreatedOn,
		UpdatedAt: &bIssue.UpdatedOn,
		ClosedAt:  &bIssue.UpdatedOn,
		Assignees: []GitUser{
			assignee,
		},
	}
	return gitIssue
}

func (b *BitbucketCloudProvider) GitIssueToBitbucketIssue(gIssue GitIssue) bitbucket.Issue {

	bitbucketIssue := bitbucket.Issue{
		Title:      gIssue.Title,
		Repository: &bitbucket.Repository{Name: gIssue.Repo},
		Reporter:   &bitbucket.User{Username: b.Username},
	}

	return bitbucketIssue
}

func (b *BitbucketCloudProvider) SearchIssues(org string, name string, query string) ([]*GitIssue, error) {

	gitIssues := []*GitIssue{}

	var issues bitbucket.PaginatedIssues
	var err error

	for {
		if issues.Next == "" {
			issues, _, err = b.Client.IssueTrackerApi.RepositoriesUsernameRepoSlugIssuesGet(b.Context, org, name)
		} else {
			issues, _, err = b.Client.PagingApi.IssuesPageGet(b.Context, issues.Next)
		}

		if err != nil {
			return nil, err
		}

		for _, issue := range issues.Values {
			gitIssues = append(gitIssues, BitbucketIssueToGitIssue(issue))
		}

		if issues.Next == "" {
			break
		}
	}

	return gitIssues, nil
}

func (b *BitbucketCloudProvider) SearchIssuesClosedSince(org string, name string, t time.Time) ([]*GitIssue, error) {
	issues, err := b.SearchIssues(org, name, "")
	if err != nil {
		return issues, err
	}
	return FilterIssuesClosedSince(issues, t), nil
}

func (b *BitbucketCloudProvider) GetIssue(org string, name string, number int) (*GitIssue, error) {

	issue, _, err := b.Client.IssueTrackerApi.RepositoriesUsernameRepoSlugIssuesIssueIdGet(
		b.Context,
		org,
		strconv.FormatInt(int64(number), 10),
		name,
	)

	if err != nil {
		return nil, err
	}
	return BitbucketIssueToGitIssue(issue), nil
}

// IssueURL returns the issue URL
func (b *BitbucketCloudProvider) IssueURL(org string, name string, number int, isPull bool) string {
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

func (b *BitbucketCloudProvider) CreateIssue(owner string, repo string, issue *GitIssue) (*GitIssue, error) {

	bIssue, _, err := b.Client.IssueTrackerApi.RepositoriesUsernameRepoSlugIssuesPost(
		b.Context,
		owner,
		repo,
		b.GitIssueToBitbucketIssue(*issue),
	)

	// We need to make a second round trip to get the issue's HTML URL.
	bIssue, _, err = b.Client.IssueTrackerApi.RepositoriesUsernameRepoSlugIssuesIssueIdGet(
		b.Context,
		owner,
		strconv.FormatInt(int64(bIssue.Id), 10),
		repo,
	)

	if err != nil {
		return nil, err
	}
	return BitbucketIssueToGitIssue(bIssue), nil
}

func (b *BitbucketCloudProvider) AddPRComment(pr *GitPullRequest, comment string) error {

	if pr.Number == nil {
		return fmt.Errorf("AddPRComment requires a non-nil PR number")
	}

	rawComment := bitbucket.PullrequestComment{
		Content: &bitbucket.IssueContent{
			Raw: comment,
		},
	}

	_, _, err := b.Client.PullrequestsApi.RepositoriesUsernameRepoSlugPullrequestsPullRequestIdCommentsPost(
		b.Context,
		pr.Owner,
		pr.Repo,
		int32(*pr.Number),
		rawComment,
	)

	return err
}

func (b *BitbucketCloudProvider) CreateIssueComment(owner string, repo string, number int, comment string) error {
	rawComment := bitbucket.IssueComment{
		Content: &bitbucket.IssueContent{
			Raw: comment,
		},
	}

	_, err := b.Client.IssueTrackerApi.RepositoriesUsernameRepoSlugIssuesIssueIdCommentsPost(
		b.Context,
		strconv.FormatInt(int64(number), 10),
		owner,
		repo,
		rawComment,
	)

	return err
}

func (b *BitbucketCloudProvider) HasIssues() bool {
	return true
}

func (b *BitbucketCloudProvider) IsGitHub() bool {
	return false
}

func (b *BitbucketCloudProvider) IsGitea() bool {
	return false
}

func (b *BitbucketCloudProvider) IsBitbucketCloud() bool {
	return true
}

func (b *BitbucketCloudProvider) IsBitbucketServer() bool {
	return false
}

func (b *BitbucketCloudProvider) IsGerrit() bool {
	return false
}

func (b *BitbucketCloudProvider) Kind() string {
	return "bitbucketcloud"
}

// Exposed by Jenkins plugin; this one is for https://wiki.jenkins.io/display/JENKINS/BitBucket+Plugin
func (b *BitbucketCloudProvider) JenkinsWebHookPath(gitURL string, secret string) string {
	return "/bitbucket-scmsource-hook/notify"
}

func (b *BitbucketCloudProvider) Label() string {
	return b.Server.Label()
}

func (b *BitbucketCloudProvider) ServerURL() string {
	return b.Server.URL
}

func (b *BitbucketCloudProvider) BranchArchiveURL(org string, name string, branch string) string {
	return util.UrlJoin(b.ServerURL(), org, name, "get", branch+".zip")
}

// CurrentUsername the current user name
func (b *BitbucketCloudProvider) CurrentUsername() string {
	return b.Username
}

// UserAuth returns the user auth
func (b *BitbucketCloudProvider) UserAuth() auth.UserAuth {
	return b.User
}

// UserInfo returns the user info
func (b *BitbucketCloudProvider) UserInfo(username string) *GitUser {
	if username == "" {
		username = b.Username
	}
	user, _, err := b.Client.UsersApi.UsersUsernameGet(b.Context, username)
	if err != nil {
		log.Logger().Error("Unable to fetch user info for " + username + " due to " + err.Error())
		return nil
	}

	return &GitUser{
		Login:     username,
		Name:      user.DisplayName,
		AvatarURL: user.Links.Avatar.Href,
		URL:       user.Links.Self.Href,
	}
}

func (b *BitbucketCloudProvider) UpdateRelease(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	log.Logger().Warn("Bitbucket Cloud doesn't support releases")
	return nil
}

// UpdateReleaseStatus is not supported for this git provider
func (b *BitbucketCloudProvider) UpdateReleaseStatus(owner string, repo string, tag string, releaseInfo *GitRelease) error {
	log.Logger().Warn("Bitbucket Cloud doesn't support releases")
	return nil
}

// ListReleases lists the releases
func (b *BitbucketCloudProvider) ListReleases(org string, name string) ([]*GitRelease, error) {
	answer := []*GitRelease{}
	log.Logger().Warn("Bitbucket Cloud doesn't support releases")
	return answer, nil
}

// GetRelease is not supported on BitBucket Cloud
func (b *BitbucketCloudProvider) GetRelease(org string, name string, tag string) (*GitRelease, error) {
	log.Logger().Warn("Bitbucket Cloud doesn't support releases")
	return nil, nil
}

func (b *BitbucketCloudProvider) AddCollaborator(user string, organisation string, repo string) error {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for bitbucket. Please add user: %v as a collaborator to this project.", user)
	return nil
}

func (b *BitbucketCloudProvider) ListInvitations() ([]*github.RepositoryInvitation, *github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for bitbucket.")
	return []*github.RepositoryInvitation{}, &github.Response{}, nil
}

func (b *BitbucketCloudProvider) AcceptInvitation(ID int64) (*github.Response, error) {
	log.Logger().Infof("Automatically adding the pipeline user as a collaborator is currently not implemented for bitbucket.")
	return &github.Response{}, nil
}

func (b *BitbucketCloudProvider) GetContent(org string, name string, path string, ref string) (*GitFileContent, error) {
	log.Logger().Warn("Getting content not supported on bitbucket")
	return nil, nil
}

// ShouldForkForPullReques treturns true if we should create a personal fork of this repository
// before creating a pull request
func (b *BitbucketCloudProvider) ShouldForkForPullRequest(originalOwner string, repoName string, username string) bool {
	// return originalOwner != username
	// TODO assuming forking doesn't work yet?
	return false
}

func BitBucketCloudAccessTokenURL(url string, username string) string {
	// TODO with github we can default the scopes/flags we need on a token via adding
	// ?scopes=repo,read:user,user:email,write:repo_hook
	//
	// is there a way to do that for bitbucket?
	return util.UrlJoin(url, "/account/user", username, "/app-passwords/new")
}

// ListCommits lists the commits for the specified repo and owner
func (b *BitbucketCloudProvider) ListCommits(owner, repo string, opt *ListCommitsArguments) ([]*GitCommit, error) {
	log.Logger().Warn("Listing commits not supported on bitbucket cloud")
	return nil, nil
}

// AddLabelsToIssue adds labels to issues or pullrequests
func (b *BitbucketCloudProvider) AddLabelsToIssue(owner, repo string, number int, labels []string) error {
	log.Logger().Warn("Adding labels to issues or pullrequest not implemented/supported on bitbucket cloud")
	return nil
}

// GetLatestRelease fetches the latest release from the git provider for org and name
func (b *BitbucketCloudProvider) GetLatestRelease(org string, name string) (*GitRelease, error) {
	return nil, nil
}

// UploadReleaseAsset will upload an asset to org/repo to a release with id, giving it a name, it will return the release asset from the git provider
func (b *BitbucketCloudProvider) UploadReleaseAsset(org string, repo string, id int64, name string, asset *os.File) (*GitReleaseAsset, error) {
	return nil, nil
}

// GetBranch returns the branch information for an owner/repo, including the commit at the tip
func (b *BitbucketCloudProvider) GetBranch(owner string, repo string, branch string) (*GitBranch, error) {
	return nil, nil
}

// GetProjects returns all the git projects in owner/repo
func (b *BitbucketCloudProvider) GetProjects(owner string, repo string) ([]GitProject, error) {
	return nil, nil
}

//ConfigureFeatures sets specific features as enabled or disabled for owner/repo
func (b *BitbucketCloudProvider) ConfigureFeatures(owner string, repo string, issues *bool, projects *bool, wikis *bool) (*GitRepository, error) {
	return nil, nil
}

// IsWikiEnabled returns true if a wiki is enabled for owner/repo
func (b *BitbucketCloudProvider) IsWikiEnabled(owner string, repo string) (bool, error) {
	return false, nil
}
