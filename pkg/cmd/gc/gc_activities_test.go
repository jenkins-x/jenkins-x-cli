// +build unit

package gc

import (
	"testing"
	"time"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	tektonv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	prowjobv1 "k8s.io/test-infra/prow/apis/prowjobs/v1"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
)

func TestGCPipelineActivitiesWithBatchAndPRBuilds(t *testing.T) {
	t.Parallel()

	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	o := &GCActivitiesOptions{
		CommonOptions:           options,
		PullRequestAgeLimit:     time.Hour * 24 * 30,
		ReleaseAgeLimit:         time.Hour * 48,
		ReleaseHistoryLimit:     5,
		PullRequestHistoryLimit: 2,
		PipelineRunAgeLimit:     time.Hour * 2,
		ProwJobAgeLimit:         time.Hour * 24 * 7,
	}

	jxClient, ns, err := options.JXClientAndDevNamespace()
	assert.NoError(t, err)
	tektonClient, ns, err := options.TektonClient()
	assert.NoError(t, err)
	prowJobClient, ns, err := options.ProwJobClient()
	assert.NoError(t, err)

	err = options.ModifyDevEnvironment(func(env *v1.Environment) error {
		env.Spec.TeamSettings.PromotionEngine = jenkinsv1.PromotionEngineProw
		return nil
	})
	assert.NoError(t, err)

	nowMinusThirtyOneDays := time.Now().AddDate(0, 0, -31)
	nowMinusThreeDays := time.Now().AddDate(0, 0, -3)
	nowMinusTwoDays := time.Now().AddDate(0, 0, -2)
	nowMinusOneDay := time.Now().AddDate(0, 0, -1)
	nowMinusOneHour := time.Now().Add(-1 * time.Hour)
	nowMinusThreeHours := time.Now().Add(-3 * time.Hour)

	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "1",
			Labels: map[string]string{
				v1.LabelBranch: "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline:           "org/project/PR-1",
			CompletedTimestamp: &metav1.Time{Time: nowMinusThreeDays},
		},
	})
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "2",
			Labels: map[string]string{
				v1.LabelBranch: "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline: "org/project/PR-1",
			// No completion time, to make sure this doesn't get deleted.
		},
	})
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "3",
			Labels: map[string]string{
				v1.LabelBranch: "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline:           "org/project/PR-1",
			CompletedTimestamp: &metav1.Time{Time: nowMinusTwoDays},
		},
	})
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "4",
			Labels: map[string]string{
				v1.LabelBranch: "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline:           "org/project/PR-1",
			CompletedTimestamp: &metav1.Time{Time: nowMinusOneDay},
		},
	})
	assert.NoError(t, err)

	// To handle potential weirdness around ordering, make sure that the oldest PR activity is in a random
	// spot in the order.
	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "0",
			Labels: map[string]string{
				v1.LabelBranch: "PR-1",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline:           "org/project/PR-1",
			CompletedTimestamp: &metav1.Time{Time: nowMinusThirtyOneDays},
		},
	})
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "5",
			Labels: map[string]string{
				v1.LabelBranch: "batch",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline:           "org/project/batch",
			CompletedTimestamp: &metav1.Time{Time: nowMinusThreeDays},
		},
	})
	assert.NoError(t, err)

	_, err = jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: "6",
			Labels: map[string]string{
				v1.LabelBranch: "master",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Pipeline:           "org/project/master",
			CompletedTimestamp: &metav1.Time{Time: nowMinusThreeDays},
		},
	})
	assert.NoError(t, err)

	_, err = tektonClient.TektonV1alpha1().PipelineRuns(ns).Create(&tektonv1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "run1",
		},
		Status: tektonv1alpha1.PipelineRunStatus{
			CompletionTime: &metav1.Time{Time: nowMinusThreeHours},
		},
	})
	assert.NoError(t, err)
	_, err = tektonClient.TektonV1alpha1().PipelineRuns(ns).Create(&tektonv1alpha1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "run2",
		},
		Status: tektonv1alpha1.PipelineRunStatus{
			CompletionTime: &metav1.Time{Time: nowMinusOneHour},
		},
	})
	assert.NoError(t, err)

	_, err = prowJobClient.ProwV1().ProwJobs(ns).Create(&prowjobv1.ProwJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job1",
		},
		Status: prowjobv1.ProwJobStatus{
			CompletionTime: &metav1.Time{Time: nowMinusThirtyOneDays},
		},
	})
	assert.NoError(t, err)
	_, err = prowJobClient.ProwV1().ProwJobs(ns).Create(&prowjobv1.ProwJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job2",
		},
		Status: prowjobv1.ProwJobStatus{},
	})
	assert.NoError(t, err)
	_, err = prowJobClient.ProwV1().ProwJobs(ns).Create(&prowjobv1.ProwJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: "job3",
		},
		Status: prowjobv1.ProwJobStatus{
			CompletionTime: &metav1.Time{Time: nowMinusThreeDays},
		},
	})
	assert.NoError(t, err)

	err = o.Run()
	assert.NoError(t, err)

	activities, err := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
	assert.NoError(t, err)

	assert.Len(t, activities.Items, 4, "Two of the activities should've been garbage collected")

	var verifier []bool
	for _, v := range activities.Items {
		if v.BranchName() == "batch" || v.BranchName() == "PR-1" {
			verifier = append(verifier, true)
		}
	}
	assert.Len(t, verifier, 4, "Both PR and Batch builds should've been verified")

	runs, err := tektonClient.TektonV1alpha1().PipelineRuns(ns).List(metav1.ListOptions{})
	assert.NoError(t, err)

	assert.Len(t, runs.Items, 1, "One PipelineRun should've been garbage collected")

	remainingRun := runs.Items[0]
	assert.NotNil(t, remainingRun.Status.CompletionTime)
	assert.Equal(t, nowMinusOneHour, remainingRun.Status.CompletionTime.Time, "Expected completion time for remaining PipelineRun of %s, but is %s", nowMinusOneHour, remainingRun.Status.CompletionTime.Time)

	jobs, err := prowJobClient.ProwV1().ProwJobs(ns).List(metav1.ListOptions{})
	assert.NoError(t, err)

	assert.Len(t, jobs.Items, 2, "One of three ProwJobs should've been garbage collected")

	var job1 *prowjobv1.ProwJob
	var job2 *prowjobv1.ProwJob
	var job3 *prowjobv1.ProwJob

	for _, job := range jobs.Items {
		if job.Name == "job1" {
			job1 = &job
		} else if job.Name == "job2" {
			job2 = &job
		} else if job.Name == "job3" {
			job3 = &job
		}
	}
	assert.Nil(t, job1, "ProwJob job1 completed more than 7 days ago and so should have been deleted")
	assert.NotNil(t, job2, "ProwJob job2 has no completion time and so shouldn't have been deleted")
	assert.NotNil(t, job3, "ProwJob job3 completed less than 7 days ago and so shouldn't have been deleted")
}
