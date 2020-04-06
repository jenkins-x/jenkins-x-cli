package pipelinescheduler

import (
	"strings"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"k8s.io/test-infra/prow/plugins"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/test-infra/prow/config"
)

const (
	// DefaultAgent is the default agent vaule
	DefaultAgent = "tekton"
	// DefaultMergeType is the default merge type
	DefaultMergeType = "merge"
)

// BuildSchedulers turns prow config in to schedulers
func BuildSchedulers(prowConfig *config.Config, pluginConfig *plugins.Configuration) ([]*jenkinsv1.SourceRepositoryGroup, []*jenkinsv1.SourceRepository, map[string]*jenkinsv1.SourceRepository, map[string]*jenkinsv1.Scheduler, error) {
	log.Logger().Info("Building scheduler resources from prow config")
	sourceRepos := make(map[string]*jenkinsv1.SourceRepository, 0)
	if prowConfig.Presubmits != nil {
		for repo := range prowConfig.Presubmits {
			orgRepo := strings.Split(repo, "/")
			sourceRepos[repo] = buildSourceRepo(orgRepo[0], orgRepo[1])
		}
	}
	if prowConfig.Postsubmits != nil {
		for repo := range prowConfig.Postsubmits {
			orgRepo := strings.Split(repo, "/")
			if _, ok := sourceRepos[repo]; !ok {
				sourceRepos[repo] = buildSourceRepo(orgRepo[0], orgRepo[1])
			}
		}
	}
	schedulers := make(map[string]*jenkinsv1.Scheduler, 0)
	sourceRepoSlice := make([]*jenkinsv1.SourceRepository, 0, len(sourceRepos))
	for sourceRepoName, sourceRepo := range sourceRepos {
		scheduler, err := buildScheduler(sourceRepoName, prowConfig, pluginConfig)
		if err == nil {
			sourceRepo.Spec.Scheduler = jenkinsv1.ResourceReference{
				Name: scheduler.Name,
				Kind: "Scheduler",
			}
			schedulers[scheduler.Name] = scheduler
			sourceRepoSlice = append(sourceRepoSlice, sourceRepo)
		}
	}
	defaultScheduler := buildDefaultScheduler(prowConfig)
	if defaultScheduler != nil {
		schedulers[defaultScheduler.Name] = defaultScheduler
	}
	// TODO Dedupe in to source repo groups
	return nil, sourceRepoSlice, sourceRepos, schedulers, nil
}

func buildSourceRepo(org string, repo string) *jenkinsv1.SourceRepository {
	return &jenkinsv1.SourceRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SourceRepository",
			APIVersion: jenkinsio.GroupName + "/" + jenkinsio.Version,
		},
		Spec: jenkinsv1.SourceRepositorySpec{
			Org:          org,
			Repo:         repo,
			Provider:     gits.GitHubURL,
			ProviderName: gits.KindGitHub,
		},
	}
}

func buildScheduler(repo string, prowConfig *config.Config, pluginConfig *plugins.Configuration) (*jenkinsv1.Scheduler, error) {
	scheduler := &jenkinsv1.Scheduler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scheduler",
			APIVersion: jenkinsio.GroupName + "/" + jenkinsio.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.Replace(repo, "/", "-", -1) + "-scheduler",
		},
		Spec: jenkinsv1.SchedulerSpec{
			ScehdulerAgent:  buildSchedulerAgent(),
			Policy:          buildSchedulerGlobalProtectionPolicy(prowConfig),
			Presubmits:      buildSchedulerPresubmits(repo, prowConfig),
			Postsubmits:     buildSchedulerPostsubmits(repo, prowConfig),
			Trigger:         buildSchedulerTrigger(repo, pluginConfig),
			Approve:         buildSchedulerApprove(repo, pluginConfig),
			LGTM:            buildSchedulerLGTM(repo, pluginConfig),
			ExternalPlugins: buildSchedulerExternalPlugins(repo, pluginConfig),
			Merger:          buildSchedulerMerger(repo, prowConfig),
			Plugins:         buildSchedulerPlugins(repo, pluginConfig),
			ConfigUpdater:   buildSchedulerConfigUpdater(repo, pluginConfig),
			Welcome:         buildSchedulerWelcome(pluginConfig),
		},
	}
	return scheduler, nil
}

func buildDefaultScheduler(prowConfig *config.Config) *jenkinsv1.Scheduler {
	scheduler := &jenkinsv1.Scheduler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Scheduler",
			APIVersion: jenkinsio.GroupName + "/" + jenkinsio.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "default-scheduler",
		},
		Spec: jenkinsv1.SchedulerSpec{
			Periodics:   buildSchedulerPeriodics(prowConfig),
			Attachments: buildSchedulerAttachments(prowConfig),
		},
	}
	return scheduler
}

func buildSchedulerAttachments(configuration *config.Config) []*jenkinsv1.Attachment {
	attachments := make([]*jenkinsv1.Attachment, 0)
	jobURLPrefix := configuration.Plank.JobURLPrefix
	if jobURLPrefix != "" {
		attachments = buildSchedulerAttachment("jobURLPrefix", jobURLPrefix, attachments)
	}
	jobURLTemplate := configuration.Plank.JobURLTemplateString
	if jobURLTemplate != "" {
		attachments = buildSchedulerAttachment("jobURLTemplate", jobURLTemplate, attachments)
	}
	reportTemplate := configuration.Plank.ReportTemplateString
	if reportTemplate != "" {
		attachments = buildSchedulerAttachment("reportTemplate", reportTemplate, attachments)
	}
	if len(attachments) > 0 {
		return attachments
	}
	return nil
}

func buildSchedulerAttachment(name string, value string, attachments []*jenkinsv1.Attachment) []*jenkinsv1.Attachment {
	return append(attachments, &jenkinsv1.Attachment{
		Name: name,
		URLs: []string{value},
	})
}

func buildSchedulerPeriodics(configuration *config.Config) *jenkinsv1.Periodics {
	periodics := configuration.Periodics
	if periodics != nil && len(periodics) > 0 {

		schedulerPeriodics := &jenkinsv1.Periodics{
			Items: make([]*jenkinsv1.Periodic, 0),
		}
		for i := range periodics {
			periodic := periodics[i]
			schedulerPeriodic := &jenkinsv1.Periodic{
				JobBase:  buildSchedulerJobBase(&periodic.JobBase),
				Interval: &periodic.Interval,
				Cron:     &periodic.Cron,
				Tags: &jenkinsv1.ReplaceableSliceOfStrings{
					Items: make([]string, 0),
				},
			}
			for _, tag := range periodic.Tags {
				schedulerPeriodic.Tags.Items = append(schedulerPeriodic.Tags.Items, tag)
			}
			schedulerPeriodics.Items = append(schedulerPeriodics.Items, schedulerPeriodic)

		}
		return schedulerPeriodics
	}
	return nil
}

func buildSchedulerWelcome(configuration *plugins.Configuration) []*jenkinsv1.Welcome {
	welcomes := configuration.Welcome
	if welcomes != nil && len(welcomes) > 0 {
		schedulerWelcomes := make([]*jenkinsv1.Welcome, 0)
		for _, welcome := range welcomes {
			schedulerWelcomes = append(schedulerWelcomes, &jenkinsv1.Welcome{MessageTemplate: &welcome.MessageTemplate})

		}
		return schedulerWelcomes
	}
	return nil
}

func buildSchedulerConfigUpdater(repo string, pluginConfig *plugins.Configuration) *jenkinsv1.ConfigUpdater {
	if plugins, ok := pluginConfig.Plugins[repo]; !ok {
		for _, plugin := range plugins {
			if plugin == "config-updater" {
				configMapSpec := make(map[string]jenkinsv1.ConfigMapSpec)
				for location, conf := range pluginConfig.ConfigUpdater.Maps {
					spec := jenkinsv1.ConfigMapSpec{
						Name:                 conf.Name,
						Namespace:            conf.Namespace,
						Key:                  conf.Key,
						AdditionalNamespaces: conf.AdditionalNamespaces,
						Namespaces:           conf.Namespaces,
					}
					configMapSpec[location] = spec
				}
				return &jenkinsv1.ConfigUpdater{
					PluginFile: pluginConfig.ConfigUpdater.PluginFile,
					ConfigFile: pluginConfig.ConfigUpdater.ConfigFile,
					Map:        configMapSpec,
				}
			}
		}
	}
	return nil
}

func buildSchedulerPlugins(repo string, pluginConfig *plugins.Configuration) *jenkinsv1.ReplaceableSliceOfStrings {
	if plugins, ok := pluginConfig.Plugins[repo]; ok {
		pluginList := &jenkinsv1.ReplaceableSliceOfStrings{
			Items: make([]string, 0),
		}
		for _, plugin := range plugins {
			pluginList.Items = append(pluginList.Items, plugin)
		}
		if len(pluginList.Items) > 0 {
			return pluginList
		}

	}
	return nil
}

func buildSchedulerMerger(repo string, prowConfig *config.Config) *jenkinsv1.Merger {
	tide := prowConfig.Tide
	merger := &jenkinsv1.Merger{
		SyncPeriod:         &tide.SyncPeriod,
		StatusUpdatePeriod: &tide.StatusUpdatePeriod,
		TargetURL:          &tide.TargetURL,
		PRStatusBaseURL:    &tide.PRStatusBaseURL,
		BlockerLabel:       &tide.BlockerLabel,
		SquashLabel:        &tide.BlockerLabel,
		MaxGoroutines:      &tide.MaxGoroutines,
		ContextPolicy:      buildSchedulerContextPolicy(repo, &tide),
	}
	if mergeType, ok := tide.MergeType[repo]; ok {
		mergeTypeStr := string(mergeType)
		merger.MergeType = &mergeTypeStr

	} else {
		defaultMergeType := string(DefaultMergeType)
		merger.MergeType = &defaultMergeType
	}
	return merger
}

func buildSchedulerContextPolicy(orgRepo string, tideConfig *config.Tide) *jenkinsv1.ContextPolicy {
	orgRepoArr := strings.Split(orgRepo, "/")
	orgContextPolicy, orgContextPolicyFound := tideConfig.ContextOptions.Orgs[orgRepoArr[0]]
	if orgContextPolicyFound {
		repoContextPolicy, repoContextPolicyFound := orgContextPolicy.Repos[orgRepoArr[1]]
		if repoContextPolicyFound {
			repoPolicy := jenkinsv1.ContextPolicy{}
			repoPolicy.OptionalContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: repoContextPolicy.OptionalContexts}
			repoPolicy.FromBranchProtection = repoContextPolicy.FromBranchProtection
			repoPolicy.RequiredContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: repoContextPolicy.RequiredContexts}
			repoPolicy.RequiredIfPresentContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: repoContextPolicy.RequiredIfPresentContexts}
			repoPolicy.SkipUnknownContexts = repoContextPolicy.SkipUnknownContexts
			return &repoPolicy
		}
		orgPolicy := jenkinsv1.ContextPolicy{}
		orgPolicy.OptionalContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: orgContextPolicy.OptionalContexts}
		orgPolicy.FromBranchProtection = orgContextPolicy.FromBranchProtection
		orgPolicy.RequiredContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: orgContextPolicy.RequiredContexts}
		orgPolicy.RequiredIfPresentContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: orgContextPolicy.RequiredIfPresentContexts}
		orgPolicy.SkipUnknownContexts = orgContextPolicy.SkipUnknownContexts
		return &orgPolicy

	}
	contextPolicy := jenkinsv1.ContextPolicy{}
	globalContextPolicy := tideConfig.ContextOptions
	contextPolicy.OptionalContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: globalContextPolicy.OptionalContexts}
	contextPolicy.RequiredIfPresentContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: globalContextPolicy.RequiredIfPresentContexts}
	contextPolicy.RequiredContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: globalContextPolicy.RequiredContexts}
	contextPolicy.FromBranchProtection = globalContextPolicy.FromBranchProtection
	contextPolicy.SkipUnknownContexts = globalContextPolicy.SkipUnknownContexts
	return &contextPolicy
}

func buildSchedulerRepoContextPolicy(orgRepo string, tideConfig *config.Tide) *jenkinsv1.RepoContextPolicy {
	orgRepoArr := strings.Split(orgRepo, "/")
	orgContextPolicy, orgContextPolicyFound := tideConfig.ContextOptions.Orgs[orgRepoArr[0]]
	if orgContextPolicyFound {
		repoContextPolicy, repoContextPolicyFound := orgContextPolicy.Repos[orgRepoArr[1]]
		if repoContextPolicyFound && repoContextPolicy.OptionalContexts != nil {
			repoPolicy := jenkinsv1.RepoContextPolicy{
				ContextPolicy: &jenkinsv1.ContextPolicy{},
			}
			repoPolicy.OptionalContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: repoContextPolicy.OptionalContexts}
			repoPolicy.FromBranchProtection = repoContextPolicy.FromBranchProtection
			repoPolicy.RequiredContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: repoContextPolicy.RequiredContexts}
			repoPolicy.RequiredIfPresentContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: repoContextPolicy.RequiredIfPresentContexts}
			repoPolicy.SkipUnknownContexts = repoContextPolicy.SkipUnknownContexts
			branchPolicies := make(map[string]*jenkinsv1.ContextPolicy)
			for branch, policy := range repoContextPolicy.Branches {
				branchPolicy := jenkinsv1.ContextPolicy{}
				branchPolicy.OptionalContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: policy.OptionalContexts}
				branchPolicy.FromBranchProtection = policy.FromBranchProtection
				branchPolicy.RequiredContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: policy.RequiredContexts}
				branchPolicy.RequiredIfPresentContexts = &jenkinsv1.ReplaceableSliceOfStrings{Items: policy.RequiredIfPresentContexts}
				branchPolicy.SkipUnknownContexts = policy.SkipUnknownContexts
				branchPolicies[branch] = &branchPolicy
			}
			repoPolicy.Branches = &jenkinsv1.ReplaceableMapOfStringContextPolicy{Items: branchPolicies}
			return &repoPolicy
		}
	}
	return nil
}

func buildSchedulerQuery(orgRepo string, tideQueries *config.TideQueries) []*jenkinsv1.Query {
	queries := make([]*jenkinsv1.Query, 0)
	if orgRepo != "" && strings.Contains(orgRepo, "/") {
		for _, tideQuery := range *tideQueries {
			if util.Contains(tideQuery.Repos, orgRepo) {
				query := &jenkinsv1.Query{
					ExcludedBranches: &jenkinsv1.ReplaceableSliceOfStrings{
						Items: tideQuery.ExcludedBranches,
					},
					IncludedBranches: &jenkinsv1.ReplaceableSliceOfStrings{
						Items: tideQuery.IncludedBranches,
					},
					Labels: &jenkinsv1.ReplaceableSliceOfStrings{
						Items: tideQuery.Labels,
					},
					MissingLabels: &jenkinsv1.ReplaceableSliceOfStrings{
						Items: tideQuery.MissingLabels,
					},
					Milestone:              &tideQuery.Milestone,
					ReviewApprovedRequired: &tideQuery.ReviewApprovedRequired,
				}
				queries = append(queries, query)
			}
		}
	}
	if len(queries) > 0 {
		return queries
	}
	return nil
}

func buildSchedulerExternalPlugins(repo string, pluginConfig *plugins.Configuration) *jenkinsv1.ReplaceableSliceOfExternalPlugins {
	pluginList := &jenkinsv1.ReplaceableSliceOfExternalPlugins{
		Items: nil,
	}
	if plugins, ok := pluginConfig.ExternalPlugins[repo]; ok {
		if plugins != nil {
			for _, plugin := range plugins {
				if pluginList.Items == nil {
					pluginList.Items = make([]*jenkinsv1.ExternalPlugin, 0)
				}
				events := &jenkinsv1.ReplaceableSliceOfStrings{
					Items: plugin.Events,
				}
				externalPlugin := &jenkinsv1.ExternalPlugin{
					Name:     &plugin.Name,
					Endpoint: &plugin.Endpoint,
					Events:   events,
				}
				pluginList.Items = append(pluginList.Items, externalPlugin)
			}
			return pluginList
		}

	}
	return nil
}

func buildSchedulerLGTM(repo string, pluginConfig *plugins.Configuration) *jenkinsv1.Lgtm {
	lgtms := pluginConfig.Lgtm
	for _, lgtm := range lgtms {
		for _, lgtmRepo := range lgtm.Repos {
			if repo == lgtmRepo {
				return &jenkinsv1.Lgtm{
					ReviewActsAsLgtm: &lgtm.ReviewActsAsLgtm,
					StoreTreeHash:    &lgtm.StoreTreeHash,
					StickyLgtmTeam:   &lgtm.StickyLgtmTeam,
				}
			}
		}
	}
	return nil
}

func buildSchedulerApprove(repo string, pluginConfig *plugins.Configuration) *jenkinsv1.Approve {
	orgRepo := strings.Split(repo, "/")
	approves := pluginConfig.Approve
	for _, approve := range approves {
		for _, approveRepo := range approve.Repos {
			if repo == approveRepo || orgRepo[0] == approveRepo {
				return &jenkinsv1.Approve{
					IssueRequired:       &approve.IssueRequired,
					RequireSelfApproval: approve.RequireSelfApproval,
					LgtmActsAsApprove:   &approve.LgtmActsAsApprove,
					IgnoreReviewState:   approve.IgnoreReviewState,
				}
			}
		}
	}
	return nil
}

func buildSchedulerTrigger(repo string, pluginConfig *plugins.Configuration) *jenkinsv1.Trigger {
	triggers := pluginConfig.Triggers
	for _, trigger := range triggers {
		for _, triggerRepo := range trigger.Repos {
			if repo == triggerRepo {
				return &jenkinsv1.Trigger{
					TrustedOrg:     &trigger.TrustedOrg,
					JoinOrgURL:     &trigger.JoinOrgURL,
					OnlyOrgMembers: &trigger.OnlyOrgMembers,
					IgnoreOkToTest: &trigger.IgnoreOkToTest,
				}
			}
		}
	}
	return nil
}

func buildSchedulerGlobalProtectionPolicy(prowConfig *config.Config) *jenkinsv1.GlobalProtectionPolicy {
	return &jenkinsv1.GlobalProtectionPolicy{
		ProtectTested: &prowConfig.BranchProtection.ProtectTested,
		ProtectionPolicy: &jenkinsv1.ProtectionPolicy{
			Admins:                     prowConfig.BranchProtection.Admins,
			Protect:                    prowConfig.BranchProtection.Protect,
			RequiredPullRequestReviews: buildSchedulerRequiredPullRequestReviews(prowConfig.BranchProtection.RequiredPullRequestReviews),
			RequiredStatusChecks:       buildSchedulerRequiredStatusChecks(prowConfig.BranchProtection.RequiredStatusChecks),
			Restrictions:               buildSchedulerRestrictions(prowConfig.BranchProtection.Restrictions),
		},
	}
}

func buildSchedulerProtectionPolicies(repo string, prowConfig *config.Config) *jenkinsv1.ProtectionPolicies {
	orgRepo := strings.Split(repo, "/")
	orgBranchProtection, err := prowConfig.BranchProtection.GetOrg(orgRepo[0])
	if err == nil {
		repoBranchProtection, err := orgBranchProtection.GetRepo(orgRepo[1])
		if err == nil {
			var protectionPolicies map[string]*jenkinsv1.ProtectionPolicy
			for branchName, branch := range repoBranchProtection.Branches {
				if protectionPolicies == nil {
					protectionPolicies = make(map[string]*jenkinsv1.ProtectionPolicy)
				}
				protectionPolicies[branchName] = &jenkinsv1.ProtectionPolicy{
					Admins:                     branch.Admins,
					Protect:                    branch.Protect,
					RequiredPullRequestReviews: buildSchedulerRequiredPullRequestReviews(branch.RequiredPullRequestReviews),
					RequiredStatusChecks:       buildSchedulerRequiredStatusChecks(branch.RequiredStatusChecks),
					Restrictions:               buildSchedulerRestrictions(branch.Restrictions),
				}
			}
			var repoPolicy *jenkinsv1.ProtectionPolicy
			requiredPullRequestReviews := buildSchedulerRequiredPullRequestReviews(repoBranchProtection.RequiredPullRequestReviews)
			requiredStatusChecks := buildSchedulerRequiredStatusChecks(repoBranchProtection.RequiredStatusChecks)
			restrictions := buildSchedulerRestrictions(repoBranchProtection.Restrictions)
			if repoBranchProtection.Admins != nil || repoBranchProtection.Protect != nil || requiredPullRequestReviews != nil || requiredStatusChecks != nil || restrictions != nil {
				repoPolicy = &jenkinsv1.ProtectionPolicy{
					Admins:                     repoBranchProtection.Admins,
					Protect:                    repoBranchProtection.Protect,
					RequiredPullRequestReviews: requiredPullRequestReviews,
					RequiredStatusChecks:       requiredStatusChecks,
					Restrictions:               buildSchedulerRestrictions(repoBranchProtection.Restrictions),
				}
			}
			return &jenkinsv1.ProtectionPolicies{
				ProtectionPolicy: repoPolicy,
				Items:            protectionPolicies,
			}
		}
	}
	return nil
}

func buildSchedulerRequiredPullRequestReviews(requiredPullRequestReviews *config.ReviewPolicy) *jenkinsv1.ReviewPolicy {
	if requiredPullRequestReviews != nil {
		return &jenkinsv1.ReviewPolicy{
			DismissalRestrictions: buildSchedulerRestrictions(requiredPullRequestReviews.DismissalRestrictions),
			DismissStale:          requiredPullRequestReviews.DismissStale,
			RequireOwners:         requiredPullRequestReviews.RequireOwners,
			Approvals:             requiredPullRequestReviews.Approvals,
		}
	}
	return nil
}

func buildSchedulerRequiredStatusChecks(requiredStatusChecks *config.ContextPolicy) *jenkinsv1.BranchProtectionContextPolicy {
	if requiredStatusChecks != nil {
		return &jenkinsv1.BranchProtectionContextPolicy{
			Contexts: &jenkinsv1.ReplaceableSliceOfStrings{
				Items: requiredStatusChecks.Contexts,
			},
			Strict: requiredStatusChecks.Strict,
		}
	}
	return nil
}

func buildSchedulerRestrictions(restrictions *config.Restrictions) *jenkinsv1.Restrictions {
	if restrictions != nil {
		return &jenkinsv1.Restrictions{
			Users: &jenkinsv1.ReplaceableSliceOfStrings{
				Items: restrictions.Users,
			},
			Teams: &jenkinsv1.ReplaceableSliceOfStrings{
				Items: restrictions.Teams,
			},
		}
	}
	return nil
}

func buildSchedulerAgent() *jenkinsv1.SchedulerAgent {
	defaultAgent := string(DefaultAgent)
	agent := &jenkinsv1.SchedulerAgent{
		Agent: &defaultAgent,
	}
	return agent
}

func buildSchedulerPostsubmits(repo string, prowConfig *config.Config) *jenkinsv1.Postsubmits {
	schedulerPostsubmits := &jenkinsv1.Postsubmits{}
	for postSubmitIndex := range prowConfig.Postsubmits[repo] {
		postsubmit := prowConfig.Postsubmits[repo][postSubmitIndex]
		skipBranches := &jenkinsv1.ReplaceableSliceOfStrings{
			Items: postsubmit.SkipBranches,
		}
		branches := &jenkinsv1.ReplaceableSliceOfStrings{
			Items: postsubmit.Branches,
		}
		schedulerPostsubmit := &jenkinsv1.Postsubmit{
			JobBase: buildSchedulerJobBase(&postsubmit.JobBase),
			Brancher: &jenkinsv1.Brancher{
				SkipBranches: skipBranches,
				Branches:     branches,
			},
			RegexpChangeMatcher: &jenkinsv1.RegexpChangeMatcher{
				RunIfChanged: &postsubmit.RunIfChanged,
			},
			Context: &postsubmit.Context,
			Report:  &postsubmit.Report,
		}
		schedulerPostsubmits.Items = append(schedulerPostsubmits.Items, schedulerPostsubmit)
	}
	return schedulerPostsubmits
}

func buildSchedulerJobBase(jobBase *config.JobBase) *jenkinsv1.JobBase {
	labels := &jenkinsv1.ReplaceableMapOfStringString{
		Items: jobBase.Labels,
	}
	return &jenkinsv1.JobBase{
		Name:           &jobBase.Name,
		Labels:         labels,
		MaxConcurrency: &jobBase.MaxConcurrency,
		Agent:          &jobBase.Agent,
		Cluster:        &jobBase.Cluster,
		Namespace:      jobBase.Namespace,
		Spec:           jobBase.Spec,
	}
}

func buildSchedulerPresubmits(repo string, prowConfig *config.Config) *jenkinsv1.Presubmits {
	schedulerPresubmits := &jenkinsv1.Presubmits{}
	presubmits := prowConfig.Presubmits[repo]
	for presubmitIndex := range presubmits {
		presubmit := presubmits[presubmitIndex]
		skipBranches := &jenkinsv1.ReplaceableSliceOfStrings{
			Items: presubmit.SkipBranches,
		}
		branches := &jenkinsv1.ReplaceableSliceOfStrings{
			Items: presubmit.Branches,
		}
		report := !presubmit.SkipReport
		mergeType := prowConfig.Tide.MergeType[repo]
		mt := string(mergeType)
		schedulerPresubmit := &jenkinsv1.Presubmit{
			JobBase: buildSchedulerJobBase(&presubmit.JobBase),
			Brancher: &jenkinsv1.Brancher{
				SkipBranches: skipBranches,
				Branches:     branches,
			},
			RegexpChangeMatcher: &jenkinsv1.RegexpChangeMatcher{
				RunIfChanged: &presubmit.RunIfChanged,
			},
			AlwaysRun:     &presubmit.AlwaysRun,
			Context:       &presubmit.Context,
			Optional:      &presubmit.Optional,
			Report:        &report,
			Trigger:       &presubmit.Trigger,
			RerunCommand:  &presubmit.RerunCommand,
			MergeType:     &mt,
			Queries:       buildSchedulerQuery(repo, &prowConfig.Tide.Queries),
			Policy:        buildSchedulerProtectionPolicies(repo, prowConfig),
			ContextPolicy: buildSchedulerRepoContextPolicy(repo, &prowConfig.Tide),
		}
		schedulerPresubmits.Items = append(schedulerPresubmits.Items, schedulerPresubmit)
	}
	return schedulerPresubmits
}
