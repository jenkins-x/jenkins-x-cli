package pipelinescheduler

import "time"

// TODO wire in this file

// ProwConfig is the SchedulerSpec config that relates explicitly to Prow
type ProwConfig struct {
	Reviewers Reviewers `yaml:"blunderbuss,omitempty"`
	Owners    Owners    `json:"owners,omitempty"`
	// DefaultNamespace defines the namespace to run the jobs, by default the team namespace
	DefaultNamespace string `yaml:"jobNamespace,omitempty"`
	// TODO PushGateway
	DefaultOwnersDirBlacklist DefaultOwnersDirBlacklist `yaml:"defaultOwnersDirBlacklist,omitempty"`
	GarbageCollection         GarbageCollection         `yaml:"garbageCollection,omitempty"`
	Heart                     Heart                     `yaml:"heart,omitempty"`
}

// Heart contains the configuration for adding emojis
type Heart struct {
	// Adorees is a list of GitHub logins for members
	// for whom we will add emojis to comments
	Adorees []string `json:"adorees,omitempty"`
	// CommentRegexp is the regular expression for comments
	// made by adorees that the plugin adds emojis to.
	// If not specified, the plugin will not add emojis to
	// any comments.
	// Compiles into CommentRe during config load.
	CommentRegexp string `json:"commentregexp,omitempty"`
}

// GarbageCollection defines the configuration for cleaning up pipeline related resources
type GarbageCollection struct {
	// Interval is how often a Garbage Collection will be performed. Defaults to one hour.
	Interval time.Duration `json:"-"`
	// PipelineAge is how old a Pipeline can be before it is garbage-collected.
	// Defaults to one week.
	PipelineAge time.Duration `json:"-"`
	// PodAge is how old a Pod can be before it is garbage-collected.
	// Defaults to one day.
	PodAge time.Duration `json:"-"`
}

// Reviewers defines configuration for PR review
type Reviewers struct {
	// ReviewerCount is the minimum number of reviewers to request
	// reviews from. Defaults to requesting reviews from 2 reviewers
	// if FileWeightCount is not set.
	ReviewerCount *int `json:"request_count,omitempty"`
	// MaxReviewerCount is the maximum number of reviewers to request
	// reviews from. Defaults to 0 meaning no limit.
	MaxReviewerCount int `json:"max_request_count,omitempty"`
	// FileWeightCount is the maximum number of reviewers to request
	// reviews from. Selects reviewers based on file weighting.
	// This and request_count are mutually exclusive options.
	FileWeightCount *int `json:"file_weight_count,omitempty"`
	// ExcludeApprovers controls whether approvers are considered to be
	// reviewers. By default, approvers are considered as reviewers if
	// insufficient reviewers are available. If ExcludeApprovers is true,
	// approvers will never be considered as reviewers.
	ExcludeApprovers bool `json:"exclude_approvers,omitempty"`
}

// Owners contains configuration related to handling OWNERS files.
type Owners struct {
	// SkipCollaborators disables collaborator cross-checks and forces both
	// the approve and lgtm plugins to use solely OWNERS files for access
	// control in the provided repos.
	SkipCollaborators []string `json:"skip_collaborators,omitempty"`

	// LabelsBlackList holds a list of labels that should not be present in any
	// OWNERS file, preventing their automatic addition by the owners-label plugin.
	// This check is performed by the verify-owners plugin.
	LabelsBlackList []string `json:"labels_blacklist,omitempty"`
}

// DefaultOwnersDirBlacklist is the default blacklist
type DefaultOwnersDirBlacklist struct {
	// Blacklist configures a default blacklist for repos (or orgs) not
	// specifically configured
	Blacklist []string `json:"default"`
}

// TODO Deck
