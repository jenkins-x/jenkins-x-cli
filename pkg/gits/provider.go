package gits

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"gopkg.in/AlecAivazis/survey.v1"
)

type GitOrganisation struct {
	Login string
}

type GitRepository struct {
	ID               int64
	Name             string
	AllowMergeCommit bool
	HTMLURL          string
	CloneURL         string
	SSHURL           string
	Language         string
	Fork             bool
	Stars            int
	URL              string
	Scheme           string
	Host             string
	Organisation     string
	Project          string
	Private          bool
	HasIssues        bool
	OpenIssueCount   int
	HasWiki          bool
	HasProjects      bool
	Archived         bool
}

type GitPullRequest struct {
	URL                string
	Author             *GitUser
	Owner              string
	Repo               string
	Number             *int
	Mergeable          *bool
	Merged             *bool
	HeadRef            *string
	State              *string
	StatusesURL        *string
	IssueURL           *string
	DiffURL            *string
	MergeCommitSHA     *string
	ClosedAt           *time.Time
	MergedAt           *time.Time
	LastCommitSha      string
	Title              string
	Body               string
	Assignees          []*GitUser
	RequestedReviewers []*GitUser
	Labels             []*Label
	UpdatedAt          *time.Time
	HeadOwner          *string // HeadOwner is the string the PR is created from
}

// Label represents a label on an Issue
type Label struct {
	ID          *int64
	URL         *string
	Name        *string
	Color       *string
	Description *string
	Default     *bool
}

type GitCommit struct {
	SHA       string
	Message   string
	Author    *GitUser
	URL       string
	Branch    string
	Committer *GitUser
}

type ListCommitsArguments struct {
	SHA     string
	Path    string
	Author  string
	Since   time.Time
	Until   time.Time
	Page    int
	PerPage int
}

type GitIssue struct {
	URL           string
	Owner         string
	Repo          string
	Number        *int
	Key           string
	Title         string
	Body          string
	State         *string
	Labels        []GitLabel
	StatusesURL   *string
	IssueURL      *string
	CreatedAt     *time.Time
	UpdatedAt     *time.Time
	ClosedAt      *time.Time
	IsPullRequest bool
	User          *GitUser
	ClosedBy      *GitUser
	Assignees     []GitUser
}

type GitUser struct {
	URL       string
	Login     string
	Name      string
	Email     string
	AvatarURL string
}

type GitRelease struct {
	ID            int64
	Name          string
	TagName       string
	Body          string
	PreRelease    bool
	URL           string
	HTMLURL       string
	DownloadCount int
	Assets        *[]GitReleaseAsset
}

// GitReleaseAsset represents a release stored in Git
type GitReleaseAsset struct {
	ID                 int64
	BrowserDownloadURL string
	Name               string
	ContentType        string
}

type GitLabel struct {
	URL   string
	Name  string
	Color string
}

type GitRepoStatus struct {
	ID      string
	Context string
	URL     string

	// State is the current state of the repository. Possible values are:
	// pending, success, error, or failure.
	State string `json:"state,omitempty"`

	// TargetURL is the URL of the page representing this status
	TargetURL string `json:"target_url,omitempty"`

	// Description is a short high level summary of the status.
	Description string
}

type GitPullRequestArguments struct {
	Title         string
	Body          string
	Head          string
	Base          string
	GitRepository *GitRepository
	Labels        []string
}

func (a *GitPullRequestArguments) String() string {
	return fmt.Sprintf("Title: %s; Body: %s; Head: %s; Base: %s; Labels: %s; Git Repo: %s", a.Title, a.Body, a.Head, a.Base, strings.Join(a.Labels, ", "), a.GitRepository.URL)
}

type GitWebHookArguments struct {
	ID          int64
	Owner       string
	Repo        *GitRepository
	URL         string
	ExistingURL string
	Secret      string
	InsecureSSL bool
}

type GitFileContent struct {
	Type        string
	Encoding    string
	Size        int
	Name        string
	Path        string
	Content     string
	Sha         string
	Url         string
	GitUrl      string
	HtmlUrl     string
	DownloadUrl string
}

// GitBranch is info on a git branch including the commit at the tip
type GitBranch struct {
	Name      string
	Commit    *GitCommit
	Protected bool
}

// PullRequestInfo describes a pull request that has been created
type PullRequestInfo struct {
	GitProvider          GitProvider
	PullRequest          *GitPullRequest
	PullRequestArguments *GitPullRequestArguments
}

// GitProject is a project for managing issues
type GitProject struct {
	Name        string
	Description string
	Number      int
	State       string
}

// IsClosed returns true if the PullRequest has been closed
func (pr *GitPullRequest) IsClosed() bool {
	return pr.ClosedAt != nil
}

// NumberString returns the string representation of the Pull Request number or blank if its missing
func (pr *GitPullRequest) NumberString() string {
	n := pr.Number
	if n == nil {
		return ""
	}
	return "#" + strconv.Itoa(*n)
}

// ShortSha returns short SHA of the commit.
func (c *GitCommit) ShortSha() string {
	shortLen := 9
	if len(c.SHA) < shortLen+1 {
		return c.SHA
	}
	return c.SHA[:shortLen]
}

// Subject returns the subject line of the commit message.
func (c *GitCommit) Subject() string {
	lines := strings.Split(c.Message, "\n")
	return lines[0]
}

// OneLine returns the commit in the Oneline format
func (c *GitCommit) OneLine() string {
	return fmt.Sprintf("%s %s", c.ShortSha(), c.Subject())
}

// CreateProvider creates a git provider for the given auth details
func CreateProvider(server *auth.AuthServer, user *auth.UserAuth, git Gitter) (GitProvider, error) {
	if server.Kind == "" {
		server.Kind = SaasGitKind(server.URL)
	}
	if server.Kind == KindBitBucketCloud {
		return NewBitbucketCloudProvider(server, user, git)
	} else if server.Kind == KindBitBucketServer {
		return NewBitbucketServerProvider(server, user, git)
	} else if server.Kind == KindGitea {
		return NewGiteaProvider(server, user, git)
	} else if server.Kind == KindGitlab {
		return NewGitlabProvider(server, user, git)
	} else if server.Kind == KindGitFake {
		return NewFakeProvider(), nil
	} else {
		return NewGitHubProvider(server, user, git)
	}
}

// GetHost returns the Git Provider hostname, e.g github.com
func GetHost(gitProvider GitProvider) (string, error) {
	if gitProvider == nil {
		return "", fmt.Errorf("no Git provider")
	}

	if gitProvider.ServerURL() == "" {
		return "", fmt.Errorf("no Git provider server URL found")
	}
	url, err := url.Parse(gitProvider.ServerURL())
	if err != nil {
		return "", fmt.Errorf("error parsing ")
	}
	return url.Host, nil
}

func ProviderAccessTokenURL(kind string, url string, username string) string {
	switch kind {
	case KindBitBucketCloud:
		// TODO pass in the username
		return BitBucketCloudAccessTokenURL(url, username)
	case KindBitBucketServer:
		return BitBucketServerAccessTokenURL(url)
	case KindGitea:
		return GiteaAccessTokenURL(url)
	case KindGitlab:
		return GitlabAccessTokenURL(url)
	default:
		return GitHubAccessTokenURL(url)
	}
}

// PickOwner allows to select a potential owner of a repository
func PickOwner(orgLister OrganisationLister, userName string, handles util.IOFileHandles) (string, error) {
	msg := "Who should be the owner of the repository?"
	return pickOwner(orgLister, userName, msg, handles)
}

// PickOrganisation picks an organisations login if there is one available
func PickOrganisation(orgLister OrganisationLister, userName string, handles util.IOFileHandles) (string, error) {
	msg := "Which organisation do you want to use?"
	return pickOwner(orgLister, userName, msg, handles)
}

func pickOwner(orgLister OrganisationLister, userName string, message string, handles util.IOFileHandles) (string, error) {
	prompt := &survey.Select{
		Message: message,
		Options: GetOrganizations(orgLister, userName),
		Default: userName,
	}

	orgName := ""
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	err := survey.AskOne(prompt, &orgName, nil, surveyOpts)
	if err != nil {
		return "", err
	}
	if orgName == userName {
		return "", nil
	}
	return orgName, nil
}

// GetOrganizations gets the organisation
func GetOrganizations(orgLister OrganisationLister, userName string) []string {
	var orgNames []string
	// Always include the username as a pseudo organization
	if userName != "" {
		orgNames = append(orgNames, userName)
	}

	orgs, _ := orgLister.ListOrganisations()
	for _, o := range orgs {
		if name := o.Login; name != "" {
			orgNames = append(orgNames, name)
		}
	}
	sort.Strings(orgNames)
	return orgNames
}

func PickRepositories(provider GitProvider, owner string, message string, selectAll bool, filter string, handles util.IOFileHandles) ([]*GitRepository, error) {
	answer := []*GitRepository{}
	repos, err := provider.ListRepositories(owner)
	if err != nil {
		return answer, err
	}

	repoMap := map[string]*GitRepository{}
	allRepoNames := []string{}
	for _, repo := range repos {
		n := repo.Name
		if n != "" && (filter == "" || strings.Contains(n, filter)) {
			allRepoNames = append(allRepoNames, n)
			repoMap[n] = repo
		}
	}
	if len(allRepoNames) == 0 {
		return answer, fmt.Errorf("No matching repositories could be found!")
	}
	sort.Strings(allRepoNames)

	prompt := &survey.MultiSelect{
		Message: message,
		Options: allRepoNames,
	}
	if selectAll {
		prompt.Default = allRepoNames
	}
	repoNames := []string{}
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	err = survey.AskOne(prompt, &repoNames, nil, surveyOpts)

	for _, n := range repoNames {
		repo := repoMap[n]
		if repo != nil {
			answer = append(answer, repo)
		}
	}
	return answer, err
}

// IsGitRepoStatusSuccess returns true if all the statuses are successful
func IsGitRepoStatusSuccess(statuses ...*GitRepoStatus) bool {
	for _, status := range statuses {
		if !status.IsSuccess() {
			return false
		}
	}
	return true
}

// IsGitRepoStatusFailed returns true if any of the statuses have failed
func IsGitRepoStatusFailed(statuses ...*GitRepoStatus) bool {
	for _, status := range statuses {
		if status.IsFailed() {
			return true
		}
	}
	return false
}

func (s *GitRepoStatus) IsSuccess() bool {
	return s.State == "success"
}

func (s *GitRepoStatus) IsFailed() bool {
	return s.State == "error" || s.State == "failure"
}

// PickOrCreateProvider picks an existing server and auth or creates a new one if required
// then create a GitProvider for it
func (i *GitRepository) PickOrCreateProvider(authConfigSvc auth.ConfigService, message string, batchMode bool, gitKind string, githubAppMode bool, git Gitter, handles util.IOFileHandles) (GitProvider, error) {
	config := authConfigSvc.Config()
	hostUrl := i.HostURLWithoutUser()
	server := config.GetOrCreateServer(hostUrl)
	if server.Kind == "" {
		server.Kind = gitKind
	}
	var userAuth *auth.UserAuth
	var err error
	if githubAppMode && i.Organisation != "" {
		for _, u := range server.Users {
			if i.Organisation == u.GithubAppOwner {
				userAuth = u
				break
			}
		}
	}
	if userAuth == nil {
		userAuth, err = config.PickServerUserAuth(server, message, batchMode, i.Organisation, handles)
		if err != nil {
			return nil, err
		}
	}
	if userAuth.IsInvalid() {
		userAuth, err = createUserForServer(batchMode, userAuth, authConfigSvc, server, git, handles)
	}
	return i.CreateProviderForUser(server, userAuth, gitKind, git)
}

func (i *GitRepository) CreateProviderForUser(server *auth.AuthServer, user *auth.UserAuth, gitKind string, git Gitter) (GitProvider, error) {
	if i.Host == GitHubHost {
		return NewGitHubProvider(server, user, git)
	}
	if gitKind != "" && server.Kind != gitKind {
		server.Kind = gitKind
	}
	return CreateProvider(server, user, git)
}

func (i *GitRepository) CreateProvider(inCluster bool, authConfigSvc auth.ConfigService, gitKind string, ghOwner string, git Gitter, batchMode bool, handles util.IOFileHandles) (GitProvider, error) {
	hostUrl := i.HostURLWithoutUser()
	return CreateProviderForURL(inCluster, authConfigSvc, gitKind, hostUrl, ghOwner, git, batchMode, handles)
}

// ProviderURL returns the git provider URL
func (i *GitRepository) ProviderURL() string {
	scheme := i.Scheme
	if !strings.HasPrefix(scheme, "http") {
		scheme = "https"
	}
	return scheme + "://" + i.Host
}

// CreateProviderForURL creates the Git provider for the given git kind and host URL
func CreateProviderForURL(inCluster bool, authConfigSvc auth.ConfigService, gitKind string, hostURL string, ghOwner string, git Gitter, batchMode bool,
	handles util.IOFileHandles) (GitProvider, error) {
	config := authConfigSvc.Config()
	server := config.GetOrCreateServer(hostURL)
	if gitKind != "" {
		server.Kind = gitKind
	}

	var userAuth *auth.UserAuth
	if ghOwner != "" {
		for _, u := range server.Users {
			if ghOwner == u.GithubAppOwner {
				userAuth = u
				break
			}
		}
	} else {
		userAuth = config.CurrentUser(server, inCluster)
	}
	if userAuth != nil && !userAuth.IsInvalid() {
		return CreateProvider(server, userAuth, git)
	}

	if ghOwner == "" {
		kind := server.Kind
		if kind == "" {
			kind = "GIT"
		}
		userAuthVar := auth.CreateAuthUserFromEnvironment(strings.ToUpper(kind))
		if !userAuthVar.IsInvalid() {
			return CreateProvider(server, &userAuthVar, git)
		}

		var err error
		userAuth, err = createUserForServer(batchMode, &auth.UserAuth{}, authConfigSvc, server, git, handles)
		if err != nil {
			return nil, errors.Wrapf(err, "creating user for server %q", server.URL)
		}
	}
	if userAuth != nil && !userAuth.IsInvalid() {
		return CreateProvider(server, userAuth, git)
	}
	return nil, fmt.Errorf("no valid git user found for kind %s host %s %s", gitKind, hostURL, ghOwner)
}

func createUserForServer(batchMode bool, userAuth *auth.UserAuth, authConfigSvc auth.ConfigService, server *auth.AuthServer,
	git Gitter, handles util.IOFileHandles) (*auth.UserAuth, error) {

	f := func(username string) error {
		git.PrintCreateRepositoryGenerateAccessToken(server, username, handles.Out)
		return nil
	}

	defaultUserName := ""
	err := authConfigSvc.Config().EditUserAuth(server.Label(), userAuth, defaultUserName, false, batchMode, f, handles)
	if err != nil {
		return userAuth, err
	}

	err = authConfigSvc.SaveUserAuth(server.URL, userAuth)
	if err != nil {
		return userAuth, fmt.Errorf("failed to store git auth configuration %s", err)
	}
	if userAuth.IsInvalid() {
		return userAuth, fmt.Errorf("you did not properly define the user authentication")
	}
	return userAuth, nil
}

// ToGitLabels converts the list of label names into an array of GitLabels
func ToGitLabels(names []string) []GitLabel {
	answer := []GitLabel{}
	for _, n := range names {
		answer = append(answer, GitLabel{Name: n})
	}
	return answer
}

// IsRepoStatusUpToDate takes a provider, an owner, repo, sha, and GitRepoStatus, and checks if there's an existing commit
// status for the owner/repo/sha/context (from the GitRepoStatus) with the GitRepoStatus's status, target URL, and description
func IsRepoStatusUpToDate(provider GitProvider, owner string, repo string, sha string, commitStatus *GitRepoStatus) (bool, error) {
	statuses, err := provider.ListCommitStatus(owner, repo, sha)
	if err != nil {
		return false, errors.Wrapf(err, "fetching commit statuses for %s/%s, sha %s", owner, repo, sha)
	}
	for _, existingStatus := range statuses {
		if existingStatus != nil && existingStatus.Context == commitStatus.Context {
			if existingStatus.State == commitStatus.State &&
				existingStatus.TargetURL == commitStatus.TargetURL &&
				existingStatus.Description == commitStatus.Description {
				return true, nil
			}
		}
	}
	return false, nil
}
