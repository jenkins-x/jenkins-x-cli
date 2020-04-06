package opts

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/kube/cluster"

	"github.com/jenkins-x/jx/pkg/gits/features"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/issues"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	gitcfg "gopkg.in/src-d/go-git.v4/config"
)

// FindGitInfo parses the git information from the given directory
func (o *CommonOptions) FindGitInfo(dir string) (*gits.GitRepository, error) {
	_, gitConf, err := o.Git().FindGitConfigDir(dir)
	if err != nil {
		return nil, fmt.Errorf("Could not find a .git directory: %s\n", err)
	} else {
		if gitConf == "" {
			return nil, fmt.Errorf("No git conf dir found")
		}
		gitURL, err := o.Git().DiscoverUpstreamGitURL(gitConf)
		if err != nil {
			return nil, fmt.Errorf("Could not find the remote git source URL:  %s", err)
		}
		return gits.ParseGitURL(gitURL)
	}
}

// NewGitProvider creates a new git provider for the given list of argumentes
func (o *CommonOptions) NewGitProvider(gitURL string, message string, authConfigSvc auth.ConfigService, gitKind string, ghOwner string, batchMode bool, gitter gits.Gitter) (gits.GitProvider, error) {
	if o.factory == nil {
		return nil, errors.New("command factory is not initialized")
	}
	return o.factory.CreateGitProvider(gitURL, message, authConfigSvc, gitKind, ghOwner, batchMode, gitter, o.GetIOFileHandles())
}

// CreateGitProvider creates a git from the given directory
func (o *CommonOptions) CreateGitProvider(dir string) (*gits.GitRepository, gits.GitProvider, issues.IssueProvider, error) {
	gitDir, gitConfDir, err := o.Git().FindGitConfigDir(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	if gitDir == "" || gitConfDir == "" {
		log.Logger().Warnf("No git directory could be found from dir %s", dir)
		return nil, nil, nil, nil
	}

	gitUrl, err := o.Git().DiscoverUpstreamGitURL(gitConfDir)
	if err != nil {
		return nil, nil, nil, err
	}
	gitInfo, err := gits.ParseGitURL(gitUrl)
	if err != nil {
		return nil, nil, nil, err
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return gitInfo, nil, nil, err
	}
	gitKind, err := o.GitServerKind(gitInfo)
	ghOwner, err := o.GetGitHubAppOwner(gitInfo)
	if err != nil {
		return gitInfo, nil, nil, err
	}
	gitProvider, err := gitInfo.CreateProvider(cluster.IsInCluster(), authConfigSvc, gitKind, ghOwner, o.Git(), o.BatchMode, o.GetIOFileHandles())
	if err != nil {
		return gitInfo, gitProvider, nil, err
	}
	tracker, err := o.CreateIssueProvider(dir)
	if err != nil {
		return gitInfo, gitProvider, tracker, err
	}
	return gitInfo, gitProvider, tracker, nil
}

// EnsureGitServiceCRD ensure that the GitService CRD is installed
func (o *CommonOptions) EnsureGitServiceCRD(server *auth.AuthServer) error {
	kind := server.Kind
	if kind == "github" && server.URL == gits.GitHubURL {
		return nil
	}
	if kind == "" {
		log.Logger().Warnf("Kind of git server %s with URL %s is empty", server.Name, server.URL)
		return nil
	}
	// lets lazily populate the name if its empty
	if server.Name == "" {
		server.Name = kind
	}

	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create JX Client")
	}
	err = kube.EnsureGitServiceExistsForHost(jxClient, devNs, kind, server.Name, server.URL, o.Out)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure GitService exists for kind %s server %s in namespace %s", kind, server.URL, devNs)
	}
	log.Logger().Infof("Ensured we have a GitService called %s for URL %s in namespace %s", server.Name, server.URL, devNs)
	return nil
}

// DiscoverGitURL discovers the Git URL
func (o *CommonOptions) DiscoverGitURL(gitConf string) (string, error) {
	if gitConf == "" {
		return "", fmt.Errorf("No GitConfDir defined!")
	}
	cfg := gitcfg.NewConfig()
	data, err := ioutil.ReadFile(gitConf)
	if err != nil {
		return "", fmt.Errorf("Failed to load %s due to %s", gitConf, err)
	}

	err = cfg.Unmarshal(data)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal %s due to %s", gitConf, err)
	}
	remotes := cfg.Remotes
	if len(remotes) == 0 {
		return "", nil
	}
	url := o.Git().GetRemoteUrl(cfg, "origin")
	if url == "" {
		url = o.Git().GetRemoteUrl(cfg, "upstream")
		if url == "" {
			url, err = o.PickGitRemoteURL(cfg)
			if err != nil {
				return "", err
			}
		}
	}
	return url, nil
}

// AddGitRepoOptionsArgumentsWithGithubDefault adds common git flags with a default provider URL of github.com to the given cobra command
func AddGitRepoOptionsArgumentsWithGithubDefault(cmd *cobra.Command, repositoryOptions *gits.GitRepositoryOptions) {
	AddGitRepoOptionsArgumentsWithDefaultProviderURL(cmd, repositoryOptions, "https://github.com")
}

// AddGitRepoOptionsArguments adds common git flags with no default provider URL to the given cobra command
func AddGitRepoOptionsArguments(cmd *cobra.Command, repositoryOptions *gits.GitRepositoryOptions) {
	AddGitRepoOptionsArgumentsWithDefaultProviderURL(cmd, repositoryOptions, "")
}

// AddGitRepoOptionsArgumentsWithDefaultProviderURL adds common git flags with the given default provider URL to the given cobra command
func AddGitRepoOptionsArgumentsWithDefaultProviderURL(cmd *cobra.Command, repositoryOptions *gits.GitRepositoryOptions, defaultProviderURL string) {
	cmd.Flags().StringVarP(&repositoryOptions.ServerURL, "git-provider-url", "", defaultProviderURL, "The Git server URL to create new Git repositories inside")
	cmd.Flags().StringVarP(&repositoryOptions.ServerKind, "git-provider-kind", "", "",
		"Kind of Git server. If not specified, kind of server will be autodetected from Git provider URL. Possible values: bitbucketcloud, bitbucketserver, gitea, gitlab, github, fakegit")
	cmd.Flags().StringVarP(&repositoryOptions.Username, "git-username", "", "", "The Git username to use for creating new Git repositories")
	cmd.Flags().StringVarP(&repositoryOptions.ApiToken, "git-api-token", "", "", "The Git API token to use for creating new Git repositories")
	cmd.Flags().BoolVarP(&repositoryOptions.Public, "git-public", "", false, "Create new Git repositories as public")
}

// GitServerKind returns the kind of the git server
func (o *CommonOptions) GitServerKind(gitInfo *gits.GitRepository) (string, error) {
	return o.GitServerHostURLKind(gitInfo.HostURL())
}

// GitServerHostURLKind returns the kind of git server host URL
func (o *CommonOptions) GitServerHostURLKind(hostURL string) (string, error) {
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return "", err
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return "", err
	}

	var clusterAuthConfig *auth.AuthConfig

	// Pass the in-cluster git auth config to GetGitServiceKind
	clusterAuthConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return "", errors.Wrapf(err, "getting cluster-based auth configmap for checking kind of git url %s", hostURL)
	}
	if clusterAuthConfigSvc != nil {
		clusterAuthConfig = clusterAuthConfigSvc.Config()
	}

	kind, err := kube.GetGitServiceKind(jxClient, kubeClient, devNs, clusterAuthConfig, hostURL)

	if err != nil {
		return kind, err
	}
	if kind == "" {
		if o.BatchMode {
			return "", fmt.Errorf("No Git server kind could be found for URL %s\nPlease try specify it via: jx create git server someKind %s", hostURL, hostURL)
		}
		kind, err = util.PickName(gits.KindGits, fmt.Sprintf("Pick what kind of Git server is: %s", hostURL), "", o.GetIOFileHandles())
		if err != nil {
			return "", err
		}
		if kind == "" {
			return "", fmt.Errorf("No Git kind chosen!")
		}
	}
	return kind, nil
}

// GitProviderForURL returns a GitProvider for the given git URL
func (o *CommonOptions) GitProviderForURL(gitURL string, message string) (gits.GitProvider, error) {
	if o.fakeGitProvider != nil {
		return o.fakeGitProvider, nil
	}
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return nil, err
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return nil, err
	}
	gitKind, err := o.GitServerKind(gitInfo)
	if err != nil {
		return nil, err
	}
	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return nil, err
	}
	return gitInfo.PickOrCreateProvider(authConfigSvc, message, o.BatchMode, gitKind, gha, o.Git(), o.GetIOFileHandles())
}

// GitProviderForGitServerURL returns a GitProvider for the given Git server URL
func (o *CommonOptions) GitProviderForGitServerURL(gitServiceURL string, gitKind string, ghOwner string) (gits.GitProvider, error) {
	if o.fakeGitProvider != nil {
		return o.fakeGitProvider, nil
	}

	var err error
	gha := ghOwner != ""
	// if the github owner is empty then check if github app mode is enabled
	if !gha {
		gha, err = o.IsGitHubAppMode()
		if err != nil {
			return nil, errors.Wrap(err, "when trying to check if in GitHub App mode")
		}
	}

	var authConfigSvc auth.ConfigService
	if gha {
		authConfigSvc, err = o.GitAuthConfigServiceGitHubAppMode(gitKind)
		if err != nil {
			return nil, errors.Wrap(err, "when creating auth config service using GitAuthConfigServiceGitHubAppMode")
		}
	} else {
		authConfigSvc, err = o.GitAuthConfigService()
		if err != nil {
			return nil, errors.Wrap(err, "when creating auth config service using GitAuthConfigService")
		}
	}

	return gits.CreateProviderForURL(cluster.IsInCluster(), authConfigSvc, gitKind, gitServiceURL, ghOwner, o.Git(), o.BatchMode, o.GetIOFileHandles())
}

// CreateGitProviderForURLWithoutKind creates a git provider from URL wihtout kind
func (o *CommonOptions) CreateGitProviderForURLWithoutKind(gitURL string) (gits.GitProvider, *gits.GitRepository, error) {
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return nil, gitInfo, err
	}
	gitKind, err := o.GitServerKind(gitInfo)
	if err != nil {
		return nil, gitInfo, err
	}
	gitServer := gitInfo.HostURL()
	ghOwner, err := o.GetGitHubAppOwner(gitInfo)
	if err != nil {
		return nil, gitInfo, err
	}
	provider, err := o.GitProviderForGitServerURL(gitServer, gitKind, ghOwner)
	return provider, gitInfo, err
}

// GetGitHubAppOwner returns the github app owner to filter tokens by if using a GitHub app model
// which requires a separate token per owner
func (o *CommonOptions) GetGitHubAppOwner(gitInfo *gits.GitRepository) (string, error) {
	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return "", err
	}
	if gha {
		return gitInfo.Organisation, nil
	}
	return "", nil
}

// GetGitHubAppOwnerForRepository returns the github app owner to filter tokens by if using a GitHub app model
//// which requires a separate token per owner
func (o *CommonOptions) GetGitHubAppOwnerForRepository(repository *jenkinsv1.SourceRepository) (string, error) {
	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return "", err
	}
	if gha {
		return repository.Spec.Org, nil
	}
	return "", nil
}

// IsGitHubAppMode returns true if we have enabled github app mode
func (o *CommonOptions) IsGitHubAppMode() (bool, error) {
	teamSettings, err := o.TeamSettings()
	if err != nil {
		return false, errors.Wrap(err, "error loading TeamSettings to determine if in GitHub app mode")
	}
	requirements, err := config.GetRequirementsConfigFromTeamSettings(teamSettings)
	if err != nil {
		return false, errors.Wrap(err, "error getting Requirements from TeamSettings to determine if in GitHub app mode")
	}
	return requirements != nil && requirements.GithubApp != nil && requirements.GithubApp.Enabled, nil
}

// InitGitConfigAndUser validates we have git setup
func (o *CommonOptions) InitGitConfigAndUser() error {
	// lets validate we have git configured
	_, _, err := gits.EnsureUserAndEmailSetup(o.Git())
	if err != nil {
		return err
	}

	err = o.RunCommandVerbose("git", "config", "--global", "credential.helper", "store")
	if err != nil {
		return err
	}
	if os.Getenv("XDG_CONFIG_HOME") == "" {
		log.Logger().Warnf("Note that the environment variable $XDG_CONFIG_HOME is not defined so we may not be able to push to git!")
	}
	return nil
}

func (o *CommonOptions) getAuthConfig() (*auth.AuthConfig, error) {
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the git auth config service")
	}
	if authConfigSvc == nil {
		return nil, errors.New("empty auth config")
	}
	authConfig := authConfigSvc.Config()
	if authConfig == nil {
		return nil, errors.New("empty Git config")
	}
	return authConfig, nil
}

// GetPipelineGitAuth returns the pipeline git authentication credentials
func (o *CommonOptions) GetPipelineGitAuth() (*auth.AuthServer, *auth.UserAuth, error) {
	authConfig, err := o.getAuthConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get auth config")
	}
	server, user := authConfig.GetPipelineAuth()
	return server, user, nil
}

// GetPipelineGitHubAppAuth returns the pipeline git authentication credentials
func (o *CommonOptions) GetPipelineGitHubAppAuth(ghOwner string) (*auth.AuthServer, *auth.UserAuth, error) {
	authConfig, err := o.getAuthConfig()
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to get auth config")
	}
	server, user := authConfig.GetPipelineAuth()
	if ghOwner != "" {
		if server != nil {
			for _, u := range server.Users {
				if u.GithubAppOwner == ghOwner {
					user = u
					break
				}
			}
		} else {
			for _, server = range authConfig.Servers {
				if server != nil {
					for _, u := range server.Users {
						if u.GithubAppOwner == ghOwner {
							user = u
							break
						}
					}
				}
			}
		}
	}
	return server, user, nil
}

// GetPipelineGitAuthForRepo returns the pipeline git authentication credentials for a repo
func (o *CommonOptions) GetPipelineGitAuthForRepo(gitInfo *gits.GitRepository) (*auth.AuthServer, *auth.UserAuth, error) {
	ghOwner, err := o.GetGitHubAppOwner(gitInfo)
	if err != nil {
		return nil, nil, err
	}
	if ghOwner != "" {
		return o.GetPipelineGitHubAppAuth(ghOwner)
	}
	return o.GetPipelineGitAuth()
}

// DisableFeatures iterates over all the repositories in org (except those that match excludes) and disables issue
// trackers, projects and wikis if they are not in use.
//
// Issue trackers are not in use if they have no open or closed issues
// Projects are not in use if there are no open projects
// Wikis are not in use if the provider returns that the wiki is not enabled
//
// Note that the requirement for issues is no issues at all so that we don't close issue trackers that have historic info
//
// If includes is not empty only those that match an include will be operated on. If dryRun is true, the operations to
// be done will printed and but nothing done. If batchMode is false, then each change will be prompted.
func (o *CommonOptions) DisableFeatures(orgs []string, includes []string, excludes []string, dryRun bool) error {
	for _, org := range orgs {
		info, err := gits.ParseGitOrganizationURL(org)
		if err != nil {
			return errors.Wrapf(err, "parsing %s", org)
		}
		kind, err := o.GitServerHostURLKind(info.HostURL())
		if err != nil {
			return errors.Wrapf(err, "determining git provider kind from %s", org)
		}
		ghOwner, err := o.GetGitHubAppOwner(info)
		if err != nil {
			return err
		}
		provider, err := o.GitProviderForGitServerURL(info.HostURL(), kind, ghOwner)
		if err != nil {
			return errors.Wrapf(err, "creating git provider for %s", org)
		}
		err = features.DisableFeaturesForOrg(info.Organisation, includes, excludes, dryRun, o.BatchMode, provider, o.GetIOFileHandles())
		if err != nil {
			return errors.Wrapf(err, "disabling features for %s", org)
		}
	}
	return nil
}
