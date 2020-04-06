package boot

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/boot"
	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"
	"github.com/jenkins-x/jx/pkg/cmd/step/create"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// BootOptions options for the command
type BootOptions struct {
	*opts.CommonOptions

	Dir          string
	GitURL       string
	GitRef       string
	StartStep    string
	EndStep      string
	HelmLogLevel string

	// The bootstrap URL for the version stream. Once we have a jx-requirements.yml files, we read that
	VersionStreamURL string
	// The bootstrap ref for the version stream. Once we have a jx-requirements.yml, we read that
	VersionStreamRef string

	// RequirementsFile provided by the user to override the default requirements file from repository
	RequirementsFile string

	AttemptRestore bool

	// UpgradeGit if we want to automatically upgrade this boot clone if there have been changes since the current clone
	NoUpgradeGit bool
}

var (
	bootLong = templates.LongDesc(`
		Boots up Jenkins X in a Kubernetes cluster using GitOps and a Jenkins X Pipeline

		For more documentation see: [https://jenkins-x.io/docs/getting-started/setup/boot/](https://jenkins-x.io/docs/getting-started/setup/boot/)

`)

	bootExample = templates.Examples(`
		# create a kubernetes cluster via Terraform or via jx
		jx create cluster gke --skip-installation

		# now lets boot up Jenkins X installing/upgrading whatever is needed
		jx boot

		# if we have already booted and just want to apply some environment changes without
        # re-applying ingress and so forth we can start at the environment step:
		jx boot --start-step install-env
`)
)

// NewCmdBoot creates the command
func NewCmdBoot(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &BootOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "boot",
		Aliases: []string{"bootstrap"},
		Short:   "Boots up Jenkins X in a Kubernetes cluster using GitOps and a Jenkins X Pipeline",
		Long:    bootLong,
		Example: bootExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the Jenkins X Pipeline, requirements and charts")
	cmd.Flags().StringVarP(&options.GitURL, "git-url", "u", "", "override the Git clone URL for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-ref as well")
	cmd.Flags().StringVarP(&options.GitRef, "git-ref", "", "", "override the Git ref for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-url as well")
	cmd.Flags().StringVarP(&options.VersionStreamURL, "versions-repo", "", config.DefaultVersionsURL, "the bootstrap URL for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	cmd.Flags().StringVarP(&options.VersionStreamRef, "versions-ref", "", config.DefaultVersionsRef, "the bootstrap ref for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	cmd.Flags().StringVarP(&options.StartStep, "start-step", "s", "", "the step in the pipeline to start from")
	cmd.Flags().StringVarP(&options.EndStep, "end-step", "e", "", "the step in the pipeline to end at")
	cmd.Flags().StringVarP(&options.HelmLogLevel, "helm-log", "v", "", "sets the helm logging level from 0 to 9. Passed into the helm CLI via the '-v' argument. Useful to diagnose helm related issues")
	cmd.Flags().StringVarP(&options.RequirementsFile, "requirements", "r", "", "requirements file which will overwrite the default requirements file")
	cmd.Flags().BoolVarP(&options.AttemptRestore, "attempt-restore", "a", false, "attempt to boot from an existing dev environment repository")
	cmd.Flags().BoolVarP(&options.NoUpgradeGit, "no-update-git", "", false, "disables any attempt to update the local git clone if its old")

	return cmd
}

// Run runs this command
func (o *BootOptions) Run() error {
	err := o.verifyClusterConnection()
	if err != nil {
		return err
	}

	o.overrideSteps()

	if o.AttemptRestore {
		err := o.restoreFromDevEnvRepo()
		if err != nil {
			return errors.Wrapf(err, "unable tp restore dev environment repo")
		}
	}

	gitURL, gitRef := o.determineGitURLAndRef()
	if gitURL == "" {
		return util.MissingOption("git-url")
	}

	isBootClone, err := existingBootClone(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to check if this is an existing boot clone")
	}

	isGitRepo := o.isGitRepo(o.Dir)
	if isGitRepo && !isBootClone {
		return errors.Errorf("trying to execute 'jx boot' from a non requirements repo")
	}

	if !isBootClone {
		gitInfo, err := gits.ParseGitURL(gitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to parse git URL %s", gitURL)
		}
		dir := filepath.Join(o.Dir, gitInfo.Name)
		cloneDir, err := o.createBootClone(gitURL, gitRef, dir)
		if err != nil {
			return errors.Wrapf(err, "unable to clone %s", gitURL)
		}
		o.Dir = cloneDir
	}

	requirements, requirementsFile, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "unable to load %s", config.RequirementsConfigFileName)
	}

	err = o.ConfigureCommonOptions(requirements)
	if err != nil {
		return err
	}

	// lets report errors parsing this file after the check we are outside of a git clone
	o.defaultVersionStream(requirements)

	resolver, err := o.CreateVersionResolver(requirements.VersionStream.URL, requirements.VersionStream.Ref)
	if err != nil {
		return errors.Wrapf(err, "there was a problem creating a version resolver from versions stream repository %s and ref %s", requirements.VersionStream.URL, requirements.VersionStream.Ref)
	}

	// lets avoid trying to reset the current git clone to master if using NoUpgradeGit
	if o.GitRef == "" && !o.NoUpgradeGit {
		gitRef, err = o.determineGitRef(resolver, requirements, gitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to determine git ref")
		}
	}

	projectConfig, pipelineFile, err := config.LoadProjectConfig(o.Dir)
	if err != nil {
		return err
	}

	if err := o.overrideRequirements(gitURL); err != nil {
		return errors.Wrap(err, "overwriting the default requirements")
	}

	// only update boot if the a GitRef has not been supplied
	if o.GitRef == "" && !o.NoUpgradeGit {
		err = o.updateBootCloneIfOutOfDate(gitRef)
		if err != nil {
			return err
		}
	}

	err = o.verifyRequirements(requirements, requirementsFile)
	if err != nil {
		return err
	}

	log.Logger().Infof("Booting Jenkins X")

	// now lets really boot
	_, so := create.NewCmdStepCreateTaskAndOption(o.CommonOptions)
	so.CloneDir = o.Dir
	so.CloneDir = o.Dir
	so.InterpretMode = true
	so.NoReleasePrepare = true
	so.StartStep = o.StartStep
	so.EndStep = o.EndStep

	so.AdditionalEnvVars = map[string]string{
		"JX_NO_TILLER":                     "true",
		boot.ConfigRepoURLEnvVarName:       gitURL,
		boot.ConfigBaseRefEnvVarName:       gitRef,
		boot.VersionsRepoURLEnvVarName:     requirements.VersionStream.URL,
		boot.VersionsRepoBaseRefEnvVarName: requirements.VersionStream.Ref,
	}
	if o.NoUpgradeGit {
		so.AdditionalEnvVars[boot.DisablePushUpdatesToDevEnvironment] = "true"
	}
	if requirements.Cluster.HelmMajorVersion == "3" {
		so.AdditionalEnvVars["JX_HELM3"] = "true"
	} else {
		so.AdditionalEnvVars["JX_NO_TILLER"] = "true"
	}
	if o.HelmLogLevel != "" {
		so.AdditionalEnvVars["JX_HELM_VERBOSE"] = o.HelmLogLevel
	}

	// Set the namespace in the pipeline
	so.CommonOptions.SetDevNamespace(requirements.Cluster.Namespace)
	// lets ensure the namespace is set in the jenkins-x.yml file
	envVars := make([]v1.EnvVar, 0)
	for _, e := range projectConfig.PipelineConfig.Pipelines.Release.Pipeline.Environment {
		if e.Name == "DEPLOY_NAMESPACE" {
			envVars = append(envVars, v1.EnvVar{
				Name:  "DEPLOY_NAMESPACE",
				Value: requirements.Cluster.Namespace,
			})
		} else {
			envVars = append(envVars, e)
		}
	}
	projectConfig.PipelineConfig.Pipelines.Release.Pipeline.Environment = envVars
	err = projectConfig.SaveConfig(pipelineFile)
	if err != nil {
		return errors.Wrapf(err, "setting namespace in jenkins-x.yml")
	}
	so.VersionResolver = resolver

	if o.BatchMode {
		so.AdditionalEnvVars["JX_BATCH_MODE"] = "true"
	}
	err = so.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to interpret pipeline file %s", pipelineFile)
	}

	log.Logger().Debugf("Using additional vars: %+v", so.AdditionalEnvVars)

	// lets switch kubernetes context to it so the user can use `jx` commands immediately
	no := &namespace.NamespaceOptions{}
	no.CommonOptions = o.CommonOptions
	no.Args = []string{requirements.Cluster.Namespace}
	return no.Run()
}

func (o *BootOptions) isGitRepo(dir string) bool {
	_, _, err := gits.GetGitInfoFromDirectory(dir, o.Git())
	if err == nil {
		return true
	}
	return false
}

func (o *BootOptions) determineGitURLAndRef() (string, string) {
	gitURL, gitRef, err := gits.GetGitInfoFromDirectory(o.Dir, o.Git())
	if err != nil {
		log.Logger().Info("Creating boot config with defaults, as not in an existing boot directory with a git repository.")
		gitURL = config.DefaultBootRepository
		gitRef = config.DefaultVersionsRef
	}

	if o.GitURL != "" {
		log.Logger().Infof("GitURL provided, overriding the current value: %s with %s", util.ColorInfo(gitURL), util.ColorInfo(o.GitURL))
		gitURL = o.GitURL
	}

	if o.GitRef != "" {
		log.Logger().Infof("GitRef provided, overriding the current value: %s with %s", util.ColorInfo(gitRef), util.ColorInfo(o.GitRef))
		gitRef = o.GitRef
	}
	return gitURL, gitRef
}

func (o *BootOptions) createBootClone(bootConfigGitURL string, bootConfigGitRef string, cloneDir string) (string, error) {
	info := util.ColorInfo
	log.Logger().Infof("No Jenkins X pipeline file %s or no jx boot requirements file %s found. You are not running this command from inside a "+
		"Jenkins X Boot git clone", info(config.ProjectConfigFileName), info(config.RequirementsConfigFileName))

	if !o.BatchMode {
		log.Logger().Infof("To continue we will clone %s @ %s to %s", info(bootConfigGitURL), info(bootConfigGitRef), info(cloneDir))

		help := "A git clone of a Jenkins X Boot source repository is required for 'jx boot'"
		message := "Do you want to clone the Jenkins X Boot Git repository?"
		if answer, err := util.Confirm(message, true, help, o.GetIOFileHandles()); err != nil {
			return "", errors.Wrapf(err, "unable to process user input")
		} else if !answer {
			return "", fmt.Errorf("please run this command again inside a git clone from a Jenkins X Boot repository")
		}
	}

	bootCloneExists, err := util.DirExists(cloneDir)
	if err != nil {
		return "", errors.Wrapf(err, "unable to check whether directory %s exists", cloneDir)
	}
	if bootCloneExists {
		return "", fmt.Errorf("cannot clone git repository to %s as the dir already exists", cloneDir)
	}

	log.Logger().Infof("Cloning %s @ %s to %s\n", info(bootConfigGitURL), info(bootConfigGitRef), info(cloneDir))

	err = os.MkdirAll(cloneDir, util.DefaultWritePermissions)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create directory: %s", cloneDir)
	}

	err = o.Git().Clone(bootConfigGitURL, cloneDir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone git URL %s to directory: %s", bootConfigGitURL, cloneDir)
	}

	commitish, err := gits.FindTagForVersion(cloneDir, bootConfigGitRef, o.Git())
	if err != nil {
		log.Logger().Debugf(errors.Wrapf(err, "finding tag for %s", bootConfigGitRef).Error())
	}

	if commitish != "" {
		err = o.Git().Reset(cloneDir, commitish, true)
		if err != nil {
			return "", errors.Wrapf(err, "setting HEAD to %s", commitish)
		}
	} else {
		log.Logger().Infof("fetching branch %s", bootConfigGitRef)
		err = o.Git().FetchBranch(cloneDir, "origin", bootConfigGitRef)
		if err != nil {
			return "", errors.Wrapf(err, "fetching branch %s", bootConfigGitRef)
		}

		branchName := uuid.New().String()

		err = o.Git().CreateBranchFrom(cloneDir, branchName, bootConfigGitRef)
		if err != nil {
			return "", errors.Wrapf(err, "create branch %s from %s", branchName, bootConfigGitRef)
		}

		err = o.Git().Checkout(cloneDir, branchName)
		if err != nil {
			return "", errors.Wrapf(err, "checkout branch %s", branchName)
		}
	}

	cloneDir, err = filepath.Abs(cloneDir)
	if err != nil {
		return "", errors.Wrapf(err, "unable to determine absolute path for %s", cloneDir)
	}
	return cloneDir, nil
}

func (o *BootOptions) restoreFromDevEnvRepo() error {
	url := o.determineDevEnvironmentUrl()
	if url != "" {
		cloned, dir, err := o.cloneDevEnvironment(url)
		if err != nil {
			return err
		}
		if cloned {
			err = os.Chdir(dir)
			if err != nil {
				return errors.Wrapf(err, "failed to change into new directory: %s", dir)
			}
		} else {
			log.Logger().Infof("failed to clone dev environment booting from %s", o.GitURL)
		}
	} else {
		log.Logger().Infof("cannot determine dev environment url booting from %s", o.GitURL)
	}
	return nil
}

func (o *BootOptions) determineDevEnvironmentUrl() string {
	gitProvider := os.Getenv("JX_VALUE_GITPROVIDER")
	gitOwner := os.Getenv(config.RequirementEnvGitOwner)
	clusterName := os.Getenv(config.RequirementClusterName)
	if gitProvider != "" && gitOwner != "" && clusterName != "" {
		repo := fmt.Sprintf("environment-%s-dev", clusterName)
		repoName := o.Git().RepoName(gitOwner, repo)
		url := fmt.Sprintf("https://%s.com/%s", gitProvider, repoName)
		log.Logger().Infof("dev environment url is %s", url)
		return url
	}
	return ""
}

func (o *BootOptions) cloneDevEnvironment(gitURL string) (bool, string, error) {
	log.Logger().Infof("dev environment url specified %t ", o.AttemptRestore)
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to parse git URL %s", gitURL)
	}

	repo := gitInfo.Name
	cloneDir := filepath.Join(o.Dir, repo)

	err = os.MkdirAll(cloneDir, util.DefaultWritePermissions)
	if err != nil {
		return false, "", errors.Wrapf(err, "failed to create directory: %s", cloneDir)
	}

	err = o.Git().Clone(gitURL, cloneDir)
	if err != nil {
		log.Logger().Infof("failed to clone git URL %s to directory: %s", gitURL, cloneDir)
		rmErr := os.RemoveAll(cloneDir)
		if rmErr != nil {
			log.Logger().Warnf("Unable to remove dev env directory")
		}
		return false, "", nil
	}

	return true, cloneDir, nil
}

func (o *BootOptions) updateBootCloneIfOutOfDate(gitRef string) error {
	// Get the tag corresponding to the git ref.
	commitish, err := gits.FindTagForVersion(o.Dir, gitRef, o.Git())
	if err != nil {
		log.Logger().Debugf(errors.Wrapf(err, "finding tag for %s", gitRef).Error())
		commitish = fmt.Sprintf("%s/%s", "origin", gitRef)
	}

	// Check if the current HEAD is an ancestor of the tag.
	isAncestor, err := o.Git().IsAncestor(o.Dir, "HEAD", commitish)
	if err != nil {
		log.Logger().Debugf(errors.Wrapf(err, "couldn't determine whether HEAD is an ancestor of %s", commitish).Error())
	}

	// Get the tag on the current boot clone HEAD, if any.
	resolved, _, err := o.Git().Describe(o.Dir, true, "HEAD", "0", true)
	if err != nil {
		return errors.Wrap(err, "could not describe boot clone HEAD to find its tag with git describe HEAD --abbrev=0 --contains --always")
	}

	if resolved != commitish {
		if isAncestor {
			log.Logger().Infof("Local boot clone is out of date. It is based on %s, but the version stream is using %s. The clone will now be updated to %s.",
				resolved, commitish, commitish)
			log.Logger().Info("Stashing any changes made in local boot clone.")

			err = o.Git().StashPush(o.Dir)
			if err != nil {
				return errors.WithStack(err)
			}
			err = o.Git().Reset(o.Dir, commitish, true)
			if err != nil {
				return errors.Wrapf(err, "Could not reset local boot clone to %s", commitish)
			}

			err = o.Git().StashPop(o.Dir)
			if err != nil && !gits.IsNoStashEntriesError(err) { // Ignore no stashes as that's just because there was nothing to stash
				return fmt.Errorf("Could not update local boot clone due to conflicts between local changes and %s.\n"+
					"To fix this, resolve the conflicts listed below manually, run 'git reset HEAD', and run 'jx boot' again.\n%s",
					commitish, gits.GetSimpleIndentedStashPopErrorMessage(err))
			}
		} else {
			log.Logger().Infof("Current HEAD %s in %s is not an ancestor of %s, the boot config version from the version stream.\n"+
				"Proceeding with current HEAD.", resolved, o.Dir, commitish)
			return nil
		}
	}
	return nil
}

func existingBootClone(dir string) (bool, error) {
	pipelineExists, err := util.FileExists(filepath.Join(dir, config.ProjectConfigFileName))
	if err != nil {
		return false, err
	}
	requirementsExist, err := util.FileExists(filepath.Join(dir, config.RequirementsConfigFileName))
	if err != nil {
		return false, err
	}
	return requirementsExist && pipelineExists, nil
}

func (o *BootOptions) overrideRequirements(defaultBootConfigURL string) error {
	requirements, requirementsFile, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "loading requirements from dir %q", o.Dir)
	}

	// overwrite the default requirements with provided requirements
	if o.RequirementsFile != "" {
		providedRequirements, err := config.LoadRequirementsConfigFile(o.RequirementsFile)
		if err != nil {
			return errors.Wrapf(err, "loading requirements from file %q", o.RequirementsFile)
		}
		*requirements = *providedRequirements
	}

	o.defaultVersionStream(requirements)
	if requirements.BootConfigURL == "" {
		requirements.BootConfigURL = defaultBootConfigURL
	}

	if err := requirements.SaveConfig(requirementsFile); err != nil {
		return errors.Wrapf(err, "saving the requirements into file %q", requirementsFile)
	}

	return nil
}

func (o *BootOptions) determineGitRef(resolver *versionstream.VersionResolver, requirements *config.RequirementsConfig, gitURL string) (string, error) {
	// If the GitRef is not overridden and is set to it's default value then look up the version number
	log.Logger().Infof("Attempting to resolve version for boot config %s from %s", util.ColorInfo(gitURL), util.ColorInfo(requirements.VersionStream.URL))
	gitRef, err := resolver.ResolveGitVersion(gitURL)
	if err != nil {
		return "", errors.Wrapf(err, fmt.Sprintf("failed to resolve version for %s in version stream %s",
			gitURL, requirements.VersionStream.URL))
	}
	if gitRef == "" {
		log.Logger().Infof("no version for %s found in version stream %s, defaulting to %s",
			util.ColorInfo(gitURL), util.ColorInfo(requirements.VersionStream.URL), util.ColorInfo("master"))
		gitRef = "master"
	}
	return gitRef, nil
}

func (o *BootOptions) defaultVersionStream(requirements *config.RequirementsConfig) {
	if requirements.VersionStream.URL == "" && requirements.VersionStream.Ref == "" {
		requirements.VersionStream.URL = o.VersionStreamURL
		requirements.VersionStream.Ref = o.VersionStreamRef
	}
	// If we still don't have a complete version stream ref then we better set to a default
	if requirements.VersionStream.URL == "" || requirements.VersionStream.Ref == "" {
		log.Logger().Warnf("Incomplete version stream reference %s @ %s", requirements.VersionStream.URL, requirements.VersionStream.Ref)
		o.VersionStreamRef = config.DefaultVersionsRef
		o.VersionStreamURL = config.DefaultVersionsURL
		requirements.VersionStream.URL = o.VersionStreamURL
		requirements.VersionStream.Ref = o.VersionStreamRef
		log.Logger().Infof("Setting version stream reference to default %s @ %s", requirements.VersionStream.URL, requirements.VersionStream.Ref)
	}
}

func (o *BootOptions) verifyRequirements(requirements *config.RequirementsConfig, requirementsFile string) error {
	provider := requirements.Cluster.Provider
	if provider == "" {
		return config.MissingRequirement("provider", requirementsFile)
	}
	if requirements.Cluster.Namespace == "" {
		return config.MissingRequirement("namespace", requirementsFile)
	}
	return nil
}

func (o *BootOptions) verifyClusterConnection() error {
	client, err := o.KubeClient()
	if err == nil {
		_, err = client.CoreV1().Namespaces().List(metav1.ListOptions{})
	}

	if err != nil {
		return fmt.Errorf("You are not currently connected to a cluster, please connect to the cluster that you intend to %s\n"+
			"Alternatively create a new cluster using %s", util.ColorInfo("jx boot"), util.ColorInfo("jx create cluster"))
	}
	return nil
}

func (o *BootOptions) overrideSteps() {
	if o.StartStep == "" {
		startStep := os.Getenv("JX_BOOT_START_STEP")
		if startStep != "" {
			log.Logger().Debugf("Overriding start-step with env var: '%s'", startStep)
			o.StartStep = startStep
		}
	}

	if o.EndStep == "" {
		endStep := os.Getenv("JX_BOOT_END_STEP")
		if endStep != "" {
			log.Logger().Debugf("Overriding end-step with env var: '%s'", endStep)
			o.EndStep = endStep
		}
	}
}
