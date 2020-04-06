package promote

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/builds"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/pkg/errors"
	survey "gopkg.in/AlecAivazis/survey.v1"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/environments"

	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/jenkins-x/jx/pkg/kube/services"

	"github.com/blang/semver"
	typev1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	optionPullRequestPollTime = "pull-request-poll-time"

	GitStatusSuccess = "success"
)

var (
	waitAfterPullRequestCreated = time.Second * 3
)

// PromoteOptions containers the CLI options
type PromoteOptions struct {
	*opts.CommonOptions

	Namespace               string
	Environment             string
	Application             string
	Pipeline                string
	Build                   string
	Version                 string
	ReleaseName             string
	LocalHelmRepoName       string
	HelmRepositoryURL       string
	NoHelmUpdate            bool
	AllAutomatic            bool
	NoMergePullRequest      bool
	NoPoll                  bool
	NoWaitAfterMerge        bool
	IgnoreLocalFiles        bool
	NoWaitForUpdatePipeline bool
	Timeout                 string
	PullRequestPollTime     string
	Filter                  string
	Alias                   string

	// calculated fields
	TimeoutDuration         *time.Duration
	PullRequestPollDuration *time.Duration
	Activities              typev1.PipelineActivityInterface
	GitInfo                 *gits.GitRepository
	jenkinsURL              string
	releaseResource         *v1.Release
	ReleaseInfo             *ReleaseInfo
	prow                    bool

	// Used for testing
	CloneDir string
}

type ReleaseInfo struct {
	ReleaseName     string
	FullAppName     string
	Version         string
	PullRequestInfo *gits.PullRequestInfo
}

var (
	promote_long = templates.LongDesc(`
		Promotes a version of an application to zero to many permanent environments.

		For more documentation see: [https://jenkins-x.io/docs/getting-started/promotion/](https://jenkins-x.io/docs/getting-started/promotion/)

`)

	promote_example = templates.Examples(`
		# Promote a version of the current application to staging
        # discovering the application name from the source code
		jx promote --version 1.2.3 --env staging

		# Promote a version of the myapp application to production
		jx promote --app myapp --version 1.2.3 --env production

		# To search for all the available charts for a given name use -f.
		# e.g. to find a redis chart to install
		jx promote -f redis

		# To promote a postgres chart using an alias
		jx promote -f postgres --alias mydb

		# To create or update a Preview Environment please see the 'jx preview' command if you are inside a git clone of a repo
		jx preview
	`)
)

// NewCmdPromote creates the new command for: jx get prompt
func NewCmdPromote(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &PromoteOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "promote [application]",
		Short:   "Promotes a version of an application to an Environment",
		Long:    promote_long,
		Example: promote_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Namespace to promote to")
	cmd.Flags().StringVarP(&options.Environment, opts.OptionEnvironment, "e", "", "The Environment to promote to")
	cmd.Flags().BoolVarP(&options.AllAutomatic, "all-auto", "", false, "Promote to all automatic environments in order")

	options.AddPromoteOptions(cmd)
	return cmd
}

// AddPromoteOptions adds command level options to `promote`
func (o *PromoteOptions) AddPromoteOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Application, opts.OptionApplication, "a", "", "The Application to promote")
	cmd.Flags().StringVarP(&o.Filter, "filter", "f", "", "The search filter to find charts to promote")
	cmd.Flags().StringVarP(&o.Alias, "alias", "", "", "The optional alias used in the 'requirements.yaml' file")
	cmd.Flags().StringVarP(&o.Pipeline, "pipeline", "", "", "The Pipeline string in the form 'folderName/repoName/branch' which is used to update the PipelineActivity. If not specified its defaulted from  the '$BUILD_NUMBER' environment variable")
	cmd.Flags().StringVarP(&o.Build, "build", "", "", "The Build number which is used to update the PipelineActivity. If not specified its defaulted from  the '$BUILD_NUMBER' environment variable")
	cmd.Flags().StringVarP(&o.Version, "version", "v", "", "The Version to promote")
	cmd.Flags().StringVarP(&o.LocalHelmRepoName, "helm-repo-name", "r", kube.LocalHelmRepoName, "The name of the helm repository that contains the app")
	cmd.Flags().StringVarP(&o.HelmRepositoryURL, "helm-repo-url", "u", "", "The Helm Repository URL to use for the App")
	cmd.Flags().StringVarP(&o.ReleaseName, "release", "", "", "The name of the helm release")
	cmd.Flags().StringVarP(&o.Timeout, opts.OptionTimeout, "t", "1h", "The timeout to wait for the promotion to succeed in the underlying Environment. The command fails if the timeout is exceeded or the promotion does not complete")
	cmd.Flags().StringVarP(&o.PullRequestPollTime, optionPullRequestPollTime, "", "20s", "Poll time when waiting for a Pull Request to merge")
	cmd.Flags().BoolVarP(&o.NoHelmUpdate, "no-helm-update", "", false, "Allows the 'helm repo update' command if you are sure your local helm cache is up to date with the version you wish to promote")
	cmd.Flags().BoolVarP(&o.NoMergePullRequest, "no-merge", "", false, "Disables automatic merge of promote Pull Requests")
	cmd.Flags().BoolVarP(&o.NoPoll, "no-poll", "", false, "Disables polling for Pull Request or Pipeline status")
	cmd.Flags().BoolVarP(&o.NoWaitAfterMerge, "no-wait", "", false, "Disables waiting for completing promotion after the Pull request is merged")
	cmd.Flags().BoolVarP(&o.IgnoreLocalFiles, "ignore-local-file", "", false, "Ignores the local file system when deducing the Git repository")
}

func (o *PromoteOptions) hasApplicationFlag() bool {
	return o.Application != ""
}

func (o *PromoteOptions) hasArgs() bool {
	return len(o.Args) > 0
}

func (o *PromoteOptions) setApplicationNameFromArgs() {
	o.Application = o.Args[0]
}

func (o *PromoteOptions) hasFilterFlag() bool {
	return o.Filter != ""
}

type searchForChartFn func(filter string) (string, error)

func (o *PromoteOptions) setApplicationNameFromFilter(searchForChart searchForChartFn) error {
	app, err := searchForChart(o.Filter)
	if err != nil {
		return errors.Wrap(err, "searching app name in chart failed")
	}

	o.Application = app

	return nil
}

type discoverAppNameFn func() (string, error)

func (o *PromoteOptions) setApplicationNameFromDiscoveredAppName(discoverAppName discoverAppNameFn) error {
	app, err := discoverAppName()
	if err != nil {
		return errors.Wrap(err, "discovering app name failed")
	}

	if !o.BatchMode {
		var continueWithAppName bool

		question := fmt.Sprintf("Are you sure you want to promote the application named: %s?", app)

		prompt := &survey.Confirm{
			Message: question,
			Default: true,
		}
		surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
		err = survey.AskOne(prompt, &continueWithAppName, nil, surveyOpts)
		if err != nil {
			return err
		}

		if !continueWithAppName {
			return errors.New("user canceled execution")
		}
	}

	o.Application = app

	return nil
}

// EnsureApplicationNameIsDefined validates if an application name flag was provided by the user. If missing it will
// try to set it up or return an error
func (o *PromoteOptions) EnsureApplicationNameIsDefined(sf searchForChartFn, df discoverAppNameFn) error {
	if !o.hasApplicationFlag() && o.hasArgs() {
		o.setApplicationNameFromArgs()
	}

	if !o.hasApplicationFlag() && o.hasFilterFlag() {
		err := o.setApplicationNameFromFilter(sf)
		if err != nil {
			return err
		}
	}

	if !o.hasApplicationFlag() {
		return o.setApplicationNameFromDiscoveredAppName(df)
	}

	return nil
}

// Run implements this command
func (o *PromoteOptions) Run() error {
	err := o.EnsureApplicationNameIsDefined(o.SearchForChart, o.DiscoverAppName)
	if err != nil {
		return err
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	if o.Namespace == "" {
		o.Namespace = ns
	}

	prow, err := o.IsProw()
	if err != nil {
		return err
	}
	if prow {
		o.prow = true
		log.Logger().Warn("prow based install so skip waiting for the merge of Pull Requests to go green as currently there is an issue with getting" +
			"statuses from the PR, see https://github.com/jenkins-x/jx/issues/2410")
		o.NoWaitForUpdatePipeline = true
	}

	if o.HelmRepositoryURL == "" {
		o.HelmRepositoryURL = o.DefaultChartRepositoryURL()
	}
	if o.Environment == "" && !o.BatchMode {
		names := []string{}
		m, allEnvNames, err := kube.GetOrderedEnvironments(jxClient, ns)
		if err != nil {
			return err
		}
		for _, n := range allEnvNames {
			env := m[n]
			if env.Spec.Kind == v1.EnvironmentKindTypePermanent {
				names = append(names, n)
			}
		}
		o.Environment, err = kube.PickEnvironment(names, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	if o.PullRequestPollTime != "" {
		duration, err := time.ParseDuration(o.PullRequestPollTime)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.PullRequestPollTime, optionPullRequestPollTime, err)
		}
		o.PullRequestPollDuration = &duration
	}
	if o.Timeout != "" {
		duration, err := time.ParseDuration(o.Timeout)
		if err != nil {
			return fmt.Errorf("Invalid duration format %s for option --%s: %s", o.Timeout, opts.OptionTimeout, err)
		}
		o.TimeoutDuration = &duration
	}

	targetNS, env, err := o.GetTargetNamespace(o.Namespace, o.Environment)
	if err != nil {
		return err
	}

	o.Activities = jxClient.JenkinsV1().PipelineActivities(ns)

	releaseName := o.ReleaseName
	if releaseName == "" {
		releaseName = targetNS + "-" + o.Application
		o.ReleaseName = releaseName
	}

	if o.AllAutomatic {
		return o.PromoteAllAutomatic()
	}
	if env == nil {
		if o.Environment == "" {
			return util.MissingOption(opts.OptionEnvironment)
		}
		env, err := jxClient.JenkinsV1().Environments(ns).Get(o.Environment, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if env == nil {
			return fmt.Errorf("Could not find an Environment called %s", o.Environment)
		}
	}
	releaseInfo, err := o.Promote(targetNS, env, true)
	if err != nil {
		return err
	}

	o.ReleaseInfo = releaseInfo
	if !o.NoPoll {
		err = o.WaitForPromotion(targetNS, env, releaseInfo)
		if err != nil {
			return err
		}
	}
	return err
}

func (o *PromoteOptions) PromoteAllAutomatic() error {
	kubeClient, currentNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	team, _, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}
	jxClient, _, err := o.JXClient()
	if err != nil {
		return err
	}
	envs, err := jxClient.JenkinsV1().Environments(team).List(metav1.ListOptions{})
	if err != nil {
		log.Logger().Warnf("No Environments found: %s/n", err)
		return nil
	}
	environments := envs.Items
	if len(environments) == 0 {
		log.Logger().Warnf("No Environments have been created yet in team %s. Please create some via 'jx create env'", team)
		return nil
	}
	kube.SortEnvironments(environments)

	for _, env := range environments {
		kind := env.Spec.Kind
		if env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic && kind.IsPermanent() {
			ns := env.Spec.Namespace
			if ns == "" {
				return fmt.Errorf("No namespace for environment %s", env.Name)
			}
			releaseInfo, err := o.Promote(ns, &env, false)
			if err != nil {
				return err
			}
			o.ReleaseInfo = releaseInfo
			if !o.NoPoll {
				err = o.WaitForPromotion(ns, &env, releaseInfo)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (o *PromoteOptions) Promote(targetNS string, env *v1.Environment, warnIfAuto bool) (*ReleaseInfo, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	app := o.Application
	if app == "" {
		log.Logger().Warnf("No application name could be detected so cannot promote via Helm. If the detection of the helm chart name is not working consider adding it with the --%s argument on the 'jx promote' command", opts.OptionApplication)
		return nil, nil
	}
	version := o.Version
	info := util.ColorInfo
	if version == "" {
		log.Logger().Infof("Promoting latest version of app %s to namespace %s", info(app), info(targetNS))
	} else {
		log.Logger().Infof("Promoting app %s version %s to namespace %s", info(app), info(version), info(targetNS))
	}
	fullAppName := app
	if o.LocalHelmRepoName != "" {
		fullAppName = o.LocalHelmRepoName + "/" + app
	}
	releaseName := o.ReleaseName
	if releaseName == "" {
		releaseName = targetNS + "-" + app
		o.ReleaseName = releaseName
	}
	releaseInfo := &ReleaseInfo{
		ReleaseName: releaseName,
		FullAppName: fullAppName,
		Version:     version,
	}

	if warnIfAuto && env != nil && env.Spec.PromotionStrategy == v1.PromotionStrategyTypeAutomatic && !o.BatchMode {
		log.Logger().Infof("%s", util.ColorWarning(fmt.Sprintf("WARNING: The Environment %s is setup to promote automatically as part of the CI/CD Pipelines.\n", env.Name)))

		confirm := &survey.Confirm{
			Message: "Do you wish to promote anyway? :",
			Default: false,
		}
		flag := false
		err := survey.AskOne(confirm, &flag, nil, surveyOpts)
		if err != nil {
			return releaseInfo, err
		}
		if !flag {
			return releaseInfo, nil
		}
	}

	jxClient, _, err := o.JXClient()
	if err != nil {
		return releaseInfo, err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return releaseInfo, err
	}
	promoteKey := o.CreatePromoteKey(env)
	if env != nil {
		source := &env.Spec.Source
		if source.URL != "" && env.Spec.Kind.IsPermanent() {
			err := o.PromoteViaPullRequest(env, releaseInfo)
			if err == nil {
				startPromotePR := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
					kube.StartPromotionPullRequest(a, s, ps, p)
					pr := releaseInfo.PullRequestInfo
					if pr != nil && pr.PullRequest != nil && p.PullRequestURL == "" {
						p.PullRequestURL = pr.PullRequest.URL
					}
					if version != "" && a.Spec.Version == "" {
						a.Spec.Version = version
					}
					return nil
				}
				err = promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, startPromotePR)
				if err != nil {
					log.Logger().Warnf("Failed to update PipelineActivity: %s", err)
				}
				// lets sleep a little before we try poll for the PR status
				time.Sleep(waitAfterPullRequestCreated)
			}
			return releaseInfo, err
		}
	}

	err = o.verifyHelmConfigured()
	if err != nil {
		return releaseInfo, err
	}

	// lets do a helm update to ensure we can find the latest version
	if !o.NoHelmUpdate {
		log.Logger().Info("Updating the helm repositories to ensure we can find the latest versions...")
		err = o.Helm().UpdateRepo()
		if err != nil {
			return releaseInfo, err
		}
	}

	startPromote := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
		kube.StartPromotionUpdate(a, s, ps, p)
		if version != "" && a.Spec.Version == "" {
			a.Spec.Version = version
		}
		return nil
	}
	promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, startPromote)

	setValues, setStrings := o.GetEnvChartValues(o.Namespace, env)

	helmOptions := helm.InstallChartOptions{
		Chart:       fullAppName,
		ReleaseName: releaseName,
		Ns:          targetNS,
		Version:     version,
		SetValues:   setValues,
		SetStrings:  setStrings,
		NoForce:     true,
		Wait:        true,
	}

	err = o.InstallChartWithOptions(helmOptions)
	if err == nil {
		err = o.CommentOnIssues(targetNS, env, promoteKey)
		if err != nil {
			log.Logger().Warnf("Failed to comment on issues for release %s: %s", releaseName, err)
		}
		err = promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, kube.CompletePromotionUpdate)
	} else {
		err = promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, kube.FailedPromotionUpdate)
	}
	return releaseInfo, err
}

func (o *PromoteOptions) PromoteViaPullRequest(env *v1.Environment, releaseInfo *ReleaseInfo) error {
	version := o.Version
	versionName := version
	if versionName == "" {
		versionName = "latest"
	}
	app := o.Application

	details := gits.PullRequestDetails{
		BranchName: "promote-" + app + "-" + versionName,
		Title:      "chore: " + app + " to " + versionName,
		Message:    fmt.Sprintf("chore: Promote %s to version %s", app, versionName),
	}

	modifyChartFn := func(requirements *helm.Requirements, metadata *chart.Metadata, values map[string]interface{},
		templates map[string]string, dir string, details *gits.PullRequestDetails) error {
		var err error
		if version == "" {
			version, err = o.findLatestVersion(app)
			if err != nil {
				return err
			}
		}
		requirements.SetAppVersion(app, version, o.HelmRepositoryURL, o.Alias)
		return nil
	}
	gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(env.Spec.Source.URL)
	if err != nil {
		return errors.Wrapf(err, "creating git provider for %s", env.Spec.Source.URL)
	}

	envDir := ""
	if o.CloneDir != "" {
		envDir = o.CloneDir
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter:        o.Git(),
		ModifyChartFn: modifyChartFn,
		GitProvider:   gitProvider,
	}
	filter := &gits.PullRequestFilter{}
	if releaseInfo.PullRequestInfo != nil && releaseInfo.PullRequestInfo.PullRequest != nil {
		filter.Number = releaseInfo.PullRequestInfo.PullRequest.Number
	}
	info, err := options.Create(env, envDir, &details, filter, "", true)
	releaseInfo.PullRequestInfo = info
	return err
}

func (o *PromoteOptions) GetTargetNamespace(ns string, env string) (string, *v1.Environment, error) {
	kubeClient, currentNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return "", nil, err
	}
	team, _, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return "", nil, err
	}

	jxClient, _, err := o.JXClient()
	if err != nil {
		return "", nil, err
	}

	m, envNames, err := kube.GetEnvironments(jxClient, team)
	if err != nil {
		return "", nil, err
	}
	if len(envNames) == 0 {
		return "", nil, fmt.Errorf("No Environments have been created yet in team %s. Please create some via 'jx create env'", team)
	}

	var envResource *v1.Environment
	targetNS := currentNs
	if env != "" {
		envResource = m[env]
		if envResource == nil {
			return "", nil, util.InvalidOption(opts.OptionEnvironment, env, envNames)
		}
		targetNS = envResource.Spec.Namespace
		if targetNS == "" {
			return "", nil, fmt.Errorf("environment %s does not have a namespace associated with it!", env)
		}
	} else if ns != "" {
		targetNS = ns
	}

	labels := map[string]string{}
	annotations := map[string]string{}
	err = kube.EnsureNamespaceCreated(kubeClient, targetNS, labels, annotations)
	if err != nil {
		return "", nil, err
	}
	return targetNS, envResource, nil
}

func (o *PromoteOptions) WaitForPromotion(ns string, env *v1.Environment, releaseInfo *ReleaseInfo) error {
	if o.TimeoutDuration == nil {
		log.Logger().Infof("No --%s option specified on the 'jx promote' command so not waiting for the promotion to succeed", opts.OptionTimeout)
		return nil
	}
	if o.PullRequestPollDuration == nil {
		log.Logger().Infof("No --%s option specified on the 'jx promote' command so not waiting for the promotion to succeed", optionPullRequestPollTime)
		return nil
	}
	duration := *o.TimeoutDuration
	end := time.Now().Add(duration)

	jxClient, _, err := o.JXClient()
	if err != nil {
		return errors.Wrap(err, "Getting jx client")
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "Getting kube client")
	}
	pullRequestInfo := releaseInfo.PullRequestInfo
	if pullRequestInfo != nil {
		promoteKey := o.CreatePromoteKey(env)

		err := o.waitForGitOpsPullRequest(ns, env, releaseInfo, end, duration, promoteKey)
		if err != nil {
			// TODO based on if the PR completed or not fail the PR or the Promote?
			promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, kube.FailedPromotionPullRequest)
			return err
		}
	}
	return nil
}

// TODO This could do with a refactor and some tests...
func (o *PromoteOptions) waitForGitOpsPullRequest(ns string, env *v1.Environment, releaseInfo *ReleaseInfo, end time.Time, duration time.Duration, promoteKey *kube.PromoteStepActivityKey) error {
	pullRequestInfo := releaseInfo.PullRequestInfo
	logMergeFailure := false
	logNoMergeCommitSha := false
	logHasMergeSha := false
	logMergeStatusError := false
	logNoMergeStatuses := false
	urlStatusMap := map[string]string{}
	urlStatusTargetURLMap := map[string]string{}

	jxClient, _, err := o.JXClient()
	if err != nil {
		return errors.Wrap(err, "Getting jx client")
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrapf(err, "Getting kube client")
	}

	if pullRequestInfo != nil {
		for {
			pr := pullRequestInfo.PullRequest
			gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(env.Spec.Source.URL)
			if err != nil {
				return errors.Wrapf(err, "creating git provider for %s", env.Spec.Source.URL)
			}
			err = gitProvider.UpdatePullRequestStatus(pr)
			if err != nil {
				log.Logger().Warnf("Failed to query the Pull Request status for %s %s", pr.URL, err)
			} else {
				if pr.Merged != nil && *pr.Merged {
					if pr.MergeCommitSHA == nil {
						if !logNoMergeCommitSha {
							logNoMergeCommitSha = true
							log.Logger().Infof("Pull Request %s is merged but waiting for Merge SHA", util.ColorInfo(pr.URL))
						}
					} else {
						mergeSha := *pr.MergeCommitSHA
						if !logHasMergeSha {
							logHasMergeSha = true
							log.Logger().Infof("Pull Request %s is merged at sha %s", util.ColorInfo(pr.URL), util.ColorInfo(mergeSha))

							mergedPR := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
								kube.CompletePromotionPullRequest(a, s, ps, p)
								p.MergeCommitSHA = mergeSha
								return nil
							}
							promoteKey.OnPromotePullRequest(kubeClient, jxClient, o.Namespace, mergedPR)

							if o.NoWaitAfterMerge {
								log.Logger().Infof("Pull requests are merged, No wait on promotion to complete")
								return err
							}
						}

						promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, kube.StartPromotionUpdate)

						if o.NoWaitForUpdatePipeline {
							log.Logger().Info("Pull Request merged but we are not waiting for the update pipeline to complete!")
							err = o.CommentOnIssues(ns, env, promoteKey)
							if err == nil {
								err = promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, kube.CompletePromotionUpdate)
							}
							return err
						}

						statuses, err := gitProvider.ListCommitStatus(pr.Owner, pr.Repo, mergeSha)
						if err != nil {
							if !logMergeStatusError {
								logMergeStatusError = true
								log.Logger().Warnf("Failed to query merge status of repo %s/%s with merge sha %s due to: %s", pr.Owner, pr.Repo, mergeSha, err)
							}
						} else {
							if len(statuses) == 0 {
								if !logNoMergeStatuses {
									logNoMergeStatuses = true
									log.Logger().Infof("Merge commit has not yet any statuses on repo %s/%s merge sha %s", pr.Owner, pr.Repo, mergeSha)
								}
							} else {
								for _, status := range statuses {
									if status.IsFailed() {
										log.Logger().Warnf("merge status: %s URL: %s description: %s",
											status.State, status.TargetURL, status.Description)
										return fmt.Errorf("Status: %s URL: %s description: %s\n",
											status.State, status.TargetURL, status.Description)
									}
									url := status.URL
									state := status.State
									if urlStatusMap[url] == "" || urlStatusMap[url] != GitStatusSuccess {
										if urlStatusMap[url] != state {
											urlStatusMap[url] = state
											urlStatusTargetURLMap[url] = status.TargetURL
											log.Logger().Infof("merge status: %s for URL %s with target: %s description: %s",
												util.ColorInfo(state), util.ColorInfo(status.URL), util.ColorInfo(status.TargetURL), util.ColorInfo(status.Description))
										}
									}
								}
								prStatuses := []v1.GitStatus{}
								keys := util.SortedMapKeys(urlStatusMap)
								for _, url := range keys {
									state := urlStatusMap[url]
									targetURL := urlStatusTargetURLMap[url]
									if targetURL == "" {
										targetURL = url
									}
									prStatuses = append(prStatuses, v1.GitStatus{
										URL:    targetURL,
										Status: state,
									})
								}
								updateStatuses := func(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
									p.Statuses = prStatuses
									return nil
								}
								promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, updateStatuses)

								succeeded := true
								for _, v := range urlStatusMap {
									if v != GitStatusSuccess {
										succeeded = false
									}
								}
								if succeeded {
									log.Logger().Info("Merge status checks all passed so the promotion worked!")
									err = o.CommentOnIssues(ns, env, promoteKey)
									if err == nil {
										err = promoteKey.OnPromoteUpdate(kubeClient, jxClient, o.Namespace, kube.CompletePromotionUpdate)
									}
									return err
								}
							}
						}
					}
				} else {
					if pr.IsClosed() {
						log.Logger().Warnf("Pull Request %s is closed", util.ColorInfo(pr.URL))
						return fmt.Errorf("Promotion failed as Pull Request %s is closed without merging", pr.URL)
					}

					// lets try merge if the status is good
					status, err := gitProvider.PullRequestLastCommitStatus(pr)
					if err != nil {
						log.Logger().Warnf("Failed to query the Pull Request last commit status for %s ref %s %s", pr.URL, pr.LastCommitSha, err)
						//return fmt.Errorf("Failed to query the Pull Request last commit status for %s ref %s %s", pr.URL, pr.LastCommitSha, err)
					} else if status == "in-progress" {
						log.Logger().Info("The build for the Pull Request last commit is currently in progress.")
					} else {
						if status == "success" {
							if !(o.NoMergePullRequest) {
								tideMerge := false
								// Now check if tide is running or not
								commitStatues, err := gitProvider.ListCommitStatus(pr.Owner, pr.Repo, pr.LastCommitSha)
								if err != nil {
									log.Logger().Warnf("unable to get commit statuses for %s", pr.URL)
								} else {
									for _, s := range commitStatues {
										if s.State == "tide" {
											tideMerge = true
											break
										}
									}
								}
								if !tideMerge {
									err = gitProvider.MergePullRequest(pr, "jx promote automatically merged promotion PR")
									if err != nil {
										if !logMergeFailure {
											logMergeFailure = true
											log.Logger().Warnf("Failed to merge the Pull Request %s due to %s maybe I don't have karma?", pr.URL, err)
										}
									}
								}
							}
						} else if status == "error" || status == "failure" {
							return fmt.Errorf("Pull request %s last commit has status %s for ref %s", pr.URL, status, pr.LastCommitSha)
						} else {
							log.Logger().Infof("got git provider status %s from PR %s", status, pr.URL)
						}
					}
				}
				if pr.Mergeable != nil && !*pr.Mergeable {
					log.Logger().Info("Rebasing PullRequest due to conflict")

					err = o.PromoteViaPullRequest(env, releaseInfo)
					if releaseInfo.PullRequestInfo != nil {
						pullRequestInfo = releaseInfo.PullRequestInfo
					}
				}
			}
			if time.Now().After(end) {
				return fmt.Errorf("Timed out waiting for pull request %s to merge. Waited %s", pr.URL, duration.String())
			}
			time.Sleep(*o.PullRequestPollDuration)
		}
	}
	return nil
}

func (o *PromoteOptions) findLatestVersion(app string) (string, error) {
	charts, err := o.Helm().SearchCharts(app, true)
	if err != nil {
		return "", err
	}

	var maxSemVer *semver.Version
	maxString := ""
	for _, chart := range charts {
		sv, err := semver.Parse(chart.ChartVersion)
		if err != nil {
			log.Logger().Warnf("Invalid semantic version: %s %s", chart.ChartVersion, err)
			if maxString == "" || strings.Compare(chart.ChartVersion, maxString) > 0 {
				maxString = chart.ChartVersion
			}
		} else {
			if maxSemVer == nil || maxSemVer.Compare(sv) > 0 {
				maxSemVer = &sv
			}
		}
	}

	if maxSemVer != nil {
		return maxSemVer.String(), nil
	}
	if maxString == "" {
		return "", fmt.Errorf("Could not find a version of app %s in the helm repositories", app)
	}
	return maxString, nil
}

func (o *PromoteOptions) verifyHelmConfigured() error {
	helmHomeDir := filepath.Join(util.HomeDir(), ".helm")
	exists, err := util.DirExists(helmHomeDir)
	if err != nil {
		return err
	}
	if !exists {
		log.Logger().Warnf("No helm home dir at %s so lets initialise helm client", helmHomeDir)

		err = o.HelmInit("")
		if err != nil {
			return err
		}
	}

	_, ns, _ := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	// lets add the releases chart
	return o.RegisterLocalHelmRepo(o.LocalHelmRepoName, ns)
}

func (o *PromoteOptions) CreatePromoteKey(env *v1.Environment) *kube.PromoteStepActivityKey {
	pipeline := o.Pipeline
	if o.Build == "" {
		o.Build = builds.GetBuildNumber()
	}
	build := o.Build
	buildURL := os.Getenv("BUILD_URL")
	buildLogsURL := os.Getenv("BUILD_LOG_URL")
	releaseNotesURL := ""
	gitInfo := o.GitInfo
	if !o.IgnoreLocalFiles {
		var err error
		gitInfo, err = o.Git().Info("")
		releaseName := o.ReleaseName
		if o.releaseResource == nil && releaseName != "" {
			jxClient, _, err := o.JXClient()
			if err == nil && jxClient != nil {
				release, err := jxClient.JenkinsV1().Releases(env.Spec.Namespace).Get(releaseName, metav1.GetOptions{})
				if err == nil && release != nil {
					o.releaseResource = release
				}
			}
		}
		if o.releaseResource != nil {
			releaseNotesURL = o.releaseResource.Spec.ReleaseNotesURL
		}
		if err != nil {
			log.Logger().Warnf("Could not discover the Git repository info %s", err)
		} else {
			o.GitInfo = gitInfo
		}
	}
	if pipeline == "" {
		pipeline, build = o.GetPipelineName(gitInfo, pipeline, build, o.Application)
	}
	if pipeline != "" && build == "" {
		log.Logger().Warnf("No $BUILD_NUMBER environment variable found so cannot record promotion activities into the PipelineActivity resources in kubernetes")
		var err error
		build, err = o.GetLatestPipelineBuildByCRD(pipeline)
		if err != nil {
			log.Logger().Warnf("Could not discover the latest PipelineActivity build %s", err)
		}
	}
	name := pipeline
	if build != "" {
		name += "-" + build
		if (buildURL == "" || buildLogsURL == "") && !o.prow {
			jenkinsURL := o.getAndUpdateJenkinsURL()
			if jenkinsURL != "" {
				path := pipeline
				if !strings.HasPrefix(path, "job/") && !strings.HasPrefix(path, "/job/") {
					// lets split the path and prefix it with /job
					path = strings.Join(strings.Split(path, "/"), "/job/")
					path = util.UrlJoin("job", path)
				}
				path = util.UrlJoin(path, build)
				if !strings.HasSuffix(path, "/") {
					path += "/"
				}
				if buildURL == "" {
					buildURL = util.UrlJoin(jenkinsURL, path)
				}
				if buildLogsURL == "" {
					buildLogsURL = util.UrlJoin(buildURL, "console")
				}
			}
		}
	}
	name = naming.ToValidName(name)
	log.Logger().Debugf("Using pipeline: %s build: %s", util.ColorInfo(pipeline), util.ColorInfo("#"+build))
	return &kube.PromoteStepActivityKey{
		PipelineActivityKey: kube.PipelineActivityKey{
			Name:            name,
			Pipeline:        pipeline,
			Build:           build,
			BuildURL:        buildURL,
			BuildLogsURL:    buildLogsURL,
			GitInfo:         gitInfo,
			ReleaseNotesURL: releaseNotesURL,
		},
		Environment: env.Name,
	}
}

func (o *PromoteOptions) getAndUpdateJenkinsURL() string {
	if o.jenkinsURL == "" {
		o.jenkinsURL = os.Getenv("JENKINS_URL")
	}
	url, err := o.GetJenkinsURL()
	if err != nil {
		log.Logger().Warnf("Could not find Jenkins URL: %s", err)
	} else {
		o.jenkinsURL = url
	}
	return o.jenkinsURL
}

// CommentOnIssues comments on any issues for a release that the fix is available in the given environment
func (o *PromoteOptions) CommentOnIssues(targetNS string, environment *v1.Environment, promoteKey *kube.PromoteStepActivityKey) error {
	ens := environment.Spec.Namespace
	envName := environment.Spec.Label
	app := o.Application
	version := o.Version
	if ens == "" {
		log.Logger().Warnf("Environment %s has no namespace", envName)
		return nil
	}
	if app == "" {
		log.Logger().Warnf("No application name so cannot comment on issues that they are now in %s", envName)
		return nil
	}
	if version == "" {
		log.Logger().Warnf("No version name so cannot comment on issues that they are now in %s", envName)
		return nil
	}
	gitInfo := o.GitInfo
	if gitInfo == nil {
		log.Logger().Warnf("No GitInfo discovered so cannot comment on issues that they are now in %s", envName)
		return nil
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	gitKind, err := o.GitServerKind(gitInfo)
	if err != nil {
		return err
	}

	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return err
	}
	provider, err := gitInfo.PickOrCreateProvider(authConfigSvc, "user name to comment on issues", o.BatchMode, gitKind, gha, o.Git(), o.GetIOFileHandles())
	if err != nil {
		return err
	}

	releaseName := naming.ToValidNameWithDots(app + "-" + version)
	jxClient, _, err := o.JXClient()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	appNames := []string{app, o.ReleaseName, ens + "-" + app}
	url := ""
	for _, n := range appNames {
		url, err = services.FindServiceURL(kubeClient, ens, naming.ToValidName(n))
		if url != "" {
			break
		}
	}
	if url == "" {
		log.Logger().Warnf("Could not find the service URL in namespace %s for names %s", ens, strings.Join(appNames, ", "))
	}
	available := ""
	if url != "" {
		available = fmt.Sprintf(" and available [here](%s)", url)
	}

	if available == "" {
		ing, err := kubeClient.ExtensionsV1beta1().Ingresses(ens).Get(app, metav1.GetOptions{})
		if err != nil || ing == nil && o.ReleaseName != "" && o.ReleaseName != app {
			ing, err = kubeClient.ExtensionsV1beta1().Ingresses(ens).Get(o.ReleaseName, metav1.GetOptions{})
		}
		if ing != nil {
			if len(ing.Spec.Rules) > 0 {
				hostname := ing.Spec.Rules[0].Host
				if hostname != "" {
					available = fmt.Sprintf(" and available at %s", hostname)
					url = hostname
				}
			}
		}
	}

	// lets try update the PipelineActivity
	if url != "" && promoteKey.ApplicationURL == "" {
		promoteKey.ApplicationURL = url
		log.Logger().Debugf("Application is available at: %s", util.ColorInfo(url))
	}

	release, err := jxClient.JenkinsV1().Releases(ens).Get(releaseName, metav1.GetOptions{})
	if err == nil && release != nil {
		o.releaseResource = release
		issues := release.Spec.Issues

		versionMessage := version
		if release.Spec.ReleaseNotesURL != "" {
			versionMessage = "[" + version + "](" + release.Spec.ReleaseNotesURL + ")"
		}
		for _, issue := range issues {
			if issue.IsClosed() {
				log.Logger().Infof("Commenting that issue %s is now in %s", util.ColorInfo(issue.URL), util.ColorInfo(envName))

				comment := fmt.Sprintf(":white_check_mark: the fix for this issue is now deployed to **%s** in version %s %s", envName, versionMessage, available)
				id := issue.ID
				if id != "" {
					number, err := strconv.Atoi(id)
					if err != nil {
						log.Logger().Warnf("Could not parse issue id %s for URL %s", id, issue.URL)
					} else {
						if number > 0 {
							err = provider.CreateIssueComment(gitInfo.Organisation, gitInfo.Name, number, comment)
							if err != nil {
								log.Logger().Warnf("Failed to add comment to issue %s: %s", issue.URL, err)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (o *PromoteOptions) SearchForChart(filter string) (string, error) {
	answer := ""
	charts, err := o.Helm().SearchCharts(filter, false)
	if err != nil {
		return answer, err
	}
	if len(charts) == 0 {
		return answer, fmt.Errorf("No charts available for search filter: %s", filter)
	}
	m := map[string]*helm.ChartSummary{}
	names := []string{}
	for i, chart := range charts {
		text := chart.Name
		if chart.Description != "" {
			text = fmt.Sprintf("%-36s: %s", chart.Name, chart.Description)
		}
		names = append(names, text)
		m[text] = &charts[i]
	}
	name, err := util.PickName(names, "Pick chart to promote: ", "", o.GetIOFileHandles())
	if err != nil {
		return answer, err
	}
	chart := m[name]
	chartName := chart.Name
	// TODO now we split the chart into name and repo
	parts := strings.Split(chartName, "/")
	if len(parts) != 2 {
		return answer, fmt.Errorf("Invalid chart name '%s' was expecting single / character separating repo name and chart name", chartName)
	}
	repoName := parts[0]
	appName := parts[1]

	repos, err := o.Helm().ListRepos()
	if err != nil {
		return answer, err
	}

	repoUrl := repos[repoName]
	if repoUrl == "" {
		return answer, fmt.Errorf("Failed to find helm chart repo URL for '%s' when possible values are %s", repoName, util.SortedMapKeys(repos))

	}
	o.Version = chart.ChartVersion
	o.HelmRepositoryURL = repoUrl
	return appName, nil
}

func (o *PromoteOptions) GetEnvChartValues(targetNS string, env *v1.Environment) ([]string, []string) {
	kind := string(env.Spec.Kind)
	values := []string{
		fmt.Sprintf("tags.jx-ns-%s=true", targetNS),
		fmt.Sprintf("global.jxNs%s=true", util.ToCamelCase(targetNS)),
		fmt.Sprintf("tags.jx-%s=true", strings.ToLower(kind)),
		fmt.Sprintf("tags.jx-env-%s=true", env.ObjectMeta.Name),
		fmt.Sprintf("global.jx%s=true", kind),
		fmt.Sprintf("global.jxEnv%s=true", util.ToCamelCase(env.ObjectMeta.Name)),
	}
	valueString := []string{
		fmt.Sprintf("global.jxNs=%s", targetNS),
		fmt.Sprintf("global.jxTypeEnv=%s", strings.ToLower(kind)),
		fmt.Sprintf("global.jxEnv=%s", env.ObjectMeta.Name),
	}
	if env.Spec.Kind == v1.EnvironmentKindTypePreview {
		valueString = append(valueString,
			fmt.Sprintf("global.jxPreviewApp=%s", env.Spec.PreviewGitSpec.ApplicationName),
			fmt.Sprintf("global.jxPreviewPr=%s", env.Spec.PreviewGitSpec.Name),
		)
	}
	return values, valueString
}
