package gits

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	gitcfg "gopkg.in/src-d/go-git.v4/config"
)

const (
	replaceInvalidBranchChars = '_'
)

var (
	numberRegex        = regexp.MustCompile("[0-9]")
	splitDescribeRegex = regexp.MustCompile(`(?:~|\^|-g)`)
)

// GitCLI implements common git actions based on git CLI
type GitCLI struct {
	Env map[string]string
}

// NewGitCLI creates a new GitCLI instance
func NewGitCLI() *GitCLI {
	cli := &GitCLI{
		Env: map[string]string{},
	}
	// Ensure that error output is in English so parsing work
	cli.Env["LC_ALL"] = "C"
	// When jx is called as credential helper we want to make sure that potential debug trace is not interfering with the process
	cli.Env["JX_LOG_LEVEL"] = "error"
	return cli
}

// Config runs a 'git config' command in the specified directory
func (g *GitCLI) Config(dir string, args ...string) error {
	if args == nil {
		args = []string{"config"}
	} else {
		args = append([]string{"config"}, args...)
	}
	return g.gitCmd(dir, args...)
}

// FindGitConfigDir tries to find the `.git` directory either in the current directory or in parent directories
func (g *GitCLI) FindGitConfigDir(dir string) (string, string, error) {
	d := dir
	var err error
	if d == "" {
		d, err = os.Getwd()
		if err != nil {
			return "", "", err
		}
	}
	for {
		gitDir := filepath.Join(d, ".git/config")
		f, err := util.FileExists(gitDir)
		if err != nil {
			return "", "", err
		}
		if f {
			return d, gitDir, nil
		}
		dirPath := strings.TrimSuffix(d, "/")
		if dirPath == "" {
			return "", "", nil
		}
		p, _ := filepath.Split(dirPath)
		if d == "/" || p == d {
			return "", "", nil
		}
		d = p
	}
}

// Clone clones the given git URL into the given directory
func (g *GitCLI) Clone(url string, dir string) error {
	return g.clone(dir, url, "", false, false, "", "", "")
}

// Clone clones a single branch of the given git URL into the given directory
func (g *GitCLI) ShallowCloneBranch(gitURL string, branch string, dir string) error {
	var err error
	verbose := true
	remoteName := "origin"
	shallow := true
	err = g.Init(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to init a new git repository in directory %s", dir)
	}
	if verbose {
		log.Logger().Infof("ran git init in %s", dir)
	}
	err = g.AddRemote(dir, "origin", gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to add remote %s with url %s in directory %s", remoteName, gitURL, dir)
	}
	if verbose {
		log.Logger().Infof("ran git add remote %s %s in %s", remoteName, gitURL, dir)
	}

	err = g.fetchBranch(dir, remoteName, false, shallow, verbose, branch)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s from %s in directory %s", branch, gitURL,
			dir)
	}
	err = g.gitCmd(dir, "checkout", "-t", fmt.Sprintf("%s/%s", remoteName, branch))
	if err != nil {
		log.Logger().Warnf("failed to checkout remote tracking branch %s/%s in directory %s due to: %s", remoteName,
			branch, dir, err.Error())
		if branch != "master" {
			// git init checks out the master branch by default
			err = g.CreateBranch(dir, branch)
			if err != nil {
				return errors.Wrapf(err, "failed to create branch %s in directory %s", branch, dir)
			}

			if verbose {
				log.Logger().Infof("ran git branch %s in directory %s", branch, dir)
			}
		}
		err = g.Reset(dir, fmt.Sprintf("%s/%s", remoteName, branch), true)
		if err != nil {
			return errors.Wrapf(err, "failed to reset hard to %s in directory %s", branch, dir)
		}
		err = g.gitCmd(dir, "branch", "--set-upstream-to", fmt.Sprintf("%s/%s", remoteName, branch), branch)
		if err != nil {
			return errors.Wrapf(err, "failed to set tracking information to %s/%s %s in directory %s", remoteName,
				branch, branch, dir)
		}
	}
	return nil
}

// ShallowClone shallow clones the repo at url from the specified commitish or pull request to a local master branch
func (g *GitCLI) ShallowClone(dir string, url string, commitish string, pullRequest string) error {
	return g.clone(dir, url, "", true, false, "master", commitish, pullRequest)
}

// clone is a safer implementation of the `git clone` method
func (g *GitCLI) clone(dir string, gitURL string, remoteName string, shallow bool, verbose bool, localBranch string,
	commitish string, pullRequest string) error {
	var err error
	if verbose {
		log.Logger().Infof("cloning repository %s to dir %s", gitURL, dir)
	}
	if remoteName == "" {
		remoteName = "origin"
	}
	if commitish == "" {
		if pullRequest == "" {
			commitish = "master"
		} else {
			pullRequestNumber, err := strconv.Atoi(strings.TrimPrefix(pullRequest, "PR-"))
			if err != nil {
				return errors.Wrapf(err, "converting %s to a pull request number", pullRequest)
			}
			fmt.Sprintf("refs/pull/%d/head", pullRequestNumber)
		}
	} else if pullRequest != "" {
		return errors.Errorf("cannot specify both pull request and commitish")
	}
	if localBranch == "" {
		localBranch = commitish
	}

	err = g.Init(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to init a new git repository in directory %s", dir)
	}
	if verbose {
		log.Logger().Infof("ran git init in %s", dir)
	}
	err = g.AddRemote(dir, "origin", gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to add remote %s with url %s in directory %s", remoteName, gitURL, dir)
	}
	if verbose {
		log.Logger().Infof("ran git add remote %s %s in %s", remoteName, gitURL, dir)
	}

	err = g.fetchBranch(dir, remoteName, false, shallow, verbose, commitish)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch %s from %s in directory %s", commitish, gitURL,
			dir)
	}
	if localBranch != "master" {
		// git init checks out the master branch by default
		err = g.CreateBranch(dir, localBranch)
		if err != nil {
			return errors.Wrapf(err, "failed to create branch %s in directory %s", localBranch, dir)
		}

		if verbose {
			log.Logger().Infof("ran git branch %s in directory %s", localBranch, dir)
		}
	}
	if commitish == "" {
		commitish = localBranch
		if commitish == "" {
			commitish = "master"
		}
	}
	err = g.Reset(dir, fmt.Sprintf("%s/%s", remoteName, commitish), true)
	if err != nil {
		return errors.Wrapf(err, "failed to reset hard to %s in directory %s", commitish, dir)
	}
	if verbose {
		log.Logger().Infof("ran git reset --hard %s in directory %s", commitish, dir)
	}
	err = g.gitCmd(dir, "branch", "--set-upstream-to", fmt.Sprintf("%s/%s", remoteName, commitish), localBranch)
	if err != nil {
		return errors.Wrapf(err, "failed to set tracking information to %s/%s %s in directory %s", remoteName,
			commitish, localBranch, dir)
	}
	if verbose {
		log.Logger().Infof("ran git branch --set-upstream-to %s/%s %s in directory %s", remoteName, commitish,
			localBranch, dir)
	}
	return nil
}

// Pull pulls the Git repository in the given directory
func (g *GitCLI) Pull(dir string) error {
	return g.gitCmd(dir, "pull")
}

// PullRemoteBranches pulls the remote Git tags from the given directory
func (g *GitCLI) PullRemoteBranches(dir string) error {
	return g.gitCmd(dir, "pull", "--all")
}

// DeleteRemoteBranch deletes the remote branch in the given directory
func (g *GitCLI) DeleteRemoteBranch(dir string, remoteName string, branch string) error {
	return g.gitCmd(dir, "push", remoteName, "--delete", branch)
}

// DeleteLocalBranch deletes the local branch in the given directory
func (g *GitCLI) DeleteLocalBranch(dir string, branch string) error {
	return g.gitCmd(dir, "branch", "-D", branch)
}

// CloneOrPull clones  the given git URL or pull if it already exists
func (g *GitCLI) CloneOrPull(url string, dir string) error {
	empty, err := util.IsEmpty(dir)
	if err != nil {
		return err
	}

	if !empty {
		return g.Pull(dir)
	}
	return g.Clone(url, dir)
}

// PullUpstream pulls the remote upstream branch into master branch into the given directory
func (g *GitCLI) PullUpstream(dir string) error {
	return g.gitCmd(dir, "pull", "-r", "upstream", "master")
}

// ResetToUpstream resets the given branch to the upstream version
func (g *GitCLI) ResetToUpstream(dir string, branch string) error {
	err := g.gitCmd(dir, "fetch", "upstream")
	if err != nil {
		return err
	}
	return g.gitCmd(dir, "reset", "--hard", "upstream/"+branch)
}

// AddRemote adds a remote repository at the given URL and with the given name
func (g *GitCLI) AddRemote(dir string, name string, url string) error {
	err := g.gitCmd(dir, "remote", "add", name, url)
	if err != nil {
		err = g.gitCmd(dir, "remote", "set-url", name, url)
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateRemote updates the URL of the remote repository
func (g *GitCLI) UpdateRemote(dir, url string) error {
	return g.gitCmd(dir, "remote", "set-url", "origin", url)
}

// RemoteUpdate performs a git remote update
func (g *GitCLI) RemoteUpdate(dir string) error {
	return g.gitCmd(dir, "remote", "update")
}

// StashPush stashes the current changes from the given directory
func (g *GitCLI) StashPush(dir string) error {
	return g.gitCmd(dir, "stash", "push")
}

// StashPop applies the last stash , will error if there is no stash. Error can be checked using IsNoStashEntriesError
func (g *GitCLI) StashPop(dir string) error {
	return g.gitCmd(dir, "stash", "pop")
}

// CheckoutRemoteBranch checks out the given remote tracking branch
func (g *GitCLI) CheckoutRemoteBranch(dir string, branch string) error {
	remoteBranch := "origin/" + branch
	remoteBranches, err := g.RemoteBranches(dir)
	if err != nil {
		return err
	}
	if util.StringArrayIndex(remoteBranches, remoteBranch) < 0 {
		return g.gitCmd(dir, "checkout", "-t", remoteBranch)
	}
	cur, err := g.Branch(dir)
	if err != nil {
		return err
	}
	if cur == branch {
		return nil
	}
	return g.Checkout(dir, branch)
}

// RemoteBranches returns the remote branches
func (g *GitCLI) RemoteBranches(dir string) ([]string, error) {
	answer := []string{}
	text, err := g.gitCmdWithOutput(dir, "branch", "-r")
	if err != nil {
		return answer, err
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		columns := strings.Split(line, " ")
		for _, col := range columns {
			if col != "" {
				answer = append(answer, col)
				break
			}
		}
	}
	return answer, nil
}

// LocalBranches will list all local branches
func (g *GitCLI) LocalBranches(dir string) ([]string, error) {
	text, err := g.gitCmdWithOutput(dir, "branch")
	answer := make([]string, 0)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		columns := strings.Split(line, " ")
		for _, col := range columns {
			if col != "" && col != "*" {
				answer = append(answer, col)
				break
			}
		}
	}
	return answer, nil
}

// Checkout checks out the given branch
func (g *GitCLI) Checkout(dir string, branch string) error {
	return g.gitCmd(dir, "checkout", branch)
}

// CheckoutCommitFiles checks out the given files to a commit
func (g *GitCLI) CheckoutCommitFiles(dir string, commit string, files []string) error {
	var err error
	for _, file := range files {
		err = g.gitCmd(dir, "checkout", commit, "--", file)
	}
	return err
}

// Checkout checks out the given branch
func (g *GitCLI) CheckoutOrphan(dir string, branch string) error {
	return g.gitCmd(dir, "checkout", "--orphan", branch)
}

// Init inits a git repository into the given directory
func (g *GitCLI) Init(dir string) error {
	return g.gitCmd(dir, "init")
}

// Remove removes the given file from a Git repository located at the given directory
func (g *GitCLI) Remove(dir, fileName string) error {
	return g.gitCmd(dir, "rm", "-r", fileName)
}

// Remove force removes the given file from a git repository located at the given directory
func (g *GitCLI) RemoveForce(dir, fileName string) error {
	return g.gitCmd(dir, "rm", "-rf", fileName)
}

// Clean force cleans a git repository located at a given directory
func (g *GitCLI) CleanForce(dir, fileName string) error {
	return g.gitCmd(dir, "clean", "-fd", fileName)
}

// Status returns the status of the git repository at the given directory
func (g *GitCLI) Status(dir string) error {
	return g.gitCmd(dir, "status")
}

// Branch returns the current branch of the repository located at the given directory
func (g *GitCLI) Branch(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// WriteOperation performs a generic write operation, with nicer error handling
func (g *GitCLI) WriteOperation(dir string, args ...string) error {
	return errors.Wrap(g.gitCmd(dir, args...),
		"Have you set up a git credential helper? See https://help.github.com/articles/caching-your-github-password-in-git/\n")
}

// Push pushes the changes from the repository at the given directory
func (g *GitCLI) Push(dir string, remote string, force bool, refspec ...string) error {
	args := []string{"push", remote}
	if force {
		args = append(args, "--force")
	}
	args = append(args, refspec...)
	return g.WriteOperation(dir, args...)
}

// ForcePushBranch does a force push of the local branch into the remote branch of the repository at the given directory
func (g *GitCLI) ForcePushBranch(dir string, localBranch string, remoteBranch string) error {
	return g.Push(dir, "origin", true, fmt.Sprintf("%s:%s", localBranch, remoteBranch))
}

// PushMaster pushes the master branch into the origin, setting the upstream
func (g *GitCLI) PushMaster(dir string) error {
	return g.Push(dir, "origin", false, "master")
}

// Pushtag pushes the given tag into the origin
func (g *GitCLI) PushTag(dir string, tag string) error {
	return g.Push(dir, "origin", false, tag)
}

// Add does a git add for all the given arguments
func (g *GitCLI) Add(dir string, args ...string) error {
	add := append([]string{"add"}, args...)
	return g.gitCmd(dir, add...)
}

// HasChanges indicates if there are any changes in the repository from the given directory
func (g *GitCLI) HasChanges(dir string) (bool, error) {
	text, err := g.gitCmdWithOutput(dir, "status", "-s")
	if err != nil {
		return false, err
	}
	text = strings.TrimSpace(text)
	return len(text) > 0, nil
}

// HasFileChanged indicates if there are any changes to a file in the repository from the given directory
func (g *GitCLI) HasFileChanged(dir string, fileName string) (bool, error) {
	text, err := g.gitCmdWithOutput(dir, "status", "-s", fileName)
	if err != nil {
		return false, err
	}
	text = strings.TrimSpace(text)
	return len(text) > 0, nil
}

// CommiIfChanges does a commit if there are any changes in the repository at the given directory
func (g *GitCLI) CommitIfChanges(dir string, message string) error {
	changed, err := g.HasChanges(dir)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return g.CommitDir(dir, message)
}

// GetCommits returns the commits in a range, exclusive of startSha and inclusive of endSha
func (g *GitCLI) GetCommits(dir string, startSha string, endSha string) ([]GitCommit, error) {
	return g.getCommits(dir, fmt.Sprintf("%s..%s", startSha, endSha))
}
func (g *GitCLI) getCommits(dir string, args ...string) ([]GitCommit, error) {
	// use a custom format to get commits, using %x1e to separate commits and %x1f to separate fields
	args = append([]string{"log", "--format=%H%x1f%an%x1f%ae%x1f%cn%x1f%ce%x1f%s%n%b%x1e"}, args...)
	out, err := g.gitCmdWithOutput(dir, args...)
	if err != nil {
		return nil, errors.Wrapf(err, "running git %s", strings.Join(args, " "))
	}
	answer := make([]GitCommit, 0)
	commits := strings.Split(out, "\x1e")
	for _, rawCommit := range commits {
		rawCommit = strings.TrimSpace(rawCommit)
		if rawCommit == "" {
			continue
		}
		fields := strings.Split(rawCommit, "\x1f")
		commit := GitCommit{}
		commit.SHA = fields[0]

		commit.Author = &GitUser{
			Name:  fields[1],
			Email: fields[2],
		}

		commit.Committer = &GitUser{
			Name:  fields[3],
			Email: fields[4],
		}

		commit.Message = fields[5]
		answer = append(answer, commit)
	}
	return answer, nil
}

// GetCommitsNotOnAnyRemote returns a list of commits which are on branch but not present on a remoteGet

func (g *GitCLI) GetCommitsNotOnAnyRemote(dir string, branch string) ([]GitCommit, error) {
	return g.getCommits(dir, branch, "--not", "--remotes")
}

// CommitDir commits all changes from the given directory
func (g *GitCLI) CommitDir(dir string, message string) error {
	return g.gitCmd(dir, "commit", "-m", message)
}

// AddCommit perform an add and commit of the changes from the repository at the given directory with the given messages
func (g *GitCLI) AddCommit(dir string, msg string) error {
	return g.gitCmd(dir, "commit", "-a", "-m", msg, "--allow-empty")
}

// AddCommitFiles perform an add and commit selected files from the repository at the given directory with the given messages
func (g *GitCLI) AddCommitFiles(dir string, msg string, files []string) error {
	for _, file := range files {
		err := g.Add(dir, file)
		if err != nil {
			return err
		}
	}
	return g.gitCmd(dir, "commit", "-m", msg)
}

func (g *GitCLI) gitCmd(dir string, args ...string) error {
	cmd := util.Command{
		Dir:  dir,
		Name: "git",
		Args: args,
		Env:  g.Env,
	}
	log.Logger().Debug(cmd.String())
	output, err := cmd.RunWithoutRetry()
	return errors.Wrapf(err, "git output: %s", output)
}

func (g *GitCLI) gitCmdWithOutput(dir string, args ...string) (string, error) {
	cmd := util.Command{
		Dir:  dir,
		Name: "git",
		Args: args,
		Env:  g.Env,
	}
	log.Logger().Debug(cmd.String())
	return cmd.RunWithoutRetry()
}

// CreateAuthenticatedURL creates the Git repository URL with the username and password encoded for HTTPS based URLs
func (g *GitCLI) CreateAuthenticatedURL(cloneURL string, userAuth *auth.UserAuth) (string, error) {
	u, err := url.Parse(cloneURL)
	if err != nil {
		// already a git/ssh url?
		return cloneURL, nil
	}
	// The file scheme doesn't support auth
	if u.Scheme == "file" {
		return cloneURL, nil
	}
	if userAuth.Username != "" || userAuth.ApiToken != "" {
		u.User = url.UserPassword(userAuth.Username, userAuth.ApiToken)
		return u.String(), nil
	}
	return cloneURL, nil
}

// RepoName formats the repository names based on the organization
func (g *GitCLI) RepoName(org, repoName string) string {
	if org != "" {
		return org + "/" + repoName
	}
	return repoName
}

// Server returns the Git server of the repository at the given directory
func (g *GitCLI) Server(dir string) (string, error) {
	repo, err := g.Info(dir)
	if err != nil {
		return "", err
	}
	return repo.HostURL(), err
}

// Info returns the git info of the repository at the given directory
func (g *GitCLI) Info(dir string) (*GitRepository, error) {
	text, err := g.gitCmdWithOutput(dir, "status")
	var rUrl string
	if err != nil && strings.Contains(text, "Not a git repository") {
		rUrl = os.Getenv("SOURCE_URL")
		if rUrl == "" {
			// Relevant in a Jenkins pipeline triggered by a PR
			rUrl = os.Getenv("CHANGE_URL")
			if rUrl == "" {
				return nil, fmt.Errorf("you are not in a Git repository - promotion command should be executed from an application directory")
			}
		}
	} else {
		text, err = g.gitCmdWithOutput(dir, "config", "--get", "remote.origin.url")
		rUrl = strings.TrimSpace(text)
	}

	repo, err := ParseGitURL(rUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Git URL %s due to %s", rUrl, err)
	}
	return repo, err
}

// ConvertToValidBranchName converts the given branch name into a valid git branch string
// replacing any dodgy characters
func (g *GitCLI) ConvertToValidBranchName(name string) string {
	name = strings.TrimSuffix(name, "/")
	name = strings.TrimSuffix(name, ".lock")
	var buffer bytes.Buffer

	last := ' '
	for _, ch := range name {
		if ch <= 32 {
			ch = replaceInvalidBranchChars
		}
		switch ch {
		case '~':
			ch = replaceInvalidBranchChars
		case '^':
			ch = replaceInvalidBranchChars
		case ':':
			ch = replaceInvalidBranchChars
		case ' ':
			ch = replaceInvalidBranchChars
		case '\n':
			ch = replaceInvalidBranchChars
		case '\r':
			ch = replaceInvalidBranchChars
		case '\t':
			ch = replaceInvalidBranchChars
		}
		if ch != replaceInvalidBranchChars || last != replaceInvalidBranchChars {
			buffer.WriteString(string(ch))
		}
		last = ch
	}
	return buffer.String()
}

// FetchBranch fetches the refspecs from the repo
func (g *GitCLI) FetchBranch(dir string, repo string, refspecs ...string) error {
	return g.fetchBranch(dir, repo, false, false, false, refspecs...)
}

// FetchBranchShallow fetches the refspecs from the repo
func (g *GitCLI) FetchBranchShallow(dir string, repo string, refspecs ...string) error {
	return g.fetchBranch(dir, repo, false, true, false, refspecs...)
}

// FetchBranch fetches the refspecs from the repo
func (g *GitCLI) FetchBranchUnshallow(dir string, repo string, refspecs ...string) error {
	return g.fetchBranch(dir, repo, true, false, false, refspecs...)
}

// FetchBranch fetches the refspecs from the repo
func (g *GitCLI) fetchBranch(dir string, repo string, unshallow bool, shallow bool,
	verbose bool, refspecs ...string) error {
	args := []string{"fetch", repo}
	if shallow && unshallow {
		return errors.Errorf("cannot use --depth=1 and --unshallow at the same time")
	}
	if shallow {
		args = append(args, "--depth=1")
	}
	if unshallow {
		args = append(args, "--unshallow")
	}
	for _, refspec := range refspecs {
		args = append(args, refspec)
	}
	err := g.gitCmd(dir, args...)
	if err != nil {
		return errors.WithStack(err)
	}
	if verbose {
		if shallow {
			log.Logger().Infof("ran git fetch %s --depth=1 %s in dir %s", repo, strings.Join(refspecs, " "), dir)
		} else if unshallow {
			log.Logger().Infof("ran git fetch %s unshallow %s in dir %s", repo, strings.Join(refspecs, " "), dir)
		} else {
			log.Logger().Infof("ran git fetch %s --depth=1 %s in dir %s", repo, strings.Join(refspecs, " "), dir)
		}

	}
	return nil
}

// GetAuthorEmailForCommit returns the author email from commit message with the given SHA
func (g *GitCLI) GetAuthorEmailForCommit(dir string, sha string) (string, error) {
	text, err := g.gitCmdWithOutput(dir, "show", "-s", "--format=%aE", sha)
	if err != nil {
		return "", fmt.Errorf("failed to invoke git %s in %s due to %s", "show "+sha, dir, err)
	}

	return strings.TrimSpace(text), nil
}

// SetRemoteURL sets the remote URL of the remote with the given name
func (g *GitCLI) SetRemoteURL(dir string, name string, gitURL string) error {
	err := g.gitCmd(dir, "remote", "add", name, gitURL)
	if err != nil {
		err = g.gitCmd(dir, "remote", "set-url", name, gitURL)
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *GitCLI) parseGitConfig(gitConf string) (*gitcfg.Config, error) {
	if gitConf == "" {
		return nil, fmt.Errorf("no GitConfDir defined")
	}
	cfg := gitcfg.NewConfig()
	data, err := ioutil.ReadFile(gitConf)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s due to %s", gitConf, err)
	}

	err = cfg.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s due to %s", gitConf, err)
	}
	return cfg, nil
}

// DiscoverRemoteGitURL discovers the remote git URL from the given git configuration
func (g *GitCLI) DiscoverRemoteGitURL(gitConf string) (string, error) {
	cfg, err := g.parseGitConfig(gitConf)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal %s due to %s", gitConf, err)
	}
	remotes := cfg.Remotes
	if len(remotes) == 0 {
		return "", nil
	}
	rUrl := g.GetRemoteUrl(cfg, "origin")
	if rUrl == "" {
		rUrl = g.GetRemoteUrl(cfg, "upstream")
	}
	return rUrl, nil
}

// DiscoverUpstreamGitURL discovers the upstream git URL from the given git configuration
func (g *GitCLI) DiscoverUpstreamGitURL(gitConf string) (string, error) {
	cfg, err := g.parseGitConfig(gitConf)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal %s due to %s", gitConf, err)
	}
	remotes := cfg.Remotes
	if len(remotes) == 0 {
		return "", nil
	}
	rUrl := g.GetRemoteUrl(cfg, "upstream")
	if rUrl == "" {
		rUrl = g.GetRemoteUrl(cfg, "origin")
	}
	return rUrl, nil
}

func (g *GitCLI) firstRemoteUrl(remote *gitcfg.RemoteConfig) string {
	if remote != nil {
		urls := remote.URLs
		if urls != nil && len(urls) > 0 {
			return urls[0]
		}
	}
	return ""
}

// GetRemoteUrl returns the remote URL from the given git config
func (g *GitCLI) GetRemoteUrl(config *gitcfg.Config, name string) string {
	if config.Remotes != nil {
		return g.firstRemoteUrl(config.Remotes[name])
	}
	return ""
}

// RemoteBranchNames returns all remote branches with the given remoteRefPrefix
func (g *GitCLI) RemoteBranchNames(dir string, remoteRefPrefix string) ([]string, error) {
	return g.remoteBranchNames(dir, remoteRefPrefix, false)
}

// RemoteMergedBranchNames returns all remote branches that were merged with the given remoteRefPrefix
func (g *GitCLI) RemoteMergedBranchNames(dir string, remoteRefPrefix string) ([]string, error) {
	return g.remoteBranchNames(dir, remoteRefPrefix, true)
}

func (g *GitCLI) remoteBranchNames(dir string, remoteRefPrefix string, merged bool) ([]string, error) {
	answer := []string{}
	args := []string{"branch", "-a"}
	if merged {
		args = append(args, "--merged")
	}

	text, err := g.gitCmdWithOutput(dir, args...)

	if err != nil {
		return answer, err
	}
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if remoteRefPrefix != "" {
			if strings.HasPrefix(line, remoteRefPrefix) {
				line = strings.TrimPrefix(line, remoteRefPrefix)
				answer = append(answer, line)
			}
		} else {
			answer = append(answer, line)
		}
	}

	if merged {
		filtered := []string{}

		for _, name := range answer {
			if name != "master" {
				filtered = append(filtered, name)
			}
		}

		return filtered, nil
	}

	return answer, nil
}

// GetCommitPointedToByPreviousTag return the SHA of the commit pointed to by the latest-but-1 git tag as well as the tag
// name for the git repo in dir
func (g *GitCLI) GetCommitPointedToByPreviousTag(dir string) (string, string, error) {
	tagSHA, tagName, err := g.NthTag(dir, 2)
	if err != nil {
		return "", "", errors.Wrapf(err, "getting commit pointed to by previous tag in %s", dir)
	}
	if tagSHA == "" {
		return tagSHA, tagName, nil
	}
	commitSHA, err := g.gitCmdWithOutput(dir, "rev-list", "-n", "1", tagSHA)
	if err != nil {
		return "", "", errors.Wrapf(err, "running for git rev-list -n 1 %s", tagSHA)
	}
	return commitSHA, tagName, err
}

// GetRevisionBeforeDate returns the revision before the given date
func (g *GitCLI) GetRevisionBeforeDate(dir string, t time.Time) (string, error) {
	dateText := util.FormatDate(t)
	return g.GetRevisionBeforeDateText(dir, dateText)
}

// GetRevisionBeforeDateText returns the revision before the given date in format "MonthName dayNumber year"
func (g *GitCLI) GetRevisionBeforeDateText(dir string, dateText string) (string, error) {
	branch, err := g.Branch(dir)
	if err != nil {
		return "", err
	}
	return g.gitCmdWithOutput(dir, "rev-list", "-1", "--before=\""+dateText+"\"", "--max-count=1", branch)
}

// GetCommitPointedToByLatestTag return the SHA of the commit pointed to by the latest git tag as well as the tag name
// for the git repo in dir
func (g *GitCLI) GetCommitPointedToByLatestTag(dir string) (string, string, error) {
	tagSHA, tagName, err := g.NthTag(dir, 1)
	if err != nil {
		return "", "", errors.Wrapf(err, "getting commit pointed to by latest tag in %s", dir)
	}
	if tagSHA == "" {
		return tagSHA, tagName, nil
	}
	commitSHA, err := g.gitCmdWithOutput(dir, "rev-list", "-n", "1", tagSHA)
	if err != nil {
		return "", "", errors.Wrapf(err, "running for git rev-list -n 1 %s", tagSHA)
	}
	return commitSHA, tagName, err
}

// GetCommitPointedToByTag return the SHA of the commit pointed to by the given git tag
func (g *GitCLI) GetCommitPointedToByTag(dir string, tag string) (string, error) {
	commitSHA, err := g.gitCmdWithOutput(dir, "rev-list", "-n", "1", tag)
	if err != nil {
		return "", errors.Wrapf(err, "running for git rev-list -n 1 %s", tag)
	}
	return commitSHA, err
}

// GetLatestCommitMessage returns the latest git commit message
func (g *GitCLI) GetLatestCommitMessage(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "log", "-1", "--pretty=%B")
}

// FetchTags fetches all the tags
func (g *GitCLI) FetchTags(dir string) error {
	return g.gitCmd(dir, "fetch", "--tags")
}

// FetchRemoteTags fetches all the tags from a remote repository
func (g *GitCLI) FetchRemoteTags(dir string, repo string) error {
	return g.gitCmd(dir, "fetch", repo, "--tags")
}

// Tags returns all tags from the repository at the given directory
func (g *GitCLI) Tags(dir string) ([]string, error) {
	return g.FilterTags(dir, "")
}

// FilterTags returns all tags from the repository at the given directory that match the filter
func (g *GitCLI) FilterTags(dir string, filter string) ([]string, error) {
	args := []string{"tag"}
	if filter != "" {
		args = append(args, "--list", filter)
	}
	text, err := g.gitCmdWithOutput(dir, args...)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSuffix(text, "\n")
	split := strings.Split(text, "\n")
	// Split will return the original string if it can't split it, and it may be empty
	if len(split) == 1 && split[0] == "" {
		return make([]string, 0), nil
	}
	return split, nil
}

// CreateTag creates a tag with the given name and message in the repository at the given directory
func (g *GitCLI) CreateTag(dir string, tag string, msg string) error {
	return g.gitCmd(dir, "tag", "-fa", tag, "-m", msg)
}

// PrintCreateRepositoryGenerateAccessToken prints the access token URL of a Git repository
func (g *GitCLI) PrintCreateRepositoryGenerateAccessToken(server *auth.AuthServer, username string, o io.Writer) {
	tokenUrl := ProviderAccessTokenURL(server.Kind, server.URL, username)

	fmt.Fprintf(o, "To be able to create a repository on %s we need an API Token\n", server.Label())
	fmt.Fprintf(o, "Please click this URL and generate a token \n%s\n\n", util.ColorInfo(tokenUrl))
	fmt.Fprint(o, "Then COPY the token and enter it below:\n\n")
}

// IsFork indicates if the repository at the given directory is a fork
func (g *GitCLI) IsFork(dir string) (bool, error) {
	// lets ignore errors as that just means there's no config
	originUrl, _ := g.gitCmdWithOutput(dir, "config", "--get", "remote.origin.url")
	upstreamUrl, _ := g.gitCmdWithOutput(dir, "config", "--get", "remote.upstream.url")

	if originUrl != upstreamUrl && originUrl != "" && upstreamUrl != "" {
		return true, nil
	}
	return false, fmt.Errorf("could not confirm the repo is a fork")
}

// Version returns the git version
func (g *GitCLI) Version() (string, error) {
	out, err := g.gitCmdWithOutput("", "version")
	idxs := numberRegex.FindStringIndex(out)
	if len(idxs) > 0 {
		return out[idxs[0]:], err
	}
	return out, err
}

// Username return the username from the git configuration
func (g *GitCLI) Username(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "config", "--get", "user.name")
}

// SetUsername sets the username in the git configuration
func (g *GitCLI) SetUsername(dir string, username string) error {
	// Will return status 1 silently if the user is not set.
	_, err := g.gitCmdWithOutput(dir, "config", "--get", "user.name")
	if err != nil {
		return g.gitCmd(dir, "config", "--global", "--add", "user.name", username)
	}
	return nil
}

// Email returns the email from the git configuration
func (g *GitCLI) Email(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "config", "--get", "user.email")
}

// SetEmail sets the given email in the git configuration
func (g *GitCLI) SetEmail(dir string, email string) error {
	// Will return status 1 silently if the email is not set.
	_, err := g.gitCmdWithOutput(dir, "config", "--get", "user.email")
	if err != nil {
		return g.gitCmd(dir, "config", "--global", "--add", "user.email", email)
	}
	return nil
}

// CreateBranch creates a branch with the given name in the Git repository from the given directory
func (g *GitCLI) CreateBranch(dir string, branch string) error {
	return g.gitCmd(dir, "branch", branch)
}

// Diff runs git diff
func (g *GitCLI) Diff(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "diff")
}

// ListChangedFilesFromBranch lists files changed between branches
func (g *GitCLI) ListChangedFilesFromBranch(dir string, branch string) (string, error) {
	return g.gitCmdWithOutput(dir, "diff", "--name-status", branch)
}

// LoadFileFromBranch returns a files's contents from a branch
func (g *GitCLI) LoadFileFromBranch(dir string, branch string, file string) (string, error) {
	return g.gitCmdWithOutput(dir, "show", branch+":"+file)
}

// FetchUnshallow runs git fetch --unshallow in dir
func (g *GitCLI) FetchUnshallow(dir string) error {
	err := g.gitCmd(dir, "fetch", "--unshallow")
	if err != nil {
		return errors.Wrapf(err, "running git fetch --unshallow %s", dir)
	}
	return nil
}

// IsShallow runs git rev-parse --is-shallow-repository in dir and returns the result
func (g *GitCLI) IsShallow(dir string) (bool, error) {
	out, err := g.gitCmdWithOutput(dir, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, errors.Wrapf(err, "running git rev-parse --is-shallow-repository %s", dir)
	}
	if out == "--is-shallow-repository" {
		// Newer git has a method to do it, but we use an old git in our builders, so resort to doing it manually
		gitDir, _, err := g.FindGitConfigDir(dir)
		if err != nil {
			return false, errors.Wrapf(err, "finding .git for %s", dir)
		}
		if _, err := os.Stat(filepath.Join(gitDir, ".git", "shallow")); os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	}
	b, err := util.ParseBool(out)
	if err != nil {
		return false, errors.Wrapf(err, "converting %v to bool", b)
	}
	return b, nil

}

// CreateBranchFrom creates a new branch called branchName from startPoint
func (g *GitCLI) CreateBranchFrom(dir string, branchName string, startPoint string) error {
	return g.gitCmd(dir, "branch", branchName, startPoint)
}

// Merge merges the commitish into the current branch
func (g *GitCLI) Merge(dir string, commitish string) error {
	return g.gitCmd(dir, "merge", commitish)
}

// GetLatestCommitSha returns the sha of the last commit
func (g *GitCLI) GetLatestCommitSha(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "rev-parse", "HEAD")
}

// GetFirstCommitSha returns the sha of the first commit
func (g *GitCLI) GetFirstCommitSha(dir string) (string, error) {
	return g.gitCmdWithOutput(dir, "rev-list", "--max-parents=0", "HEAD")
}

// Reset performs a git reset --hard back to the commitish specified
func (g *GitCLI) Reset(dir string, commitish string, hard bool) error {
	args := []string{"reset"}
	if hard {
		args = append(args, "--hard")
	}
	if commitish != "" {
		args = append(args, commitish)
	}
	return g.gitCmd(dir, args...)
}

// MergeTheirs will do a recursive merge of commitish with the strategy option theirs
func (g *GitCLI) MergeTheirs(dir string, commitish string) error {
	return g.gitCmd(dir, "merge", "--strategy-option=theirs", commitish)
}

// RebaseTheirs runs git rebase upstream branch with the strategy option theirs
func (g *GitCLI) RebaseTheirs(dir string, upstream string, branch string, skipEmpty bool) error {
	args := []string{
		"rebase",
		"--strategy-option=theirs",
		upstream,
	}
	if branch != "" {
		args = append(args, branch)
	}
	err := g.gitCmd(dir, args...)
	if skipEmpty {
		// If skipEmpty is passed, then if the failure is due to an empty commit, run `git rebase --skip` to move on
		// Weirdly git has no option on rebase to just do this
		for err != nil && IsEmptyCommitError(err) {
			err = g.gitCmd(dir, "rebase", "--skip")
		}
	}
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// RevParse runs git rev-parse on rev
func (g *GitCLI) RevParse(dir string, rev string) (string, error) {
	return g.gitCmdWithOutput(dir, "rev-parse", rev)
}

// SetUpstreamTo will set the given branch to track the origin branch with the same name
func (g *GitCLI) SetUpstreamTo(dir string, branch string) error {
	return g.gitCmd(dir, "branch", "--set-upstream-to", fmt.Sprintf("origin/%s", branch), branch)
}

// NthTag return the SHA and tag name of nth tag in reverse chronological order from the repository at the given directory.
// If the nth tag does not exist empty strings without an error are returned.
func (g *GitCLI) NthTag(dir string, n int) (string, string, error) {
	args := []string{
		"for-each-ref",
		"--sort=-creatordate",
		"--format=%(objectname)%00%(refname:short)",
		fmt.Sprintf("--count=%d", n),
		"refs/tags",
	}
	out, err := g.gitCmdWithOutput(dir, args...)
	if err != nil {
		return "", "", errors.Wrapf(err, "running git %s", strings.Join(args, " "))
	}

	tagList := strings.Split(out, "\n")

	if len(tagList) < n {
		return "", "", nil
	}

	fields := strings.Split(tagList[n-1], "\x00")

	if len(fields) != 2 {
		return "", "", errors.Errorf("Unexpected format for returned tag and sha: '%s'", tagList[n-1])
	}

	return fields[0], fields[1], nil
}

// Remotes will list the names of the remotes
func (g *GitCLI) Remotes(dir string) ([]string, error) {
	out, err := g.gitCmdWithOutput(dir, "remote")
	if err != nil {
		return nil, errors.Wrapf(err, "running git remote")
	}
	return strings.Split(out, "\n"), nil
}

// CloneBare will create a bare clone of url
func (g *GitCLI) CloneBare(dir string, url string) error {
	err := g.gitCmd(dir, "clone", "--bare", url, dir)
	if err != nil {
		return errors.Wrapf(err, "running git clone --bare %s", url)
	}
	return nil
}

// PushMirror will push the dir as a mirror to url
func (g *GitCLI) PushMirror(dir string, url string) error {
	err := g.gitCmd(dir, "push", "--mirror", url)
	if err != nil {
		return errors.Wrapf(err, "running git push --mirror %s", url)
	}
	return nil
}

// CherryPick does a git cherry-pick of commit
func (g *GitCLI) CherryPick(dir string, commitish string) error {
	return g.gitCmd(dir, "cherry-pick", commitish)
}

// CherryPickTheirs does a git cherry-pick of commit
func (g *GitCLI) CherryPickTheirs(dir string, commitish string) error {
	return g.gitCmd(dir, "cherry-pick", commitish, "--strategy=recursive", "-X", "theirs")
}

// CherryPickTheirsKeepRedundantCommits does a git cherry-pick of commit
func (g *GitCLI) CherryPickTheirsKeepRedundantCommits(dir string, commitish string) error {
	return g.gitCmd(dir, "cherry-pick", commitish, "--strategy=recursive", "-X", "theirs", "--keep-redundant-commits")
}

// Describe does a git describe of commitish, optionally adding the abbrev arg if not empty, falling back to just the commit ref if it's untagged
func (g *GitCLI) Describe(dir string, contains bool, commitish string, abbrev string, fallback bool) (string, string, error) {
	args := []string{"describe", commitish}
	if abbrev != "" {
		args = append(args, fmt.Sprintf("--abbrev=%s", abbrev))
	}
	if contains {
		args = append(args, "--contains")
	}
	out, err := g.gitCmdWithOutput(dir, args...)
	if err != nil {
		if fallback {
			// If the commit-ish is untagged, it'll fail with "fatal: cannot describe '<commit-ish>'". In those cases, just return
			// the original commit-ish.
			if strings.Contains(err.Error(), "fatal: cannot describe") {
				return commitish, "", nil
			}
		}
		log.Logger().Warnf("err: %s", err.Error())
		return "", "", errors.Wrapf(err, "running git %s", strings.Join(args, " "))
	}
	trimmed := strings.TrimSpace(strings.Trim(out, "\n"))
	parts := splitDescribeRegex.Split(trimmed, -1)
	if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return parts[0], "", nil
}

// IsAncestor checks if the possible ancestor commit-ish is an ancestor of the given commit-ish.
func (g *GitCLI) IsAncestor(dir string, possibleAncestor string, commitish string) (bool, error) {
	args := []string{"merge-base", "--is-ancestor", possibleAncestor, commitish}
	_, err := g.gitCmdWithOutput(dir, args...)
	if err != nil {
		// Treat any error as meaning that it's not an ancestor. Switch this to use ExitError.ExitCode() when we move to go >=1.12
		return false, err
	}
	// Default case is that this is an ancestor, since there's no error from the merge-base call.
	return true, nil
}

// WriteRepoAttributes writes the given content to .git/info/attributes
func (g *GitCLI) WriteRepoAttributes(dir string, content string) error {
	// Read the existing .git/info/attributes if it exists
	gitAttrFile := filepath.Join(dir, ".git", "info", "attributes")

	err := ioutil.WriteFile(gitAttrFile, []byte(content), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "writing new repo-local git attributes to %s", gitAttrFile)
	}
	return nil
}

// ReadRepoAttributes reads the existing content, if any, in .git/info/attributes
func (g *GitCLI) ReadRepoAttributes(dir string) (string, error) {
	gitAttrFile := filepath.Join(dir, ".git", "info", "attributes")
	gaExists, err := util.FileExists(gitAttrFile)
	if err != nil {
		return "", errors.Wrapf(err, "checking if repo-local git attributes file %s exists", gitAttrFile)
	}
	if gaExists {
		gaBytes, err := ioutil.ReadFile(gitAttrFile)
		if err != nil {
			return "", errors.Wrapf(err, "reading existing repo-local git attributes from %s", gitAttrFile)
		}
		return string(gaBytes), nil
	}
	return "", nil
}
