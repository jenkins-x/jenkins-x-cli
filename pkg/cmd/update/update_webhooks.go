package update

import (
	"fmt"
	"io/ioutil"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateWebhooksOptions the flags for updating webhooks
type UpdateWebhooksOptions struct {
	*opts.CommonOptions
	Org             string
	User            string
	Repo            string
	ExactHookMatch  bool
	PreviousHookUrl string
	HMAC            string
	Endpoint        string
	DryRun          bool
	WarnOnFail      bool
}

var (
	updateWebhooksLong = templates.LongDesc(`
		Updates the webhooks for all the source repositories optionally filtering by owner and/or repository

`)

	updateWebhooksExample = templates.Examples(`
		# update all the webhooks for all SourceRepository and Environment resource:
		jx update webhooks

		# only update the webhooks for a given owner
		jx update webhooks --org=mycorp

`)
)

func NewCmdUpdateWebhooks(commonOpts *opts.CommonOptions) *cobra.Command {
	options := createUpdateWebhooksOptions(commonOpts)

	cmd := &cobra.Command{
		Use:     "webhooks",
		Aliases: []string{"webhook"},
		Short:   "Updates the webhooks for all the source repositories optionally filtering by owner and/or repository",
		Long:    updateWebhooksLong,
		Example: updateWebhooksExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Org, "owner", "o", "", "The name of the git organisation or user to filter on")
	cmd.Flags().StringVarP(&options.Repo, "repo", "r", "", "The name of the repository to filter on")
	cmd.Flags().BoolVarP(&options.ExactHookMatch, "exact-hook-url-match", "", true, "Whether to exactly match the hook based on the URL")
	cmd.Flags().StringVarP(&options.PreviousHookUrl, "previous-hook-url", "", "", "Whether to match based on an another URL")
	cmd.Flags().StringVarP(&options.HMAC, "hmac", "", "", "Don't use the HMAC token from the cluster, use the provided token")
	cmd.Flags().StringVarP(&options.Endpoint, "endpoint", "", "", "Don't use the endpoint from the cluster, use the provided endpoint")
	cmd.Flags().BoolVarP(&options.WarnOnFail, "warn-on-fail", "", false, "If enabled lets just log a warning that we could not update the webhook")

	return cmd
}

func createUpdateWebhooksOptions(commonOpts *opts.CommonOptions) UpdateWebhooksOptions {
	options := UpdateWebhooksOptions{
		CommonOptions: commonOpts,
	}
	return options
}

// Run runs the command
func (o *UpdateWebhooksOptions) Run() error {
	client, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to get kube client")
	}

	webhookURL := ""
	if o.Endpoint != "" {
		webhookURL = o.Endpoint
	} else {
		webhookURL, err = o.GetWebHookEndpoint()
		if err != nil {
			return err
		}
	}

	isProwEnabled, err := o.IsProw()
	if err != nil {
		return err
	}

	hmacToken := ""
	if isProwEnabled {
		if o.HMAC != "" {
			hmacToken = o.HMAC
		} else {
			hmacTokenSecret, err := client.CoreV1().Secrets(ns).Get("hmac-token", metav1.GetOptions{})
			if err != nil {
				return err
			}
			hmacToken = string(hmacTokenSecret.Data["hmac"])
		}
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	srList, err := jxClient.JenkinsV1().SourceRepositories(ns).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to find any SourceRepositories in namespace %s", ns)
	}

	envMap, _, err := kube.GetEnvironments(jxClient, ns)

	for _, sr := range srList.Items {
		if o.matchesRepository(&sr) {
			err = o.ensureWebHookCreated(&sr, webhookURL, isProwEnabled, hmacToken)
			if err != nil {
				if o.WarnOnFail {
					log.Logger().Warnf(err.Error())
				} else {
					return err
				}
			}
			if !isProwEnabled {
				isEnv := kube.IsEnvironmentRepository(envMap, &sr)
				err = o.ensureJenkinsJobExists(&sr, isEnv)
				if err != nil {
					if o.WarnOnFail {
						log.Logger().Warnf(err.Error())
					} else {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (o *UpdateWebhooksOptions) ensureWebHookCreated(repository *v1.SourceRepository, webhookURL string, isProwEnabled bool, hmacToken string) error {
	spec := repository.Spec
	gitServerURL := spec.Provider
	owner := spec.Org
	repo := spec.Repo
	gitKind, err := o.GitServerHostURLKind(gitServerURL)
	if err != nil {
		return errors.Wrapf(err, "failed to find Git Server kind for host %s", gitServerURL)
	}

	ghOwner, err := o.GetGitHubAppOwnerForRepository(repository)
	if err != nil {
		return err
	}
	provider, err := o.GitProviderForGitServerURL(gitServerURL, gitKind, ghOwner)
	if err != nil {
		return errors.Wrapf(err, "failed to find Git provider for host %s and kind %s", gitServerURL, gitKind)
	}

	info := util.ColorInfo
	if o.Verbose {
		log.Logger().Infof("Updating webhooks for Owner: %s and Repository: %s in git server: %s", info(owner), info(repo), info(gitServerURL))
	}

	err = o.updateRepoHook(provider, owner, repo, webhookURL, isProwEnabled, hmacToken)
	if err != nil {
		return errors.Wrapf(err, "failed to update webhooks for Owner: %s and Repository: %s in git server: %s", owner, repo, gitServerURL)
	}
	return nil
}

func (o *UpdateWebhooksOptions) ensureJenkinsJobExists(repository *v1.SourceRepository, isEnvironment bool) error {
	authConfigSvc := auth.NewMemoryAuthConfigService()
	gitURL, err := kube.GetRepositoryGitURL(repository)
	if err != nil {
		return errors.Wrapf(err, "failed to find GitURL for repository %s", repository.Name)
	}
	if gitURL == "" {
		return fmt.Errorf("no GitURL for repository %s", repository.Name)
	}

	envDir, err := ioutil.TempDir("", "jx-boot-jenkins-repo-")
	if err != nil {
		return errors.Wrapf(err, "failed to create a temporary directory for repository %s", repository.Name)
	}

	err = o.Git().Clone(gitURL, envDir)
	if err != nil {
		return errors.Wrapf(err, "failed to clone %s to %s for repository %s", gitURL, envDir, repository.Name)
	}

	spec := repository.Spec
	gitServerURL := spec.Provider
	gitKind, err := o.GitServerHostURLKind(gitServerURL)
	if err != nil {
		return errors.Wrapf(err, "failed to find Git Server kind for host %s", gitServerURL)
	}
	ghOwner, err := o.GetGitHubAppOwnerForRepository(repository)
	if err != nil {
		return err
	}
	gitProvider, err := o.GitProviderForGitServerURL(gitServerURL, gitKind, ghOwner)
	if err != nil {
		return errors.Wrapf(err, "failed to find Git provider for host %s and kind %s", gitServerURL, gitKind)
	}

	log.Logger().Infof("ensuring we have a Jenkins job for git URL %s isEnvironment: %v", gitURL, isEnvironment)
	return o.ImportProject(gitURL, envDir, jenkinsfile.Name, "", "", false, gitProvider, authConfigSvc, isEnvironment, true)
}

// GetOrgOrUserFromOptions returns the Org if set,
// if not set, returns the user if that is set
// or "" if neither is set
func (o *UpdateWebhooksOptions) GetOrgOrUserFromOptions() string {
	owner := o.Org
	if owner == "" && o.Username != "" {
		owner = o.Username
	}
	return owner
}

func (o *UpdateWebhooksOptions) updateRepoHook(git gits.GitProvider, owner string, repoName string, webhookURL string, isProwEnabled bool, hmacToken string) error {
	userName := git.UserAuth().Username
	log.Logger().Infof("Checking hooks for repository %s/%s with user %s", util.ColorInfo(owner), util.ColorInfo(repoName), util.ColorInfo(userName))

	webhooks, err := git.ListWebHooks(owner, repoName)
	if err != nil {
		log.Logger().Infof("no webhooks found repository %s/%s", util.ColorInfo(owner), util.ColorInfo(repoName))
	}
	isInsecureSSL, err := o.IsInsecureSSLWebhooks()
	if err != nil {
		return errors.Wrapf(err, "failed to check if we need to setup insecure SSL webhook")
	}
	webHookArgs := &gits.GitWebHookArguments{
		Owner: owner,
		Repo: &gits.GitRepository{
			Name: repoName,
		},
		URL:         webhookURL,
		InsecureSSL: isInsecureSSL,
	}
	if userName != owner {
		webHookArgs.Repo.Organisation = owner
	}
	if isProwEnabled {
		webHookArgs.Secret = hmacToken
	}
	if len(webhooks) > 0 {
		// find matching hook
		for _, webHook := range webhooks {
			if o.matchesWebhookURL(git, webhookURL, webHook) {
				log.Logger().Infof("Found matching hook for url %s", util.ColorInfo(webHook.URL))
				webHookArgs.ID = webHook.ID
				webHookArgs.ExistingURL = o.PreviousHookUrl
				if !o.DryRun {
					if err := git.UpdateWebHook(webHookArgs); err != nil {
						return errors.Wrapf(err, "updating the webhook %q on repository '%s/%s'",
							webhookURL, owner, repoName)
					}
					return nil
				}
			}
		}
	}
	if !o.DryRun {
		if err := git.CreateWebHook(webHookArgs); err != nil {
			return errors.Wrapf(err, "creating the webhook %q on repository '%s/%s'",
				webhookURL, owner, repoName)
		}
	}
	return nil
}

func (o *UpdateWebhooksOptions) matchesWebhookURL(git gits.GitProvider, webhookURL string, webHookArgs *gits.GitWebHookArguments) bool {
	if "" != o.PreviousHookUrl {
		return o.PreviousHookUrl == webHookArgs.URL
	}

	if git.Kind() == "gitlab" {
		return strings.HasPrefix(webHookArgs.URL, webhookURL)
	}
	if o.ExactHookMatch {
		return webhookURL == webHookArgs.URL
	} else {
		return strings.Contains(webHookArgs.URL, "hook.jx")
	}
}

// matchesRepository returns true if the given source repository matchesWebhookURL the current filters
func (o *UpdateWebhooksOptions) matchesRepository(repository *v1.SourceRepository) bool {
	if o.Org != "" && o.Org != repository.Spec.Org {
		return false
	}
	if o.Repo != "" && o.Repo != repository.Spec.Repo {
		return false
	}
	return true
}
