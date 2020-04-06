// +build unit

package pipelinescheduler_test

import (
	"testing"

	"github.com/pborman/uuid"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/jx/pkg/pipelinescheduler"
	"github.com/jenkins-x/jx/pkg/pipelinescheduler/testhelpers"
	"github.com/stretchr/testify/assert"
)

func TestBuildWithEverythingInParent(t *testing.T) {
	child := &v1.SchedulerSpec{
		// Override nothing, everything comes from
	}
	parent := testhelpers.CompleteScheduler()
	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, parent, merged)
}

func TestBuildWithEverythingInChild(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, child, merged)
}

func TestBuildWithSomePropertiesMergedLgtm(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	child.LGTM.ReviewActsAsLgtm = nil
	parent := testhelpers.CompleteScheduler()
	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, parent.LGTM.ReviewActsAsLgtm, merged.LGTM.ReviewActsAsLgtm)
	assert.Equal(t, child.LGTM.StickyLgtmTeam, merged.LGTM.StickyLgtmTeam)
}

func TestBuildWithLgtmEmptyInChild(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	child.LGTM = &v1.Lgtm{}
	parent := testhelpers.CompleteScheduler()
	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, child.LGTM, merged.LGTM)
}

func TestBuildWithSomePropertiesMergedMerger(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	child.Merger.ContextPolicy = nil
	child.Merger.MergeType = nil
	parent := testhelpers.CompleteScheduler()
	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, parent.Merger.ContextPolicy, merged.Merger.ContextPolicy)
	assert.Equal(t, parent.Merger.MergeType, merged.Merger.MergeType)
	assert.Equal(t, child.Merger.SquashLabel, merged.Merger.SquashLabel)
}

func TestBuildWithEmptyMerger(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	child.Merger = &v1.Merger{}
	parent := testhelpers.CompleteScheduler()
	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, parent.Merger, merged.Merger)
}

func TestPostSubmitWithEmptyChildPostSubmit(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Postsubmits = &v1.Postsubmits{
		Items: []*v1.Postsubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Postsubmits.Items[0].Name,
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(merged.Postsubmits.Items))
	postSubmit := merged.Postsubmits.Items[0]
	assert.NotNil(t, postSubmit.Branches)
	assert.NotNil(t, postSubmit.Report)
	assert.NotNil(t, postSubmit.Context)
}

func TestPostSubmitMultipleChildrenWithSameNameFound(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Postsubmits = &v1.Postsubmits{
		Items: []*v1.Postsubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Postsubmits.Items[0].Name,
				},
			},
			{
				JobBase: &v1.JobBase{
					Name: parent.Postsubmits.Items[0].Name,
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than one postsubmit with name")
	assert.Nil(t, merged)
}

func TestPostSubmitWithMergedJobBaseLabels(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Postsubmits = &v1.Postsubmits{
		Items: []*v1.Postsubmit{
			{
				JobBase: &v1.JobBase{
					Name:   parent.Postsubmits.Items[0].Name,
					Labels: testhelpers.PointerToReplaceableMapOfStringString(),
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(merged.Postsubmits.Items))
	postSubmit := merged.Postsubmits.Items[0]
	var labelKey string
	for k := range parent.Presubmits.Items[0].Labels.Items {
		labelKey = k
		break
	}
	assert.Equal(t, postSubmit.Labels.Items[labelKey], parent.Postsubmits.Items[0].Labels.Items[labelKey])
	assert.Len(t, postSubmit.Labels.Items, 2)
}

func TestPostSubmitWithNilJobBaseLabels(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Postsubmits = &v1.Postsubmits{
		Items: []*v1.Postsubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Postsubmits.Items[0].Name,
					Labels: &v1.ReplaceableMapOfStringString{
						Items: nil,
					},
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(merged.Postsubmits.Items))
	postSubmit := merged.Postsubmits.Items[0]
	assert.EqualValues(t, postSubmit.Labels.Items, parent.Postsubmits.Items[0].Labels.Items)
}

func TestPostSubmitApplyBrancherWithEmptyChildPostSubmitBrancher(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Postsubmits = &v1.Postsubmits{
		Items: []*v1.Postsubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Postsubmits.Items[0].Name,
				},
				Brancher: &v1.Brancher{},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(merged.Postsubmits.Items))
	assert.Equal(t, parent.Postsubmits.Items[0].Branches, merged.Postsubmits.Items[0].Brancher.Branches)
	assert.Equal(t, parent.Postsubmits.Items[0].SkipBranches, merged.Postsubmits.Items[0].Brancher.SkipBranches)
}

func TestPostSubmitApplyBrancherWithAppendingChildPostSubmitBrancher(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Postsubmits = &v1.Postsubmits{
		Items: []*v1.Postsubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Postsubmits.Items[0].Name,
				},
				Brancher: &v1.Brancher{
					Branches:     testhelpers.PointerToReplaceableSliceOfStrings(),
					SkipBranches: testhelpers.PointerToReplaceableSliceOfStrings(),
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(merged.Postsubmits.Items))
	assert.Equal(t, 2, len(merged.Postsubmits.Items[0].Brancher.Branches.Items))
	assert.Equal(t, 2, len(merged.Postsubmits.Items[0].Brancher.SkipBranches.Items))
}

func TestPreSubmitWithEmptyChildPreSubmit(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	runIfChanged := ""
	child.Presubmits = &v1.Presubmits{
		Items: []*v1.Presubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
				RegexpChangeMatcher: &v1.RegexpChangeMatcher{
					RunIfChanged: &runIfChanged,
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(merged.Presubmits.Items))
	preSubmit := merged.Presubmits.Items[0]
	assert.NotNil(t, preSubmit.Branches)
	assert.NotNil(t, preSubmit.AlwaysRun)
	assert.NotNil(t, preSubmit.ContextPolicy)
	assert.NotNil(t, preSubmit.Queries)
}

func TestPreSubmitApplyToQueryWithEmptyChildQuery(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Presubmits = &v1.Presubmits{
		Items: []*v1.Presubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
				RegexpChangeMatcher: &v1.RegexpChangeMatcher{},
				Queries:             []*v1.Query{{}},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	preSubmit := merged.Presubmits.Items[0]
	assert.True(t, len(preSubmit.Queries) == 1)
}

func TestPreSubmitApplyToProtectionPolicyWithAppendingChildPolicy(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Presubmits = &v1.Presubmits{
		Items: []*v1.Presubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
				RegexpChangeMatcher: &v1.RegexpChangeMatcher{},
				Policy: &v1.ProtectionPolicies{
					Items: map[string]*v1.ProtectionPolicy{
						"policy1": {},
					},
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(merged.Presubmits.Items[0].Policy.Items))
	var protectionPolicyKey string
	for k := range parent.Presubmits.Items[0].Policy.Items {
		protectionPolicyKey = k
		break
	}
	assert.Equal(t, parent.Presubmits.Items[0].Policy.Items[protectionPolicyKey],
		merged.Presubmits.Items[0].Policy.Items[protectionPolicyKey])
}

func TestPreSubmitApplyToRepoContextPolicyWithEmptyChildRepoContextPolicy(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Presubmits = &v1.Presubmits{
		Items: []*v1.Presubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
				RegexpChangeMatcher: &v1.RegexpChangeMatcher{},
				ContextPolicy:       &v1.RepoContextPolicy{},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.Equal(t, parent.Presubmits.Items[0].ContextPolicy.Branches,
		merged.Presubmits.Items[0].ContextPolicy.Branches)
	assert.Equal(t, parent.Presubmits.Items[0].ContextPolicy, merged.Presubmits.Items[0].ContextPolicy)
}

func TestPreSubmitApplyToRepoContextPolicyWithAppendingChildRepoContextPolicy(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Presubmits = &v1.Presubmits{
		Items: []*v1.Presubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
				RegexpChangeMatcher: &v1.RegexpChangeMatcher{},
				ContextPolicy: &v1.RepoContextPolicy{
					Branches: &v1.ReplaceableMapOfStringContextPolicy{
						Items: map[string]*v1.ContextPolicy{
							uuid.New(): {},
						},
					},
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.NoError(t, err)
	assert.NotEqual(t, parent.Presubmits.Items[0].ContextPolicy.Branches,
		merged.Presubmits.Items[0].ContextPolicy.Branches)
	assert.Len(t, merged.Presubmits.Items[0].ContextPolicy.Branches.Items, 2)
}

func TestPreSubmitMultipleChildrenWithSameNameFound(t *testing.T) {
	t.Parallel()
	child := testhelpers.CompleteScheduler()
	parent := testhelpers.CompleteScheduler()
	child.Presubmits = &v1.Presubmits{
		Items: []*v1.Presubmit{
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
			},
			{
				JobBase: &v1.JobBase{
					Name: parent.Presubmits.Items[0].Name,
				},
			},
		},
	}

	merged, err := pipelinescheduler.Build([]*v1.SchedulerSpec{parent, child})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "more than one presubmit with name ")
	assert.Nil(t, merged)
}
