package pipelinescheduler

import (
	"fmt"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/pkg/errors"
	"github.com/rollout/rox-go/core/utils"
	"k8s.io/test-infra/prow/config"
	"k8s.io/test-infra/prow/github"
	"k8s.io/test-infra/prow/plugins"
)

// BuildProwConfig takes a list of schedulers and creates a Prow Config from it
func BuildProwConfig(schedulers []*SchedulerLeaf) (*config.Config, *plugins.Configuration,
	error) {
	configResult := config.Config{
		JobConfig:  config.JobConfig{},
		ProwConfig: config.ProwConfig{},
	}
	pluginsResult := plugins.Configuration{}
	for _, scheduler := range schedulers {
		err := buildJobConfig(&configResult.JobConfig, &configResult.ProwConfig, scheduler.SchedulerSpec, scheduler.Org, scheduler.Repo)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "building JobConfig for %v", scheduler)
		}
		err = buildProwConfig(&configResult.ProwConfig, scheduler.SchedulerSpec, scheduler.Org, scheduler.Repo)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "building ProwConfig for %v", scheduler)
		}
		err = buildPlugins(&pluginsResult, scheduler.SchedulerSpec, scheduler.Org, scheduler.Repo)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "building Plugins for %v", scheduler)
		}
	}
	return &configResult, &pluginsResult, nil
}

func buildPlugins(answer *plugins.Configuration, scheduler *jenkinsv1.SchedulerSpec, orgName string,
	repoName string) error {
	if scheduler.Plugins != nil {
		if answer.Plugins == nil {
			answer.Plugins = make(map[string][]string)
		}
		answer.Plugins[orgSlashRepo(orgName, repoName)] = scheduler.Plugins.Items
	}
	if answer.ExternalPlugins == nil {
		answer.ExternalPlugins = make(map[string][]plugins.ExternalPlugin)
	}

	if scheduler.ExternalPlugins != nil {
		var res []plugins.ExternalPlugin
		for _, src := range scheduler.ExternalPlugins.Items {
			if res == nil {
				res = make([]plugins.ExternalPlugin, 0)
			}
			externalPlugin := plugins.ExternalPlugin{}
			err := buildExternalPlugin(&externalPlugin, src)
			if err != nil {
				return errors.Wrapf(err, "building ExternalPlugin for %v", src)
			}
			res = append(res, externalPlugin)
		}
		answer.ExternalPlugins[orgSlashRepo(orgName, repoName)] = res
	}
	if answer.Approve == nil {
		answer.Approve = make([]plugins.Approve, 0)
	}
	if scheduler.Approve != nil {
		approve := plugins.Approve{}
		err := buildApprove(&approve, scheduler.Approve, orgName, repoName)
		if err != nil {
			return errors.Wrapf(err, "building Approve for %v", scheduler.Approve)
		}
		answer.Approve = append(answer.Approve, approve)
	}
	if scheduler.Welcome != nil {
		if answer.Welcome == nil {
			answer.Welcome = make([]plugins.Welcome, 0)
		}
		for _, welcome := range scheduler.Welcome {
			welcomeExists := false
			// TODO support Welcome.Repos
			for _, existingWelcome := range answer.Welcome {
				if *welcome.MessageTemplate == existingWelcome.MessageTemplate {
					welcomeExists = true
					break
				}
			}
			if !welcomeExists {
				answer.Welcome = append(answer.Welcome, plugins.Welcome{MessageTemplate: *welcome.MessageTemplate})
			}
		}
	}
	if scheduler.ConfigUpdater != nil {
		if answer.ConfigUpdater.Maps == nil {
			answer.ConfigUpdater.Maps = make(map[string]plugins.ConfigMapSpec)
			for key, value := range scheduler.ConfigUpdater.Map {
				answer.ConfigUpdater.Maps[key] = plugins.ConfigMapSpec{
					Name:                 value.Name,
					Namespace:            value.Namespace,
					Key:                  value.Key,
					AdditionalNamespaces: value.AdditionalNamespaces,
				}
			}

		}
		if answer.ConfigUpdater.ConfigFile == "" {
			answer.ConfigUpdater.ConfigFile = scheduler.ConfigUpdater.ConfigFile
		}
		if answer.ConfigUpdater.PluginFile == "" {
			answer.ConfigUpdater.PluginFile = scheduler.ConfigUpdater.PluginFile
		}
	}
	if answer.Lgtm == nil {
		answer.Lgtm = make([]plugins.Lgtm, 0)
	}
	if scheduler.LGTM != nil {
		lgtm := plugins.Lgtm{}
		err := buildLgtm(&lgtm, scheduler.LGTM, orgName, repoName)
		if err != nil {
			return errors.Wrapf(err, "building LGTM for %v", scheduler.LGTM)
		}
		answer.Lgtm = append(answer.Lgtm, lgtm)
	}
	if answer.Triggers == nil {
		answer.Triggers = make([]plugins.Trigger, 0)
	}
	if scheduler.Trigger != nil {
		trigger := plugins.Trigger{}
		err := buildTrigger(&trigger, scheduler.Trigger, orgName, repoName)
		if err != nil {
			return errors.Wrapf(err, "building Triggers for %v", scheduler.Trigger)
		}
		answer.Triggers = append(answer.Triggers, trigger)
	}
	return nil
}

func buildTrigger(answer *plugins.Trigger, trigger *jenkinsv1.Trigger, orgName string, repoName string) error {
	if trigger.TrustedOrg != nil {
		answer.TrustedOrg = *trigger.TrustedOrg
	} else {
		answer.TrustedOrg = orgName
	}
	if trigger.OnlyOrgMembers != nil {
		answer.OnlyOrgMembers = *trigger.OnlyOrgMembers
	}
	if trigger.JoinOrgURL != nil {
		answer.JoinOrgURL = *trigger.JoinOrgURL
	}
	if trigger.IgnoreOkToTest != nil {
		answer.IgnoreOkToTest = *trigger.IgnoreOkToTest
	}
	answer.Repos = []string{
		orgSlashRepo(orgName, repoName),
	}
	return nil
}

func buildLgtm(answer *plugins.Lgtm, lgtm *jenkinsv1.Lgtm, orgName string, repoName string) error {
	if lgtm.StickyLgtmTeam != nil {
		answer.StickyLgtmTeam = *lgtm.StickyLgtmTeam
	}
	if lgtm.ReviewActsAsLgtm != nil {
		answer.ReviewActsAsLgtm = *lgtm.ReviewActsAsLgtm
	}
	if lgtm.StoreTreeHash != nil {
		answer.StoreTreeHash = *lgtm.StoreTreeHash
	}
	answer.Repos = []string{
		orgSlashRepo(orgName, repoName),
	}
	return nil
}

func buildApprove(answer *plugins.Approve, approve *jenkinsv1.Approve, orgName string, repoName string) error {
	answer.IgnoreReviewState = approve.IgnoreReviewState
	answer.RequireSelfApproval = approve.RequireSelfApproval
	if approve.IssueRequired != nil {
		answer.IssueRequired = *approve.IssueRequired
	}
	if approve.LgtmActsAsApprove != nil {
		answer.LgtmActsAsApprove = *approve.LgtmActsAsApprove
	}
	answer.Repos = []string{
		orgSlashRepo(orgName, repoName),
	}
	return nil
}

func buildExternalPlugin(answer *plugins.ExternalPlugin, plugin *jenkinsv1.ExternalPlugin) error {
	if plugin.Name != nil {
		answer.Name = *plugin.Name
	}
	if plugin.Endpoint != nil {
		answer.Endpoint = *plugin.Endpoint
	}
	if plugin.Events != nil {
		answer.Events = plugin.Events.Items
	}
	return nil
}

func buildProwConfig(prowConfig *config.ProwConfig, scheduler *jenkinsv1.SchedulerSpec, org string, repo string) error {
	if scheduler.Policy != nil {
		err := buildGlobalBranchProtection(&prowConfig.BranchProtection, scheduler.Policy)
		if err != nil {
			return errors.Wrapf(err, "building BranchProtection for %v", scheduler)
		}
	}
	if scheduler.Merger != nil {
		err := buildMerger(&prowConfig.Tide, scheduler.Merger, org, repo)
		if err != nil {
			return errors.Wrapf(err, "building Merger for %v", scheduler)
		}
	}
	return nil
}

func buildPolicy(answer *config.Policy, policy *jenkinsv1.ProtectionPolicy) error {
	if policy.Protect != nil {
		answer.Protect = policy.Protect
	}
	if policy.Admins != nil {
		answer.Admins = policy.Admins
	}
	if policy.RequiredStatusChecks != nil {
		if answer.RequiredStatusChecks == nil {
			answer.RequiredStatusChecks = &config.ContextPolicy{}
		}
		err := buildBranchProtectionContextPolicy(answer.RequiredStatusChecks, policy.RequiredStatusChecks)
		if err != nil {
			return errors.Wrapf(err, "building ContextPolicy from %v", policy)
		}
	}
	if policy.RequiredPullRequestReviews != nil {
		if answer.RequiredPullRequestReviews == nil {
			answer.RequiredPullRequestReviews = &config.ReviewPolicy{}
		}
		err := buildRequiredPullRequestReviews(answer.RequiredPullRequestReviews, policy.RequiredPullRequestReviews)
		if err != nil {
			return errors.Wrapf(err, "building RequiredPullRequestReviews from %v", policy)
		}
	}
	if policy.Restrictions != nil {
		if answer.Restrictions == nil {
			answer.Restrictions = &config.Restrictions{}
		}
		err := buildRestrictions(answer.Restrictions, policy.Restrictions)
		if err != nil {
			return errors.Wrapf(err, "building Restrictions from %v", policy)
		}
	}
	return nil
}

func buildBranchProtectionContextPolicy(answer *config.ContextPolicy,
	policy *jenkinsv1.BranchProtectionContextPolicy) error {
	if policy.Contexts != nil {
		if answer.Contexts == nil {
			answer.Contexts = make([]string, 0)
		}
		for _, i := range policy.Contexts.Items {
			found := false
			for _, existing := range answer.Contexts {
				if existing == i {
					found = true
					break
				}
			}
			if !found {
				answer.Contexts = append(answer.Contexts, i)
			}
		}
	}
	if policy.Strict != nil {
		answer.Strict = policy.Strict
	}
	return nil
}

func buildRequiredPullRequestReviews(answer *config.ReviewPolicy, policy *jenkinsv1.ReviewPolicy) error {
	if policy.Approvals != nil {
		answer.Approvals = policy.Approvals
	}
	if policy.DismissStale != nil {
		answer.DismissStale = policy.DismissStale
	}
	if policy.RequireOwners != nil {
		answer.RequireOwners = policy.RequireOwners
	}
	if policy.DismissalRestrictions != nil {
		if answer.DismissalRestrictions == nil {
			answer.DismissalRestrictions = &config.Restrictions{}
		}
		err := buildRestrictions(answer.DismissalRestrictions, policy.DismissalRestrictions)
		if err != nil {
			return errors.Wrapf(err, "building DismissalRestrictions from %v", policy)
		}
	}
	return nil
}

func buildRestrictions(answer *config.Restrictions, restrictions *jenkinsv1.Restrictions) error {
	if restrictions.Users != nil {
		if answer.Users == nil {
			answer.Users = make([]string, 0)
		}
		for _, i := range restrictions.Users.Items {
			answer.Users = append(answer.Users, i)
		}
	}
	if restrictions.Teams != nil {
		if answer.Teams == nil {
			answer.Teams = make([]string, 0)
		}
		for _, i := range restrictions.Teams.Items {
			answer.Teams = append(answer.Teams, i)
		}
	}
	return nil
}

func buildJobConfig(jobConfig *config.JobConfig, prowConfig *config.ProwConfig,
	scheduler *jenkinsv1.SchedulerSpec, org string, repo string) error {
	if scheduler.Postsubmits != nil && scheduler.Postsubmits.Items != nil {
		err := buildPostsubmits(jobConfig, scheduler.Postsubmits.Items, org, repo)
		if err != nil {
			return errors.Wrapf(err, "building Postsubmits from %v", scheduler)
		}
	}
	if scheduler.Presubmits != nil && scheduler.Presubmits.Items != nil {
		err := buildPresubmits(jobConfig, prowConfig, scheduler.Presubmits.Items, org, repo)
		if err != nil {
			return errors.Wrapf(err, "building Presubmits from %v", scheduler)
		}
	}
	if scheduler.Periodics != nil && len(scheduler.Periodics.Items) > 0 {
		err := buildPeriodics(jobConfig, scheduler.Periodics)
		if err != nil {
			return errors.Wrapf(err, "building periodics for %v", scheduler)
		}
	}
	if scheduler.Attachments != nil && len(scheduler.Attachments) > 0 {
		buildPlank(prowConfig, scheduler.Attachments)
	}
	return nil
}

func buildPostsubmits(jobConfig *config.JobConfig, items []*jenkinsv1.Postsubmit, orgName string, repoName string) error {
	if jobConfig.Postsubmits == nil {
		jobConfig.Postsubmits = make(map[string][]config.Postsubmit)
	}
	orgSlashRepo := orgSlashRepo(orgName, repoName)
	for _, postsubmit := range items {
		if _, ok := jobConfig.Postsubmits[orgSlashRepo]; !ok {
			jobConfig.Postsubmits[orgSlashRepo] = make([]config.Postsubmit, 0)
		}
		c := config.Postsubmit{}
		err := buildJobBase(&c.JobBase, postsubmit.JobBase)
		if err != nil {
			return errors.Wrapf(err, "building JobBase from %v", postsubmit.JobBase)
		}
		if postsubmit.Brancher != nil {
			err = buildBrancher(&c.Brancher, postsubmit.Brancher)
			if err != nil {
				return errors.Wrapf(err, "building Brancher from %v", postsubmit.Brancher)
			}
		}
		if postsubmit.RegexpChangeMatcher != nil {
			err = buildRegexChangeMatacher(&c.RegexpChangeMatcher, postsubmit.RegexpChangeMatcher)
			if err != nil {
				return errors.Wrapf(err, "building RegexpChangeMatcher from %v", postsubmit.RegexpChangeMatcher)
			}
		}
		if postsubmit.Report != nil {
			c.Report = *postsubmit.Report
		}
		if postsubmit.Context != nil {
			c.Context = *postsubmit.Context
		}
		jobConfig.Postsubmits[orgSlashRepo] = append(jobConfig.Postsubmits[orgSlashRepo], c)
	}
	return nil
}

func buildPresubmits(jobConfig *config.JobConfig, prowConfig *config.ProwConfig,
	items []*jenkinsv1.Presubmit, orgName string, repoName string) error {
	if jobConfig.Presubmits == nil {
		jobConfig.Presubmits = make(map[string][]config.Presubmit)
	}
	orgSlashRepo := orgSlashRepo(orgName, repoName)
	for _, presubmit := range items {
		if _, ok := jobConfig.Presubmits[orgSlashRepo]; !ok {
			jobConfig.Presubmits[orgSlashRepo] = make([]config.Presubmit, 0)
		}
		c := config.Presubmit{}
		err := buildJobBase(&c.JobBase, presubmit.JobBase)
		if err != nil {
			return errors.Wrapf(err, "building JobBase from %v", presubmit.JobBase)
		}
		if presubmit.Brancher != nil {
			err = buildBrancher(&c.Brancher, presubmit.Brancher)
			if err != nil {
				return errors.Wrapf(err, "building Brancher from %v", presubmit.Brancher)
			}
		}
		if presubmit.RegexpChangeMatcher != nil {
			err = buildRegexChangeMatacher(&c.RegexpChangeMatcher, presubmit.RegexpChangeMatcher)
			if err != nil {
				return errors.Wrapf(err, "building RegexpChangeMatcher from %v", presubmit.RegexpChangeMatcher)
			}
		}
		if presubmit.Trigger != nil {
			c.Trigger = *presubmit.Trigger
		}
		if presubmit.RerunCommand != nil {
			c.RerunCommand = *presubmit.RerunCommand
		}
		if presubmit.Optional != nil {
			c.Optional = *presubmit.Optional
		}
		if presubmit.AlwaysRun != nil {
			c.AlwaysRun = *presubmit.AlwaysRun
		}
		if presubmit.Report != nil {
			c.SkipReport = !*presubmit.Report
		}
		if presubmit.Context != nil {
			c.Context = *presubmit.Context
		}
		jobConfig.Presubmits[orgSlashRepo] = append(jobConfig.Presubmits[orgSlashRepo], c)

		if presubmit.Queries != nil && len(presubmit.Queries) > 0 {
			err := buildQuery(&prowConfig.Tide, presubmit.Queries, orgName, repoName)
			if err != nil {
				return errors.Wrapf(err, "building Query from %v", presubmit.Queries)
			}
		}
		if presubmit.MergeType != nil {
			mt := github.PullRequestMergeType(*presubmit.MergeType)
			if prowConfig.Tide.MergeType == nil && mt != "" {
				prowConfig.Tide.MergeType = make(map[string]github.PullRequestMergeType)
			}
			if mt != "" {
				prowConfig.Tide.MergeType[orgSlashRepo] = mt
			}
		}
		if presubmit.Policy != nil {
			if presubmit.Policy.ProtectionPolicy != nil {
				err := buildBranchProtection(&prowConfig.BranchProtection, presubmit.Policy.ProtectionPolicy,
					orgName, repoName, "")
				if err != nil {
					return errors.WithStack(err)
				}
			}
			for k, v := range presubmit.Policy.Items {
				err := buildBranchProtection(&prowConfig.BranchProtection, v, orgName, repoName, k)
				if err != nil {
					return errors.WithStack(err)
				}
			}

		}
		if presubmit.ContextPolicy != nil {
			policy := config.TideRepoContextPolicy{}
			err := buildRepoContextPolicy(&policy, presubmit.ContextPolicy)
			if err != nil {
				return errors.Wrapf(err, "building RepoContextPolicy from %v", presubmit)
			}
			if prowConfig.Tide.ContextOptions.Orgs == nil {
				prowConfig.Tide.ContextOptions.Orgs = make(map[string]config.TideOrgContextPolicy)
			}
			if _, ok := prowConfig.Tide.ContextOptions.Orgs[orgName]; !ok {
				prowConfig.Tide.ContextOptions.Orgs[orgName] = config.TideOrgContextPolicy{
					Repos: make(map[string]config.TideRepoContextPolicy),
				}
			}
			prowConfig.Tide.ContextOptions.Orgs[orgName].Repos[repoName] = policy
		}
		// TODO handle LGTM's here
	}
	return nil
}

func buildGlobalBranchProtection(answer *config.BranchProtection,
	globalProtectionPolicy *jenkinsv1.GlobalProtectionPolicy) error {
	if globalProtectionPolicy.ProtectTested != nil {
		answer.ProtectTested = *globalProtectionPolicy.ProtectTested
	}
	if globalProtectionPolicy.ProtectionPolicy != nil {
		err := buildBranchProtection(answer, globalProtectionPolicy.ProtectionPolicy, "", "", "")
		if err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}

func buildBranchProtection(answer *config.BranchProtection,
	protectionPolicy *jenkinsv1.ProtectionPolicy, orgName string, repoName string, branchName string) error {
	if orgName != "" {
		if answer.Orgs == nil {
			answer.Orgs = make(map[string]config.Org)
		}
		if _, ok := answer.Orgs[orgName]; !ok {
			answer.Orgs[orgName] = config.Org{}
		}
		org := answer.Orgs[orgName]
		if repoName != "" {
			if org.Repos == nil {
				org.Repos = make(map[string]config.Repo)
			}
			if _, ok := answer.Orgs[orgName].Repos[repoName]; !ok {
				org.Repos[repoName] = config.Repo{}
			}
			repo := answer.Orgs[orgName].Repos[repoName]
			if branchName != "" {
				if repo.Branches == nil {
					repo.Branches = make(map[string]config.Branch)
				}
				if _, ok := repo.Branches[branchName]; !ok {
					repo.Branches[branchName] = config.Branch{}
				}
				branch := repo.Branches[branchName]
				err := buildPolicy(&branch.Policy, protectionPolicy)
				if err != nil {
					return errors.Wrapf(err, "building ProtectionPolicy from %v", protectionPolicy)
				}
			} else {
				err := buildPolicy(&repo.Policy, protectionPolicy)
				if err != nil {
					return errors.Wrapf(err, "building ProtectionPolicy from %v", protectionPolicy)
				}
			}
			org.Repos[repoName] = repo
		} else {
			err := buildPolicy(&org.Policy, protectionPolicy)
			if err != nil {
				return errors.Wrapf(err, "building ProtectionPolicy from %v", protectionPolicy)
			}
		}
		answer.Orgs[orgName] = org
	} else {
		err := buildPolicy(&answer.Policy, protectionPolicy)
		if err != nil {
			return errors.Wrapf(err, "building ProtectionPolicy from %v", protectionPolicy)
		}
	}
	return nil
}

func orgSlashRepo(org string, repo string) string {
	if repo == "" {
		return org
	}
	return fmt.Sprintf("%s/%s", org, repo)
}

func buildJobBase(answer *config.JobBase, jobBase *jenkinsv1.JobBase) error {
	if jobBase.Agent != nil {
		answer.Agent = *jobBase.Agent
	}
	if jobBase.Labels != nil && jobBase.Labels.Items != nil {
		answer.Labels = jobBase.Labels.Items
	}
	if jobBase.MaxConcurrency != nil {
		answer.MaxConcurrency = *jobBase.MaxConcurrency
	}
	if jobBase.Cluster != nil {
		answer.Cluster = *jobBase.Cluster
	}
	if jobBase.Namespace != nil {
		answer.Namespace = jobBase.Namespace
	}
	if jobBase.Name != nil {
		answer.Name = *jobBase.Name
	}
	if jobBase.Spec != nil {
		answer.Spec = jobBase.Spec
	}
	return nil
}

func buildBrancher(answer *config.Brancher, brancher *jenkinsv1.Brancher) error {
	if brancher.SkipBranches != nil && brancher.SkipBranches.Items != nil {
		answer.SkipBranches = brancher.SkipBranches.Items
	}
	if brancher.Branches != nil {
		answer.Branches = brancher.Branches.Items
	}
	return nil
}

func buildRegexChangeMatacher(answer *config.RegexpChangeMatcher,
	matcher *jenkinsv1.RegexpChangeMatcher) error {
	if matcher.RunIfChanged != nil {
		answer.RunIfChanged = *matcher.RunIfChanged
	}
	return nil
}

func buildPlank(answer *config.ProwConfig, attachments []*jenkinsv1.Attachment) {
	for attachmentIndex := range attachments {
		attachment := attachments[attachmentIndex]
		if attachment.Name == "reportTemplate" {
			answer.Plank.ReportTemplateString = attachment.URLs[0]
		}
		if attachment.Name == "jobURLPrefix" {
			answer.Plank.JobURLPrefix = attachment.URLs[0]
		}
		if attachment.Name == "jobURLTemplate" {
			answer.Plank.JobURLTemplateString = attachment.URLs[0]
		}
	}
}

func buildPeriodics(answer *config.JobConfig, periodics *jenkinsv1.Periodics) error {
	if answer.Periodics == nil {
		answer.Periodics = make([]config.Periodic, 0)
	}
	for _, schedulerPeriodic := range periodics.Items {
		periodicAlreadyExists := false
		for existingPeriodicIndex := range answer.Periodics {
			if answer.Periodics[existingPeriodicIndex].Name == *schedulerPeriodic.Name {
				periodicAlreadyExists = true
				break
			}
		}
		if !periodicAlreadyExists {
			periodic := config.Periodic{
				Cron:     *schedulerPeriodic.Cron,
				Interval: *schedulerPeriodic.Interval,
			}
			if schedulerPeriodic.Tags.Items != nil && len(schedulerPeriodic.Tags.Items) > 0 {
				periodic.Tags = schedulerPeriodic.Tags.Items
			}
			err := buildJobBase(&periodic.JobBase, schedulerPeriodic.JobBase)
			if err != nil {
				return errors.Wrapf(err, "building periodic for %v", periodic)
			}
			answer.Periodics = append(answer.Periodics, periodic)
		}
	}
	return nil
}

func buildMerger(answer *config.Tide, merger *jenkinsv1.Merger, org string, repo string) error {
	if merger.SyncPeriod != nil {
		answer.SyncPeriod = *merger.SyncPeriod
	}
	if merger.StatusUpdatePeriod != nil {
		answer.StatusUpdatePeriod = *merger.StatusUpdatePeriod
	}
	if merger.TargetURL != nil {
		answer.TargetURL = *merger.TargetURL
	}
	if merger.PRStatusBaseURL != nil {
		answer.PRStatusBaseURL = *merger.PRStatusBaseURL
	}
	if merger.BlockerLabel != nil {
		answer.BlockerLabel = *merger.BlockerLabel
	}
	if merger.SquashLabel != nil {
		answer.SquashLabel = *merger.SquashLabel
	}
	if merger.MaxGoroutines != nil {
		answer.MaxGoroutines = *merger.MaxGoroutines
	}
	if merger.MergeType != nil {
		if answer.MergeType == nil {
			answer.MergeType = make(map[string]github.PullRequestMergeType)
		}
		answer.MergeType[fmt.Sprintf("%s/%s", org, repo)] = github.PullRequestMergeType(*merger.MergeType)
	}
	if merger.ContextPolicy != nil {
		err := buildContextPolicy(&answer.ContextOptions.TideContextPolicy, merger.ContextPolicy)
		if err != nil {
			return errors.Wrapf(err, "building ContextPolicy for %v", merger.ContextPolicy)
		}
	}
	return nil
}

func buildRepoContextPolicy(answer *config.TideRepoContextPolicy,
	repoContextPolicy *jenkinsv1.RepoContextPolicy) error {
	err := buildContextPolicy(&answer.TideContextPolicy, repoContextPolicy.ContextPolicy)
	if err != nil {
		return errors.Wrapf(err, "building ContextPolicy for %v", repoContextPolicy)
	}
	if repoContextPolicy.Branches != nil {
		for branch, policy := range repoContextPolicy.Branches.Items {
			if answer.Branches == nil {
				answer.Branches = make(map[string]config.TideContextPolicy)
			}
			tidePolicy := config.TideContextPolicy{}
			err := buildContextPolicy(&tidePolicy, policy)
			if err != nil {
				return errors.Wrapf(err, "building ContextPolicy for %v", policy)
			}
			answer.Branches[branch] = tidePolicy
		}
	}
	return nil
}

func buildContextPolicy(answer *config.TideContextPolicy,
	contextOptions *jenkinsv1.ContextPolicy) error {
	if contextOptions != nil {
		if contextOptions.SkipUnknownContexts != nil {
			answer.SkipUnknownContexts = contextOptions.SkipUnknownContexts
		}
		if contextOptions.FromBranchProtection != nil {
			answer.FromBranchProtection = contextOptions.FromBranchProtection
		}
		if contextOptions.RequiredIfPresentContexts != nil {
			answer.RequiredIfPresentContexts = contextOptions.RequiredIfPresentContexts.Items
		}
		if contextOptions.RequiredContexts != nil {
			answer.RequiredContexts = contextOptions.RequiredContexts.Items
		}
		if contextOptions.OptionalContexts != nil {
			answer.OptionalContexts = contextOptions.OptionalContexts.Items
		}
	}
	return nil
}

func buildQuery(answer *config.Tide, queries []*jenkinsv1.Query, org string, repo string) error {
	if answer.Queries == nil {
		answer.Queries = config.TideQueries{}
	}
	tideQuery := &config.TideQuery{
		Repos: []string{orgSlashRepo(org, repo)},
	}
	for _, query := range queries {
		if query.ExcludedBranches != nil {
			tideQuery.ExcludedBranches = query.ExcludedBranches.Items
		}
		if query.IncludedBranches != nil {
			tideQuery.IncludedBranches = query.IncludedBranches.Items
		}
		if query.Labels != nil {
			tideQuery.Labels = query.Labels.Items
		}
		if query.MissingLabels != nil {
			tideQuery.MissingLabels = query.MissingLabels.Items
		}
		if query.Milestone != nil {
			tideQuery.Milestone = *query.Milestone
		}
		if query.ReviewApprovedRequired != nil {
			tideQuery.ReviewApprovedRequired = *query.ReviewApprovedRequired
		}
		mergedWithExisting := false
		for index := range answer.Queries {
			existingQuery := &answer.Queries[index]
			if cmp.Equal(existingQuery, tideQuery, cmpopts.IgnoreFields(config.TideQuery{}, "Repos")) {
				mergedWithExisting = true
				for _, newRepo := range tideQuery.Repos {
					if !utils.ContainsString(existingQuery.Repos, newRepo) {
						existingQuery.Repos = append(existingQuery.Repos, newRepo)
					}
				}
			}
		}
		if !mergedWithExisting {
			answer.Queries = append(answer.Queries, *tideQuery)
		}
	}
	return nil
}
