package gits

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/src-d/go-git.v4/config"

	uuid "github.com/satori/go.uuid"

	jxconfig "github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/pkg/errors"
	gitcfg "gopkg.in/src-d/go-git.v4/config"
)

const (
	// LabelUpdatebot is the label applied to PRs created by updatebot
	LabelUpdatebot = "updatebot"
)

// EnsureUserAndEmailSetup returns the user name and email for the gitter
// lazily setting them if they are blank either from the environment variables
// `GIT_AUTHOR_NAME` and `GIT_AUTHOR_EMAIL` or using default values
func EnsureUserAndEmailSetup(gitter Gitter) (string, string, error) {
	userName, _ := gitter.Username("")
	userEmail, _ := gitter.Email("")
	if userName == "" {
		userName = os.Getenv("GIT_AUTHOR_NAME")
		if userName == "" {
			user, err := user.Current()
			if err == nil && user != nil {
				userName = user.Username
			}
		}
		if userName == "" {
			userName = util.DefaultGitUserName
		}
		err := gitter.SetUsername("", userName)
		if err != nil {
			return userName, userEmail, errors.Wrapf(err, "Failed to set the git username to %s", userName)
		}
	}
	if userEmail == "" {
		userEmail = os.Getenv("GIT_AUTHOR_EMAIL")
		if userEmail == "" {
			userEmail = util.DefaultGitUserEmail
		}
		err := gitter.SetEmail("", userEmail)
		if err != nil {
			return userName, userEmail, errors.Wrapf(err, "Failed to set the git email to %s", userEmail)
		}
	}
	return userName, userEmail, nil
}

// Unshallow converts a shallow git repo (one cloned with --depth=n) into one has full depth for the current branch
// and all tags. Note that remote branches are still not fetched,
// you need to do this manually. It checks if the repo is shallow or not before trying to unshallow it.
func Unshallow(dir string, gitter Gitter) error {
	shallow, err := gitter.IsShallow(dir)
	if err != nil {
		return err
	}
	if shallow {
		if err := gitter.FetchUnshallow(dir); err != nil {
			return err
		}
		if err := gitter.FetchTags(dir); err != nil {
			return err
		}
		log.Logger().Infof("Converted %s to an unshallow repository", dir)
		return nil
	}
	return nil
}

// FetchAndMergeSHAs merges any SHAs into the baseBranch which has a tip of baseSha,
// fetching the commits from remote for the git repo in dir. It will try to fetch individual commits (
// if the remote repo supports it - see https://github.
// com/git/git/commit/68ee628932c2196742b77d2961c5e16360734a62) otherwise it uses git remote update to pull down the
// whole repo.
func FetchAndMergeSHAs(SHAs []string, baseBranch string, baseSha string, remote string, dir string,
	gitter Gitter) error {
	refspecs := make([]string, 0)
	for _, sha := range SHAs {
		refspecs = append(refspecs, fmt.Sprintf("%s:", sha))
	}
	refspecs = append(refspecs, fmt.Sprintf("%s:", baseSha))
	// First lets make sure we have the commits - remember that this may be a shallow clone
	err := gitter.FetchBranchUnshallow(dir, remote, refspecs...)
	if err != nil {
		// Unshallow fetch failed, so do a full unshallow
		// First ensure we actually have the branch refs
		err := gitter.FetchBranch(dir, remote, refspecs...)
		if err != nil {
			// This can be caused by git not being configured to allow fetching individual SHAs
			// There is not a nice way to solve this except to attempt to do a full fetch
			err = gitter.RemoteUpdate(dir)
			if err != nil {
				return errors.Wrapf(err, "updating remote %s", remote)
			}
			log.Logger().Debugf("ran %s in %s", util.ColorInfo("git remote update"), dir)
		}
		log.Logger().Debugf("ran git fetch %s %s in %s", remote, strings.Join(refspecs, " "), dir)

		err = Unshallow(dir, gitter)
		if err != nil {
			return errors.WithStack(err)
		}
		log.Logger().Debugf("Unshallowed git repo in %s", dir)
	} else {
		log.Logger().Debugf("ran git fetch --unshallow %s %s in %s", remote, strings.Join(refspecs, " "), dir)
	}
	branches, err := gitter.LocalBranches(dir)
	if err != nil {
		return errors.Wrapf(err, "listing local branches")
	}
	found := false
	for _, b := range branches {
		if b == baseBranch {
			found = true
			break
		}
	}
	if !found {
		err = gitter.CreateBranch(dir, baseBranch)
		if err != nil {
			return errors.Wrapf(err, "creating branch %s", baseBranch)
		}
	}
	// Ensure we are on baseBranch
	err = gitter.Checkout(dir, baseBranch)
	if err != nil {
		return errors.Wrapf(err, "checking out %s", baseBranch)
	}
	log.Logger().Debugf("ran git checkout %s in %s", baseBranch, dir)
	// Ensure we are on the right revision
	err = gitter.Reset(dir, baseSha, true)
	if err != nil {
		return errors.Wrapf(err, "resetting %s to %s", baseBranch, baseSha)
	}
	log.Logger().Debugf("ran git reset --hard %s in %s", baseSha, dir)
	err = gitter.CleanForce(dir, ".")
	if err != nil {
		return errors.Wrapf(err, "cleaning up the git repo")
	}
	log.Logger().Debugf("ran clean --force -d . in %s", dir)
	// Now do the merges
	for _, sha := range SHAs {
		err := gitter.Merge(dir, sha)
		if err != nil {
			return errors.Wrapf(err, "merging %s into master", sha)
		}
		log.Logger().Debugf("ran git merge %s in %s", sha, dir)

	}
	return nil
}

// SourceRepositoryProviderURL returns the git provider URL for the SourceRepository which is something like
// either `https://hostname` or `http://hostname`
func SourceRepositoryProviderURL(gitProvider GitProvider) string {
	return GitProviderURL(gitProvider.ServerURL())
}

// GitProviderURL returns the git provider host URL for the SourceRepository which is something like
// either `https://hostname` or `http://hostname`
func GitProviderURL(text string) string {
	if text == "" {
		return text
	}
	u, err := url.Parse(text)
	if err != nil {
		log.Logger().Warnf("failed to parse git provider URL %s: %s", text, err.Error())
		return text
	}
	u.Path = ""
	if !strings.HasPrefix(u.Scheme, "http") {
		// lets convert other schemes like 'git' to 'https'
		u.Scheme = "https"
	}
	return u.String()
}

// PushRepoAndCreatePullRequest commits and pushes the changes in the repo rooted at dir.
// It creates a branch called branchName from a base.
// It uses the pullRequestDetails for the message and title for the commit and PR.
// It uses and updates pullRequestInfo to identify whether to rebase an existing PR.
func PushRepoAndCreatePullRequest(dir string, upstreamRepo *GitRepository, forkRepo *GitRepository, base string, prDetails *PullRequestDetails, filter *PullRequestFilter, commit bool, commitMessage string, push bool, dryRun bool, gitter Gitter, provider GitProvider) (*PullRequestInfo, error) {
	userAuth := provider.UserAuth()
	if commit {
		err := gitter.Add(dir, "-A")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		changed, err := gitter.HasChanges(dir)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if !changed {
			log.Logger().Warnf("No changes made to the source code in %s. Code must be up to date!", dir)
			return nil, nil
		}
		if commitMessage == "" {
			commitMessage = prDetails.Message
		}
		err = gitter.CommitDir(dir, commitMessage)
		if err != nil {
			return nil, errors.WithStack(err)
		}
	}

	headPrefix := ""

	username := upstreamRepo.Organisation
	cloneURL := upstreamRepo.CloneURL
	if forkRepo != nil {
		username = forkRepo.Organisation
		cloneURL = forkRepo.CloneURL
	}

	if upstreamRepo.Organisation != username {
		headPrefix = username + ":"
	}

	gha := &GitPullRequestArguments{
		GitRepository: upstreamRepo,
		Title:         prDetails.Title,
		Body:          prDetails.Message,
		Base:          base,
		Labels:        prDetails.Labels,
	}
	var existingPr *GitPullRequest

	forkPushURL, err := gitter.CreateAuthenticatedURL(cloneURL, &userAuth)
	if err != nil {
		return nil, errors.Wrapf(err, "creating push URL for %s", cloneURL)
	}

	if filter != nil && push {
		// lets rebase an existing PR
		existingPrs, err := FilterOpenPullRequests(provider, upstreamRepo.Organisation, upstreamRepo.Name, *filter)
		if err != nil {
			return nil, errors.Wrapf(err, "finding existing PRs using filter %s on repo %s/%s", filter.String(), upstreamRepo.Organisation, upstreamRepo.Name)
		}

		if len(existingPrs) > 1 {
			sort.SliceStable(existingPrs, func(i, j int) bool {
				// sort in descending order of PR numbers (assumes PRs numbers increment!)
				return util.DereferenceInt(existingPrs[j].Number) < util.DereferenceInt(existingPrs[i].Number)
			})
			prs := make([]string, 0)
			for _, pr := range existingPrs {
				prs = append(prs, pr.URL)
			}
			log.Logger().Debugf("Found more than one PR %s using filter %s on repo %s/%s so rebasing latest PR %s", strings.Join(prs, ", "), filter.String(), upstreamRepo.Organisation, upstreamRepo.Name, existingPrs[:1][0].URL)
			existingPr = existingPrs[0]
		} else if len(existingPrs) == 1 {
			existingPr = existingPrs[0]
		}
	}
	remoteBranch := prDetails.BranchName
	if existingPr != nil {
		if util.DereferenceString(existingPr.HeadOwner) == username && existingPr.HeadRef != nil && existingPr.Number != nil {
			remote := "origin"
			if forkRepo != nil && forkRepo.Fork {
				remote = "upstream"
			}
			changeBranch, err := gitter.Branch(dir)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			localBranchUUID, err := uuid.NewV4()
			if err != nil {
				return nil, errors.Wrapf(err, "creating UUID for local branch")
			}
			// We use this "dummy" local branch to pull into to avoid having to work with FETCH_HEAD as our local
			// representation of the remote branch. This is an oddity of the pull/%d/head remote.
			localBranch := localBranchUUID.String()
			remoteBranch = *existingPr.HeadRef
			fetchRefSpec := fmt.Sprintf("pull/%d/head:%s", *existingPr.Number, localBranch)
			err = gitter.FetchBranch(dir, remote, fetchRefSpec)
			if err != nil {
				return nil, errors.Wrapf(err, "fetching %s for merge", fetchRefSpec)
			}

			err = gitter.CreateBranchFrom(dir, prDetails.BranchName, localBranch)
			if err != nil {
				return nil, errors.Wrapf(err, "creating branch %s from %s", prDetails.BranchName, fetchRefSpec)
			}
			err = gitter.Checkout(dir, prDetails.BranchName)
			if err != nil {
				return nil, errors.Wrapf(err, "checking out branch %s", prDetails.BranchName)
			}
			err = gitter.MergeTheirs(dir, changeBranch)
			if err != nil {
				return nil, errors.Wrapf(err, "merging %s into %s", changeBranch, fetchRefSpec)
			}
			err = gitter.RebaseTheirs(dir, fmt.Sprintf(localBranch), "", true)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			changedFiles, err := gitter.ListChangedFilesFromBranch(dir, localBranch)
			if err != nil {
				return nil, errors.Wrap(err, "failed to list changed files")
			}
			if changedFiles == "" {
				log.Logger().Info("No file changes since the existing PR. Nothing to push.")
				return nil, nil
			}
		} else {
			// We can only update an existing PR if the owner of that PR is this user, so we clear the existingPr
			existingPr = nil
		}
	}
	var pr *GitPullRequest
	if !dryRun && existingPr != nil {
		gha.Head = headPrefix + remoteBranch
		// work out the minimal similar title
		if strings.HasPrefix(existingPr.Title, "chore(deps): bump ") {
			origWords := strings.Split(existingPr.Title, " ")
			newWords := strings.Split(prDetails.Title, " ")
			answer := make([]string, 0)
			for i, w := range newWords {
				if len(origWords) > i && origWords[i] == w {
					answer = append(answer, w)
				}
			}
			if answer[len(answer)-1] == "bump" {
				// if there are no similarities in the actual dependency, then add a generic form of words
				answer = append(answer, "dependency", "versions")
			}
			if answer[len(answer)-1] == "to" || answer[len(answer)-1] == "from" {
				// remove trailing prepositions
				answer = answer[:len(answer)-1]
			}
			gha.Title = strings.Join(answer, " ")
		} else {
			gha.Title = prDetails.Title
		}
		gha.Body = fmt.Sprintf("%s\n<hr />\n\n%s", prDetails.Message, existingPr.Body)
		var err error
		pr, err = provider.UpdatePullRequest(gha, *existingPr.Number)
		if err != nil {
			return nil, errors.Wrapf(err, "updating pull request %s", existingPr.URL)
		}
		log.Logger().Infof("Updated Pull Request: %s", util.ColorInfo(pr.URL))
	}
	if dryRun {
		log.Logger().Infof("Commit created but not pushed; would have updated pull request %s with %s and used commit message %s. Please manually delete %s when you are done", util.ColorInfo(existingPr.URL), prDetails.String(), commitMessage, util.ColorInfo(dir))
		return nil, nil
	} else if push {
		err := gitter.Push(dir, forkPushURL, true, fmt.Sprintf("%s:%s", "HEAD", remoteBranch))
		if err != nil {
			return nil, errors.Wrapf(err, "pushing merged branch %s", remoteBranch)
		}
	}
	if existingPr == nil {
		gha.Head = headPrefix + prDetails.BranchName

		pr, err = provider.CreatePullRequest(gha)
		if err != nil {
			return nil, errors.Wrapf(err, "creating pull request with arguments %v", gha.String())
		}
		log.Logger().Infof("Created Pull Request: %s", util.ColorInfo(pr.URL))
	}

	prInfo := &PullRequestInfo{
		GitProvider:          provider,
		PullRequest:          pr,
		PullRequestArguments: gha,
	}

	err = addLabelsToPullRequest(prInfo, prDetails.Labels)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to add labels %+v to PR %s", prDetails.Labels, pr.URL)
	}

	return prInfo, nil
}

// addLabelsToPullRequest adds the provided labels, if not already present, to the provided pull request
// Labels are applied after PR creation as they use the GitHub issues API instead of the PR one.
func addLabelsToPullRequest(prInfo *PullRequestInfo, labels []string) error {
	if prInfo == nil {
		return errors.New("pull request to label cannot be nil")
	}
	pr := prInfo.PullRequest
	provider := prInfo.GitProvider

	if len(labels) > 0 {
		number := *pr.Number
		var err error
		err = provider.AddLabelsToIssue(pr.Owner, pr.Repo, number, labels)
		if err != nil {
			return err
		}
		log.Logger().Infof("Added label %s to Pull Request %s", util.ColorInfo(strings.Join(labels, ", ")), pr.URL)
	}
	return nil
}

// GetRemoteForURL returns the remote name for the specified remote URL. The empty string is returned if the remoteURL is not known.
// An error is returned if there are errors processing the git repository data.
func GetRemoteForURL(repoDir string, remoteURL string, gitter Gitter) (string, error) {
	_, config, err := gitter.FindGitConfigDir(repoDir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to load git config in %s", repoDir)
	}

	cfg := gitcfg.NewConfig()
	data, err := ioutil.ReadFile(config)
	if err != nil {
		return "", errors.Wrapf(err, "failed to read git config in %s", repoDir)
	}

	err = cfg.Unmarshal(data)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse git config in %s", repoDir)
	}

	for _, remote := range cfg.Remotes {
		for _, url := range remote.URLs {
			if url == remoteURL {
				return remote.Name, nil
			}
		}
	}
	return "", nil
}

// ForkAndPullRepo pulls the specified gitUrl into dir using gitter, creating a remote fork if needed using the git provider
//
// If there are existing files in dir (and dir is already a git clone), the existing files will pushed into the stash
// and then popped at the end. If they cannot be popped then an error will be returned which can be checked for using
// IsCouldNotPopTheStashError
func ForkAndPullRepo(gitURL string, dir string, baseRef string, branchName string, provider GitProvider, gitter Gitter, forkName string) (string, string, *GitRepository, *GitRepository, error) {
	// Validate the arguments
	if gitURL == "" {
		return "", "", nil, nil, fmt.Errorf("gitURL cannot be nil")
	}
	if provider == nil {
		return "", "", nil, nil, errors.Errorf("provider cannot be nil for %s", gitURL)
	}
	originalInfo, err := ParseGitURL(gitURL)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "failed to parse gitter URL %s", gitURL)
	}

	log.Logger().Debugf("ForkAndPullRepo gitURL: %s dir: %s baseRef: %s branchName: %s forkName: %s", gitURL, dir, baseRef, branchName, forkName)

	username := provider.CurrentUsername()
	originalOrg := originalInfo.Organisation
	repoOwner := originalInfo.Organisation
	originalRepo := originalInfo.Name
	originRemote := "origin"
	upstreamRemote := originRemote

	// lets check if we need to fork the repository...
	fork := false
	if originalOrg != username && username != "" && originalOrg != "" && provider.ShouldForkForPullRequest(originalOrg, originalRepo, username) {
		fork = true
	}

	// Check if we are working with an existing dir
	dirExists, err := util.DirExists(dir)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "failed to check if directory %s dirExists", dir)
	}
	dirIsGitRepo := false

	// check whether we are going to change the upstream remote (e.g. on initial boot)
	upstreamChange := false
	if dirExists {
		log.Logger().Debugf("dir %s already exists", dir)
		d, gitConfig, err := gitter.FindGitConfigDir(dir)
		if err != nil {
			return "", "", nil, nil, errors.Wrapf(err, "checking if %s is already a git repository", dir)
		}
		dirIsGitRepo = dir == d

		currentUpstreamURL, err := gitter.DiscoverUpstreamGitURL(gitConfig)
		if err != nil {
			log.Logger().Warn("")
		}

		finalUpstreamURL, err := AddUserToURL(gitURL, username)
		if err != nil {
			return "", "", nil, nil, errors.Wrapf(err, "unable to add username to git url %s", gitURL)
		}
		if currentUpstreamURL != finalUpstreamURL {
			upstreamChange = true
		}
	}

	if baseRef == "" {
		baseRef = "master"
	}

	// It's important to go via the provider to get the info and urls for cloning so that we resolve the URL to a clone
	// url. This is also useful for tests
	upstreamInfo, err := provider.GetRepository(originalOrg, originalRepo)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "getting repository %s/%s", originalOrg, originalRepo)
	}
	originURL := upstreamInfo.CloneURL
	if forkName == "" {
		forkName = upstreamInfo.Name
	}

	var forkInfo *GitRepository
	// Create or use a fork on the git provider if needed
	if fork {
		forkInfo, err = provider.GetRepository(username, forkName)
		if err != nil {
			log.Logger().Debugf(errors.Wrapf(err, "getting repository %s/%s", username, forkName).Error())
			// lets try create a fork as it probably doesn't exist- using a blank organisation to force a user specific fork
			forkInfo, err = provider.ForkRepository(originalOrg, originalRepo, "")
			if err != nil {
				return "", "", nil, nil, errors.Wrapf(err, "failed to fork GitHub repo %s/%s to user %s", originalOrg, originalRepo, username)
			}
			if forkName != "" {
				renamedInfo, err := provider.RenameRepository(forkInfo.Organisation, forkInfo.Name, forkName)
				if err != nil {
					return "", "", nil, nil, errors.Wrapf(err, "failed to rename fork %s/%s to %s/%s", forkInfo.Organisation, forkInfo.Name, renamedInfo.Organisation, renamedInfo.Name)
				}
				forkInfo = renamedInfo
			}
			log.Logger().Infof("Forked Git repository to %s", util.ColorInfo(forkInfo.HTMLURL))
			repoOwner = forkInfo.Organisation
		}
		originURL = forkInfo.CloneURL
	}

	// Prepare the git repo
	if !dirExists {
		// If the directory doesn't already exist, create it
		err := os.MkdirAll(dir, util.DefaultWritePermissions)
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("failed to create directory %s due to %s", dir, err)
		}
	}

	stashed := false
	if !dirIsGitRepo {
		err = gitter.Init(dir)
		if err != nil {
			return "", "", nil, nil, errors.Wrapf(err, "failed to run git init in %s", dir)
		}
		// Need an initial commit to make branching work. We'll do an empty commit
		err = gitter.AddCommit(dir, "initial commit")
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
	} else {
		err = gitter.Add(dir, ".")
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
		err = gitter.StashPush(dir)
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
		stashed = true
	}

	// configure jx as git credential helper for this repo
	err = configureJxAsGitCredentialHelper(dir, gitter, repoOwner)
	if err != nil {
		return "", "", nil, nil, errors.Wrap(err, "unable to configure jx as git credential helper")
	}

	originURLWithUser, err := AddUserToURL(originURL, username)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "unable to add username to git url %s", originURL)
	}
	err = gitter.SetRemoteURL(dir, originRemote, originURLWithUser)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "failed to set %s url to %s", originRemote, originURL)
	}
	if fork {
		upstreamRemote = "upstream"
		upstreamURLWithUser, err := AddUserToURL(upstreamInfo.CloneURL, username)
		if err != nil {
			return "", "", nil, nil, errors.Wrapf(err, "unable to add username to git url %s", upstreamInfo.CloneURL)
		}
		err = gitter.SetRemoteURL(dir, upstreamRemote, upstreamURLWithUser)
		if err != nil {
			return "", "", nil, nil, errors.Wrapf(err, "setting remote upstream %q in forked environment repo", gitURL)
		}
	}

	branchNameUUID, err := uuid.NewV4()
	if err != nil {
		return "", "", nil, nil, errors.WithStack(err)
	}
	originFetchBranch := branchNameUUID.String()

	var upstreamFetchBranch string
	err = gitter.FetchBranch(dir, "origin", fmt.Sprintf("%s:%s", branchName, originFetchBranch))

	if err != nil {
		if IsCouldntFindRemoteRefError(err, branchName) { // We can safely ignore missing remote branches, as they just don't exist
			originFetchBranch = ""
		} else {
			return "", "", nil, nil, errors.Wrapf(err, "fetching %s %s", originRemote, branchName)
		}
	}

	if upstreamRemote != originRemote || baseRef != branchName {
		branchNameUUID, err := uuid.NewV4()
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
		upstreamFetchBranch = branchNameUUID.String()

		// We're going to start our work from baseRef on the upstream
		err = gitter.FetchBranch(dir, upstreamRemote, fmt.Sprintf("%s:%s", baseRef, upstreamFetchBranch))
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
	}

	// Work out what branch to use and check it out
	localBranchName := gitter.ConvertToValidBranchName(branchName)
	localBranches, err := gitter.LocalBranches(dir)
	if err != nil {
		return "", "", nil, nil, errors.WithStack(err)
	}
	localBranchExists := util.Contains(localBranches, localBranchName)

	// We always want to make sure our local branch is in the right state, whether we created it or checked it out
	resetish := originFetchBranch
	if upstreamFetchBranch != "" {
		resetish = upstreamFetchBranch
	}

	var toCherryPick []GitCommit
	if !localBranchExists {
		err = gitter.CreateBranch(dir, localBranchName)
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
	} else if dirExists && !upstreamChange {
		toCherryPick, err = gitter.GetCommitsNotOnAnyRemote(dir, localBranchName)
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
	}

	err = gitter.Checkout(dir, localBranchName)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "failed to run git checkout %s", localBranchName)
	}
	err = gitter.Reset(dir, resetish, true)
	if err != nil {
		return "", "", nil, nil, errors.Wrapf(err, "failed to run git reset --hard %s", resetish)
	}

	// Merge in any local committed changes we found
	if len(toCherryPick) > 0 {
		var shas = make([]string, len(toCherryPick))
		for _, commit := range toCherryPick {
			shas = append(shas, commit.SHA)
		}
		log.Logger().Debugf("Attempting to cherry pick commits %s that were on %s but not yet pushed", shas, localBranchName)
	}
	for _, c := range toCherryPick {
		err = gitter.CherryPick(dir, c.SHA)
		if err != nil {
			if IsEmptyCommitError(err) {
				log.Logger().Debugf("  Ignoring %s as is empty", c.OneLine())
				err = gitter.Reset(dir, "", true)
				if err != nil {
					return "", "", nil, nil, errors.Wrapf(err, "running git reset --hard")
				}
			} else {
				return "", "", nil, nil, errors.Wrapf(err, "Unable to cherry-pick %s", util.ColorWarning(c.SHA))
			}
		} else {
			log.Logger().Infof("  Cherry-picked %s", c.OneLine())
		}
	}

	// one possibility is that the baseRef is not in the same state as an already existing branch, so let's merge in the
	// already existing branch from the users fork. This is idempotent (safe if that's already the state)
	// Only do this if the remote branch exists
	if upstreamFetchBranch != "" && originFetchBranch != "" {
		err = gitter.Merge(dir, originFetchBranch)
		if err != nil {
			return "", "", nil, nil, errors.WithStack(err)
		}
	}

	if stashed {
		err = gitter.StashPop(dir)
		if err != nil && !IsNoStashEntriesError(err) { // Ignore no stashes as that's just because there was nothing to stash
			err2 := gitter.Status(dir)
			out, err3 := gitter.Diff(dir)
			log.Logger().Infof("Got diff %s", out)
			return "", "", nil, nil, errors.Wrapf(util.CombineErrors(err, err2, err3), "unable to pop the stash")
		}
	}

	return dir, baseRef, upstreamInfo, forkInfo, nil
}

func configureJxAsGitCredentialHelper(dir string, gitter Gitter, repoOwner string) error {
	// configure jx as git credential helper for this repo
	jxProcessBinary, err := os.Executable()
	if err != nil {
		return errors.Wrapf(err, "unable to determine jx binary location")
	}
	config := []string{"--local", "credential.helper", fmt.Sprintf("%s step git credentials --credential-helper --repo-owner %s", jxProcessBinary, repoOwner)}
	log.Logger().Debugf("setting git config to: %s", strings.Join(config, " "))
	return gitter.Config(dir, config...)
}

// AddUserToURL adds the specified user to the given git URL and returns the new URL.
// An error is returned if the specified git URL is not valid. In this case the return URL is empty.
func AddUserToURL(gitURL string, user string) (string, error) {
	u, err := url.Parse(gitURL)
	if err != nil {
		return "", errors.Wrapf(err, "invalid git URL: %s", gitURL)
	}

	if user != "" && u.Scheme != "file" {
		userInfo := url.User(user)
		u.User = userInfo
	}

	return u.String(), nil
}

// A PullRequestFilter defines a filter for finding pull requests
type PullRequestFilter struct {
	Labels []string
	Number *int
}

func (f *PullRequestFilter) String() string {
	if f.Number != nil {
		return fmt.Sprintf("Pull Request #%d", f.Number)
	}
	return strings.Join(f.Labels, ", ")
}

// FilterOpenPullRequests looks for any pull requests on the owner/repo where all the labels match
func FilterOpenPullRequests(provider GitProvider, owner string, repo string, filter PullRequestFilter) ([]*GitPullRequest, error) {
	openPRs, err := provider.ListOpenPullRequests(owner, repo)
	if err != nil {
		return nil, errors.Wrapf(err, "listing open pull requests on %s/%s", owner, repo)
	}
	answer := make([]*GitPullRequest, 0)
	for _, pr := range openPRs {
		if len(filter.Labels) > 0 {
			found := 0
			for _, label := range filter.Labels {
				f := false
				for _, prLabel := range pr.Labels {
					if label == util.DereferenceString(prLabel.Name) {
						f = true
					}
				}
				if f {
					found++
				}
			}
			if len(filter.Labels) == found {
				answer = append(answer, pr)
			}
		}
		if filter.Number != nil && filter.Number == pr.Number {
			answer = append(answer, pr)
		}
	}
	return answer, nil
}

//IsUnadvertisedObjectError returns true if the reason for the error is that the request was for an object that is unadvertised (i.e. doesn't exist)
func IsUnadvertisedObjectError(err error) bool {
	return strings.Contains(err.Error(), "Server does not allow request for unadvertised object")
}

// IsCouldntFindRemoteRefError returns true if the error is due to the remote ref not being found
func IsCouldntFindRemoteRefError(err error, ref string) bool {
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(fmt.Sprintf("couldn't find remote ref %s", ref)))
}

// IsCouldNotPopTheStashError returns true if the error is due to the stash not being able to be popped, often because
// whatever is in the stash cannot be applied to the new state
func IsCouldNotPopTheStashError(err error) bool {
	return strings.Contains(err.Error(), "could not pop the stash")
}

// IsNoStashEntriesError returns true if the error is due to no stash entries found
func IsNoStashEntriesError(err error) bool {
	return strings.Contains(err.Error(), "No stash entries found.")
}

// GetSimpleIndentedStashPopErrorMessage gets the output of a failed git stash pop without duplication or additional content,
// with each line indented four characters.
func GetSimpleIndentedStashPopErrorMessage(err error) string {
	errStr := err.Error()
	idx := strings.Index(errStr, ": failed to run 'git stash pop'")
	if idx > -1 {
		errStr = errStr[:idx]
	}

	var indentedLines []string

	for _, line := range strings.Split(errStr, "\n") {
		indentedLines = append(indentedLines, "    "+line)
	}

	return strings.Join(indentedLines, "\n")
}

// FindTagForVersion will find a tag for a version number (first fetching the tags, then looking for a tag <version>
// then trying the common convention v<version>). It will return the tag or an error if the tag can't be found.
func FindTagForVersion(dir string, version string, gitter Gitter) (string, error) {
	err := gitter.FetchTags(dir)
	if err != nil {
		return "", errors.Wrapf(err, "fetching tags for %s", dir)
	}
	answer := ""
	tags, err := gitter.FilterTags(dir, version)
	if err != nil {
		return "", errors.Wrapf(err, "listing tags for %s", version)
	}
	if len(tags) == 1 {
		answer = tags[0]
	} else if len(tags) == 0 {
		// try with v
		filter := fmt.Sprintf("v%s", version)
		tags, err := gitter.FilterTags(dir, filter)
		if err != nil {
			return "", errors.Wrapf(err, "listing tags for %s", filter)
		}
		if len(tags) == 1 {
			answer = tags[0]
		} else {
			return "", errors.Errorf("cannot resolve %s to a single git object (searching for tag %s and tag %s), found %+v", version, version, filter, tags)
		}
	} else {
		return "", errors.Errorf("cannot resolve %s to a single git object, found %+v", version, tags)
	}
	return answer, nil
}

// DuplicateGitRepoFromCommitish will duplicate branches (but not tags) from fromGitURL to toOrg/toName. It will reset the
// head of the toBranch on the duplicated repo to fromCommitish. It returns the GitRepository for the duplicated repo.
// If the repository already exist and error is returned.
func DuplicateGitRepoFromCommitish(toOrg string, toName string, fromGitURL string, fromCommitish string, toBranch string, private bool, provider GitProvider, gitter Gitter, fromInfo *GitRepository) (*GitRepository, error) {
	log.Logger().Debugf("getting repo %s/%s", toOrg, toName)
	_, err := provider.GetRepository(toOrg, toName)
	if err == nil {
		return nil, errors.Errorf("repository %s/%s already exists", toOrg, toName)
	}

	if fromInfo == nil {
		// If the duplicate doesn't exist create it
		log.Logger().Debugf(errors.Wrapf(err, "getting repository %s/%s", toOrg, toName).Error())
		fromInfo, err = ParseGitURL(fromGitURL)
		if err != nil {
			return nil, errors.Wrapf(err, "parsing %s", fromGitURL)
		}
		fromInfo, err = provider.GetRepository(fromInfo.Organisation, fromInfo.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "getting repo for %s", fromGitURL)
		}
	}

	duplicateInfo, err := provider.CreateRepository(toOrg, toName, private)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create GitHub repo %s/%s", toOrg, toName)
	}
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.WithStack(err)
	}
	defer func() {
		err = os.RemoveAll(dir)
		if err != nil {
			log.Logger().Warnf("unable to delete temporary directory %s", dir)
		}
	}()
	log.Logger().Debugf("Using %s to duplicate git repo %s", dir, fromGitURL)
	err = gitter.Clone(fromInfo.CloneURL, dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone %s", fromInfo.CloneURL)
	}
	if !strings.Contains(fromCommitish, "/") {
		// if the commitish looks like a tag, fetch the tags
		err = gitter.FetchTags(dir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch tags fromGitURL %s", fromInfo.CloneURL)
		}
		err = gitter.Reset(dir, fromCommitish, true)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to reset to %s", fromCommitish)
		}
	} else {
		parts := strings.Split(fromCommitish, "/")
		err = gitter.FetchBranch(dir, parts[0], parts[1])
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fetch %s fromGitURL %s", fromCommitish, fromInfo.CloneURL)
		}
		if len(parts[1]) == 40 {
			uuid, _ := uuid.NewV4()
			branchName := fmt.Sprintf("pr-%s", uuid.String())

			err = gitter.CreateBranchFrom(dir, branchName, parts[1])
			if err != nil {
				return nil, errors.Wrapf(err, "create branch %s from %s", branchName, parts[1])
			}

			err = gitter.Checkout(dir, branchName)
			if err != nil {
				return nil, errors.Wrapf(err, "checkout branch %s", branchName)
			}
		} else {
			err = gitter.Reset(dir, fromCommitish, true)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to reset to %s", fromCommitish)
			}
		}
	}

	err = SquashIntoSingleCommit(dir, fmt.Sprintf("initial config based of %s/%s with ref %s", fromInfo.Organisation, fromInfo.Name, fromCommitish), gitter)
	if err != nil {
		return nil, err
	}

	err = configureJxAsGitCredentialHelper(dir, gitter, duplicateInfo.Organisation)
	if err != nil {
		return nil, errors.Wrap(err, "unable to configure jx as git credential helper")
	}
	username := provider.CurrentUsername()
	cloneURLWithUser, err := AddUserToURL(duplicateInfo.CloneURL, username)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to add username to git url %s", duplicateInfo.CloneURL)
	}
	err = gitter.SetRemoteURL(dir, "origin", cloneURLWithUser)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set remote url to %s", duplicateInfo.CloneURL)
	}
	err = gitter.Push(dir, "origin", true, fmt.Sprintf("%s:%s", "HEAD", toBranch))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to push HEAD to %s", toBranch)
	}
	log.Logger().Infof("Duplicated Git repository %s to %s\n", util.ColorInfo(fromInfo.HTMLURL), util.ColorInfo(duplicateInfo.HTMLURL))
	log.Logger().Infof("Setting upstream to %s\n\n", util.ColorInfo(duplicateInfo.HTMLURL))

	return duplicateInfo, nil
}

// SquashIntoSingleCommit takes the git repository in the specified directory and squashes all commits into a single
// one using the specified message.
func SquashIntoSingleCommit(repoDir string, commitMsg string, gitter Gitter) error {
	cfg := config.NewConfig()
	data, err := ioutil.ReadFile(filepath.Join(repoDir, ".git", "config"))
	if err != nil {
		return errors.Wrapf(err, "failed to load git config from %s", repoDir)
	}

	err = cfg.Unmarshal(data)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal %s", repoDir)
	}

	err = os.RemoveAll(path.Join(repoDir, ".git"))
	if err != nil {
		return errors.Wrap(err, "unable to squash")
	}

	err = gitter.Init(repoDir)
	if err != nil {
		return errors.Wrap(err, "unable to init git")
	}

	for _, remote := range cfg.Remotes {
		err = gitter.AddRemote(repoDir, remote.Name, remote.URLs[0])
		if err != nil {
			return errors.Wrap(err, "unable to update remote")
		}
	}

	err = gitter.Add(repoDir, ".")
	if err != nil {
		return errors.Wrap(err, "unable to add stage commit")
	}

	err = gitter.CommitDir(repoDir, commitMsg)
	if err != nil {
		return errors.Wrap(err, "unable to add initial commit")
	}
	return nil
}

// GetGitInfoFromDirectory obtains remote origin HTTPS and current branch of a given directory and fails if it's not a git repository
func GetGitInfoFromDirectory(dir string, gitter Gitter) (string, string, error) {
	_, gitConfig, err := gitter.FindGitConfigDir(dir)
	if err != nil {
		return "", "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
	}
	remoteGitURL, err := gitter.DiscoverRemoteGitURL(gitConfig)
	if err != nil {
		return "", "", errors.Wrapf(err, "there was a problem obtaining the remote Git URL of directory %s", dir)
	}
	currentBranch, err := gitter.Branch(dir)
	if err != nil {
		return "", "", errors.Wrapf(err, "there was a problem obtaining the current branch on directory %s", dir)
	}
	g, err := ParseGitURL(remoteGitURL)
	if err != nil {
		return "", "", errors.Wrapf(err, "there was a problem parsing the Git URL %s to HTTPS", remoteGitURL)
	}

	return g.HttpsURL(), currentBranch, nil
}

// RefIsBranch looks for remove branches in ORIGIN for the provided directory and returns true if ref is found
func RefIsBranch(dir string, ref string, gitter Gitter) (bool, error) {
	remoteBranches, err := gitter.RemoteBranches(dir)
	if err != nil {
		return false, errors.Wrapf(err, "error getting remote branches to find provided ref %s", ref)
	}
	for _, b := range remoteBranches {
		if strings.Contains(b, ref) {
			return true, nil
		}
	}
	return false, nil
}

// IsDefaultBootConfigURL checks if the given URL corresponds to the default boot config URL
func IsDefaultBootConfigURL(url string) (bool, error) {
	gitInfo, err := ParseGitURL(url)
	if err != nil {
		return false, errors.Wrap(err, "couldn't parse provided repo URL")
	}
	defaultInfo, err := ParseGitURL(jxconfig.DefaultBootRepository)
	if err != nil {
		return false, errors.Wrap(err, "couldn't parse default boot config URL")
	}
	return gitInfo.HttpsURL() == defaultInfo.HttpsURL(), nil
}
