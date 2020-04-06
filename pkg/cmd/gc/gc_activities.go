package gc

import (
	"sort"
	"strings"
	"time"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/pkg/errors"
	prowjobv1 "k8s.io/test-infra/prow/apis/prowjobs/v1"

	jv1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	tektonv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/typed/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	prowjobclient "k8s.io/test-infra/prow/client/clientset/versioned/typed/prowjobs/v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
)

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type GCActivitiesOptions struct {
	*opts.CommonOptions

	DryRun                  bool
	ReleaseHistoryLimit     int
	PullRequestHistoryLimit int
	ReleaseAgeLimit         time.Duration
	PullRequestAgeLimit     time.Duration
	PipelineRunAgeLimit     time.Duration
	ProwJobAgeLimit         time.Duration
	jclient                 gojenkins.JenkinsClient
}

var (
	GCActivitiesLong = templates.LongDesc(`
		Garbage collect the Jenkins X PipelineActivity and PipelineRun resources

`)

	GCActivitiesExample = templates.Examples(`
		# garbage collect PipelineActivity and PipelineRun resources
		jx gc activities

		# dry run mode
		jx gc pa --dry-run
`)
)

type buildCounter struct {
	ReleaseCount int
	PRCount      int
}

type buildsCount struct {
	cache map[string]*buildCounter
}

// AddBuild adds the build and returns the number of builds for this repo and branch
func (c *buildsCount) AddBuild(repoAndBranch string, isPR bool) int {
	if c.cache == nil {
		c.cache = map[string]*buildCounter{}
	}
	bc := c.cache[repoAndBranch]
	if bc == nil {
		bc = &buildCounter{}
		c.cache[repoAndBranch] = bc
	}
	if isPR {
		bc.PRCount++
		return bc.PRCount
	}
	bc.ReleaseCount++
	return bc.ReleaseCount
}

// NewCmd s a command object for the "step" command
func NewCmdGCActivities(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GCActivitiesOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "activities",
		Aliases: []string{"pa", "act", "pr"},
		Short:   "garbage collection for PipelineActivities and PipelineRun resources",
		Long:    GCActivitiesLong,
		Example: GCActivitiesExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.DryRun, "dry-run", "d", false, "Dry run mode. If enabled just list the resources that would be removed")
	cmd.Flags().IntVarP(&options.ReleaseHistoryLimit, "release-history-limit", "l", 5, "Maximum number of PipelineActivities to keep around per repository release")
	cmd.Flags().IntVarP(&options.PullRequestHistoryLimit, "pr-history-limit", "", 2, "Minimum number of PipelineActivities to keep around per repository Pull Request")
	cmd.Flags().DurationVarP(&options.PullRequestAgeLimit, "pull-request-age", "p", time.Hour*48, "Maximum age to keep PipelineActivities for Pull Requests")
	cmd.Flags().DurationVarP(&options.ReleaseAgeLimit, "release-age", "r", time.Hour*24*30, "Maximum age to keep PipelineActivities for Releases")
	cmd.Flags().DurationVarP(&options.PipelineRunAgeLimit, "pipelinerun-age", "", time.Hour*2, "Maximum age to keep completed PipelineRuns for all pipelines")
	cmd.Flags().DurationVarP(&options.ProwJobAgeLimit, "prowjob-age", "", time.Hour*24*7, "Maximum age to keep completed ProwJobs for all pipelines")
	return cmd
}

// Run implements this command
func (o *GCActivitiesOptions) Run() error {
	client, currentNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	prowEnabled, err := o.IsProw()
	if err != nil {
		return err
	}

	// cannot use field selectors like `spec.kind=Preview` on CRDs so list all environments
	activityInterface := client.JenkinsV1().PipelineActivities(currentNs)
	activities, err := activityInterface.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(activities.Items) == 0 {
		// no preview environments found so lets return gracefully
		log.Logger().Debug("no activities found")
		return nil
	}

	var jobNames []string
	if !prowEnabled {
		o.jclient, err = o.JenkinsClient()
		if err != nil {
			return err
		}

		jobs, err := o.jclient.GetJobs()
		if err != nil {
			return err
		}
		for _, j := range jobs {
			err = o.GetAllPipelineJobNames(o.jclient, &jobNames, j.Name)
			if err != nil {
				return err
			}
		}
	}

	now := time.Now()
	counters := &buildsCount{}

	var completedActivities []v1.PipelineActivity

	// Filter out running activities
	for _, a := range activities.Items {
		if a.Spec.CompletedTimestamp != nil {
			completedActivities = append(completedActivities, a)
		}
	}

	// Sort with newest created activities first
	sort.Slice(completedActivities, func(i, j int) bool {
		return !completedActivities[i].Spec.CompletedTimestamp.Before(completedActivities[j].Spec.CompletedTimestamp)
	})

	//
	for _, a := range completedActivities {
		branchName := a.BranchName()
		isPR, isBatch := o.isPullRequestOrBatchBranch(branchName)
		maxAge, revisionHistory := o.ageAndHistoryLimits(isPR, isBatch)
		// lets remove activities that are too old
		if a.Spec.CompletedTimestamp != nil && a.Spec.CompletedTimestamp.Add(maxAge).Before(now) {
			err = o.deleteActivity(activityInterface, &a)
			if err != nil {
				return err
			}
			continue
		}

		repoBranchAndContext := a.RepositoryOwner() + "/" + a.RepositoryName() + "/" + a.BranchName() + "/" + a.Spec.Context
		c := counters.AddBuild(repoBranchAndContext, isPR)
		if c > revisionHistory && a.Spec.CompletedTimestamp != nil {
			err = o.deleteActivity(activityInterface, &a)
			if err != nil {
				return err
			}
			continue
		}

		if !prowEnabled {
			// if activity has no job in Jenkins delete it
			matched := false
			for _, j := range jobNames {
				if a.Spec.Pipeline == j {
					matched = true
					break
				}
			}
			if !matched {
				err = o.deleteActivity(activityInterface, &a)
				if err != nil {
					return err
				}
			}
		}
	}

	// Clean up completed PipelineRuns
	err = o.gcPipelineRuns(currentNs)
	if err != nil {
		return err
	}

	// Clean up completed ProwJobs
	err = o.gcProwJobs(currentNs)
	if err != nil {
		return err
	}

	return nil
}

func (o *GCActivitiesOptions) deleteActivity(activityInterface jv1.PipelineActivityInterface, a *v1.PipelineActivity) error {
	prefix := ""
	if o.DryRun {
		prefix = "not "
	}
	log.Logger().Infof("%sdeleting PipelineActivity %s", prefix, util.ColorInfo(a.Name))
	if o.DryRun {
		return nil
	}
	return activityInterface.Delete(a.Name, metav1.NewDeleteOptions(0))
}

func (o *GCActivitiesOptions) gcPipelineRuns(ns string) error {
	tektonClient, _, err := o.TektonClient()
	if err != nil {
		return err
	}
	pipelineRunInterface := tektonClient.TektonV1alpha1().PipelineRuns(ns)
	runList, err := pipelineRunInterface.List(metav1.ListOptions{})
	if err != nil {
		log.Logger().Warnf("no PipelineRun instances found: %s", err.Error())
		return nil
	}

	now := time.Now()

	for _, pr := range runList.Items {
		completionTime := pr.Status.CompletionTime
		if completionTime != nil && completionTime.Add(o.PipelineRunAgeLimit).Before(now) {
			err = o.deletePipelineRun(pipelineRunInterface, &pr)
			if err != nil {
				return err
			}
			continue
		}
	}
	return nil
}

func (o *GCActivitiesOptions) deletePipelineRun(pipelineRunInterface tektonclient.PipelineRunInterface, pr *tektonv1alpha1.PipelineRun) error {
	prefix := ""
	if o.DryRun {
		prefix = "not "
	}
	log.Logger().Infof("%sdeleting PipelineRun %s", prefix, util.ColorInfo(pr.Name))
	if o.DryRun {
		return nil
	}
	return pipelineRunInterface.Delete(pr.Name, metav1.NewDeleteOptions(0))
}

func (o *GCActivitiesOptions) gcProwJobs(ns string) error {
	prowJobClient, _, err := o.ProwJobClient()
	if err != nil {
		return err
	}
	pjInterface := prowJobClient.ProwV1().ProwJobs(ns)
	pjList, err := pjInterface.List(metav1.ListOptions{})
	if err != nil {
		log.Logger().Warnf("no ProwJob instances found: %s", err.Error())
		return nil
	}

	now := time.Now()

	for _, pj := range pjList.Items {
		completionTime := pj.Status.CompletionTime
		if completionTime != nil && completionTime.Add(o.ProwJobAgeLimit).Before(now) {
			err = o.deleteProwJob(pjInterface, &pj)
			if err != nil {
				return errors.Wrapf(err, "error deleting ProwJob %s", pj.Name)
			}
		}
	}
	return nil
}

func (o *GCActivitiesOptions) deleteProwJob(pjInterface prowjobclient.ProwJobInterface, pj *prowjobv1.ProwJob) error {
	prefix := ""
	if o.DryRun {
		prefix = "not "
	}
	log.Logger().Infof("%sdeleting ProwJob %s", prefix, util.ColorInfo(pj.Name))
	if o.DryRun {
		return nil
	}
	return pjInterface.Delete(pj.Name, metav1.NewDeleteOptions(0))
}

func (o *GCActivitiesOptions) ageAndHistoryLimits(isPR, isBatch bool) (time.Duration, int) {
	maxAge := o.ReleaseAgeLimit
	revisionLimit := o.ReleaseHistoryLimit
	if isPR || isBatch {
		maxAge = o.PullRequestAgeLimit
		revisionLimit = o.PullRequestHistoryLimit
	}
	return maxAge, revisionLimit
}

func (o *GCActivitiesOptions) isPullRequestOrBatchBranch(branchName string) (bool, bool) {
	return strings.HasPrefix(branchName, "PR-"), branchName == "batch"
}
