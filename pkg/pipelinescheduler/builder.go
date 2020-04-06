package pipelinescheduler

import (
	"github.com/davecgh/go-spew/spew"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

//Build combines the slice of schedulers into one, with the most specific schedule config defined last
func Build(schedulers []*jenkinsv1.SchedulerSpec) (*jenkinsv1.SchedulerSpec, error) {
	var answer *jenkinsv1.SchedulerSpec
	for i := len(schedulers) - 1; i >= 0; i-- {
		parent := schedulers[i]
		if answer == nil {
			answer = parent
		} else {
			if answer.ScehdulerAgent == nil {
				answer.ScehdulerAgent = parent.ScehdulerAgent
			} else if parent.ScehdulerAgent != nil {
				applyToSchedulerAgent(parent.ScehdulerAgent, answer.ScehdulerAgent)
			}
			if answer.Policy == nil {
				answer.Policy = parent.Policy
			} else if parent.Policy != nil {
				applyToGlobalProtectionPolicy(parent.Policy, answer.Policy)
			}
			if answer.Presubmits == nil {
				answer.Presubmits = parent.Presubmits
			} else if !answer.Presubmits.Replace && parent.Presubmits != nil {
				err := applyToPreSubmits(parent.Presubmits, answer.Presubmits)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			}
			if answer.Postsubmits == nil {
				answer.Postsubmits = parent.Postsubmits
			} else if !answer.Postsubmits.Replace && parent.Postsubmits != nil {
				err := applyToPostSubmits(parent.Postsubmits, answer.Postsubmits)
				if err != nil {
					return nil, errors.WithStack(err)
				}
			}
			//TODO: This should probably be an array of triggers, because the plugins yaml is expecting an array
			if answer.Trigger == nil {
				answer.Trigger = parent.Trigger
			} else if parent.Trigger != nil {
				applyToTrigger(parent.Trigger, answer.Trigger)
			}
			if answer.Approve == nil {
				answer.Approve = parent.Approve
			} else if parent.Approve != nil {
				applyToApprove(parent.Approve, answer.Approve)
			}
			if answer.LGTM == nil {
				answer.LGTM = parent.LGTM
			} else if parent.LGTM != nil {
				applyToLgtm(parent.LGTM, answer.LGTM)
			}
			if answer.ExternalPlugins == nil {
				answer.ExternalPlugins = parent.ExternalPlugins
			} else if parent.ExternalPlugins != nil {
				applyToExternalPlugins(parent.ExternalPlugins, answer.ExternalPlugins)
			}
			if answer.Plugins == nil {
				answer.Plugins = parent.Plugins
			} else if parent.Plugins != nil {
				applyToReplaceableSliceOfStrings(parent.Plugins, answer.Plugins)
			}
			if answer.Merger == nil {
				answer.Merger = parent.Merger
			} else if parent.Merger != nil {
				applyToMerger(parent.Merger, answer.Merger)
			}
			if answer.Periodics == nil {
				answer.Periodics = parent.Periodics
			}
			if answer.Attachments == nil {
				answer.Attachments = parent.Attachments
			}
		}
	}
	return answer, nil
}

func applyToTrigger(parent *jenkinsv1.Trigger, child *jenkinsv1.Trigger) {
	if child.IgnoreOkToTest != nil {
		child.IgnoreOkToTest = parent.IgnoreOkToTest
	}
	if child.JoinOrgURL == nil {
		child.JoinOrgURL = parent.JoinOrgURL
	}
	if child.OnlyOrgMembers == nil {
		child.OnlyOrgMembers = parent.OnlyOrgMembers
	}
	if child.TrustedOrg == nil {
		child.TrustedOrg = parent.TrustedOrg
	}
}

func applyToSchedulerAgent(parent *jenkinsv1.SchedulerAgent, child *jenkinsv1.SchedulerAgent) {
	if child.Agent == nil {
		child.Agent = parent.Agent
	}
}

func applyToBrancher(parent *jenkinsv1.Brancher, child *jenkinsv1.Brancher) {
	if child.Branches == nil {
		child.Branches = parent.Branches
	} else if parent.Branches != nil {
		applyToReplaceableSliceOfStrings(parent.Branches, child.Branches)
	}
	if child.SkipBranches == nil {
		child.SkipBranches = parent.SkipBranches
	} else if parent.SkipBranches != nil {
		applyToReplaceableSliceOfStrings(parent.SkipBranches, child.SkipBranches)
	}
}

func applyToRegexpChangeMatcher(parent *jenkinsv1.RegexpChangeMatcher, child *jenkinsv1.RegexpChangeMatcher) {
	if child.RunIfChanged == nil {
		child.RunIfChanged = parent.RunIfChanged
	}
}

func applyToJobBase(parent *jenkinsv1.JobBase, child *jenkinsv1.JobBase) {
	if child.Name == nil {
		child.Name = parent.Name
	}
	if child.Namespace == nil {
		child.Namespace = parent.Namespace
	}
	if child.Agent == nil {
		child.Agent = parent.Agent
	}
	if child.Cluster == nil {
		child.Cluster = parent.Cluster
	}
	if child.MaxConcurrency == nil {
		child.MaxConcurrency = parent.MaxConcurrency
	}
	if child.Labels == nil {
		child.Labels = parent.Labels
	} else if !child.Labels.Replace && parent.Labels != nil {
		if child.Labels.Items == nil {
			child.Labels.Items = make(map[string]string)
		}
		// Add any labels that are missing
		for pk, pv := range parent.Labels.Items {
			if _, ok := child.Labels.Items[pk]; !ok {
				child.Labels.Items[pk] = pv
			}
		}
	}
}

func applyToMerger(parent *jenkinsv1.Merger, child *jenkinsv1.Merger) {
	if child.ContextPolicy == nil {
		child.ContextPolicy = parent.ContextPolicy
	} else if parent.ContextPolicy != nil {
		applyToContextPolicy(parent.ContextPolicy, child.ContextPolicy)
	}
	if child.MergeType == nil {
		child.MergeType = parent.MergeType
	}
	if child.MaxGoroutines == nil {
		child.MaxGoroutines = parent.MaxGoroutines
	}
	if child.SquashLabel == nil {
		child.SquashLabel = parent.SquashLabel
	}
	if child.BlockerLabel == nil {
		child.BlockerLabel = parent.BlockerLabel
	}
	if child.PRStatusBaseURL == nil {
		child.PRStatusBaseURL = parent.PRStatusBaseURL
	}
	if child.TargetURL == nil {
		child.TargetURL = parent.TargetURL
	}
	if child.SyncPeriod == nil {
		child.SyncPeriod = parent.SyncPeriod
	}
	if child.StatusUpdatePeriod == nil {
		child.StatusUpdatePeriod = parent.StatusUpdatePeriod
	}
}

// TODO use this
//func applyToReplaceableMapOfStringString(parent *jenkinsv1.ReplaceableMapOfStringString, child *jenkinsv1.ReplaceableMapOfStringString) {
//	if !child.Replace && parent != nil {
//		if child.Items == nil {
//			child.Items = make(map[string]string)
//		}
//		for pk, pv := range parent.Items {
//			if _, ok := child.Items[pk]; !ok {
//				child.Items[pk] = pv
//			}
//		}
//	}
//}

func applyToReplaceableSliceOfStrings(parent *jenkinsv1.ReplaceableSliceOfStrings, child *jenkinsv1.ReplaceableSliceOfStrings) {
	if !child.Replace && parent != nil {
		if child.Items == nil {
			child.Items = make([]string, 0)
		}
		for i := range parent.Items {
			if !util.Contains(child.Items, parent.Items[i]) {
				child.Items = append(child.Items, parent.Items[i])
			}
		}
	}
}

func applyToRepoContextPolicy(parent *jenkinsv1.RepoContextPolicy, child *jenkinsv1.RepoContextPolicy) {
	if child.ContextPolicy == nil {
		child.ContextPolicy = parent.ContextPolicy
	} else if parent.ContextPolicy != nil {
		applyToContextPolicy(parent.ContextPolicy, child.ContextPolicy)
	}
	if child.Branches == nil {
		child.Branches = parent.Branches
	} else if !child.Branches.Replace && parent.Branches != nil {
		if child.Branches.Items == nil {
			child.Branches.Items = make(map[string]*jenkinsv1.ContextPolicy)
		}
		for pk, pv := range parent.Branches.Items {
			if cv, ok := child.Branches.Items[pk]; !ok {
				child.Branches.Items[pk] = pv
			} else if pv != nil {
				applyToContextPolicy(pv, cv)
			}
		}
	}
}

func applyToContextPolicy(parent *jenkinsv1.ContextPolicy, child *jenkinsv1.ContextPolicy) {
	if child.FromBranchProtection == nil {
		child.FromBranchProtection = parent.FromBranchProtection
	}
	if child.SkipUnknownContexts == nil {
		child.SkipUnknownContexts = parent.SkipUnknownContexts
	}
	if child.OptionalContexts == nil {
		child.OptionalContexts = parent.OptionalContexts
	} else if parent.OptionalContexts != nil {
		applyToReplaceableSliceOfStrings(parent.OptionalContexts, child.OptionalContexts)
	}
	if child.RequiredContexts == nil {
		child.RequiredContexts = parent.RequiredContexts
	} else if parent.RequiredContexts != nil {
		applyToReplaceableSliceOfStrings(parent.RequiredContexts, child.RequiredContexts)
	}
	if child.RequiredIfPresentContexts == nil {
		child.RequiredIfPresentContexts = parent.RequiredIfPresentContexts
	} else if parent.RequiredIfPresentContexts != nil {
		applyToReplaceableSliceOfStrings(parent.RequiredIfPresentContexts, child.RequiredIfPresentContexts)
	}
}

func applyToLgtm(parent *jenkinsv1.Lgtm, child *jenkinsv1.Lgtm) {
	if child.StickyLgtmTeam == nil {
		child.StickyLgtmTeam = parent.StickyLgtmTeam
	}
	if child.ReviewActsAsLgtm == nil {
		child.ReviewActsAsLgtm = parent.ReviewActsAsLgtm
	}
	if child.StoreTreeHash == nil {
		child.StoreTreeHash = parent.StoreTreeHash
	}
}

func applyToExternalPlugins(parent *jenkinsv1.ReplaceableSliceOfExternalPlugins, child *jenkinsv1.ReplaceableSliceOfExternalPlugins) {
	if child.Items == nil {
		child.Items = parent.Items
	} else if !child.Replace {
		child.Items = append(child.Items, parent.Items...)
	}
}

// TODO use this
//func applyToExternalPlugin(parent *jenkinsv1.ExternalPlugin, child *jenkinsv1.ExternalPlugin) {
//	if child.Name == nil {
//		child.Name = parent.Name
//	}
//	if child.Endpoint == nil {
//		child.Endpoint = parent.Endpoint
//	}
//	if child.Events == nil {
//		child.Events = parent.Events
//	} else if parent.Events != nil {
//		applyToReplaceableSliceOfStrings(parent.Events, child.Events)
//	}
//}

func applyToApprove(parent *jenkinsv1.Approve, child *jenkinsv1.Approve) {
	if child.IgnoreReviewState == nil {
		child.IgnoreReviewState = parent.IgnoreReviewState
	}
	if child.IssueRequired == nil {
		child.IssueRequired = parent.IssueRequired
	}
	if child.LgtmActsAsApprove == nil {
		child.LgtmActsAsApprove = parent.LgtmActsAsApprove
	}
	if child.RequireSelfApproval == nil {
		child.RequireSelfApproval = parent.RequireSelfApproval
	}
}

func applyToGlobalProtectionPolicy(parent *jenkinsv1.GlobalProtectionPolicy, child *jenkinsv1.GlobalProtectionPolicy) {
	if child.ProtectionPolicy == nil {
		child.ProtectionPolicy = parent.ProtectionPolicy
	} else if parent.ProtectionPolicy != nil {
		applyToProtectionPolicy(parent.ProtectionPolicy, child.ProtectionPolicy)
	}
	if child.ProtectTested == nil {
		child.ProtectTested = parent.ProtectTested
	}
}

func applyToProtectionPolicy(parent *jenkinsv1.ProtectionPolicy, child *jenkinsv1.ProtectionPolicy) {
	if child.Protect == nil {
		child.Protect = parent.Protect
	}
	if child.Admins == nil {
		child.Admins = parent.Admins
	}
	if child.Restrictions == nil {
		child.Restrictions = parent.Restrictions
	} else if parent.Restrictions != nil {
		applyToRestrictions(parent.Restrictions, child.Restrictions)
	}
	if child.RequiredPullRequestReviews == nil {
		child.RequiredPullRequestReviews = parent.RequiredPullRequestReviews
	} else if parent.RequiredPullRequestReviews != nil {
		applyToRequiredPullRequestReviews(parent.RequiredPullRequestReviews, child.RequiredPullRequestReviews)
	}
}

func applyToRequiredPullRequestReviews(parent *jenkinsv1.ReviewPolicy, child *jenkinsv1.ReviewPolicy) {
	if child.Approvals == nil {
		child.Approvals = parent.Approvals
	}
	if child.DismissStale == nil {
		child.DismissStale = parent.DismissStale
	}
	if child.RequireOwners == nil {
		child.RequireOwners = parent.RequireOwners
	}
	if child.DismissalRestrictions == nil {
		child.DismissalRestrictions = parent.DismissalRestrictions
	} else if parent.DismissalRestrictions != nil {
		applyToRestrictions(parent.DismissalRestrictions, child.DismissalRestrictions)
	}
}

func applyToRestrictions(parent *jenkinsv1.Restrictions, child *jenkinsv1.Restrictions) {
	if child.Teams == nil {
		child.Teams = parent.Teams
	} else if parent.Teams != nil {
		applyToReplaceableSliceOfStrings(parent.Teams, child.Teams)
	}
	if child.Users == nil {
		child.Users = parent.Users
	} else if parent.Users != nil {
		applyToReplaceableSliceOfStrings(parent.Users, child.Users)
	}
}

func applyToPostSubmits(parentPostsubmits *jenkinsv1.Postsubmits, childPostsubmits *jenkinsv1.Postsubmits) error {
	if childPostsubmits.Items == nil {
		childPostsubmits.Items = make([]*jenkinsv1.Postsubmit, 0)
	}
	// Work through each of the post submits in the parent. If we can find a name based match in child,
	// we apply it to the child, otherwise we append it
	for _, parent := range parentPostsubmits.Items {
		var found []*jenkinsv1.Postsubmit
		for _, postsubmit := range childPostsubmits.Items {
			if postsubmit.Name != nil && parent.Name != nil && *postsubmit.Name == *parent.Name {
				found = append(found, postsubmit)
			}
		}
		if len(found) > 1 {
			return errors.Errorf("more than one postsubmit with name %v in %s", *parent.Name, spew.Sdump(childPostsubmits))
		} else if len(found) == 1 {
			child := found[0]
			// Neither parent's nor child's JobBase can be nil as it would've panicked earlier
			applyToJobBase(parent.JobBase, child.JobBase)
			if child.RegexpChangeMatcher == nil {
				child.RegexpChangeMatcher = parent.RegexpChangeMatcher
			} else if parent.RegexpChangeMatcher != nil {
				applyToRegexpChangeMatcher(parent.RegexpChangeMatcher, child.RegexpChangeMatcher)
			}
			if child.Brancher == nil {
				if parent.Brancher == nil {
					child.Brancher = &jenkinsv1.Brancher{}
				}
				child.Brancher = parent.Brancher
			} else if parent.Brancher != nil {
				applyToBrancher(parent.Brancher, child.Brancher)
			}
			if child.Context == nil {
				child.Context = parent.Context
			}
			if child.Report == nil {
				child.Report = parent.Report
			}
		} else {
			childPostsubmits.Items = append(childPostsubmits.Items, parent)
		}
	}
	return nil
}

func applyToPreSubmits(parentPresubmits *jenkinsv1.Presubmits, childPresubmits *jenkinsv1.Presubmits) error {
	if childPresubmits.Items == nil {
		childPresubmits.Items = make([]*jenkinsv1.Presubmit, 0)
	}
	// Work through each of the presubmits in the parent. If we can find a name based match in child,
	// we apply it to the child, otherwise we append it
	for _, parent := range parentPresubmits.Items {
		var found []*jenkinsv1.Presubmit
		for _, child := range childPresubmits.Items {
			if child.Name == parent.Name {
				found = append(found, child)
			}
		}
		if len(found) > 1 {
			return errors.Errorf("more than one presubmit with name %v in %s", parent.Name, spew.Sdump(parentPresubmits))
		} else if len(found) == 1 {
			child := found[0]
			// Neither parent's nor child's JobBase can be nil as it would've panicked earlier
			applyToJobBase(parent.JobBase, child.JobBase)
			if child.RegexpChangeMatcher == nil {
				child.RegexpChangeMatcher = parent.RegexpChangeMatcher
			} else if parent.RegexpChangeMatcher != nil {
				applyToRegexpChangeMatcher(parent.RegexpChangeMatcher, child.RegexpChangeMatcher)
			}
			if child.Brancher == nil {
				if parent.Brancher == nil {
					child.Brancher = &jenkinsv1.Brancher{}
				} else {
					child.Brancher = parent.Brancher
				}
			} else if parent.Brancher != nil {
				applyToBrancher(parent.Brancher, child.Brancher)
			}
			if child.Context == nil {
				child.Context = parent.Context
			}
			if child.Report == nil {
				child.Report = parent.Report
			}
			if child.AlwaysRun == nil {
				child.AlwaysRun = parent.AlwaysRun
			}
			if child.Optional == nil {
				child.Optional = parent.Optional
			}
			if child.Trigger == nil {
				child.Trigger = parent.Trigger
			}
			if child.RerunCommand == nil {
				child.RerunCommand = parent.RerunCommand
			}
			if child.MergeType == nil {
				child.MergeType = parent.MergeType
			}
			if child.ContextPolicy == nil {
				child.ContextPolicy = parent.ContextPolicy
			} else if parent.ContextPolicy != nil {
				applyToRepoContextPolicy(parent.ContextPolicy, child.ContextPolicy)
			}
			if child.Branches == nil {
				child.Branches = parent.Branches
			} else if parent.Branches != nil {
				applyToReplaceableSliceOfStrings(parent.Branches, child.Branches)
			}
			if child.Policy == nil {
				child.Policy = parent.Policy
			} else if parent.Policy != nil {
				applyToProtectionPolicies(parent.Policy, child.Policy)
			}
			if child.Queries == nil {
				child.Queries = parent.Queries
			} else if parent.Queries != nil {
				applyToQueries(parent.Queries, child.Queries)
			}
		}
	}
	return nil
}

func applyToProtectionPolicies(parent *jenkinsv1.ProtectionPolicies,
	child *jenkinsv1.ProtectionPolicies) {
	if child.ProtectionPolicy == nil {
		child.ProtectionPolicy = parent.ProtectionPolicy
	} else if parent.ProtectionPolicy != nil {
		applyToProtectionPolicy(parent.ProtectionPolicy, child.ProtectionPolicy)
	}
	if child.Items == nil {
		child.Items = parent.Items
	} else if !child.Replace {
		for k, v := range parent.Items {
			if _, ok := child.Items[k]; !ok {
				child.Items[k] = v
			}
		}
	}
}

func applyToQueries(parents []*jenkinsv1.Query, children []*jenkinsv1.Query) {
	for _, v := range parents {
		children = append(children, v)
	}
}
