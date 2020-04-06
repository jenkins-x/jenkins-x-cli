package upgrade

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/boot"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/gits/operations"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
)

// UpgradeBootOptions options for the command
type UpgradeBootOptions struct {
	*opts.CommonOptions
	Dir                     string
	UpgradeVersionStreamRef string
	LatestRelease           bool
	Labels                  []string
}

var (
	upgradeBootLong = templates.LongDesc(`
		This command creates a pr for upgrading a jx boot gitOps cluster, incorporating changes to the boot
        config and version stream ref
`)

	upgradeBootExample = templates.Examples(`
		# create pr for upgrading a jx boot gitOps cluster
		jx upgrade boot
`)

	filesExcludedFromCherryPick = []string{
		"OWNERS",
	}
)

const (
	builderImage         = "gcr.io/jenkinsxio/builder-go"
	keepDevEnvKey        = "keepDevEnv"
	keepDevEnvDriverName = "Always keep the dev env version during merges and cherry-picks"
)

// NewCmdUpgradeBoot creates the command
func NewCmdUpgradeBoot(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeBootOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "boot",
		Short:   "Upgrades jx boot config",
		Long:    upgradeBootLong,
		Example: upgradeBootExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "the directory to look for the Jenkins X Pipeline and requirements")
	cmd.Flags().StringVarP(&options.UpgradeVersionStreamRef, "upgrade-version-stream-ref", "", config.DefaultVersionsRef, "a version stream ref to use to upgrade to")
	cmd.Flags().BoolVarP(&options.LatestRelease, "latest-release", "", false, "upgrade to latest release tag")
	cmd.Flags().StringArrayVarP(&options.Labels, "labels", "", []string{}, "Labels to add to the generated upgrade PR")

	return cmd
}

// Run runs this command
func (o *UpgradeBootOptions) Run() error {
	err := o.setupGitConfig(o.Dir)
	if err != nil {
		return errors.Wrap(err, "failed to setup git config")
	}

	if o.Dir == "" {
		err := o.cloneDevEnv()
		if err != nil {
			return errors.Wrap(err, "failed to clone dev environment repo")
		}
	}

	requirements, requirementsFile, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements config %s", requirementsFile)
	}
	reqsVersionStream := requirements.VersionStream
	upgradeVersionRef, err := o.upgradeAvailable(reqsVersionStream.URL, reqsVersionStream.Ref, o.UpgradeVersionStreamRef)
	if err != nil {
		return errors.Wrap(err, "failed to get check for available update")
	}
	if upgradeVersionRef == "" {
		return nil
	}

	localBranch, err := o.checkoutNewBranch()
	if err != nil {
		return errors.Wrap(err, "failed to checkout upgrade_branch")
	}

	bootConfigURL, err := o.determineBootConfigURL(reqsVersionStream.URL)
	if err != nil {
		return errors.Wrap(err, "failed to determine boot configuration URL")
	}

	err = o.updateBootConfig(reqsVersionStream.URL, reqsVersionStream.Ref, bootConfigURL, upgradeVersionRef)
	if err != nil {
		return errors.Wrap(err, "failed to update boot configuration")
	}

	err = o.updateVersionStreamRef(upgradeVersionRef)
	if err != nil {
		return errors.Wrap(err, "failed to update version stream ref")
	}
	resolver, err := o.CreateVersionResolver(reqsVersionStream.URL, o.UpgradeVersionStreamRef)
	if err != nil {
		return errors.Wrapf(err, "failed to create version resolver")
	}

	err = o.updatePipelineBuilderImage(resolver)
	if err != nil {
		return errors.Wrap(err, "failed to update pipeline version stream ref")
	}

	err = o.updateTemplateBuilderImage(resolver)
	if err != nil {
		return errors.Wrap(err, "failed to update template version stream ref")
	}

	// load modified requirements so we can merge with the base ones
	modifiedRequirements, modifiedRequirementsFile, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements config %s", modifiedRequirementsFile)
	}

	err = requirements.MergeSave(modifiedRequirements, modifiedRequirementsFile)
	if err != nil {
		return errors.Wrap(err, "error merging the modified jx-requirements.yml file with the dev environment's one")
	}

	err = o.createCommitForRequirements(requirementsFile)
	if err != nil {
		return errors.Wrap(err, "failed to create a merge commit for jx-requirements.yml")
	}

	err = o.raisePR()
	if err != nil {
		return errors.Wrap(err, "failed to raise pr")
	}

	err = o.deleteLocalBranch(localBranch)
	if err != nil {
		return errors.Wrapf(err, "failed to delete local branch %s", localBranch)
	}
	return nil
}

func (o UpgradeBootOptions) createCommitForRequirements(requirementsFileName string) error {
	reqsChanged, err := o.Git().HasFileChanged(o.Dir, requirementsFileName)
	if err != nil {
		return errors.Wrap(err, "failed to list changed files")
	}
	if reqsChanged {
		err := o.Git().AddCommitFiles(o.Dir, "Merge jx-requirements.yml", []string{requirementsFileName})
		if err != nil {
			return errors.Wrapf(err, "error creating a commit with the merged jx-requirements.yml file from dir %s",
				requirementsFileName)
		}
	}
	return nil
}

func (o UpgradeBootOptions) determineBootConfigURL(versionStreamURL string) (string, error) {
	var bootConfigURL string
	if versionStreamURL == config.DefaultVersionsURL {
		bootConfigURL = config.DefaultBootRepository
	}

	if bootConfigURL == "" {
		requirements, requirementsFile, err := config.LoadRequirementsConfig(o.Dir)
		if err != nil {
			return "", errors.Wrapf(err, "failed to load requirements config %s", requirementsFile)
		}
		exists, err := util.FileExists(requirementsFile)
		if err != nil {
			return "", errors.Wrapf(err, "failed to check if file %s exists", requirementsFile)
		}
		if !exists {
			return "", fmt.Errorf("no requirements file %s ensure you are running this command inside a GitOps clone", requirementsFile)
		}
		if requirements.BootConfigURL != "" {
			bootConfigURL = requirements.BootConfigURL
		}
	}

	if bootConfigURL == "" {
		return "", fmt.Errorf("unable to determine default boot config URL")
	}
	log.Logger().Infof("using default boot config %s", bootConfigURL)
	return bootConfigURL, nil
}

func (o *UpgradeBootOptions) upgradeAvailable(versionStreamURL string, versionStreamRef string, upgradeRef string) (string, error) {
	versionsDir, _, err := o.CloneJXVersionsRepo(versionStreamURL, upgradeRef)
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone versions repo %s", versionStreamURL)
	}

	if o.LatestRelease {
		_, upgradeRef, err = o.Git().GetCommitPointedToByLatestTag(versionsDir)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get latest tag at %s", o.Dir)
		}
	} else {
		upgradeRef, err = o.Git().GetCommitPointedToByTag(versionsDir, upgradeRef)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get commit pointed to by %s", upgradeRef)
		}
	}

	if versionStreamRef == upgradeRef {
		log.Logger().Infof(util.ColorInfo("No version stream upgrade available"))
		return "", nil
	}
	log.Logger().Infof(util.ColorInfo("Version stream upgrade available"))
	return upgradeRef, nil
}

func (o *UpgradeBootOptions) checkoutNewBranch() (string, error) {
	localBranchUUID, err := uuid.NewV4()
	if err != nil {
		return "", errors.Wrapf(err, "creating UUID for local branch")
	}
	localBranch := localBranchUUID.String()
	err = o.Git().CreateBranch(o.Dir, localBranch)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create local branch %s", localBranch)
	}
	err = o.Git().Checkout(o.Dir, localBranch)
	if err != nil {
		return "", errors.Wrapf(err, "failed to checkout local branch %s", localBranch)
	}
	return localBranch, nil
}

func (o *UpgradeBootOptions) updateVersionStreamRef(upgradeRef string) error {
	requirements, requirementsFile, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements file %s", requirementsFile)
	}

	if requirements.VersionStream.Ref != upgradeRef {
		log.Logger().Infof("Upgrading version stream ref to %s", util.ColorInfo(upgradeRef))
		requirements.VersionStream.Ref = upgradeRef
		err = requirements.SaveConfig(requirementsFile)
		if err != nil {
			return errors.Wrapf(err, "failed to write version stream to %s", requirementsFile)
		}
		err = o.Git().AddCommitFiles(o.Dir, "feat: upgrade version stream", []string{requirementsFile})
		if err != nil {
			return errors.Wrapf(err, "failed to commit requirements file %s", requirementsFile)
		}
	}
	return nil
}

func (o *UpgradeBootOptions) updateBootConfig(versionStreamURL string, versionStreamRef string, bootConfigURL string, upgradeVersionRef string) error {
	configCloneDir, err := o.cloneBootConfig(bootConfigURL)
	if err != nil {
		return errors.Wrapf(err, "failed to clone boot config repo %s", bootConfigURL)
	}
	defer func() {
		err := os.RemoveAll(configCloneDir)
		if err != nil {
			log.Logger().Infof("Error removing tmpDir: %v", err)
		}
	}()

	currentSha, currentVersion, err := o.bootConfigRef(configCloneDir, versionStreamURL, versionStreamRef, bootConfigURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get boot config ref for version stream: %s", versionStreamRef)
	}

	upgradeSha, upgradeVersion, err := o.bootConfigRef(configCloneDir, versionStreamURL, upgradeVersionRef, bootConfigURL)
	if err != nil {
		return errors.Wrapf(err, "failed to get boot config ref for version stream ref: %s", upgradeVersionRef)
	}

	// check if boot config upgrade available
	if upgradeSha == currentSha {
		log.Logger().Infof(util.ColorInfo("No boot config upgrade available"))
		return nil
	}
	log.Logger().Infof(util.ColorInfo("boot config upgrade available"))
	log.Logger().Infof("Upgrading from %s to %s", util.ColorInfo(currentVersion), util.ColorInfo(upgradeVersion))

	// Fetch the tag we're upgrading from.
	err = o.Git().FetchBranch(o.Dir, bootConfigURL, currentVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch current tag %s from %s", currentVersion, bootConfigURL)
	}

	// Fetch the tag we're upgrading to.
	err = o.Git().FetchBranch(o.Dir, bootConfigURL, upgradeVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch upgrade tag %s from %s", upgradeVersion, bootConfigURL)
	}

	// Set up custom merge driver to ensure that specified files always use the local/dev env version in merges/cherry picks with conflicts
	err = o.configureGitMergeExcludes()
	if err != nil {
		return errors.Wrapf(err, "configuring files (%s) to always use existing version when conflicting with upstream", strings.Join(filesExcludedFromCherryPick, ", "))
	}

	defer func() {
		err = o.removeMergeExcludesFromAttributes()
	}()

	err = o.cherryPickCommits(configCloneDir, currentSha, upgradeSha)
	if err != nil {
		return errors.Wrap(err, "failed to cherry pick upgrade commits")
	}

	return nil
}

// Add a custom merge driver to .git/config that will always choose the current version when there's a change upstream
// in selected files, and use that merge driver in .git/info/attributes for those selected files.
// see https://stackoverflow.com/a/930495 for more details
func (o *UpgradeBootOptions) configureGitMergeExcludes() error {
	err := o.Git().Config(o.Dir, "--local", fmt.Sprintf("merge.%s.name", keepDevEnvKey), keepDevEnvDriverName)
	if err != nil {
		return errors.Wrapf(err, "configuring name for exclude merge driver in %s", o.Dir)
	}
	err = o.Git().Config(o.Dir, "--local", fmt.Sprintf("merge.%s.driver", keepDevEnvKey), "true")
	if err != nil {
		return errors.Wrapf(err, "configuring driver for exclude merge driver in %s", o.Dir)
	}

	existingGitAttr, err := o.Git().ReadRepoAttributes(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "reading repo-local git attributes")
	}

	// Write the existing .git/info/attributes content and marking the selected files as using our custom driver
	gitAttrContent := existingGitAttr + "\n"
	for _, excludedFile := range filesExcludedFromCherryPick {
		newGitAttrLine := fmt.Sprintf("%s merge=%s\n", excludedFile, keepDevEnvKey)
		if !strings.Contains(gitAttrContent, newGitAttrLine) {
			gitAttrContent += newGitAttrLine
		}
	}

	err = o.Git().WriteRepoAttributes(o.Dir, gitAttrContent)
	if err != nil {
		return errors.Wrapf(err, "writing new repo-local git attributes")
	}

	return nil
}

func (o *UpgradeBootOptions) removeMergeExcludesFromAttributes() error {
	// Read the existing .git/info/attributes if it exists
	existingGitAttr, err := o.Git().ReadRepoAttributes(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "reading current repo-local git attributes")
	}
	if existingGitAttr != "" {
		var newGitAttrLines []string
		for _, l := range strings.Split(existingGitAttr, "\n") {
			if !strings.Contains(l, fmt.Sprintf("merge=%s", keepDevEnvKey)) {
				newGitAttrLines = append(newGitAttrLines, l)
			}
		}
		err = o.Git().WriteRepoAttributes(o.Dir, strings.Join(newGitAttrLines, "\n"))
		if err != nil {
			return errors.Wrapf(err, "writing cleaned-up git attributes")
		}
	}
	return nil
}

func (o *UpgradeBootOptions) bootConfigRef(dir string, versionStreamURL string, versionStreamRef string, configURL string) (string, string, error) {
	resolver, err := o.CreateVersionResolver(versionStreamURL, versionStreamRef)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to create version resolver %s", configURL)
	}
	configVersion, err := resolver.ResolveGitVersion(configURL)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to resolve config url %s", configURL)
	}
	cmtSha, err := o.Git().GetCommitPointedToByTag(dir, fmt.Sprintf("v%s", configVersion))
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to get commit pointed to by %s", cmtSha)
	}
	return cmtSha, "v" + configVersion, nil
}

func (o *UpgradeBootOptions) cloneBootConfig(configURL string) (string, error) {
	cloneDir, err := ioutil.TempDir("", "")
	err = os.MkdirAll(cloneDir, util.DefaultWritePermissions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create directory: %s", cloneDir)
	}

	err = o.Git().CloneBare(cloneDir, configURL)
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone git URL %s to directory %s", configURL, cloneDir)
	}
	return cloneDir, nil
}

func (o *UpgradeBootOptions) cherryPickCommits(cloneDir, fromSha, toSha string) error {
	cmts := make([]gits.GitCommit, 0)
	cmts, err := o.Git().GetCommits(cloneDir, fromSha, toSha)
	if err != nil {
		return errors.Wrapf(err, "failed to get commits from %s", cloneDir)
	}

	log.Logger().Infof("cherry picking commits in the range %s..%s", fromSha, toSha)
	for i := len(cmts) - 1; i >= 0; i-- {
		commitSha := cmts[i].SHA
		commitMsg := cmts[i].Subject()

		// cherry-pick commits preserving redundant commits to avoid error
		err := o.Git().CherryPickTheirsKeepRedundantCommits(o.Dir, commitSha)
		if err != nil {
			msg := fmt.Sprintf("commit %s is a merge but no -m option was given.", commitSha)
			if !strings.Contains(err.Error(), msg) {
				return errors.Wrapf(err, "cherry-picking %s", commitSha)
			}
		} else {
			log.Logger().Infof("%s - %s", commitSha, commitMsg)
		}
	}
	return nil
}

func (o *UpgradeBootOptions) setupGitConfig(dir string) error {
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create/get jx client and dev namespace")
	}
	devEnv, err := kube.GetDevEnvironment(jxClient, devNs)
	if err != nil {
		return errors.Wrapf(err, "failed to get dev environment in namespace %s", devNs)
	}
	username := devEnv.Spec.TeamSettings.PipelineUsername
	email := devEnv.Spec.TeamSettings.PipelineUserEmail
	err = o.Git().SetUsername(dir, username)
	if err != nil {
		return errors.Wrapf(err, "failed to set username %s", username)
	}
	err = o.Git().SetEmail(dir, email)
	if err != nil {
		return errors.Wrapf(err, "failed to set email for %s", email)
	}
	return nil
}

func (o *UpgradeBootOptions) raisePR() error {
	gitInfo, provider, _, err := o.CreateGitProvider(o.Dir)
	if err != nil {
		return errors.Wrap(err, "failed to get git provider")
	}

	upstreamInfo, err := provider.GetRepository(gitInfo.Organisation, gitInfo.Name)
	if err != nil {
		return errors.Wrapf(err, "getting repository %s/%s", gitInfo.Organisation, gitInfo.Name)
	}

	details, filter, err := o.prDetailsAndFilter()
	if err != nil {
		return errors.Wrapf(err, "failed to get PR details and filter")
	}

	_, err = gits.PushRepoAndCreatePullRequest(o.Dir, upstreamInfo, nil, "master", &details, &filter, false, details.Title, true, false, o.Git(), provider)
	if err != nil {
		return errors.Wrapf(err, "failed to create PR for base %s and head branch %s", "master", details.BranchName)
	}
	return nil
}

func (o *UpgradeBootOptions) prDetailsAndFilter() (gits.PullRequestDetails, gits.PullRequestFilter, error) {
	details := gits.PullRequestDetails{
		BranchName: fmt.Sprintf("jx_boot_upgrade"),
		Title:      "feat(config): upgrade configuration",
		Message:    "Upgrade configuration",
	}

	labels := []string{}
	if len(o.Labels) > 0 {
		labels = append(labels, o.Labels...)
	}
	filter := gits.PullRequestFilter{
		Labels: []string{
			boot.PullRequestLabel,
		},
	}
	details.Labels = labels

	return details, filter, nil
}

func (o *UpgradeBootOptions) deleteLocalBranch(branch string) error {
	err := o.Git().Checkout(o.Dir, "master")
	if err != nil {
		return errors.Wrapf(err, "failed to checkout master branch")
	}
	err = o.Git().DeleteLocalBranch(o.Dir, branch)
	if err != nil {
		return errors.Wrapf(err, "failed to delete local branch %s", branch)
	}
	return nil
}

func (o *UpgradeBootOptions) cloneDevEnv() error {
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create/get jx client and dev namespace")
	}
	devEnv, err := kube.GetDevEnvironment(jxClient, devNs)
	if err != nil {
		return errors.Wrapf(err, "failed to get dev environment in namespace %s", devNs)
	}
	if devEnv == nil {
		return errors.Wrapf(errors.New("Cannot find environment with dev selector"), "No development environment found for namespace %s", devNs)
	}
	devEnvURL := devEnv.Spec.Source.URL

	cloneDir, err := ioutil.TempDir("", "")
	if err != nil {
		return errors.Wrapf(err, "failed to create tmp dir to clone dev env repo")
	}
	err = os.MkdirAll(cloneDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to give write perms to tmp dir to clone dev env repo")
	}

	gitInfo, err := gits.ParseGitURL(devEnvURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse url %s", devEnvURL)
	}
	_, userAuth, err := o.GetPipelineGitAuthForRepo(gitInfo)
	if err != nil {
		return errors.Wrap(err, "failed to get pipeline user auth")
	}
	cloneURL, err := o.Git().CreateAuthenticatedURL(devEnvURL, userAuth)

	if err != nil {
		return errors.Wrapf(err, "failed to create directory for dev env clone: %s", cloneDir)
	}
	err = o.Git().Clone(cloneURL, cloneDir)
	if err != nil {
		return errors.Wrapf(err, "failed to clone git URL %s to directory %s", devEnvURL, cloneDir)
	}

	o.Dir = cloneDir
	return nil
}

func (o *UpgradeBootOptions) updatePipelineBuilderImage(resolver *versionstream.VersionResolver) error {
	piplineFileGlob := "jenkins-x*.yml"
	updatedBuilderImage, err := resolver.ResolveDockerImage(builderImage)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve image %s", builderImage)
	}
	log.Logger().Infof("Updating pipeline agent images to %s", util.ColorInfo(updatedBuilderImage))
	fn, err := operations.CreatePullRequestRegexFn(updatedBuilderImage, `(?m)^\s*agent:\n\s*image: (gcr.io\/jenkinsxio\/builder-go.*$)`, piplineFileGlob)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = fn(o.Dir, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	matches, err := filepath.Glob(filepath.Join(o.Dir, piplineFileGlob))
	if err != nil {
		return errors.Wrapf(err, "applying glob %s", piplineFileGlob)
	}
	for i, match := range matches {
		matches[i], err = filepath.Rel(o.Dir, match)
		if err != nil {
			return errors.Wrapf(err, "failed to build path for pipeline file %s", match)
		}
	}
	err = o.Git().AddCommitFiles(o.Dir, "feat: upgrade pipeline builder images", matches)
	if err != nil {
		log.Logger().Info("Skipping builder image update as no changes were detected")
	}
	return nil
}

func (o *UpgradeBootOptions) updateTemplateBuilderImage(resolver *versionstream.VersionResolver) error {
	templateFile := fmt.Sprintf("env/%s", helm.ValuesTemplateFileName)
	updatedBuilderImage, err := resolver.ResolveDockerImage(builderImage)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve image %s", builderImage)
	}
	log.Logger().Infof("Updating template builder images to %s", util.ColorInfo(updatedBuilderImage))
	fn, err := operations.CreatePullRequestRegexFn(updatedBuilderImage, `(?m)^\s*builderImage: (gcr.io\/jenkinsxio\/builder-go.*$)`, templateFile)
	if err != nil {
		return errors.WithStack(err)
	}
	_, err = fn(o.Dir, nil)
	if err != nil {
		return errors.WithStack(err)
	}
	err = o.Git().AddCommitFiles(o.Dir, "feat: upgrade template builder images", []string{templateFile})
	if err != nil {
		log.Logger().Info("Skipping template builder image update as no changes were detected")
	}
	return nil
}
