package bdd

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/builds"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/boot"

	"github.com/jenkins-x/jx/pkg/cmd/step/e2e"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/deletecmd"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	configio "github.com/jenkins-x/jx/pkg/io"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	optionDefaultAdminPassword = "default-admin-password"
)

// StepBDDOptions contains the command line arguments for this command
type StepBDDOptions struct {
	step.StepOptions

	InstallOptions create.InstallOptions
	Flags          StepBDDFlags
}

type StepBDDFlags struct {
	GoPath              string
	GitProvider         string
	GitOwner            string
	ReportsOutputDir    string
	UseCurrentTeam      bool
	DeleteTeam          bool
	DisableDeleteApp    bool
	DisableDeleteRepo   bool
	IgnoreTestFailure   bool
	Parallel            bool
	VersionsDir         string
	VersionsRepository  string
	VersionsGitRef      string
	ConfigFile          string
	TestRepoGitCloneUrl string
	SkipRepoGitClone    bool
	UseRevision         bool
	TestGitBranch       string
	TestGitPrNumber     string
	JxBinary            string
	TestCases           []string
	VersionsRepoPr      bool
	BaseDomain          string
	Dir                 string
}

var (
	stepBDDLong = templates.LongDesc(`
		This pipeline step lets you run the BDD tests in the current team in a current cluster or create a new cluster/team run tests there then tear things down again.

`)

	stepBDDExample = templates.Examples(`
		# run the BDD tests in the current team
		jx step bdd --use-current-team --git-provider-url=https://my.git.server.com

        # create a new team for the tests, run the tests then tear everything down again
		jx step bdd -b --provider=gke --git-provider=ghe --git-provider-url=https://my.git.server.com --default-admin-password=myadminpwd --git-username myuser --git-api-token mygittoken
`)
)

func NewCmdStepBDD(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepBDDOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
		InstallOptions: create.CreateInstallOptions(commonOpts),
	}
	cmd := &cobra.Command{
		Use:     "bdd",
		Short:   "Performs the BDD tests on the current cluster, new clusters or teams",
		Long:    stepBDDLong,
		Example: stepBDDExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	installOptions := &options.InstallOptions
	installOptions.AddInstallFlags(cmd, true)

	cmd.Flags().StringVarP(&options.Flags.BaseDomain, "base-domain", "", "", "the base domain to use when creating the cluster")
	cmd.Flags().StringVarP(&options.Flags.ConfigFile, "config", "c", "", "the config YAML file containing the clusters to create")
	cmd.Flags().StringVarP(&options.Flags.GoPath, "gopath", "", "", "the GOPATH directory where the BDD test git repository will be cloned")
	cmd.Flags().StringVarP(&options.Flags.GitProvider, "git-provider", "g", "", "the git provider kind")
	cmd.Flags().StringVarP(&options.Flags.GitOwner, "git-owner", "", "", "the git owner of new git repositories created by the tests")
	cmd.Flags().StringVarP(&options.Flags.ReportsOutputDir, "reports-dir", "", "reports", "the directory used to copy in any generated report files")
	cmd.Flags().StringVarP(&options.Flags.TestRepoGitCloneUrl, "test-git-repo", "r", "https://github.com/jenkins-x/bdd-jx.git", "the git repository to clone for the BDD tests")
	cmd.Flags().BoolVarP(&options.Flags.SkipRepoGitClone, "skip-test-git-repo-clone", "", false, "Skip cloning the bdd test git repo")
	cmd.Flags().StringVarP(&options.Flags.JxBinary, "binary", "", "jx", "the binary location of the 'jx' executable for creating clusters")
	cmd.Flags().StringVarP(&options.Flags.TestGitBranch, "test-git-branch", "", "master", "the git repository branch to use for the BDD tests")
	cmd.Flags().StringVarP(&options.Flags.TestGitPrNumber, "test-git-pr-number", "", "", "the Pull Request number to fetch from the repository for the BDD tests")
	cmd.Flags().StringArrayVarP(&options.Flags.TestCases, "tests", "t", []string{"test-quickstart-node-http"}, "the list of the test cases to run")
	cmd.Flags().StringVarP(&options.Flags.VersionsDir, "dir", "", "", "the git clone of the jenkins-x/jenkins-x-versions git repository. Used to default the version of jenkins-x-platform when creating clusters if no --version option is supplied")
	cmd.Flags().BoolVarP(&options.Flags.DeleteTeam, "delete-team", "", true, "Whether we should delete the Team we create for each Git Provider")
	cmd.Flags().BoolVarP(&options.Flags.DisableDeleteApp, "no-delete-app", "", false, "Disables deleting the created app after the test")
	cmd.Flags().BoolVarP(&options.Flags.DisableDeleteRepo, "no-delete-repo", "", false, "Disables deleting the created repository after the test")
	cmd.Flags().BoolVarP(&options.Flags.UseCurrentTeam, "use-current-team", "", false, "If enabled lets use the current Team to run the tests")
	cmd.Flags().BoolVarP(&options.Flags.IgnoreTestFailure, "ignore-fail", "i", false, "Ignores test failures so that a BDD test run can capture the output and report on the test passes/failures")
	cmd.Flags().BoolVarP(&options.Flags.IgnoreTestFailure, "parallel", "", false, "Should we process each cluster configuration in parallel")
	cmd.Flags().BoolVarP(&options.Flags.UseRevision, "use-revision", "", true, "Use the git revision from the current git clone instead of the Pull Request branch")
	cmd.Flags().BoolVarP(&options.Flags.VersionsRepoPr, "version-repo-pr", "", false, "For use with jenkins-x-versions PR. Indicates the git revision of the PR should be used to clone the jenkins-x-versions")
	cmd.Flags().StringVarP(&options.Flags.Dir, "source-dir", "", ".", "the directory to run from where we look the requirements file")

	cmd.Flags().StringVarP(&installOptions.Flags.Provider, "provider", "", "", "Cloud service providing the Kubernetes cluster.  Supported providers: "+cloud.KubernetesProviderOptions())

	return cmd
}

func (o *StepBDDOptions) Run() error {
	flags := &o.Flags

	var err error
	if o.Flags.GoPath == "" {
		o.Flags.GoPath = os.Getenv("GOPATH")
		if o.Flags.GoPath == "" {
			o.Flags.GoPath, err = os.Getwd()
			if err != nil {
				return err
			}
		}
	}

	if o.InstallOptions.Flags.VersionsRepository == "" {
		o.InstallOptions.Flags.VersionsRepository = config.DefaultVersionsURL
	}

	gitProviderUrl := o.gitProviderUrl()
	if gitProviderUrl == "" {
		return util.MissingOption("git-provider-url")
	}

	fileName := flags.ConfigFile
	if fileName == "" {
		return o.runOnCurrentCluster()
	}

	config, err := LoadBddClusters(fileName)
	if err != nil {
		return err
	}
	if len(config.Clusters) == 0 {
		return fmt.Errorf("no clusters specified in configuration file %s", fileName)
	}

	// TODO handle parallel...
	var errors []error
	for _, cluster := range config.Clusters {
		err := o.createCluster(cluster)
		if err != nil {
			return err
		}

		err = o.runTests(o.Flags.GoPath)
		if err != nil {
			log.Logger().Warnf("Failed to perform tests on cluster %s: %s", cluster.Name, err)
			errors = append(errors, err)
		} else {
			err = o.deleteCluster(cluster)
			if err != nil {
				log.Logger().Warnf("Failed to delete cluster %s: %s", cluster.Name, err)
				errors = append(errors, err)
			}
		}
	}
	return util.CombineErrors(errors...)
}

// runOnCurrentCluster runs the tests on the current cluster
func (o *StepBDDOptions) runOnCurrentCluster() error {
	var err error

	gitProviderName := o.Flags.GitProvider
	if gitProviderName != "" && !o.Flags.UseCurrentTeam {
		gitUser := o.InstallOptions.GitRepositoryOptions.Username
		if gitUser == "" {
			return util.MissingOption("git-username")
		}
		gitToken := o.InstallOptions.GitRepositoryOptions.ApiToken
		if gitToken == "" {
			return util.MissingOption("git-api-token")
		}

		defaultAdminPassword := o.InstallOptions.AdminSecretsService.Flags.DefaultAdminPassword
		if defaultAdminPassword == "" {
			return util.MissingOption(optionDefaultAdminPassword)
		}
		defaultOptions := o.createDefaultCommonOptions()

		gitProviderUrl := o.gitProviderUrl()

		teamPrefix := "bdd-"
		if o.InstallOptions.Flags.Tekton {
			teamPrefix += "tekton-"
		}
		team := naming.ToValidName(teamPrefix + gitProviderName + "-" + o.teamNameSuffix())
		log.Logger().Infof("Creating team %s", util.ColorInfo(team))

		installOptions := o.InstallOptions
		installOptions.CommonOptions = defaultOptions
		installOptions.InitOptions.CommonOptions = defaultOptions
		installOptions.SkipAuthSecretsMerge = true
		installOptions.BatchMode = true

		installOptions.InitOptions.Flags.NoTiller = true
		installOptions.InitOptions.Flags.HelmClient = true
		installOptions.InitOptions.Flags.SkipTiller = true
		installOptions.Flags.Namespace = team
		installOptions.Flags.NoDefaultEnvironments = true
		installOptions.Flags.DefaultEnvironmentPrefix = team
		installOptions.AdminSecretsService.Flags.DefaultAdminPassword = defaultAdminPassword

		err = installOptions.Run()
		if err != nil {
			return errors.Wrapf(err, "Failed to install team %s", team)
		}

		defer o.deleteTeam(team)

		defaultOptions.SetDevNamespace(team)

		// now lets setup the git server
		createGitServer := &create.CreateGitServerOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: defaultOptions,
			},
			Kind: gitProviderName,
			Name: gitProviderName,
			URL:  gitProviderUrl,
		}
		err = o.Retry(10, time.Second*10, func() error {
			err = createGitServer.Run()
			if err != nil {
				return errors.Wrapf(err, "Failed to create git server with kind %s at url %s in team %s", gitProviderName, gitProviderUrl, team)
			}
			return nil
		})
		if err != nil {
			return err
		}

		createGitToken := &create.CreateGitTokenOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: defaultOptions,
			},
			ServerFlags: opts.ServerFlags{
				ServerURL: gitProviderUrl,
			},
			Username: gitUser,
			ApiToken: gitToken,
		}
		err = createGitToken.Run()
		if err != nil {
			return errors.Wrapf(err, "Failed to create git user token for user %s at url %s in team %s", gitProviderName, gitProviderUrl, team)
		}

		// now lets create an environment...
		createEnv := &create.CreateEnvOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: defaultOptions,
			},
			HelmValuesConfig: config.HelmValuesConfig{
				ExposeController: &config.ExposeController{},
			},
			Options: v1.Environment{
				ObjectMeta: metav1.ObjectMeta{},
				Spec: v1.EnvironmentSpec{
					PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
					Order:             100,
				},
			},
			PromotionStrategy:      string(v1.PromotionStrategyTypeAutomatic),
			ForkEnvironmentGitRepo: kube.DefaultEnvironmentGitRepoURL,
			Prefix:                 team,
		}

		createEnv.BatchMode = true
		createEnv.Options.Name = "staging"
		createEnv.Options.Spec.Label = "Staging"
		createEnv.GitRepositoryOptions.ServerURL = gitProviderUrl
		gitOwner := o.Flags.GitOwner
		if gitOwner == "" && gitUser != "" {
			// lets avoid loading the git owner from the current cluster
			gitOwner = gitUser
		}
		if gitOwner != "" {
			createEnv.GitRepositoryOptions.Owner = gitOwner
		}
		if gitUser != "" {
			createEnv.GitRepositoryOptions.Username = gitUser
		}
		createEnv.GitRepositoryOptions.Public = o.InstallOptions.Public
		log.Logger().Infof("using environment git owner: %s", util.ColorInfo(gitOwner))
		log.Logger().Infof("using environment git user: %s", util.ColorInfo(gitUser))

		err = createEnv.Run()
		if err != nil {
			return err
		}
	} else {
		log.Logger().Infof("Using the default git provider for the tests")

	}
	return o.runTests(o.Flags.GoPath)
}

func (o *StepBDDOptions) deleteTeam(team string) error {
	if !o.Flags.DeleteTeam {
		log.Logger().Infof("Disabling the deletion of team: %s", util.ColorInfo(team))
		return nil
	}

	log.Logger().Infof("Deleting team %s", util.ColorInfo(team))
	deleteTeam := &deletecmd.DeleteTeamOptions{
		CommonOptions: o.createDefaultCommonOptions(),
		Confirm:       true,
	}
	deleteTeam.Args = []string{team}
	err := deleteTeam.Run()
	if err != nil {
		return errors.Wrapf(err, "Failed to delete team %s", team)
	}
	return nil

}

func (o *StepBDDOptions) createDefaultCommonOptions() *opts.CommonOptions {
	defaultOptions := o.CommonOptions
	defaultOptions.BatchMode = true
	defaultOptions.Args = nil
	return defaultOptions
}

func (o *StepBDDOptions) gitProviderUrl() string {
	return o.InstallOptions.GitRepositoryOptions.ServerURL
}

// teamNameSuffix returns a team name suffix using the current branch +
func (o *StepBDDOptions) teamNameSuffix() string {
	repo := os.Getenv("REPO_NAME")
	branch := os.Getenv(util.EnvVarBranchName)
	buildNumber := builds.GetBuildNumber()
	if buildNumber == "" {
		buildNumber = "1"
	}
	return strings.Join([]string{repo, branch, buildNumber}, "-")
}

func (o *StepBDDOptions) runTests(gopath string) error {
	// clear the CHART_REPOSITORY env repo to avoid passing in a bogus chart repo to manual `jx promote` commands
	os.Unsetenv("CHART_REPOSITORY")

	gitURL := o.Flags.TestRepoGitCloneUrl
	gitRepository, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse git url %s", gitURL)
	}

	testDir := filepath.Join(gopath, gitRepository.Organisation, gitRepository.Name)
	if !o.Flags.SkipRepoGitClone {

		log.Logger().Infof("cloning BDD test repository to: %s", util.ColorInfo(testDir))

		err = os.MkdirAll(testDir, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "Failed to create dir %s", testDir)
		}

		log.Logger().Infof("Cloning git repository %s to dir %s", util.ColorInfo(gitURL), util.ColorInfo(testDir))
		err = o.Git().CloneOrPull(gitURL, testDir)
		if err != nil {
			return errors.Wrapf(err, "Failed to clone repo %s to %s", gitURL, testDir)
		}

		branchName := o.Flags.TestGitBranch
		pullRequestNumber := o.Flags.TestGitPrNumber
		log.Logger().Infof("Checking out repository branch %s to dir %s", util.ColorInfo(branchName), util.ColorInfo(testDir))
		if pullRequestNumber != "" {
			err = o.Git().FetchBranch(testDir, "origin", fmt.Sprintf("pull/%s/head:%s", pullRequestNumber, branchName))
			if err != nil {
				return errors.Wrapf(err, "failed to fetch Pull request number %s", pullRequestNumber)
			}
		} else {
			err = o.Git().FetchBranch(testDir, "origin", branchName)
			if err != nil {
				return errors.Wrapf(err, "failed to fetch branch %s", branchName)
			}
		}

		err = o.Git().CheckoutRemoteBranch(testDir, branchName)
		if err != nil {
			return errors.Wrapf(err, "failed to checkout branch %s", branchName)
		}
	}

	env := map[string]string{
		"GIT_PROVIDER_URL": o.gitProviderUrl(),
	}
	gitOwner := o.Flags.GitOwner
	if gitOwner != "" {
		env["GIT_ORGANISATION"] = gitOwner
	}
	if o.Flags.DisableDeleteApp {
		env["JX_DISABLE_DELETE_APP"] = "true"
	}
	if o.Flags.DisableDeleteRepo {
		env["JX_DISABLE_DELETE_REPO"] = "true"
	}
	awsAccessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	if awsAccessKey != "" {
		env["AWS_ACCESS_KEY_ID"] = awsAccessKey
	}
	awsSecret := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecret != "" {
		env["AWS_SECRET_ACCESS_KEY"] = awsSecret
	}
	awsRegion := os.Getenv("AWS_REGION")
	if awsRegion != "" {
		env["AWS_REGION"] = awsRegion
	}

	c := &util.Command{
		Dir:  testDir,
		Name: "make",
		Args: o.Flags.TestCases,
		Env:  env,
		Out:  os.Stdout,
		Err:  os.Stdout,
	}
	_, err = c.RunWithoutRetry()

	err = o.reportStatus(testDir, err)

	o.copyReports(testDir, err)

	if o.Flags.IgnoreTestFailure && err != nil {
		log.Logger().Infof("Ignoring test failure %s", err)
		return nil
	}
	return err
}

// reportStatus runs a bunch of commands to report on the status of the cluster
func (o *StepBDDOptions) reportStatus(testDir string, err error) error {
	errs := []error{}
	if err != nil {
		errs = append(errs, err)
	}

	commands := []util.Command{
		{
			Name: "kubectl",
			Args: []string{"get", "pods"},
		},
		{
			Name: "kubectl",
			Args: []string{"get", "env", "dev", "-oyaml"},
		},
		{
			Name: "jx",
			Args: []string{"status", "-b"},
		},
		{
			Name: "jx",
			Args: []string{"version", "-b"},
		},
		{
			Name: "jx",
			Args: []string{"get", "env", "-b"},
		},
		{
			Name: "jx",
			Args: []string{"get", "activities", "-b"},
		},
		{
			Name: "jx",
			Args: []string{"get", "application", "-b"},
		},
		{
			Name: "jx",
			Args: []string{"get", "preview", "-b"},
		},
		{
			Name: "jx",
			Args: []string{"open"},
		},
	}

	for _, cmd := range commands {
		fmt.Println("")
		fmt.Printf("Running %s\n\n", cmd.String())
		cmd.Dir = testDir
		cmd.Out = os.Stdout
		cmd.Err = os.Stdout

		_, err = cmd.RunWithoutRetry()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return util.CombineErrors(errs...)
}

func (o *StepBDDOptions) copyReports(testDir string, err error) error {
	reportsDir := filepath.Join(testDir, "reports")
	if _, err := os.Stat(reportsDir); os.IsNotExist(err) {
		return nil
	}
	reportsOutputDir := o.Flags.ReportsOutputDir
	if reportsOutputDir == "" {
		reportsOutputDir = "reports"
	}
	err = os.MkdirAll(reportsOutputDir, util.DefaultWritePermissions)
	if err != nil {
		log.Logger().Warnf("failed to make reports output dir: %s : %s", reportsOutputDir, err)
		return err
	}
	err = util.CopyDir(reportsDir, reportsOutputDir, true)
	if err != nil {
		log.Logger().Warnf("failed to copy reports dir: %s directory to: %s : %s", reportsDir, reportsOutputDir, err)
	}
	return err
}

func (o *StepBDDOptions) createCluster(cluster *CreateCluster) error {
	log.Logger().Infof("has %d post create cluster commands\n", len(cluster.Commands))
	for _, cmd := range cluster.Commands {
		log.Logger().Infof("post create command: %s\n", util.ColorInfo(cmd.Command+" "+strings.Join(cmd.Args, " ")))
	}

	buildNum := builds.GetBuildNumber()
	if buildNum == "" {
		log.Logger().Warnf("No build number could be found from the environment variable $BUILD_NUMBER!")
	}
	baseClusterName := naming.ToValidName(cluster.Name)
	revision := os.Getenv("PULL_PULL_SHA")
	branch := o.GetBranchName(o.Flags.VersionsDir)
	if branch == "" {
		branch = "x"
	}
	log.Logger().Infof("found git revision %s: branch %s", revision, branch)

	if o.Flags.VersionsRepoPr && o.InstallOptions.Flags.VersionsGitRef == "" {
		if revision != "" && (branch == "" || o.Flags.UseRevision) {
			o.InstallOptions.Flags.VersionsGitRef = revision
		} else {
			o.InstallOptions.Flags.VersionsGitRef = branch
		}
	} else {
		o.InstallOptions.Flags.VersionsGitRef = "master"
	}

	log.Logger().Infof("using versions git repo %s and ref %s", o.InstallOptions.Flags.VersionsRepository, o.InstallOptions.Flags.VersionsGitRef)

	cluster.Name = naming.ToValidName(branch + "-" + buildNum + "-" + cluster.Name)
	log.Logger().Infof("\nCreating cluster %s", util.ColorInfo(cluster.Name))
	binary := o.Flags.JxBinary
	args := cluster.Args

	requirements, err := o.getTestSpecificRequirementsIfExists(cluster)
	if err != nil {
		return errors.Wrapf(err, "unable to generate test specific jx-requirements.yaml")
	}
	bootType := "using"
	if requirements == nil {
		bootType = "not using"
	}
	log.Logger().Infof("cluster creation is %s jx requirements", util.ColorInfo(bootType))

	if cluster.Terraform {
		// use the cluster name as the organisation name
		args = append(args, "--organisation-name", cluster.Name)
		args = append(args, "--cluster-name", "dev")
	} else {
		args = append(args, "--cluster-name", cluster.Name)
	}

	if cluster.Terraform {
		// use the cluster name as the organisation name
		args = append(args, "--organisation-name", cluster.Name)
	}

	if util.StringArrayIndex(args, "-b") < 0 && util.StringArrayIndex(args, "--batch-mode") < 0 {
		args = append(args, "--batch-mode")
	}

	if util.StringArrayIndex(args, "--version") < 0 && util.StringArrayHasPrefixIndex(args, "--version=") < 0 {
		version, err := o.getVersion()
		if err != nil {
			return err
		}
		if version != "" {
			args = append(args, "--version", version)
		}
	}

	log.Logger().Info("Adding labels")
	if !cluster.NoLabels {
		cluster.Labels = create.AddLabel(cluster.Labels, "cluster", baseClusterName)
		cluster.Labels = create.AddLabel(cluster.Labels, "branch", branch)
		provider, err := o.getProvider(requirements, cluster)
		if err != nil {
			log.Logger().Warn(err, "error determining running provider, falling back to default provider GKE - %s", err.Error())
			provider = cloud.GKE
		}
		if provider == cloud.GKE {
			args = append(args, "--labels", cluster.Labels)
		} else if provider == cloud.EKS {
			args = append(args, "--tags", cluster.Labels)
		}
	}

	if o.Flags.BaseDomain != "" {
		args = append(args, "--domain", cluster.Name+"."+o.Flags.BaseDomain)
	}

	gitProviderURL := o.gitProviderUrl()
	if gitProviderURL != "" {
		args = append(args, "--git-provider-url", gitProviderURL)
	}

	if o.InstallOptions.Flags.VersionsRepository != "" {
		args = append(args, "--versions-repo", o.InstallOptions.Flags.VersionsRepository)
	}
	if o.InstallOptions.Flags.VersionsGitRef != "" {
		args = append(args, "--versions-ref", o.InstallOptions.Flags.VersionsGitRef)
	}
	gitUsername := o.InstallOptions.GitRepositoryOptions.Username
	if gitUsername != "" {
		args = append(args, "--git-username", gitUsername)
	}
	gitOwner := o.Flags.GitOwner
	if gitOwner != "" {
		args = append(args, "--environment-git-owner", gitOwner)
	}
	gitKind := o.InstallOptions.GitRepositoryOptions.ServerKind
	if gitKind != "" {
		args = append(args, "--git-provider-kind ", gitKind)
	}
	args = append(args, fmt.Sprintf("--git-public=%s", strconv.FormatBool(o.InstallOptions.GitRepositoryOptions.Public)))

	if o.CommonOptions.InstallDependencies {
		args = append(args, "--install-dependencies")
	}

	// expand any environment variables
	for i, arg := range args {
		args[i] = os.ExpandEnv(arg)
	}

	safeArgs := append([]string{}, args...)

	gitToken := o.InstallOptions.GitRepositoryOptions.ApiToken
	if gitToken != "" {
		args = append(args, "--git-api-token", gitToken)
		safeArgs = append(safeArgs, "--git-api-token", "**************¬")
	}
	adminPwd := o.InstallOptions.AdminSecretsService.Flags.DefaultAdminPassword
	if adminPwd != "" {
		args = append(args, "--default-admin-password", adminPwd)
		safeArgs = append(safeArgs, "--default-admin-password", "**************¬")
	}

	log.Logger().Infof("running command: %s", util.ColorInfo(fmt.Sprintf("%s %s", binary, strings.Join(safeArgs, " "))))

	// lets not log any sensitive command line arguments
	e := exec.Command(binary, args...)
	e.Stdout = o.Out
	e.Stderr = o.Err
	os.Setenv("PATH", util.PathWithBinary())

	// work around for helm apply with GitOps using a k8s local Service URL
	os.Setenv("CHART_REPOSITORY", kube.DefaultChartMuseumURL)
	os.Setenv(boot.OverrideTLSWarningEnvVarName, "true")

	err = e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", binary, strings.Join(safeArgs, " "))
	}
	if err != nil {
		return err
	}

	for _, c := range cluster.Commands {
		e := exec.Command(c.Command, c.Args...) // #nosec
		e.Stdout = o.Out
		e.Stderr = o.Err
		os.Setenv("PATH", util.PathWithBinary())

		// work around for helm apply with GitOps using a k8s local Service URL
		os.Setenv("CHART_REPOSITORY", kube.DefaultChartMuseumURL)

		log.Logger().Infof("running command: %s", util.ColorInfo(fmt.Sprintf("%s %s", c.Command, strings.Join(c.Args, " "))))

		err := e.Run()
		if err != nil {
			log.Logger().Errorf("Error: Command failed  %s %s", c.Command, strings.Join(c.Args, " "))
		}
		if err != nil {
			return err
		}
	}
	return err
}

func (o *StepBDDOptions) getTestSpecificRequirementsIfExists(cluster *CreateCluster) (*config.RequirementsConfig, error) {
	requirements, requirementsFile, _ := config.LoadRequirementsConfig(o.Flags.Dir)
	if requirements == nil {
		return nil, nil
	}

	// lets modify the version stream to use the PR
	if o.Flags.VersionsRepoPr {
		requirements.VersionStream.Ref = o.InstallOptions.Flags.VersionsGitRef
		log.Logger().Infof("setting the jx-requirements.yml version stream ref to: %s\n", util.ColorInfo(requirements.VersionStream.Ref))
	}
	requirements.VersionStream.URL = o.InstallOptions.Flags.VersionsRepository

	if cluster.Name != requirements.Cluster.ClusterName {
		requirements.Cluster.ClusterName = cluster.Name

		// lets ensure that there's git repositories setup
		o.ensureTestEnvironmentRepoSetup(requirements, "staging")
		o.ensureTestEnvironmentRepoSetup(requirements, "production")

		err := requirements.SaveConfig(requirementsFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to save file %s after setting the cluster name to %s", requirementsFile, cluster.Name)
		}
		log.Logger().Infof("wrote file %s after setting the cluster name to %s\n", requirementsFile, cluster.Name)

		data, err := ioutil.ReadFile(requirementsFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load file %s", requirementsFile)
		}
		log.Logger().Infof("%s is:\n", requirementsFile)
		log.Logger().Infof("%s\n", util.ColorStatus(string(data)))
		log.Logger().Info("\n")
	}
	return requirements, nil
}

func (o *StepBDDOptions) ensureTestEnvironmentRepoSetup(requirements *config.RequirementsConfig, envName string) {
	idx := -1
	for i, env := range requirements.Environments {
		if env.Key == envName {
			idx = i
			break
		}
	}
	if idx < 0 {
		idx = len(requirements.Environments)
		requirements.Environments = append(requirements.Environments, config.EnvironmentConfig{})
	}
	repo := requirements.Environments[idx]
	repo.Key = envName
	if repo.Owner == "" {
		repo.Owner = o.Flags.GitOwner
	}
	if repo.Owner == "" {
		repo.Owner = o.InstallOptions.GitRepositoryOptions.Username
	}
	if repo.Repository == "" {
		repo.Repository = naming.ToValidName("environment-" + requirements.Cluster.ClusterName + "-" + envName)
	}
	if repo.GitKind == "" {
		repo.GitKind = o.InstallOptions.GitRepositoryOptions.ServerKind
		if repo.GitKind == "" {
			repo.GitKind = gits.KindGitHub
		}
	}
	if repo.GitServer == "" {
		repo.GitServer = o.InstallOptions.GitRepositoryOptions.ServerKind
		if repo.GitServer == "" {
			repo.GitServer = gits.GitHubURL
		}
	}
	requirements.Environments[idx] = repo
}

func (o *StepBDDOptions) deleteCluster(cluster *CreateCluster) error {
	projectID := ""
	region := ""
	for _, arg := range cluster.Args {
		if strings.Contains(arg, "project-id=") {
			projectID = strings.Split(arg, "=")[1]
		}
		if strings.Contains(arg, "z=") || strings.Contains(arg, "zone=") || strings.Contains(arg, "region=") {
			region = strings.Split(arg, "=")[1]
		}
	}
	if projectID != "" {
		labelOptions := e2e.StepE2ELabelOptions{
			ProjectID: projectID,
			Delete:    true,
			Region:    region,
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{},
			},
		}
		if cluster.Terraform {
			labelOptions.Args = []string{fmt.Sprintf("%s-dev", cluster.Name)}
		} else {
			labelOptions.Args = []string{cluster.Name}
		}

		return labelOptions.Run()
	}
	log.Logger().Warningf("Automated cluster cleanup is not supported for cluster %s", cluster.Name)
	return nil
}

// getVersion returns the jenkins-x-platform version to use for the cluster or empty string if no specific version can be found
func (o *StepBDDOptions) getVersion() (string, error) {
	version := o.InstallOptions.Flags.Version
	if version != "" {
		return version, nil
	}

	// lets try detect a local `Makefile` to find the version
	dir := o.Flags.VersionsDir
	version, err := create.LoadVersionFromCloudEnvironmentsDir(dir, configio.NewFileStore())
	if err != nil {
		return version, errors.Wrapf(err, "failed to load jenkins-x-platform version from dir %s", dir)
	}
	log.Logger().Infof("loaded version %s from Makefile in directory %s\n", util.ColorInfo(version), util.ColorInfo(dir))
	return version, nil
}

func (o *StepBDDOptions) getProvider(requirements *config.RequirementsConfig, cluster *CreateCluster) (string, error) {
	if requirements != nil && requirements.Cluster.Provider != "" {
		log.Logger().Infof("Determined that provider is %s from requirements file", requirements.Cluster.Provider)
		return requirements.Cluster.Provider, nil
	}
	if cluster != nil && len(cluster.Args) > 2 {
		log.Logger().Infof("Determined that provider is %s from CreateCluster config", cluster.Args[2])
		return cluster.Args[2], nil
	}
	return "", fmt.Errorf("could not get provider from neither requirements nor cluster configuration yaml")
}
