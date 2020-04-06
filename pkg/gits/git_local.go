package gits

import (
	"io"
	"time"

	"github.com/jenkins-x/jx/pkg/auth"

	gitcfg "gopkg.in/src-d/go-git.v4/config"
)

// GitLocal provides a semi-fake Gitter - local operations delegate to GitCLI but remote operations are delegated to
// FakeGit. When using it in tests you must make sure you are operating on a temporary copy of a git repo NOT the
// real one on your disk (as it will get changed!).
// Faked out methods have the comment "Faked out"
type GitLocal struct {
	GitCLI  *GitCLI
	GitFake *GitFake
}

// NewGitLocal creates a new GitLocal instance
func NewGitLocal() *GitLocal {
	return &GitLocal{
		GitCLI:  NewGitCLI(),
		GitFake: &GitFake{},
	}
}

// FindGitConfigDir tries to find the `.git` directory either in the current directory or in parent directories
// Faked out
func (g *GitLocal) FindGitConfigDir(dir string) (string, string, error) {
	return g.GitCLI.FindGitConfigDir(dir)
}

func (g *GitLocal) Config(dir string, args ...string) error {
	return g.GitCLI.Config(dir, args...)
}

// Clone clones the given git URL into the given directory
// Faked out
func (g *GitLocal) Clone(url string, dir string) error {
	return g.GitFake.Clone(url, dir)
}

// ShallowCloneBranch clones a single branch of the given git URL into the given directory
// Faked out
func (g *GitLocal) ShallowCloneBranch(url string, branch string, dir string) error {
	return g.GitFake.ShallowCloneBranch(url, branch, dir)
}

// ShallowClone shallow clones the repo at url from the specified commitish or pull request to a local master branch
// Faked out
func (g *GitLocal) ShallowClone(dir string, url string, commitish string, pullRequest string) error {
	return g.GitFake.ShallowClone(dir, url, commitish, pullRequest)
}

// Pull pulls the Git repository in the given directory
// Faked out
func (g *GitLocal) Pull(dir string) error {
	return g.GitFake.Pull(dir)
}

// PullRemoteBranches pulls the remote Git tags from the given given directory
// Faked out
func (g *GitLocal) PullRemoteBranches(dir string) error {
	return g.GitFake.PullRemoteBranches(dir)
}

// DeleteRemoteBranch deletes the remote branch in the given given directory
// Faked out
func (g *GitLocal) DeleteRemoteBranch(dir string, remoteName string, branch string) error {
	return g.GitFake.DeleteRemoteBranch(dir, remoteName, branch)
}

// DeleteLocalBranch deletes a remote branch
func (g *GitLocal) DeleteLocalBranch(dir string, branch string) error {
	return g.GitFake.DeleteLocalBranch(dir, branch)
}

// CloneOrPull clones  the given git URL or pull if it already exists
// Faked out
func (g *GitLocal) CloneOrPull(url string, dir string) error {
	return g.GitFake.CloneOrPull(url, dir)
}

// PullUpstream pulls the remote upstream branch into master branch into the given directory
// Faked out
func (g *GitLocal) PullUpstream(dir string) error {
	return g.GitFake.PullUpstream(dir)
}

// ResetToUpstream resets the given branch to the upstream version
func (g *GitLocal) ResetToUpstream(dir string, branch string) error {
	return g.GitFake.ResetToUpstream(dir, branch)
}

// AddRemote adds a remote repository at the given URL and with the given name
func (g *GitLocal) AddRemote(dir string, name string, url string) error {
	return g.GitCLI.AddRemote(dir, name, url)
}

// UpdateRemote updates the URL of the remote repository
func (g *GitLocal) UpdateRemote(dir, url string) error {
	return g.GitCLI.UpdateRemote(dir, url)
}

// StashPush stashes the current changes from the given directory
func (g *GitLocal) StashPush(dir string) error {
	return g.GitCLI.StashPush(dir)
}

// CheckoutRemoteBranch checks out the given remote tracking branch
func (g *GitLocal) CheckoutRemoteBranch(dir string, branch string) error {
	return g.GitCLI.CheckoutRemoteBranch(dir, branch)
}

// RemoteBranches returns the remote branches
func (g *GitLocal) RemoteBranches(dir string) ([]string, error) {
	return g.GitCLI.RemoteBranches(dir)
}

// Checkout checks out the given branch
func (g *GitLocal) Checkout(dir string, branch string) error {
	return g.GitCLI.Checkout(dir, branch)
}

// CheckoutCommitFiles checks out the given files
func (g *GitLocal) CheckoutCommitFiles(dir string, commit string, files []string) error {
	return g.GitCLI.CheckoutCommitFiles(dir, commit, files)
}

// CheckoutOrphan checks out the given branch as an orphan
func (g *GitLocal) CheckoutOrphan(dir string, branch string) error {
	return g.GitCLI.CheckoutOrphan(dir, branch)
}

// Init inits a git repository into the given directory
func (g *GitLocal) Init(dir string) error {
	return g.GitCLI.Init(dir)
}

// Remove removes the given file from a Git repository located at the given directory
func (g *GitLocal) Remove(dir, fileName string) error {
	return g.GitCLI.Remove(dir, fileName)
}

// RemoveForce removes the given file from a git repository located at the given directory
func (g *GitLocal) RemoveForce(dir, fileName string) error {
	return g.GitCLI.RemoveForce(dir, fileName)
}

// CleanForce cleans a git repository located at a given directory
func (g *GitLocal) CleanForce(dir, fileName string) error {
	return g.CleanForce(dir, fileName)
}

// Status returns the status of the git repository at the given directory
func (g *GitLocal) Status(dir string) error {
	return g.GitCLI.Status(dir)
}

// Branch returns the current branch of the repository located at the given directory
func (g *GitLocal) Branch(dir string) (string, error) {
	return g.GitCLI.Branch(dir)
}

// Push pushes the changes from the repository at the given directory
// Faked out
func (g *GitLocal) Push(dir string, remote string, force bool, refspec ...string) error {
	return g.GitFake.Push(dir, "origin", false)
}

// ForcePushBranch does a force push of the local branch into the remote branch of the repository at the given directory
// Faked out
func (g *GitLocal) ForcePushBranch(dir string, localBranch string, remoteBranch string) error {
	return g.GitFake.ForcePushBranch(dir, localBranch, remoteBranch)
}

// PushMaster pushes the master branch into the origin
// Faked out
func (g *GitLocal) PushMaster(dir string) error {
	return g.GitFake.PushMaster(dir)
}

// PushTag pushes the given tag into the origin
// Faked out
func (g *GitLocal) PushTag(dir string, tag string) error {
	return g.GitFake.PushTag(dir, tag)
}

// Add does a git add for all the given arguments
func (g *GitLocal) Add(dir string, args ...string) error {
	return g.GitCLI.Add(dir, args...)
}

// HasChanges indicates if there are any changes in the repository from the given directory
func (g *GitLocal) HasChanges(dir string) (bool, error) {
	return g.GitCLI.HasChanges(dir)
}

// HasFileChanged returns true if file has changes in git
func (g *GitLocal) HasFileChanged(dir string, fileName string) (bool, error) {
	return g.GitCLI.HasFileChanged(dir, fileName)
}

// CommitIfChanges does a commit if there are any changes in the repository at the given directory
func (g *GitLocal) CommitIfChanges(dir string, message string) error {
	return g.GitCLI.CommitIfChanges(dir, message)
}

// CommitDir commits all changes from the given directory
func (g *GitLocal) CommitDir(dir string, message string) error {
	return g.GitCLI.CommitDir(dir, message)
}

// AddCommit perform an add and commit of the changes from the repository at the given directory with the given messages
func (g *GitLocal) AddCommit(dir string, msg string) error {
	return g.GitCLI.AddCommit(dir, msg)
}

// CreateAuthenticatedURL creates the Git repository URL with the username and password encoded for HTTPS based URLs
func (g *GitLocal) CreateAuthenticatedURL(cloneURL string, userAuth *auth.UserAuth) (string, error) {
	return g.GitCLI.CreateAuthenticatedURL(cloneURL, userAuth)
}

// AddCommitFiles add files to a commit
func (g *GitLocal) AddCommitFiles(dir string, msg string, files []string) error {
	return g.GitCLI.AddCommitFiles(dir, msg, files)
}

// RepoName formats the repository names based on the organization
func (g *GitLocal) RepoName(org, repoName string) string {
	return g.GitCLI.RepoName(org, repoName)
}

// Server returns the Git server of the repository at the given directory
func (g *GitLocal) Server(dir string) (string, error) {
	return g.GitCLI.Server(dir)
}

// Info returns the git info of the repository at the given directory
func (g *GitLocal) Info(dir string) (*GitRepository, error) {
	return g.GitCLI.Info(dir)
}

// ConvertToValidBranchName converts the given branch name into a valid git branch string
// replacing any dodgy characters
func (g *GitLocal) ConvertToValidBranchName(name string) string {
	return g.GitCLI.ConvertToValidBranchName(name)
}

// FetchBranch fetches a branch
// Faked out
func (g *GitLocal) FetchBranch(dir string, repo string, refspec ...string) error {
	return g.GitFake.FetchBranch(dir, repo, refspec...)
}

// FetchBranchShallow fetches a branch
// Faked out
func (g *GitLocal) FetchBranchShallow(dir string, repo string, refspec ...string) error {
	return g.GitFake.FetchBranchShallow(dir, repo, refspec...)
}

// FetchBranchUnshallow fetches a branch
// Faked out
func (g *GitLocal) FetchBranchUnshallow(dir string, repo string, refspec ...string) error {
	return g.GitFake.FetchBranch(dir, repo, refspec...)
}

// GetAuthorEmailForCommit returns the author email from commit message with the given SHA
func (g *GitLocal) GetAuthorEmailForCommit(dir string, sha string) (string, error) {
	return g.GitCLI.GetAuthorEmailForCommit(dir, sha)
}

// SetRemoteURL sets the remote URL of the remote with the given name
func (g *GitLocal) SetRemoteURL(dir string, name string, gitURL string) error {
	return g.GitCLI.SetRemoteURL(dir, name, gitURL)
}

// DiscoverRemoteGitURL discovers the remote git URL from the given git configuration
func (g *GitLocal) DiscoverRemoteGitURL(gitConf string) (string, error) {
	return g.GitCLI.DiscoverRemoteGitURL(gitConf)
}

// DiscoverUpstreamGitURL discovers the upstream git URL from the given git configuration
func (g *GitLocal) DiscoverUpstreamGitURL(gitConf string) (string, error) {
	return g.GitCLI.DiscoverUpstreamGitURL(gitConf)
}

// GetRemoteUrl returns the remote URL from the given git config
func (g *GitLocal) GetRemoteUrl(config *gitcfg.Config, name string) string {
	return g.GitCLI.GetRemoteUrl(config, name)
}

// RemoteBranchNames returns all remote branch names with the given prefix
func (g *GitLocal) RemoteBranchNames(dir string, prefix string) ([]string, error) {
	return g.GitCLI.RemoteBranchNames(dir, prefix)
}

// RemoteMergedBranchNames returns all remote branch names with the given prefix
func (g *GitLocal) RemoteMergedBranchNames(dir string, prefix string) ([]string, error) {
	return g.GitCLI.RemoteMergedBranchNames(dir, prefix)
}

// GetCommitPointedToByPreviousTag returns the previous git tag from the repository at the given directory
func (g *GitLocal) GetCommitPointedToByPreviousTag(dir string) (string, string, error) {
	return g.GitCLI.GetCommitPointedToByPreviousTag(dir)
}

// GetRevisionBeforeDate returns the revision before the given date
func (g *GitLocal) GetRevisionBeforeDate(dir string, t time.Time) (string, error) {
	return g.GitCLI.GetRevisionBeforeDate(dir, t)
}

// GetRevisionBeforeDateText returns the revision before the given date in format "MonthName dayNumber year"
func (g *GitLocal) GetRevisionBeforeDateText(dir string, dateText string) (string, error) {
	return g.GitCLI.GetRevisionBeforeDateText(dir, dateText)
}

// GetCommitPointedToByLatestTag return the SHA of the current git tag from the repository at the given directory
func (g *GitLocal) GetCommitPointedToByLatestTag(dir string) (string, string, error) {
	return g.GitCLI.GetCommitPointedToByLatestTag(dir)
}

// GetCommitPointedToByTag return the SHA of the commit pointed to by the given git tag
func (g *GitLocal) GetCommitPointedToByTag(dir string, tag string) (string, error) {
	return g.GitCLI.GetCommitPointedToByTag(dir, tag)
}

// GetLatestCommitMessage returns the latest git commit message
func (g *GitLocal) GetLatestCommitMessage(dir string) (string, error) {
	return g.GitCLI.GetLatestCommitMessage(dir)
}

// FetchTags fetches all the tags
// Faked out
func (g *GitLocal) FetchTags(dir string) error {
	return g.GitFake.FetchTags(dir)
}

// FetchRemoteTags fetches all the tags from a remote repository
// Faked out
func (g *GitLocal) FetchRemoteTags(dir string, repo string) error {
	return g.GitFake.FetchRemoteTags(dir, repo)
}

// Tags returns all tags from the repository at the given directory
func (g *GitLocal) Tags(dir string) ([]string, error) {
	return g.GitCLI.Tags(dir)
}

// FilterTags returns all tags from the repository at the given directory that match the filter
func (g *GitLocal) FilterTags(dir string, filter string) ([]string, error) {
	return g.GitCLI.FilterTags(dir, filter)
}

// CreateTag creates a tag with the given name and message in the repository at the given directory
func (g *GitLocal) CreateTag(dir string, tag string, msg string) error {
	return g.GitCLI.CreateTag(dir, tag, msg)
}

// PrintCreateRepositoryGenerateAccessToken prints the access token URL of a Git repository
func (g *GitLocal) PrintCreateRepositoryGenerateAccessToken(server *auth.AuthServer, username string, o io.Writer) {
	g.GitCLI.PrintCreateRepositoryGenerateAccessToken(server, username, o)
}

// IsFork indicates if the repository at the given directory is a fork
func (g *GitLocal) IsFork(dir string) (bool, error) {
	return g.GitCLI.IsFork(dir)
}

// Version returns the git version
func (g *GitLocal) Version() (string, error) {
	return g.GitCLI.Version()
}

// Username return the username from the git configuration
// Faked out
func (g *GitLocal) Username(dir string) (string, error) {
	// Use GitFake as this is a global setting
	return g.GitFake.Username(dir)
}

// SetUsername sets the username in the git configuration
// Faked out
func (g *GitLocal) SetUsername(dir string, username string) error {
	// Use GitFake as this is a global setting
	return g.GitFake.SetUsername(dir, username)
}

// Email returns the email from the git configuration
// Faked out
func (g *GitLocal) Email(dir string) (string, error) {
	// Use GitFake as this is a global setting
	return g.GitFake.Email(dir)
}

// SetEmail sets the given email in the git configuration
// Faked out
func (g *GitLocal) SetEmail(dir string, email string) error {
	// Use GitFake as this is a global setting
	return g.GitFake.SetEmail(dir, email)
}

// CreateBranch creates a branch with the given name in the Git repository from the given directory
func (g *GitLocal) CreateBranch(dir string, branch string) error {
	return g.GitCLI.CreateBranch(dir, branch)
}

// Diff runs git diff
func (g *GitLocal) Diff(dir string) (string, error) {
	return g.GitCLI.Diff(dir)
}

// ListChangedFilesFromBranch lists files changed between branches
func (g *GitLocal) ListChangedFilesFromBranch(dir string, branch string) (string, error) {
	return g.GitCLI.ListChangedFilesFromBranch(dir, branch)
}

// LoadFileFromBranch returns a files's contents from a branch
func (g *GitLocal) LoadFileFromBranch(dir string, branch string, file string) (string, error) {
	return g.GitCLI.LoadFileFromBranch(dir, branch, file)
}

// FetchUnshallow does nothing
// Faked out
func (g *GitLocal) FetchUnshallow(dir string) error {
	return g.GitFake.FetchUnshallow(dir)
}

// IsShallow runs git rev-parse --is-shalllow-repository in dir
func (g *GitLocal) IsShallow(dir string) (bool, error) {
	return g.GitCLI.IsShallow(dir)
}

// CreateBranchFrom creates a new branch called branchName from startPoint
func (g *GitLocal) CreateBranchFrom(dir string, branchName string, startPoint string) error {
	return g.GitCLI.CreateBranchFrom(dir, branchName, startPoint)
}

// Merge merges the commitish into the current branch
func (g *GitLocal) Merge(dir string, commitish string) error {
	return g.GitCLI.Merge(dir, commitish)
}

// GetLatestCommitSha returns the sha of the last commit
func (g *GitLocal) GetLatestCommitSha(dir string) (string, error) {
	return g.GitCLI.GetLatestCommitSha(dir)
}

// GetFirstCommitSha gets the first commit sha
func (g *GitLocal) GetFirstCommitSha(dir string) (string, error) {
	return g.GitCLI.GetFirstCommitSha(dir)
}

// Reset performs a git reset --hard back to the commitish specified
func (g *GitLocal) Reset(dir string, commitish string, hard bool) error {
	return g.GitCLI.Reset(dir, commitish, true)
}

// RemoteUpdate performs a git remote update
// Faked out
func (g *GitLocal) RemoteUpdate(dir string) error {
	return g.GitFake.RemoteUpdate(dir)
}

// LocalBranches will list all local branches
func (g *GitLocal) LocalBranches(dir string) ([]string, error) {
	return g.GitCLI.LocalBranches(dir)
}

// MergeTheirs performs a cherry pick of commitish
func (g *GitLocal) MergeTheirs(dir string, commitish string) error {
	return g.GitCLI.MergeTheirs(dir, commitish)
}

// RebaseTheirs runs git rebase upstream branch
func (g *GitLocal) RebaseTheirs(dir string, upstream string, branch string, skipEmpty bool) error {
	return g.GitCLI.RebaseTheirs(dir, upstream, branch, false)
}

// GetCommits returns the commits in a range, exclusive of startSha and inclusive of endSha
func (g *GitLocal) GetCommits(dir string, startSha string, endSha string) ([]GitCommit, error) {
	return g.GitCLI.GetCommits(dir, startSha, endSha)
}

// RevParse runs git rev parse
func (g *GitLocal) RevParse(dir string, rev string) (string, error) {
	return g.GitCLI.RevParse(dir, rev)
}

// SetUpstreamTo will set the given branch to track the origin branch with the same name
func (g *GitLocal) SetUpstreamTo(dir string, branch string) error {
	return g.GitCLI.SetUpstreamTo(dir, branch)
}

// Remotes will list the names of the remotes
func (g *GitLocal) Remotes(dir string) ([]string, error) {
	return g.GitCLI.Remotes(dir)
}

// StashPop runs git stash pop
func (g *GitLocal) StashPop(dir string) error {
	return g.GitCLI.StashPop(dir)
}

// CloneBare does nothing
func (g *GitLocal) CloneBare(dir string, url string) error {
	return nil
}

// PushMirror does nothing
func (g *GitLocal) PushMirror(dir string, url string) error {
	return nil
}

// GetCommitsNotOnAnyRemote returns a list of commits which are on branch but not present on a remote
func (g *GitLocal) GetCommitsNotOnAnyRemote(dir string, branch string) ([]GitCommit, error) {
	return g.GitCLI.GetCommitsNotOnAnyRemote(dir, branch)
}

// CherryPick does a git cherry-pick of commit
func (g *GitLocal) CherryPick(dir string, commit string) error {
	return g.GitCLI.CherryPick(dir, commit)
}

// CherryPickTheirs does a git cherry-pick of commit
func (g *GitLocal) CherryPickTheirs(dir string, commit string) error {
	return g.GitCLI.CherryPickTheirs(dir, commit)
}

// CherryPickTheirsKeepRedundantCommits does a git cherry-pick of commit
func (g *GitLocal) CherryPickTheirsKeepRedundantCommits(dir string, commit string) error {
	return g.GitCLI.CherryPickTheirsKeepRedundantCommits(dir, commit)
}

// Describe does a git describe of commitish, optionally adding the abbrev arg if not empty
func (g *GitLocal) Describe(dir string, contains bool, commitish string, abbrev string, fallback bool) (string, string, error) {
	return g.GitCLI.Describe(dir, false, commitish, abbrev, fallback)
}

// IsAncestor checks if the possible ancestor commit-ish is an ancestor of the given commit-ish.
func (g *GitLocal) IsAncestor(dir string, possibleAncestor string, commitish string) (bool, error) {
	return g.GitCLI.IsAncestor(dir, possibleAncestor, commitish)
}

// WriteRepoAttributes writes the given content to .git/info/attributes
func (g *GitLocal) WriteRepoAttributes(dir string, content string) error {
	return g.GitCLI.WriteRepoAttributes(dir, content)
}

// ReadRepoAttributes reads the existing content, if any, in .git/info/attributes
func (g *GitLocal) ReadRepoAttributes(dir string) (string, error) {
	return g.GitCLI.ReadRepoAttributes(dir)
}
