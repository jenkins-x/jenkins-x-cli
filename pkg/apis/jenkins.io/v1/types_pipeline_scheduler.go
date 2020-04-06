package v1

import (
	"time"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// Scheduler is configuration for a pipeline scheduler
type Scheduler struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec SchedulerSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SchedulerList is a list of configurations for a pipeline scheduler
type SchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Scheduler `json:"items"`
}

// TODO Support Label plugin?
// TODO Support Size plugin?
// TODO Support Welcome plugin?
// TODO Support Blockade plugin?
// TODO Support Golint plugin?
// TODO Support RepoMilestone plugin?
// TODO Support RequireMatchingLabel plugin?
// TODO Support Blunderbuss plugin?
// TODO Support Config Updater Plugin?
// TODO Support Owners plugin?
// TODO Support Heart plugin?
// TODO Support requiresig plugin?
// TODO support sigmention plugin?
// TODO support slack plugin?

// SchedulerSpec defines the pipeline scheduler (e.g. Prow) configuration
type SchedulerSpec struct {
	ScehdulerAgent  *SchedulerAgent                    `json:"schedulerAgent,omitempty" protobuf:"bytes,1,opt,name=schedulerAgent"`
	Policy          *GlobalProtectionPolicy            `json:"policy,omitempty" protobuf:"bytes,2,opt,name=policy"`
	Presubmits      *Presubmits                        `json:"presubmits,omitempty" protobuf:"bytes,3,opt,name=presubmits"`
	Postsubmits     *Postsubmits                       `json:"postsubmits,omitempty" protobuf:"bytes,4,opt,name=postsubmits"`
	Trigger         *Trigger                           `json:"trigger,omitempty" protobuf:"bytes,5,opt,name=trigger"`
	Approve         *Approve                           `json:"approve,omitempty" protobuf:"bytes,6,opt,name=approve"`
	LGTM            *Lgtm                              `json:"lgtm,omitempty" protobuf:"bytes,7,opt,name=lgtm"`
	ExternalPlugins *ReplaceableSliceOfExternalPlugins `json:"externalPlugins,omitempty" protobuf:"bytes,8,opt,name=externalPlugins"`

	Merger *Merger `json:"merger,omitempty" protobuf:"bytes,9,opt,name=merger"`

	// Plugins is a list of plugin names enabled for a repo
	Plugins       *ReplaceableSliceOfStrings `json:"plugins,omitempty" protobuf:"bytes,10,opt,name=plugins"`
	ConfigUpdater *ConfigUpdater             `json:"configUpdater,omitempty" protobuf:"bytes,11,opt,name=configUpdater"`
	Welcome       []*Welcome                 `json:"welcome,omitempty" protobuf:"bytes,12,opt,name=welcome"`
	Periodics     *Periodics                 `json:"periodics,omitempty" protobuf:"bytes,13,opt,name=periodics"`
	Attachments   []*Attachment              `json:"attachments,omitempty" protobuf:"bytes,13,opt,name=attachments"`
}

// ConfigMapSpec contains configuration options for the configMap being updated
// by the config-updater plugin.
type ConfigMapSpec struct {
	// Name of ConfigMap
	Name string `json:"name"`
	// Key is the key in the ConfigMap to update with the file contents.
	// If no explicit key is given, the basename of the file will be used.
	Key string `json:"key,omitempty"`
	// Namespace in which the configMap needs to be deployed. If no namespace is specified
	// it will be deployed to the ProwJobNamespace.
	Namespace string `json:"namespace,omitempty"`
	// Namespaces in which the configMap needs to be deployed, in addition to the above
	// namespace provided, or the default if it is not set.
	AdditionalNamespaces []string `json:"additional_namespaces,omitempty"`

	// Namespaces is the fully resolved list of Namespaces to deploy the ConfigMap in
	Namespaces []string `json:"-"`
}

// ConfigUpdater holds configuration for the config updater plugin
type ConfigUpdater struct {
	Map        map[string]ConfigMapSpec `json:"map,omitempty" protobuf:"bytes,1,opt,name=map"`
	ConfigFile string                   `json:"configFile,omitempty" protobuf:"bytes,2,opt,name=configFile"`
	PluginFile string                   `json:"pluginFile,omitempty" protobuf:"bytes,3,opt,name=pluginFile"`
	// +optional
	ConfigMap ConfigMapSpec
}

// ExternalPlugin holds configuration for registering an external
// plugin.
type ExternalPlugin struct {
	// Name of the plugin.
	Name *string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Endpoint is the location of the external plugin. Defaults to
	// the name of the plugin, ie. "http://{{name}}".
	Endpoint *string `json:"endpoint,omitempty" protobuf:"bytes,2,opt,name=endpoint"`
	// ReplaceableSliceOfStrings are the events that need to be demuxed by the hook
	// server to the external plugin. If no events are specified,
	// everything is sent.
	Events *ReplaceableSliceOfStrings `json:"events,omitempty" protobuf:"bytes,3,opt,name=events"`
}

// ReplaceableSliceOfStrings is a slice of strings that can optionally completely replace the slice of strings
// defined in the parent scheduler
type ReplaceableSliceOfStrings struct {
	// Items is the string values
	Items []string `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
	// Replace the existing entries
	Replace bool `json:"replace,omitempty" protobuf:"bytes,2,opt,name=replace"`
}

// Lgtm specifies a configuration for a single lgtm.
// The configuration for the lgtm plugin is defined as a list of these structures.
type Lgtm struct {
	// ReviewActsAsLgtm indicates that a Github review of "approve" or "request changes"
	// acts as adding or removing the lgtm label
	ReviewActsAsLgtm *bool `json:"reviewActsAsLgtm,omitempty" protobuf:"bytes,1,opt,name=reviewActsAsLgtm"`
	// StoreTreeHash indicates if tree_hash should be stored inside a comment to detect
	// squashed commits before removing lgtm labels
	StoreTreeHash *bool `json:"storeTreeHash,omitempty" protobuf:"bytes,2,opt,name=storeTreeHash"`
	// WARNING: This disables the security mechanism that prevents a malicious member (or
	// compromised GitHub account) from merging arbitrary code. Use with caution.
	//
	// StickyLgtmTeam specifies the Github team whose members are trusted with sticky LGTM,
	// which eliminates the need to re-lgtm minor fixes/updates.
	StickyLgtmTeam *string `json:"trustedTeamForStickyLgtm,omitempty" protobuf:"bytes,3,opt,name=stickyLgtmTeam"`
}

// Approve specifies a configuration for a single approve.
//
// The configuration for the approve plugin is defined as a list of these structures.
type Approve struct {
	// IssueRequired indicates if an associated issue is required for approval in
	// the specified repos.
	IssueRequired *bool `json:"issueRequired,omitempty" protobuf:"bytes,1,opt,name=issueRequired"`

	// RequireSelfApproval requires PR authors to explicitly approve their PRs.
	// Otherwise the plugin assumes the author of the PR approves the changes in the PR.
	RequireSelfApproval *bool `json:"requireSelfApproval,omitempty" protobuf:"bytes,2,opt,name=requireSelfApproval"`

	// LgtmActsAsApprove indicates that the lgtm command should be used to
	// indicate approval
	LgtmActsAsApprove *bool `json:"lgtmActsAsApprove,omitempty" protobuf:"bytes,3,opt,name=lgtmActsAsApprove"`

	// IgnoreReviewState causes the approve plugin to ignore the GitHub review state. Otherwise:
	// * an APPROVE github review is equivalent to leaving an "/approve" message.
	// * A REQUEST_CHANGES github review is equivalent to leaving an /approve cancel" message.
	IgnoreReviewState *bool `json:"ignoreReviewState,omitempty" protobuf:"bytes,4,opt,name=ignoreReviewState"`
}

// Trigger specifies a configuration for a single trigger.
//
// The configuration for the trigger plugin is defined as a list of these structures.
type Trigger struct {
	// TrustedOrg is the org whose members' PRs will be automatically built
	// for PRs to the above repos. The default is the PR's org.
	TrustedOrg *string `json:"trustedOrg,omitempty" protobuf:"bytes,1,opt,name=trustedOrg"`
	// JoinOrgURL is a link that redirects users to a location where they
	// should be able to read more about joining the organization in order
	// to become trusted members. Defaults to the Github link of TrustedOrg.
	JoinOrgURL *string `json:"joinOrgUrl,omitempty" protobuf:"bytes,2,opt,name=joinOrgUrl"`
	// OnlyOrgMembers requires PRs and/or /ok-to-test comments to come from org members.
	// By default, trigger also include repo collaborators.
	OnlyOrgMembers *bool `json:"onlyOrgMembers,omitempty" protobuf:"bytes,3,opt,name=onlyOrgMembers"`
	// IgnoreOkToTest makes trigger ignore /ok-to-test comments.
	// This is a security mitigation to only allow testing from trusted users.
	IgnoreOkToTest *bool `json:"ignoreOkToTest,omitempty" protobuf:"bytes,4,opt,name=ignoreOkToTest"`
}

// Postsubmits is a list of Postsubmit job configurations that can optionally completely replace the Postsubmit job
// configurations in the parent scheduler
type Postsubmits struct {
	// Items are the post submit configurations
	Items []*Postsubmit `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
	// Replace the existing entries
	Replace bool `json:"replace,omitempty" protobuf:"bytes,2,opt,name=replace"`
}

// Postsubmit runs on push events.
type Postsubmit struct {
	// +optional
	*JobBase
	// +optional
	*RegexpChangeMatcher
	// +optional
	*Brancher

	// Context is the name of the GitHub status context for the job.
	Context *string `json:"context" protobuf:"bytes,1,opt,name=context"`

	// Report will comment and set status on GitHub.
	Report *bool `json:"report,omitempty" protobuf:"bytes,2,opt,name=report"`
}

// JobBase contains attributes common to all job types
type JobBase struct {
	// The name of the job. Must match regex [A-Za-z0-9-._]+
	// e.g. pull-test-infra-bazel-build
	Name *string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ReplaceableMapOfStringString are added to jobs and pods created for this job.
	Labels *ReplaceableMapOfStringString `json:"labels,omitempty" protobuf:"bytes,2,opt,name=labels"`
	// MaximumConcurrency of this job, 0 implies no limit.
	MaxConcurrency *int `json:"maxConcurrency,omitempty" protobuf:"bytes,3,opt,name=maxConcurrency"`
	// Agent that will take care of running this job.
	Agent *string `json:"agent" protobuf:"bytes,4,opt,name=agent"`
	// Cluster is the alias of the cluster to run this job in.
	// (Default: kube.DefaultClusterAlias)
	Cluster *string `json:"cluster,omitempty" protobuf:"bytes,5,opt,name=cluster"`
	// Namespace is the namespace in which pods schedule.
	//   empty: results in scheduler.DefaultNamespace
	Namespace *string `json:"namespace,omitempty" protobuf:"bytes,6,opt,name=namespace"`
	// Spec is the Kubernetes pod spec used if Agent is kubernetes.
	Spec *v1.PodSpec `json:"spec,omitempty"`
}

// ReplaceableMapOfStringString is a map of strings that can optionally completely replace the map of strings in the
// parent scheduler
type ReplaceableMapOfStringString struct {
	Items map[string]string `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
	// Replace the existing entries
	Replace bool `json:"replace,omitempty" protobuf:"bytes,2,opt,name=replace"`
}

// Presubmits is a list of Presubmit job configurations that can optionally completely replace the Presubmit job
// configurations in the parent scheduler
type Presubmits struct {
	// Items are the Presubmit configurtations
	Items []*Presubmit `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
	// Replace the existing entries
	Replace bool `json:"replace,omitempty" protobuf:"bytes,2,opt,name=replace"`
}

// Presubmit defines a job configuration for pull requests
type Presubmit struct {
	// +optional
	*JobBase
	// +optional
	*Brancher
	// +optional
	*RegexpChangeMatcher

	// AlwaysRun automatically for every PR, or only when a comment triggers it. By default true.
	AlwaysRun *bool `json:"alwaysRun" protobuf:"bytes,1,opt,name=alwaysRun"`

	// Context is the name of the Git Provider status context for the job.
	Context *string `json:"context" protobuf:"bytes,2,opt,name=context"`
	// Optional indicates that the job's status context should not be required for merge. By default false.
	Optional *bool `json:"optional,omitempty" protobuf:"bytes,3,opt,name=optional"`
	// Report enables reporting the job status on the git provider
	Report *bool `json:"report,omitempty"  protobuf:"bytes,4,opt,name=report"`

	// Trigger is the regular expression to trigger the job.
	// e.g. `@k8s-bot e2e test this`
	// RerunCommand must also be specified if this field is specified.
	// (Default: `(?m)^/test (?:.*? )?<job name>(?: .*?)?$`)
	Trigger *string `json:"trigger"  protobuf:"bytes,5,opt,name=trigger"`
	// The RerunCommand to give users. Must match Trigger.
	// Trigger must also be specified if this field is specified.
	// (Default: `/test <job name>`)
	RerunCommand *string `json:"rerunCommand" protobuf:"bytes,6,opt,name=rerunCommand"`
	// Override the default method of merge. Valid options are squash, rebase, and merge.
	MergeType *string `json:"mergeMethod,omitempty" protobuf:"bytes,7,opt,name=mergeMethod"`

	Queries []*Query `json:"queries,omitempty" protobuf:"bytes,8,opt,name=query"`

	Policy *ProtectionPolicies `json:"policy,omitempty" protobuf:"bytes,9,opt,name=policy"`
	// ContextOptions defines the merge options. If not set it will infer
	// the required and optional contexts from the jobs configured and use the Git Provider
	// combined status; otherwise it may apply the branch protection setting or let user
	// define their own options in case branch protection is not used.
	ContextPolicy *RepoContextPolicy `json:"context_options,omitempty" protobuf:"bytes,10,opt,name=contextPolicy"`
}

// Periodics is a list of jobs to be run periodically
type Periodics struct {
	// Items are the post submit configurations
	Items []*Periodic `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
	// Replace the existing entries
	Replace bool `json:"replace,omitempty" protobuf:"bytes,2,opt,name=replace"`
}

// Periodic defines a job to be run periodically
type Periodic struct {
	// +optional
	*JobBase
	// Interval to wait between two runs of the job.
	Interval *string `json:"interval"`
	// Cron representation of job trigger time
	Cron *string `json:"cron"`
	// Tags for config entries
	Tags *ReplaceableSliceOfStrings `json:"tags,omitempty"`
}

// Query is turned into a Git Provider search query. See the docs for details:
// https://help.github.com/articles/searching-issues-and-pull-requests/
type Query struct {
	ExcludedBranches       *ReplaceableSliceOfStrings `json:"excludedBranches,omitempty" protobuf:"bytes,1,opt,name=excludedBranches"`
	IncludedBranches       *ReplaceableSliceOfStrings `json:"includedBranches,omitempty" protobuf:"bytes,2,opt,name=includedBranches"`
	Labels                 *ReplaceableSliceOfStrings `json:"labels,omitempty" protobuf:"bytes,3,opt,name=labels"`
	MissingLabels          *ReplaceableSliceOfStrings `json:"missingLabels,omitempty" protobuf:"bytes,4,opt,name=missingLabels"`
	Milestone              *string                    `json:"milestone,omitempty" protobuf:"bytes,5,opt,name=milestone"`
	ReviewApprovedRequired *bool                      `json:"reviewApprovedRequired,omitempty" protobuf:"bytes,6,opt,name=reviewApprovedRequired"`
}

// PullRequestMergeType enumerates the types of merges the Git Provider API can
// perform
// https://developer.github.com/v3/pulls/#merge-a-pull-request-merge-button
type PullRequestMergeType string

// Possible types of merges for the Git Provider merge API
const (
	MergeMerge  PullRequestMergeType = "merge"
	MergeRebase PullRequestMergeType = "rebase"
	MergeSquash PullRequestMergeType = "squash"
)

// Merger defines the options used to merge the PR
type Merger struct {
	// SyncPeriod specifies how often Merger will sync jobs with Github. Defaults to 1m.
	SyncPeriod *time.Duration `json:"-"`
	// StatusUpdatePeriod
	StatusUpdatePeriod *time.Duration `json:"-"`

	// URL for status contexts.
	TargetURL *string `json:"targetUrl,omitempty" protobuf:"bytes,1,opt,name=targetUrl"`

	// PRStatusBaseURL is the base URL for the PR status page.
	// This is used to link to a merge requirements overview
	// in the status context.
	PRStatusBaseURL *string `json:"prStatusBaseUrl,omitempty" protobuf:"bytes,2,opt,name=prStatusBaseURL"`

	// BlockerLabel is an optional label that is used to identify merge blocking
	// Git Provider issues.
	// Leave this blank to disable this feature and save 1 API token per sync loop.
	BlockerLabel *string `json:"blockerLabel,omitempty"`

	// SquashLabel is an optional label that is used to identify PRs that should
	// always be squash merged.
	// Leave this blank to disable this feature.
	SquashLabel *string `json:"squashLabel,omitempty"`

	// MaxGoroutines is the maximum number of goroutines spawned inside the
	// controller to handle org/repo:branch pools. Defaults to 20. Needs to be a
	// positive number.
	MaxGoroutines *int `json:"maxGoroutines,omitempty"`

	// Override the default method of merge. Valid options are squash, rebase, and merge.
	MergeType *string `json:"mergeMethod,omitempty"`

	// ContextOptions defines the default merge options. If not set it will infer
	// the required and optional contexts from the jobs configured and use the Git Provider
	// combined status; otherwise it may apply the branch protection setting or let user
	// define their own options in case branch protection is not used.
	ContextPolicy *ContextPolicy `json:"policy,omitempty"`
}

// RepoContextPolicy overrides the policy for repo, and any branch overrides.
type RepoContextPolicy struct {
	*ContextPolicy
	Branches *ReplaceableMapOfStringContextPolicy `json:"branches,omitempty"`
}

// ReplaceableMapOfStringContextPolicy is a map of context policies that can optionally completely replace any
// context policies defined in the parent scheduler
type ReplaceableMapOfStringContextPolicy struct {
	Replace bool `json:"replace,omitempty"`
	Items   map[string]*ContextPolicy
}

// ContextPolicy configures options about how to handle various contexts.
type ContextPolicy struct {
	// whether to consider unknown contexts optional (skip) or required.
	SkipUnknownContexts       *bool                      `json:"skipUnknownContexts,omitempty"`
	RequiredContexts          *ReplaceableSliceOfStrings `json:"requiredContexts,omitempty"`
	RequiredIfPresentContexts *ReplaceableSliceOfStrings `json:"requiredIfPresentContexts,omitempty"`
	OptionalContexts          *ReplaceableSliceOfStrings `json:"optionalContexts,omitempty"`
	// Infer required and optional jobs from Branch Protection configuration
	FromBranchProtection *bool `json:"fromBranchProtection,omitempty"`
}

// Welcome welcome plugin config
type Welcome struct {
	MessageTemplate *string `json:"message_template,omitempty"`
}

// Brancher is for shared code between jobs that only run against certain
// branches. An empty brancher runs against all branches.
type Brancher struct {
	// Do not run against these branches. Default is no branches.
	SkipBranches *ReplaceableSliceOfStrings `json:"skipBranches,omitempty"`
	// Only run against these branches. Default is all branches.
	Branches *ReplaceableSliceOfStrings `json:"branches,omitempty"`
}

// RegexpChangeMatcher is for code shared between jobs that run only when certain files are changed.
type RegexpChangeMatcher struct {
	// RunIfChanged defines a regex used to select which subset of file changes should trigger this job.
	// If any file in the changeset matches this regex, the job will be triggered
	RunIfChanged *string `json:"runIfChanged,omitempty"`
}

// GlobalProtectionPolicy defines the default branch protection policy for the scheduler
type GlobalProtectionPolicy struct {
	// +optional
	*ProtectionPolicy
	ProtectTested *bool `json:"protectTested,omitempty"`
}

// ProtectionPolicy for merging.
type ProtectionPolicy struct {
	// Protect overrides whether branch protection is enabled if set.
	Protect *bool `json:"protect,omitempty"`
	// RequiredStatusChecks configures github contexts
	RequiredStatusChecks *BranchProtectionContextPolicy `json:"requiredStatusChecks,omitempty"`
	// Admins overrides whether protections apply to admins if set.
	Admins *bool `json:"enforceAdmins,omitempty"`
	// Restrictions limits who can merge
	Restrictions *Restrictions `json:"restrictions,omitempty"`
	// RequiredPullRequestReviews specifies approval/review criteria.
	RequiredPullRequestReviews *ReviewPolicy `json:"requiredPullRequestReviews,omitempty"`
}

// ReviewPolicy specifies git provider approval/review criteria.
// Any nil values inherit the policy from the parent, otherwise bool/ints are overridden.
// Non-empty lists are appended to parent lists.
type ReviewPolicy struct {
	// Restrictions appends users/teams that are allowed to merge
	DismissalRestrictions *Restrictions `json:"dismissalRestrictions,omitempty"`
	// DismissStale overrides whether new commits automatically dismiss old reviews if set
	DismissStale *bool `json:"dismissStaleReviews,omitempty"`
	// RequireOwners overrides whether CODEOWNERS must approve PRs if set
	RequireOwners *bool `json:"requireCodeOwnerReviews,omitempty"`
	// Approvals overrides the number of approvals required if set (set to 0 to disable)
	Approvals *int `json:"requiredApprovingReviewCount,omitempty"`
}

// Restrictions limits who can merge
// Users and Teams entries are appended to parent lists.
type Restrictions struct {
	Users *ReplaceableSliceOfStrings `json:"users"`
	Teams *ReplaceableSliceOfStrings `json:"teams"`
}

// BranchProtectionContextPolicy configures required git provider contexts.
// Strict determines whether merging to the branch invalidates existing contexts.
type BranchProtectionContextPolicy struct {
	// Contexts appends required contexts that must be green to merge
	Contexts *ReplaceableSliceOfStrings `json:"contexts,omitempty"`
	// Strict overrides whether new commits in the base branch require updating the PR if set
	Strict *bool `json:"strict,omitempty"`
}

// SchedulerAgent defines the scheduler agent configuration
type SchedulerAgent struct {
	// Agent defines the agent used to schedule jobs, by default Prow
	Agent *string `json:"agent"`
}

// ProtectionPolicies defines the branch protection policies
type ProtectionPolicies struct {
	// +optional
	*ProtectionPolicy
	// +optional
	Replace bool
	Items   map[string]*ProtectionPolicy `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
}

// ReplaceableSliceOfExternalPlugins is a list of external plugins that can optionally completely replace the plugins
// in any parent SchedulerSpec
type ReplaceableSliceOfExternalPlugins struct {
	Replace bool
	Items   []*ExternalPlugin `json:"entries,omitempty" protobuf:"bytes,1,opt,name=entries"`
}
