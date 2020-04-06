package importcmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	"github.com/jenkins-x/jx/pkg/cmd/step/create/pr"
	"github.com/jenkins-x/jx/pkg/maven"

	"github.com/cenkalti/backoff"
	"github.com/denormal/go-gitignore"
	gojenkins "github.com/jenkins-x/golang-jenkins"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cmd/edit"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/start"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/github"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
	gitcfg "gopkg.in/src-d/go-git.v4/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// CallbackFn callback function
type CallbackFn func() error

// ImportOptions options struct for jx import
type ImportOptions struct {
	*opts.CommonOptions

	RepoURL string

	Dir                     string
	Organisation            string
	Repository              string
	Credentials             string
	AppName                 string
	GitHub                  bool
	DryRun                  bool
	SelectAll               bool
	DisableDraft            bool
	DisableJenkinsfileCheck bool
	DisableWebhooks         bool
	SelectFilter            string
	Jenkinsfile             string
	BranchPattern           string
	GitRepositoryOptions    gits.GitRepositoryOptions
	ImportGitCommitMessage  string
	ListDraftPacks          bool
	DraftPack               string
	DockerRegistryOrg       string
	GitDetails              gits.CreateRepoData
	DeployKind              string
	DeployOptions           v1.DeployOptions
	SchedulerName           string

	DisableDotGitSearch   bool
	InitialisedGit        bool
	Jenkins               gojenkins.JenkinsClient
	GitConfDir            string
	GitServer             *auth.AuthServer
	GitUserAuth           *auth.UserAuth
	GitProvider           gits.GitProvider
	PostDraftPackCallback CallbackFn
	DisableMaven          bool
	PipelineUserName      string
	PipelineServer        string
	ImportMode            string
	UseDefaultGit         bool
	GithubAppInstalled    bool

	reporter ImportReporter
}

var (
	importLong = templates.LongDesc(`
		Imports a local folder or Git repository into Jenkins X.

		If you specify no other options or arguments then the current directory is imported.
	    Or you can use '--dir' to specify a directory to import.

	    You can specify the git URL as an argument.
	    
		For more documentation see: [https://jenkins-x.io/docs/using-jx/creating/import/](https://jenkins-x.io/docs/using-jx/creating/import/)
	    
` + helper.SeeAlsoText("jx create project"))

	importExample = templates.Examples(`
		# Import the current folder
		jx import

		# Import a different folder
		jx import /foo/bar

		# Import a Git repository from a URL
		jx import --url https://github.com/jenkins-x/spring-boot-web-example.git

        # Select a number of repositories from a GitHub organisation
		jx import --github --org myname 

        # Import all repositories from a GitHub organisation selecting ones to not import
		jx import --github --org myname --all 

        # Import all repositories from a GitHub organisation which contain the text foo
		jx import --github --org myname --all --filter foo 
		`)

	deployKinds = []string{opts.DeployKindKnative, opts.DeployKindDefault}

	removeSourceRepositoryAnnotations = []string{"kubectl.kubernetes.io/last-applied-configuration", "jenkins.io/chart"}
)

// NewCmdImport the cobra command for jx import
func NewCmdImport(commonOpts *opts.CommonOptions) *cobra.Command {
	cmd, _ := NewCmdImportAndOptions(commonOpts)
	return cmd
}

// NewCmdImportAndOptions creates the cobra command for jx import and the options
func NewCmdImportAndOptions(commonOpts *opts.CommonOptions) (*cobra.Command, *ImportOptions) {
	options := &ImportOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "import",
		Short:   "Imports a local project or Git repository into Jenkins",
		Long:    importLong,
		Example: importExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.RepoURL, "url", "u", "", "The git clone URL to clone into the current directory and then import")
	cmd.Flags().BoolVarP(&options.GitHub, "github", "", false, "If you wish to pick the repositories from GitHub to import")
	cmd.Flags().BoolVarP(&options.SelectAll, "all", "", false, "If selecting projects to import from a Git provider this defaults to selecting them all")
	cmd.Flags().StringVarP(&options.SelectFilter, "filter", "", "", "If selecting projects to import from a Git provider this filters the list of repositories")
	options.AddImportFlags(cmd, false)
	options.Cmd = cmd
	return cmd, options
}

func (options *ImportOptions) AddImportFlags(cmd *cobra.Command, createProject bool) {
	notCreateProject := func(text string) string {
		if createProject {
			return ""
		}
		return text
	}
	cmd.Flags().StringVarP(&options.Organisation, "org", "", "", "Specify the Git provider organisation to import the project into (if it is not already in one)")
	cmd.Flags().StringVarP(&options.Repository, "name", "", notCreateProject("n"), "Specify the Git repository name to import the project into (if it is not already in one)")
	cmd.Flags().StringVarP(&options.Credentials, "credentials", notCreateProject("c"), "", "The Jenkins credentials name used by the job")
	cmd.Flags().StringVarP(&options.Jenkinsfile, "jenkinsfile", notCreateProject("j"), "", "The name of the Jenkinsfile to use. If not specified then 'Jenkinsfile' will be used")
	cmd.Flags().BoolVarP(&options.DryRun, "dry-run", "", false, "Performs local changes to the repo but skips the import into Jenkins X")
	cmd.Flags().BoolVarP(&options.DisableDraft, "no-draft", "", false, "Disable Draft from trying to default a Dockerfile and Helm Chart")
	cmd.Flags().BoolVarP(&options.DisableJenkinsfileCheck, "no-jenkinsfile", "", false, "Disable defaulting a Jenkinsfile if its missing")
	cmd.Flags().StringVarP(&options.ImportGitCommitMessage, "import-commit-message", "", "", "Specifies the initial commit message used when importing the project")
	cmd.Flags().StringVarP(&options.BranchPattern, "branches", "", "", "The branch pattern for branches to trigger CI/CD pipelines on")
	cmd.Flags().BoolVarP(&options.ListDraftPacks, "list-packs", "", false, "list available draft packs")
	cmd.Flags().StringVarP(&options.DraftPack, "pack", "", "", "The name of the pack to use")
	cmd.Flags().StringVarP(&options.SchedulerName, "scheduler", "", "", "The name of the Scheduler configuration to use for ChatOps when using Prow")
	cmd.Flags().StringVarP(&options.DockerRegistryOrg, "docker-registry-org", "", "", "The name of the docker registry organisation to use. If not specified then the Git provider organisation will be used")
	cmd.Flags().StringVarP(&options.ExternalJenkinsBaseURL, "external-jenkins-url", "", "", "The jenkins url that an external git provider needs to use")
	cmd.Flags().BoolVarP(&options.DisableMaven, "disable-updatebot", "", false, "disable updatebot-maven-plugin from attempting to fix/update the maven pom.xml")
	cmd.Flags().StringVarP(&options.ImportMode, "import-mode", "m", "", fmt.Sprintf("The import mode to use. Should be one of %s", strings.Join(v1.ImportModeStrings, ", ")))
	cmd.Flags().BoolVarP(&options.UseDefaultGit, "use-default-git", "", false, "use default git account")
	cmd.Flags().StringVarP(&options.DeployKind, "deploy-kind", "", "", fmt.Sprintf("The kind of deployment to use for the project. Should be one of %s", strings.Join(deployKinds, ", ")))
	cmd.Flags().BoolVarP(&options.DeployOptions.Canary, opts.OptionCanary, "", false, "should we use canary rollouts (progressive delivery) by default for this application. e.g. using a Canary deployment via flagger. Requires the installation of flagger and istio/gloo in your cluster")
	cmd.Flags().BoolVarP(&options.DeployOptions.HPA, opts.OptionHPA, "", false, "should we enable the Horizontal Pod Autoscaler for this application.")

	opts.AddGitRepoOptionsArguments(cmd, &options.GitRepositoryOptions)
}

// Run executes the command
func (options *ImportOptions) Run() error {
	if options.ListDraftPacks {
		packs, err := options.allDraftPacks()
		if err != nil {
			log.Logger().Error(err.Error())
			return err
		}
		log.Logger().Info("Available draft packs:")
		for i := 0; i < len(packs); i++ {
			log.Logger().Infof(packs[i])
		}
		return nil
	}

	options.SetBatchMode(options.BatchMode)

	var err error
	isProw := false
	jxClient, ns, err := options.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	if !options.DryRun {
		_, err = options.KubeClient()
		if err != nil {
			return err
		}

		isProw, err = options.IsProw()
		if err != nil {
			return err
		}

		if !isProw {
			options.Jenkins, err = options.JenkinsClient()
			if err != nil {
				return err
			}
		}
	}
	err = options.DefaultsFromTeamSettings()
	if err != nil {
		return err
	}

	var userAuth *auth.UserAuth
	if options.GitProvider == nil {
		authConfigSvc, err := options.GitLocalAuthConfigService()
		if err != nil {
			return err
		}
		config := authConfigSvc.Config()
		var server *auth.AuthServer
		if options.RepoURL != "" {
			gitInfo, err := gits.ParseGitURL(options.RepoURL)
			if err != nil {
				return err
			}
			serverURL := gitInfo.HostURLWithoutUser()
			server = config.GetOrCreateServer(serverURL)
		} else {
			server, err = config.PickOrCreateServer(gits.GitHubURL, options.GitRepositoryOptions.ServerURL, "Which Git service do you wish to use", options.BatchMode, options.GetIOFileHandles())
			if err != nil {
				return err
			}
		}

		if options.UseDefaultGit {
			userAuth = config.CurrentUser(server, options.CommonOptions.InCluster())
		} else if options.GitRepositoryOptions.Username != "" {
			userAuth = config.GetOrCreateUserAuth(server.URL, options.GitRepositoryOptions.Username)
			options.GetReporter().UsingGitUserName(options.GitRepositoryOptions.Username)
		} else {
			// Get the org in case there is more than one user auth on the server and batchMode is true
			org := options.getOrganisationOrCurrentUser()
			userAuth, err = config.PickServerUserAuth(server, "Git user name:", options.BatchMode, org, options.GetIOFileHandles())
			if err != nil {
				return err
			}
		}
		if server.Kind == "" {
			server.Kind, err = options.GitServerHostURLKind(server.URL)
			if err != nil {
				return err
			}
		}
		if userAuth.IsInvalid() {
			f := func(username string) error {
				options.Git().PrintCreateRepositoryGenerateAccessToken(server, username, options.Out)
				return nil
			}
			if options.GitRepositoryOptions.ApiToken != "" {
				userAuth.ApiToken = options.GitRepositoryOptions.ApiToken
			}
			err = config.EditUserAuth(server.Label(), userAuth, userAuth.Username, false, options.BatchMode, f, options.GetIOFileHandles())
			if err != nil {
				return err
			}

			// TODO lets verify the auth works?
			if userAuth.IsInvalid() {
				return fmt.Errorf("Authentication has failed for user %v. Please check the user's access credentials and try again", userAuth.Username)
			}
		}
		err = authConfigSvc.SaveUserAuth(server.URL, userAuth)
		if err != nil {
			return fmt.Errorf("Failed to store git auth configuration %s", err)
		}

		options.GitServer = server
		options.GitUserAuth = userAuth
		options.GitProvider, err = gits.CreateProvider(server, userAuth, options.Git())
		if err != nil {
			return err
		}
	}

	if options.GitHub {
		return options.ImportProjectsFromGitHub()
	}

	if options.Dir == "" {
		args := options.Args
		if len(args) > 0 {
			options.Dir = args[0]
		} else {
			dir, err := os.Getwd()
			if err != nil {
				return err
			}
			options.Dir = dir
		}
	}

	checkForJenkinsfile := options.Jenkinsfile == "" && !options.DisableJenkinsfileCheck
	shouldClone := checkForJenkinsfile || !options.DisableDraft

	if options.RepoURL != "" {
		if shouldClone {
			// Use the git user auth to clone the repo (needed for private repos etc)
			if options.GitUserAuth == nil {
				userAuth := options.GitProvider.UserAuth()
				options.GitUserAuth = &userAuth
			}
			options.RepoURL, err = options.Git().CreateAuthenticatedURL(options.RepoURL, options.GitUserAuth)
			if err != nil {
				return err
			}
			err = options.CloneRepository()
			if err != nil {
				return err
			}
		}
	} else {
		err = options.DiscoverGit()
		if err != nil {
			return err
		}

		if options.RepoURL == "" {
			err = options.DiscoverRemoteGitURL()
			if err != nil {
				return err
			}
		}
	}

	if options.AppName == "" {
		if options.RepoURL != "" {
			info, err := gits.ParseGitURL(options.RepoURL)
			if err != nil {
				log.Logger().Warnf("Failed to parse git URL %s : %s", options.RepoURL, err)
			} else {
				options.Organisation = info.Organisation
				options.AppName = info.Name
			}
		}
	}
	if options.AppName == "" {
		dir, err := filepath.Abs(options.Dir)
		if err != nil {
			return err
		}
		_, options.AppName = filepath.Split(dir)
	}
	options.AppName = naming.ToValidName(strings.ToLower(options.AppName))

	if !options.DisableDraft {
		err = options.DraftCreate()
		if err != nil {
			return err
		}

	}
	err = options.fixDockerIgnoreFile()
	if err != nil {
		return err
	}

	err = options.fixMaven()
	if err != nil {
		return err
	}

	if options.RepoURL == "" {
		if !options.DryRun {
			err = options.CreateNewRemoteRepository()
			if err != nil {
				if !options.DisableDraft {
					log.Logger().Warn("Remote repository creation failed. In order to retry consider adding '--no-draft' option.")
				}
				return err
			}
		}
	} else {
		if shouldClone {
			err = options.Git().Push(options.Dir, "origin", false, "HEAD")
			if err != nil {
				return err
			}
			options.GetReporter().PushedGitRepository(options.RepoURL)
		}
	}

	if options.DryRun {
		log.Logger().Info("dry-run so skipping import to Jenkins X")
		return nil
	}

	if !isProw {
		err = options.checkChartmuseumCredentialExists()
		if err != nil {
			return err
		}
	}

	_, err = kube.GetOrCreateSourceRepository(jxClient, ns, options.AppName, options.Organisation, gits.SourceRepositoryProviderURL(options.GitProvider))
	if err != nil {
		return errors.Wrapf(err, "creating application resource for %s", util.ColorInfo(options.AppName))
	}

	if !options.GithubAppInstalled {
		githubAppMode, err := options.IsGitHubAppMode()
		if err != nil {
			return err
		}

		if githubAppMode {
			githubApp := &github.GithubApp{
				Factory: options.GetFactory(),
			}

			installed, err := githubApp.Install(options.Organisation, options.Repository, options.GetIOFileHandles(), false)
			if err != nil {
				return err
			}
			options.GithubAppInstalled = installed
		}
	}

	return options.doImport()
}

// ImportProjectsFromGitHub import projects from github
func (options *ImportOptions) ImportProjectsFromGitHub() error {
	repos, err := gits.PickRepositories(options.GitProvider, options.Organisation, "Which repositories do you want to import", options.SelectAll, options.SelectFilter, options.GetIOFileHandles())
	if err != nil {
		return err
	}

	log.Logger().Info("Selected repositories")
	for _, r := range repos {
		o2 := ImportOptions{
			CommonOptions:           options.CommonOptions,
			Dir:                     options.Dir,
			RepoURL:                 r.CloneURL,
			Organisation:            options.Organisation,
			Repository:              r.Name,
			Jenkins:                 options.Jenkins,
			GitProvider:             options.GitProvider,
			DisableJenkinsfileCheck: options.DisableJenkinsfileCheck,
			DisableDraft:            options.DisableDraft,
		}
		log.Logger().Infof("Importing repository %s", util.ColorInfo(r.Name))
		err = o2.Run()
		if err != nil {
			return err
		}
	}
	return nil
}

// GetReporter returns the reporter interface
func (options *ImportOptions) GetReporter() ImportReporter {
	if options.reporter == nil {
		options.reporter = &LogImportReporter{}
	}
	return options.reporter
}

// SetReporter overrides the reporter interface
func (options *ImportOptions) SetReporter(reporter ImportReporter) {
	options.reporter = reporter
}

// DraftCreate creates a draft
func (options *ImportOptions) DraftCreate() error {
	// TODO this is a workaround of this draft issue:
	// https://github.com/Azure/draft/issues/476
	dir := options.Dir
	var err error

	defaultJenkinsfileName := jenkinsfile.Name
	jenkinsfile := defaultJenkinsfileName
	withRename := false
	if options.Jenkinsfile != "" && options.Jenkinsfile != defaultJenkinsfileName {
		jenkinsfile = options.Jenkinsfile
		withRename = true
	}
	defaultJenkinsfile := filepath.Join(dir, defaultJenkinsfileName)
	if !filepath.IsAbs(jenkinsfile) {
		jenkinsfile = filepath.Join(dir, jenkinsfile)
	}
	args := &opts.InvokeDraftPack{
		Dir:                     dir,
		CustomDraftPack:         options.DraftPack,
		Jenkinsfile:             jenkinsfile,
		DefaultJenkinsfile:      defaultJenkinsfile,
		WithRename:              withRename,
		InitialisedGit:          options.InitialisedGit,
		DisableJenkinsfileCheck: options.DisableJenkinsfileCheck,
	}
	options.DraftPack, err = options.InvokeDraftPack(args)
	if err != nil {
		return err
	}

	// lets rename the chart to be the same as our app name
	err = options.renameChartToMatchAppName()
	if err != nil {
		return err
	}

	err = options.modifyDeployKind()
	err = options.modifyDeployKind()
	if err != nil {
		return err
	}

	if options.PostDraftPackCallback != nil {
		err = options.PostDraftPackCallback()
		if err != nil {
			return err
		}
	}

	gitServerName, err := gits.GetHost(options.GitProvider)
	if err != nil {
		return err
	}

	if options.GitUserAuth == nil {
		userAuth := options.GitProvider.UserAuth()
		options.GitUserAuth = &userAuth
	}

	options.Organisation = options.GetOrganisation()
	if options.Organisation == "" {
		gitUsername := options.GitUserAuth.Username
		options.Organisation, err = gits.GetOwner(options.BatchMode, options.GitProvider, gitUsername, options.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	if options.AppName == "" {
		dir := options.Dir
		_, defaultRepoName := filepath.Split(dir)

		options.AppName, err = gits.GetRepoName(options.BatchMode, false, options.GitProvider, defaultRepoName, options.Organisation, options.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	dockerRegistryOrg := options.getDockerRegistryOrg()
	err = options.ReplacePlaceholders(gitServerName, dockerRegistryOrg)
	if err != nil {
		return err
	}

	// Create Prow owners file
	err = options.CreateProwOwnersFile()
	if err != nil {
		return err
	}
	err = options.CreateProwOwnersAliasesFile()
	if err != nil {
		return err
	}

	err = options.Git().Add(dir, "*")
	if err != nil {
		return err
	}
	err = options.Git().CommitIfChanges(dir, "Draft create")
	if err != nil {
		return err
	}

	options.GetReporter().DraftCreated(options.DraftPack)
	return nil
}

func (options *ImportOptions) getDockerRegistryOrg() string {
	dockerRegistryOrg := options.DockerRegistryOrg
	if dockerRegistryOrg == "" {
		dockerRegistryOrg = options.getOrganisationOrCurrentUser()
	}
	return strings.ToLower(dockerRegistryOrg)
}

func (options *ImportOptions) getOrganisationOrCurrentUser() string {
	org := options.GetOrganisation()
	if org == "" {
		org = options.getCurrentUser()
	}
	return org
}

func (options *ImportOptions) getCurrentUser() string {
	//walk through every file in the given dir and update the placeholders
	var currentUser string
	if options.GitServer != nil {
		currentUser = options.GitServer.CurrentUser
		if currentUser == "" {
			if options.GitProvider != nil {
				currentUser = options.GitProvider.CurrentUsername()
			}
		}
	}
	if currentUser == "" {
		log.Logger().Warn("No username defined for the current Git server!")
		currentUser = options.GitRepositoryOptions.Username
	}
	return currentUser
}

// GetOrganisation gets the organisation from the RepoURL (if in the github format of github.com/org/repo). It will
// do this in preference to the Organisation field (if set). If the repo URL does not implicitly specify an organisation
// then the Organisation specified in the options is used.
func (options *ImportOptions) GetOrganisation() string {
	org := ""
	gitInfo, err := gits.ParseGitURL(options.RepoURL)
	if err == nil && gitInfo.Organisation != "" {
		org = gitInfo.Organisation
		if options.Organisation != "" && org != options.Organisation {
			log.Logger().Warnf("organisation %s detected from URL %s. '--org %s' will be ignored", org, options.RepoURL, options.Organisation)
		}
	} else {
		org = options.Organisation
	}
	return org
}

// CreateNewRemoteRepository creates a new remote repository
func (options *ImportOptions) CreateNewRemoteRepository() error {
	authConfigSvc, err := options.GitLocalAuthConfigService()
	if err != nil {
		return err
	}

	dir := options.Dir
	_, defaultRepoName := filepath.Split(dir)

	options.GitRepositoryOptions.Owner = options.GetOrganisation()
	details := &options.GitDetails
	if details.RepoName == "" {
		details, err = gits.PickNewGitRepository(options.BatchMode, authConfigSvc, defaultRepoName, &options.GitRepositoryOptions,
			options.GitServer, options.GitUserAuth, options.Git(), options.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	repo, err := details.CreateRepository()
	if err != nil {
		return err
	}
	options.GitProvider = details.GitProvider

	options.RepoURL = repo.CloneURL
	pushGitURL, err := options.Git().CreateAuthenticatedURL(repo.CloneURL, details.User)
	if err != nil {
		return err
	}
	err = options.Git().AddRemote(dir, "origin", pushGitURL)
	if err != nil {
		return err
	}
	err = options.Git().PushMaster(dir)
	if err != nil {
		return err
	}
	repoURL := repo.HTMLURL
	options.GetReporter().PushedGitRepository(repoURL)

	githubAppMode, err := options.IsGitHubAppMode()
	if err != nil {
		return err
	}

	if !githubAppMode {

		// If the user creating the repo is not the pipeline user, add the pipeline user as a contributor to the repo
		if options.PipelineUserName != options.GitUserAuth.Username && options.GitServer != nil && options.GitServer.URL == options.PipelineServer {
			// Make the invitation
			err := options.GitProvider.AddCollaborator(options.PipelineUserName, details.Organisation, details.RepoName)
			if err != nil {
				return err
			}

			// If repo is put in an organisation that the pipeline user is not part of an invitation needs to be accepted.
			// Create a new provider for the pipeline user
			authConfig := authConfigSvc.Config()
			if err != nil {
				return err
			}
			pipelineUserAuth := authConfig.FindUserAuth(options.GitServer.URL, options.PipelineUserName)
			if pipelineUserAuth == nil {
				log.Logger().Warnf("Pipeline Git user credentials not found. %s will need to accept the invitation to collaborate"+
					"on %s if %s is not part of %s.\n",
					options.PipelineUserName, details.RepoName, options.PipelineUserName, details.Organisation)
			} else {
				pipelineServerAuth := authConfig.GetServer(authConfig.CurrentServer)
				pipelineUserProvider, err := gits.CreateProvider(pipelineServerAuth, pipelineUserAuth, options.Git())
				if err != nil {
					return err
				}

				// Get all invitations for the pipeline user
				// Wrapped in retry to not immediately fail the quickstart creation if APIs are flaky.
				f := func() error {
					invites, _, err := pipelineUserProvider.ListInvitations()
					if err != nil {
						return err
					}
					for _, x := range invites {
						// Accept all invitations for the pipeline user
						_, err = pipelineUserProvider.AcceptInvitation(*x.ID)
						if err != nil {
							return err
						}
					}
					return nil
				}
				exponentialBackOff := backoff.NewExponentialBackOff()
				timeout := 20 * time.Second
				exponentialBackOff.MaxElapsedTime = timeout
				exponentialBackOff.Reset()
				err = backoff.Retry(f, exponentialBackOff)
				if err != nil {
					return err
				}
			}

		}
	}

	return nil
}

// CloneRepository clones a repository
func (options *ImportOptions) CloneRepository() error {
	url := options.RepoURL
	if url == "" {
		return fmt.Errorf("no Git repository URL defined")
	}
	gitInfo, err := gits.ParseGitURL(url)
	if err != nil {
		return fmt.Errorf("failed to parse Git URL %s due to: %s", url, err)
	}
	if gitInfo.Host == gits.GitHubHost && strings.HasPrefix(gitInfo.Scheme, "http") {
		if !strings.HasSuffix(url, ".git") {
			url += ".git"
		}
		options.RepoURL = url
	}
	cloneDir, err := util.CreateUniqueDirectory(options.Dir, gitInfo.Name, util.MaximumNewDirectoryAttempts)
	if err != nil {
		return errors.Wrapf(err, "failed to create unique directory for '%s'", options.Dir)
	}
	err = options.Git().Clone(url, cloneDir)
	if err != nil {
		return errors.Wrapf(err, "failed to clone in directory '%s'", cloneDir)
	}
	options.GetReporter().ClonedGitRepository(url)
	options.Dir = cloneDir
	return nil
}

// DiscoverGit checks if there is a git clone or prompts the user to import it
func (options *ImportOptions) DiscoverGit() error {
	surveyOpts := survey.WithStdio(options.In, options.Out, options.Err)
	if !options.DisableDotGitSearch {
		root, gitConf, err := options.Git().FindGitConfigDir(options.Dir)
		if err != nil {
			return err
		}
		if root != "" {
			if root != options.Dir {
				options.GetReporter().Trace("Importing from directory %s as we found a .git folder there", root)
			}
			options.Dir = root
			options.GitConfDir = gitConf
			return nil
		}
	}

	dir := options.Dir
	if dir == "" {
		return fmt.Errorf("no directory specified")
	}

	// lets prompt the user to initialise the Git repository
	if !options.BatchMode {
		options.GetReporter().Trace("The directory %s is not yet using git", util.ColorInfo(dir))
		flag := false
		prompt := &survey.Confirm{
			Message: "Would you like to initialise git now?",
			Default: true,
		}
		err := survey.AskOne(prompt, &flag, nil, surveyOpts)
		if err != nil {
			return err
		}
		if !flag {
			return fmt.Errorf("please initialise git yourself then try again")
		}
	}
	options.InitialisedGit = true
	err := options.Git().Init(dir)
	if err != nil {
		return err
	}
	options.GitConfDir = filepath.Join(dir, ".git", "config")
	err = options.DefaultGitIgnore()
	if err != nil {
		return err
	}
	options.Git().Add(dir, ".gitignore")
	err = options.Git().Add(dir, "*")
	if err != nil {
		return err
	}

	err = options.Git().Status(dir)
	if err != nil {
		return err
	}

	message := options.ImportGitCommitMessage
	if message == "" {
		if options.BatchMode {
			message = "Initial import"
		} else {
			messagePrompt := &survey.Input{
				Message: "Commit message: ",
				Default: "Initial import",
			}
			err = survey.AskOne(messagePrompt, &message, nil, surveyOpts)
			if err != nil {
				return err
			}
		}
	}
	err = options.Git().CommitIfChanges(dir, message)
	if err != nil {
		return err
	}
	options.GetReporter().GitRepositoryCreated()
	return nil
}

// DefaultGitIgnore creates a default .gitignore
func (options *ImportOptions) DefaultGitIgnore() error {
	name := filepath.Join(options.Dir, ".gitignore")
	exists, err := util.FileExists(name)
	if err != nil {
		return err
	}
	if !exists {
		data := []byte(opts.DefaultGitIgnoreFile)
		err = ioutil.WriteFile(name, data, util.DefaultWritePermissions)
		if err != nil {
			return fmt.Errorf("failed to write %s due to %s", name, err)
		}
	}
	return nil
}

// DiscoverRemoteGitURL finds the git url by looking in the directory
// and looking for a .git/config file
func (options *ImportOptions) DiscoverRemoteGitURL() error {
	gitConf := options.GitConfDir
	if gitConf == "" {
		return fmt.Errorf("no GitConfDir defined")
	}
	cfg := gitcfg.NewConfig()
	data, err := ioutil.ReadFile(gitConf)
	if err != nil {
		return fmt.Errorf("failed to load %s due to %s", gitConf, err)
	}

	err = cfg.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s due to %s", gitConf, err)
	}
	remotes := cfg.Remotes
	if len(remotes) == 0 {
		return nil
	}
	url := options.Git().GetRemoteUrl(cfg, "origin")
	if url == "" {
		url = options.Git().GetRemoteUrl(cfg, "upstream")
		if url == "" {
			url, err = options.PickGitRemoteURL(cfg)
			if err != nil {
				return err
			}
		}
	}
	if url != "" {
		options.RepoURL = url
	}
	return nil
}

func (options *ImportOptions) doImport() error {
	gitURL := options.RepoURL
	gitProvider := options.GitProvider
	if gitProvider == nil {
		p, err := options.GitProviderForURL(gitURL, "user name to register webhook")
		if err != nil {
			return err
		}
		gitProvider = p
	}

	authConfigSvc, err := options.GitLocalAuthConfigService()
	if err != nil {
		return err
	}
	defaultJenkinsfileName := jenkinsfile.Name
	jenkinsfile := options.Jenkinsfile
	if jenkinsfile == "" {
		jenkinsfile = defaultJenkinsfileName
	}

	dockerfileLocation := ""
	if options.Dir != "" {
		dockerfileLocation = filepath.Join(options.Dir, "Dockerfile")
	} else {
		dockerfileLocation = "Dockerfile"
	}
	dockerfileExists, err := util.FileExists(dockerfileLocation)
	if err != nil {
		return err
	}

	if dockerfileExists {
		err = options.ensureDockerRepositoryExists()
		if err != nil {
			return err
		}
	}

	isProw, err := options.IsProw()
	if err != nil {
		return err
	}

	githubAppMode, err := options.IsGitHubAppMode()
	if err != nil {
		return err
	}

	if isProw {
		if !options.DisableWebhooks && !githubAppMode {
			// register the webhook
			err = options.CreateWebhookProw(gitURL, gitProvider)
			if err != nil {
				return err
			}
		}
		return options.addProwConfig(gitURL, gitProvider.Kind())
	}

	return options.ImportProject(gitURL, options.Dir, jenkinsfile, options.BranchPattern, options.Credentials, false, gitProvider, authConfigSvc, false, options.BatchMode)
}

func (options *ImportOptions) addProwConfig(gitURL string, gitKind string) error {
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return err
	}
	repo := gitInfo.Organisation + "/" + gitInfo.Name
	client, err := options.KubeClient()
	if err != nil {
		return err
	}
	devEnv, settings, err := options.DevEnvAndTeamSettings()
	if err != nil {
		return err
	}
	_, currentNamespace, err := options.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	gha, err := options.IsGitHubAppMode()
	if err != nil {
		return err
	}

	if settings.IsSchedulerMode() {
		jxClient, _, err := options.JXClient()
		if err != nil {
			return err
		}
		callback := func(sr *v1.SourceRepository) {
			u := gitInfo.URLWithoutUser()
			sr.Spec.ProviderKind = gitKind
			sr.Spec.URL = u
			if sr.Spec.URL == "" {
				sr.Spec.URL = gitInfo.HttpsURL()
			}
			if sr.Spec.HTTPCloneURL == "" {
				sr.Spec.HTTPCloneURL = gitInfo.HttpCloneURL()
			}
			if sr.Spec.SSHCloneURL == "" {
				sr.Spec.SSHCloneURL = gitInfo.SSHURL
			}
		}
		sr, err := kube.GetOrCreateSourceRepositoryCallback(jxClient, currentNamespace, gitInfo.Name, gitInfo.Organisation, gitInfo.HostURLWithoutUser(), callback)
		log.Logger().Debugf("have SourceRepository: %s\n", sr.Name)

		// lets update the Scheduler if one is specified and its different to the default
		schedulerName := options.SchedulerName
		if schedulerName != "" && schedulerName != sr.Spec.Scheduler.Name {
			sr.Spec.Scheduler.Name = schedulerName
			_, err = jxClient.JenkinsV1().SourceRepositories(currentNamespace).Update(sr)
			if err != nil {
				log.Logger().Warnf("failed to update the SourceRepository %s to add the Scheduler name %s due to: %s\n", sr.Name, schedulerName, err.Error())
			}
		}

		sourceGitURL, err := kube.GetRepositoryGitURL(sr)
		if err != nil {
			return errors.Wrapf(err, "failed to get the git URL for SourceRepository %s", sr.Name)
		}

		devGitURL := devEnv.Spec.Source.URL
		if devGitURL != "" && !gha {
			// lets generate a PR
			base := devEnv.Spec.Source.Ref
			if base == "" {
				base = "master"
			}
			pro := &pr.StepCreatePrOptions{
				SrcGitURL:  sourceGitURL,
				GitURLs:    []string{devGitURL},
				Base:       base,
				Fork:       true,
				BranchName: sr.Name,
			}
			pro.CommonOptions = options.CommonOptions

			changeFn := func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
				return nil, writeSourceRepoToYaml(dir, sr)
			}

			err := pro.CreatePullRequest("resource", changeFn)
			if err != nil {
				return errors.Wrapf(err, "failed to create Pull Request on the development environment git repository %s", devGitURL)
			}
			prURL := ""
			if pro.Results != nil && pro.Results.PullRequest != nil {
				prURL = pro.Results.PullRequest.URL
			}
			options.GetReporter().CreatedDevRepoPullRequest(prURL, devGitURL)
		}

		err = options.GenerateProwConfig(currentNamespace, devEnv)
		if err != nil {
			return err
		}
	} else {
		err = prow.AddApplication(client, []string{repo}, currentNamespace, options.DraftPack, settings)
		if err != nil {
			return err
		}
	}

	if !gha {
		startBuildOptions := start.StartPipelineOptions{
			CommonOptions: options.CommonOptions,
		}
		startBuildOptions.Args = []string{fmt.Sprintf("%s/%s/%s", gitInfo.Organisation, gitInfo.Name, opts.MasterBranch)}
		err = startBuildOptions.Run()
		if err != nil {
			return fmt.Errorf("failed to start pipeline build: %s", err)
		}
	}

	options.LogImportedProject(false, gitInfo)

	return nil
}

// writeSourceRepoToYaml marshals a SourceRepository to the given directory, making sure it can be loaded by boot.
func writeSourceRepoToYaml(dir string, sr *v1.SourceRepository) error {
	outDir := filepath.Join(dir, "repositories", "templates")
	err := os.MkdirAll(outDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to make directories %s", outDir)
	}

	fileName := filepath.Join(outDir, sr.Name+"-sr.yaml")
	// lets clear the fields we don't need to save
	clearSourceRepositoryMetadata(&sr.ObjectMeta)
	// Ensure it has the type information it needs
	sr.APIVersion = jenkinsio.GroupAndVersion
	sr.Kind = "SourceRepository"

	data, err := yaml.Marshal(&sr)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal SourceRepository %s to yaml", sr.Name)
	}

	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save SourceRepository file %s", fileName)
	}
	return nil
}

// clearSourceRepositoryMetadata clears unnecessary data
func clearSourceRepositoryMetadata(meta *metav1.ObjectMeta) {
	meta.CreationTimestamp.Time = time.Time{}
	meta.Namespace = ""
	meta.OwnerReferences = nil
	meta.Finalizers = nil
	meta.Generation = 0
	meta.GenerateName = ""
	meta.SelfLink = ""
	meta.UID = ""
	meta.ResourceVersion = ""

	for _, k := range removeSourceRepositoryAnnotations {
		delete(meta.Annotations, k)
	}
}

// ensureDockerRepositoryExists for some kinds of container registry we need to pre-initialise its use such as for ECR
func (options *ImportOptions) ensureDockerRepositoryExists() error {
	orgName := options.getOrganisationOrCurrentUser()
	appName := options.AppName
	if orgName == "" {
		log.Logger().Warnf("Missing organisation name!")
		return nil
	}
	if appName == "" {
		log.Logger().Warnf("Missing application name!")
		return nil
	}
	kubeClient, curNs, err := options.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	ns, _, err := kube.GetDevNamespace(kubeClient, curNs)
	if err != nil {
		return err
	}

	region, _ := kube.ReadRegion(kubeClient, ns)
	cm, err := kubeClient.CoreV1().ConfigMaps(ns).Get(kube.ConfigMapJenkinsDockerRegistry, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Could not find ConfigMap %s in namespace %s: %s", kube.ConfigMapJenkinsDockerRegistry, ns, err)
	}
	if cm.Data != nil {
		dockerRegistry := cm.Data["docker.registry"]
		if dockerRegistry != "" {
			if strings.HasSuffix(dockerRegistry, ".amazonaws.com") && strings.Index(dockerRegistry, ".ecr.") > 0 {
				return amazon.LazyCreateRegistry(kubeClient, ns, region, dockerRegistry, options.getDockerRegistryOrg(), appName)
			}
		}
	}
	return nil
}

// ReplacePlaceholders replaces app name, git server name, git org, and docker registry org placeholders
func (options *ImportOptions) ReplacePlaceholders(gitServerName, dockerRegistryOrg string) error {
	options.Organisation = naming.ToValidName(strings.ToLower(options.Organisation))
	options.GetReporter().Trace("replacing placeholders in directory %s", options.Dir)
	options.GetReporter().Trace("app name: %s, git server: %s, org: %s, Docker registry org: %s", options.AppName, gitServerName, options.Organisation, dockerRegistryOrg)

	ignore, err := gitignore.NewRepository(options.Dir)
	if err != nil {
		return err
	}

	replacer := strings.NewReplacer(
		util.PlaceHolderAppName, strings.ToLower(options.AppName),
		util.PlaceHolderGitProvider, strings.ToLower(gitServerName),
		util.PlaceHolderOrg, strings.ToLower(options.Organisation),
		util.PlaceHolderDockerRegistryOrg, strings.ToLower(dockerRegistryOrg))

	pathsToRename := []string{} // Renaming must be done post-Walk
	if err := filepath.Walk(options.Dir, func(f string, fi os.FileInfo, err error) error {
		if skip, err := options.skipPathForReplacement(f, fi, ignore); skip {
			return err
		}
		if strings.Contains(filepath.Base(f), util.PlaceHolderPrefix) {
			// Prepend so children are renamed before their parents
			pathsToRename = append([]string{f}, pathsToRename...)
		}
		if !fi.IsDir() {
			if err := replacePlaceholdersInFile(replacer, f); err != nil {
				return err
			}
		}
		return nil

	}); err != nil {
		return fmt.Errorf("error replacing placeholders %v", err)
	}

	for _, path := range pathsToRename {
		if err := replacePlaceholdersInPathBase(replacer, path); err != nil {
			return err
		}
	}
	return nil
}

func (options *ImportOptions) skipPathForReplacement(path string, fi os.FileInfo, ignore gitignore.GitIgnore) (bool, error) {
	relPath, _ := filepath.Rel(options.Dir, path)
	match := ignore.Relative(relPath, fi.IsDir())
	matchIgnore := match != nil && match.Ignore() //Defaults to including if match == nil
	if fi.IsDir() {
		if matchIgnore || fi.Name() == ".git" {
			options.GetReporter().Trace("skipping directory %q", path)
			return true, filepath.SkipDir
		}
	} else if matchIgnore {
		options.GetReporter().Trace("skipping ignored file %q", path)
		return true, nil
	}
	// Don't process nor follow symlinks
	if (fi.Mode() & os.ModeSymlink) == os.ModeSymlink {
		options.GetReporter().Trace("skipping symlink file %q", path)
		return true, nil
	}
	return false, nil
}

func replacePlaceholdersInFile(replacer *strings.Replacer, file string) error {
	input, err := ioutil.ReadFile(file)
	if err != nil {
		log.Logger().Errorf("failed to read file %s: %v", file, err)
		return err
	}

	lines := string(input)
	if strings.Contains(lines, util.PlaceHolderPrefix) { // Avoid unnecessarily rewriting files
		output := replacer.Replace(lines)
		err = ioutil.WriteFile(file, []byte(output), 0644)
		if err != nil {
			log.Logger().Errorf("failed to write file %s: %v", file, err)
			return err
		}
	}

	return nil
}

func replacePlaceholdersInPathBase(replacer *strings.Replacer, path string) error {
	base := filepath.Base(path)
	newBase := replacer.Replace(base)
	if newBase != base {
		newPath := filepath.Join(filepath.Dir(path), newBase)
		if err := os.Rename(path, newPath); err != nil {
			log.Logger().Errorf("failed to rename %q to %q: %v", path, newPath, err)
			return err
		}
	}
	return nil
}

func (options *ImportOptions) addAppNameToGeneratedFile(filename, field, value string) error {
	dir := filepath.Join(options.Dir, "charts", options.AppName)
	file := filepath.Join(dir, filename)
	exists, err := util.FileExists(file)
	if err != nil {
		return err
	}
	if !exists {
		// no file so lets ignore this
		return nil
	}
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		if strings.Contains(line, field) {
			lines[i] = fmt.Sprintf("%s%s", field, value)
		}
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(file, []byte(output), 0644)
	if err != nil {
		return err
	}
	return nil
}

func (options *ImportOptions) checkChartmuseumCredentialExists() error {
	client, devNamespace, err := options.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}
	name := jenkins.DefaultJenkinsCredentialsPrefix + jenkins.Chartmuseum
	secret, err := client.CoreV1().Secrets(devNamespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting %s secret %v", name, err)
	}

	data := secret.Data
	username := string(data["BASIC_AUTH_USER"])
	password := string(data["BASIC_AUTH_PASS"])

	if secret.Labels != nil && secret.Labels[kube.LabelCredentialsType] == kube.ValueCredentialTypeUsernamePassword {
		// no need to create a credential as this will be done via the kubernetes credential provider plugin
		return nil
	}

	_, err = options.Jenkins.GetCredential(name)
	if err != nil {
		err = options.Retry(3, 10*time.Second, func() (err error) {
			return options.Jenkins.CreateCredential(name, username, password)
		})

		if err != nil {
			return fmt.Errorf("error creating Jenkins credential %s %v", name, err)
		}
	}
	return nil
}

func (options *ImportOptions) renameChartToMatchAppName() error {
	var oldChartsDir string
	dir := options.Dir
	chartsDir := filepath.Join(dir, "charts")
	exists, err := util.DirExists(chartsDir)
	if err != nil {
		return errors.Wrapf(err, "failed to check if the charts directory exists %s", chartsDir)
	}
	if !exists {
		return nil
	}
	files, err := ioutil.ReadDir(chartsDir)
	if err != nil {
		return fmt.Errorf("error matching a Jenkins X draft pack name with chart folder %v", err)
	}
	for _, fi := range files {
		if fi.IsDir() {
			name := fi.Name()
			// TODO we maybe need to try check if the sub dir named after the build pack matches first?
			if name != "preview" && name != ".git" {
				oldChartsDir = filepath.Join(chartsDir, name)
				break
			}
		}
	}
	if oldChartsDir != "" {
		// chart expects folder name to be the same as app name
		newChartsDir := filepath.Join(dir, "charts", options.AppName)

		exists, err := util.DirExists(oldChartsDir)
		if err != nil {
			return err
		}
		if exists && oldChartsDir != newChartsDir {
			err = util.RenameDir(oldChartsDir, newChartsDir, false)
			if err != nil {
				return fmt.Errorf("error renaming %s to %s, %v", oldChartsDir, newChartsDir, err)
			}
			_, err = os.Stat(newChartsDir)
			if err != nil {
				return err
			}
		}
		// now update the chart.yaml
		err = options.addAppNameToGeneratedFile("Chart.yaml", "name: ", options.AppName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (options *ImportOptions) fixDockerIgnoreFile() error {
	filename := filepath.Join(options.Dir, ".dockerignore")
	exists, err := util.FileExists(filename)
	if err == nil && exists {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("Failed to load %s: %s", filename, err)
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if strings.TrimSpace(line) == "Dockerfile" {
				lines = append(lines[:i], lines[i+1:]...)
				text := strings.Join(lines, "\n")
				err = ioutil.WriteFile(filename, []byte(text), util.DefaultWritePermissions)
				if err != nil {
					return err
				}
				options.GetReporter().Trace("Removed old `Dockerfile` entry from %s", util.ColorInfo(filename))
			}
		}
	}
	return nil
}

// CreateProwOwnersFile creates an OWNERS file in the root of the project assigning the current Git user as an approver and a reviewer. If the file already exists, does nothing.
func (options *ImportOptions) CreateProwOwnersFile() error {
	filename := filepath.Join(options.Dir, "OWNERS")
	exists, err := util.FileExists(filename)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if options.GitUserAuth != nil && options.GitUserAuth.Username != "" {
		data := prow.Owners{
			[]string{options.GitUserAuth.Username},
			[]string{options.GitUserAuth.Username},
		}
		yaml, err := yaml.Marshal(&data)
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(filename, yaml, 0644)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("GitUserAuth.Username not set")
}

// CreateProwOwnersAliasesFile creates an OWNERS_ALIASES file in the root of the project assigning the current Git user as an approver and a reviewer.
func (options *ImportOptions) CreateProwOwnersAliasesFile() error {
	filename := filepath.Join(options.Dir, "OWNERS_ALIASES")
	exists, err := util.FileExists(filename)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if options.GitUserAuth == nil {
		return errors.New("option GitUserAuth not set")
	}
	gitUser := options.GitUserAuth.Username
	if gitUser != "" {
		data := prow.OwnersAliases{
			[]string{gitUser},
			[]string{gitUser},
			[]string{gitUser},
		}
		yaml, err := yaml.Marshal(&data)
		if err != nil {
			return err
		}
		return ioutil.WriteFile(filename, yaml, 0644)
	}
	return errors.New("GitUserAuth.Username not set")
}

func (options *ImportOptions) fixMaven() error {
	if options.DisableMaven {
		return nil
	}
	dir := options.Dir
	pomName := filepath.Join(dir, "pom.xml")
	exists, err := util.FileExists(pomName)
	if err != nil {
		return err
	}
	if exists {
		err = maven.InstallMavenIfRequired()
		if err != nil {
			return err
		}

		// lets ensure the mvn plugins are ok
		out, err := options.GetCommandOutput(dir, "mvn", "io.jenkins.updatebot:updatebot-maven-plugin:RELEASE:plugin", "-Dartifact=maven-deploy-plugin", "-Dversion="+opts.MinimumMavenDeployVersion)
		if err != nil {
			return fmt.Errorf("Failed to update maven deploy plugin: %s output: %s", err, out)
		}
		out, err = options.GetCommandOutput(dir, "mvn", "io.jenkins.updatebot:updatebot-maven-plugin:RELEASE:plugin", "-Dartifact=maven-surefire-plugin", "-Dversion=3.0.0-M1")
		if err != nil {
			return fmt.Errorf("Failed to update maven surefire plugin: %s output: %s", err, out)
		}
		if !options.DryRun {
			err = options.Git().Add(dir, "pom.xml")
			if err != nil {
				return err
			}
			err = options.Git().CommitIfChanges(dir, "fix:(plugins) use a better version of maven plugins")
			if err != nil {
				return err
			}
		}

		// lets ensure the probe paths are ok
		out, err = options.GetCommandOutput(dir, "mvn", "io.jenkins.updatebot:updatebot-maven-plugin:RELEASE:chart")
		if err != nil {
			return fmt.Errorf("Failed to update chart: %s output: %s", err, out)
		}
		if !options.DryRun {
			exists, err := util.DirExists(filepath.Join(dir, "charts"))
			if err != nil {
				return err
			}
			if exists {
				err = options.Git().Add(dir, "charts")
				if err != nil {
					return err
				}
				err = options.Git().CommitIfChanges(dir, "fix:(chart) fix up the probe path")
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (options *ImportOptions) DefaultsFromTeamSettings() error {
	settings, err := options.TeamSettings()
	if err != nil {
		return err
	}
	return options.DefaultValuesFromTeamSettings(settings)
}

// DefaultValuesFromTeamSettings defaults the repository options from the given team settings
func (options *ImportOptions) DefaultValuesFromTeamSettings(settings *v1.TeamSettings) error {
	if options.DeployKind == "" {
		options.DeployKind = settings.DeployKind
	}

	// lets override any deploy options from the team settings if they are not specified
	teamDeployOptions := settings.GetDeployOptions()
	if !options.FlagChanged(opts.OptionCanary) {
		options.DeployOptions.Canary = teamDeployOptions.Canary
	}
	if !options.FlagChanged(opts.OptionHPA) {
		options.DeployOptions.HPA = teamDeployOptions.HPA
	}
	if options.Organisation == "" {
		options.Organisation = settings.Organisation
	}
	if options.GitRepositoryOptions.Owner == "" {
		options.GitRepositoryOptions.Owner = settings.Organisation
	}
	if options.DockerRegistryOrg == "" {
		options.DockerRegistryOrg = settings.DockerRegistryOrg
	}
	if options.GitRepositoryOptions.ServerURL == "" {
		options.GitRepositoryOptions.ServerURL = settings.GitServer
	}
	options.GitRepositoryOptions.Public = settings.GitPublic || options.GitRepositoryOptions.Public
	options.PipelineServer = settings.GitServer
	options.PipelineUserName = settings.PipelineUsername
	return nil
}

func (options *ImportOptions) allDraftPacks() ([]string, error) {
	// lets make sure we have the latest draft packs
	initOpts := initcmd.InitOptions{
		CommonOptions: options.CommonOptions,
	}
	log.Logger().Debug("Getting latest packs ...")
	dir, _, err := initOpts.InitBuildPacks(nil)
	if err != nil {
		return nil, err
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	for _, f := range files {
		if f.IsDir() {
			result = append(result, f.Name())
		}
	}
	return result, err

}

// ConfigureImportOptions updates the import options struct based on values from the create repo struct
func (options *ImportOptions) ConfigureImportOptions(repoData *gits.CreateRepoData) {
	// configure the import options based on previous answers
	options.AppName = repoData.RepoName
	options.GitProvider = repoData.GitProvider
	options.Organisation = repoData.Organisation
	options.Repository = repoData.RepoName
	options.GitDetails = *repoData
	options.GitServer = repoData.GitServer
}

// GetGitRepositoryDetails determines the git repository details to use during the import command
func (options *ImportOptions) GetGitRepositoryDetails() (*gits.CreateRepoData, error) {
	err := options.DefaultsFromTeamSettings()
	if err != nil {
		return nil, err
	}
	authConfigSvc, err := options.GitLocalAuthConfigService()
	if err != nil {
		return nil, err
	}
	//config git repositoryoptions parameters: Owner and RepoName
	options.GitRepositoryOptions.Owner = options.Organisation
	options.GitRepositoryOptions.RepoName = options.Repository
	details, err := gits.PickNewOrExistingGitRepository(options.BatchMode, authConfigSvc,
		"", &options.GitRepositoryOptions, nil, nil, options.Git(), false, options.GetIOFileHandles())
	if err != nil {
		return nil, err
	}
	return details, nil
}

// modifyDeployKind lets modify the deployment kind if the team settings or CLI settings are different
func (options *ImportOptions) modifyDeployKind() error {
	deployKind := options.DeployKind
	if deployKind == "" {
		return nil
	}
	dopts := options.DeployOptions

	copy := *options.CommonOptions
	cmd, eo := edit.NewCmdEditDeployKindAndOption(&copy)
	eo.Dir = options.Dir

	// lets parse the CLI arguments so that the flags are marked as specified to force them to be overridden
	err := cmd.Flags().Parse(edit.ToDeployArguments(opts.OptionKind, deployKind, dopts.Canary, dopts.HPA))
	if err != nil {
		return err
	}
	err = eo.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to modify the deployment kind to %s", deployKind)
	}
	return nil
}
