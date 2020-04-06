package stop

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
)

// StopPipelineOptions contains the command line options
type StopPipelineOptions struct {
	get.GetOptions

	Build           int
	Filter          string
	JenkinsSelector opts.JenkinsSelectorOptions

	Jobs map[string]gojenkins.Job
}

var (
	stopPipelineLong = templates.LongDesc(`
		Stops the pipeline build.

`)

	stopPipelineExample = templates.Examples(`
		# Stop a pipeline
		jx stop pipeline foo/bar/master -b 2

		# Select the pipeline to stop
		jx stop pipeline
	`)
)

// NewCmdStopPipeline creates the command
func NewCmdStopPipeline(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StopPipelineOptions{
		GetOptions: get.GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "pipeline [flags]",
		Short:   "Stops one or more pipelines",
		Long:    stopPipelineLong,
		Example: stopPipelineExample,
		Aliases: []string{"pipe", "pipeline", "build", "run"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().IntVarP(&options.Build, "build", "", 0, "The build number to stop")
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filters all the available jobs by those that contain the given text")
	options.JenkinsSelector.AddFlags(cmd)

	return cmd
}

// Run implements this command
func (o *StopPipelineOptions) Run() error {
	devEnv, _, err := o.DevEnvAndTeamSettings()
	if err != nil {
		return err
	}

	isProw := devEnv.Spec.IsProwOrLighthouse()
	if o.JenkinsSelector.IsCustom() {
		isProw = false
	}

	if isProw {
		return o.cancelPipelineRun()
	}
	return o.stopJenkinsJob()
}

func (o *StopPipelineOptions) stopJenkinsJob() error {
	jobMap, err := o.GetJenkinsJobs(&o.JenkinsSelector, o.Filter)
	if err != nil {
		return err
	}
	o.Jobs = jobMap
	args := o.Args
	names := []string{}
	for k := range o.Jobs {
		names = append(names, k)
	}
	sort.Strings(names)

	if len(args) == 0 {
		defaultName := ""
		for _, n := range names {
			if strings.HasSuffix(n, "/master") {
				defaultName = n
				break
			}
		}
		name, err := util.PickNameWithDefault(names, "Which pipelines do you want to stop: ", defaultName, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		args = []string{name}
	}
	for _, a := range args {
		err = o.stopJob(a, names)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *StopPipelineOptions) stopJob(name string, allNames []string) error {
	job := o.Jobs[name]
	jenkinsClient, err := o.JenkinsClient()
	if err != nil {
		return err
	}
	build := o.Build
	if build <= 0 {
		last, err := jenkinsClient.GetLastBuild(job)
		if err != nil {
			return err
		}
		build = last.Number
		if build <= 0 {
			return fmt.Errorf("No build available for %s", name)
		}
	}
	return jenkinsClient.StopBuild(job, build)
}

func (o *StopPipelineOptions) cancelPipelineRun() error {
	tektonClient, ns, err := o.TektonClient()
	if err != nil {
		return errors.Wrap(err, "could not create tekton client")
	}
	pipelines := tektonClient.TektonV1alpha1().PipelineRuns(ns)
	prList, err := pipelines.List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to list PipelineRuns in namespace %s", ns)
	}

	allNames := []string{}
	m := map[string]*pipelineapi.PipelineRun{}
	for _, p := range prList.Items {
		pr := p
		if !tekton.PipelineRunIsComplete(&pr) {
			labels := pr.Labels
			if labels == nil {
				continue
			}
			owner := labels[tekton.LabelOwner]
			repo := labels[tekton.LabelRepo]
			branch := labels[tekton.LabelBranch]
			context := labels[tekton.LabelContext]
			buildNumber := labels[tekton.LabelBuild]

			if owner == "" {
				log.Logger().Warnf("missing label %s on PipelineRun %s has labels %#v", tekton.LabelOwner, pr.Name, labels)
				continue
			}
			if repo == "" {
				log.Logger().Warnf("missing label %s on PipelineRun %s has labels %#v", tekton.LabelRepo, pr.Name, labels)
				continue
			}
			if branch == "" {
				log.Logger().Warnf("missing label %s on PipelineRun %s has labels %#v", tekton.LabelBranch, pr.Name, labels)
				continue
			}

			name := fmt.Sprintf("%s/%s/%s #%s", owner, repo, branch, buildNumber)

			if context != "" {
				name = fmt.Sprintf("%s-%s", name, context)
			}
			allNames = append(allNames, name)
			m[name] = &pr
		}
	}
	sort.Strings(allNames)
	names := util.StringsContaining(allNames, o.Filter)
	if len(names) == 0 {
		log.Logger().Warnf("no PipelineRuns are still running which match the filter %s from all possible names %s", o.Filter, strings.Join(allNames, ", "))
		return nil
	}

	args := o.Args
	if len(args) == 0 {
		name, err := util.PickName(names, "Which pipeline do you want to stop: ", "select a pipeline to cancel", o.GetIOFileHandles())
		if err != nil {
			return err
		}

		if answer, err := util.Confirm(fmt.Sprintf("cancel pipeline %s", name), true, "you can always restart a cancelled pipeline with 'jx start pipeline'", o.GetIOFileHandles()); !answer {
			return err
		}
		args = []string{name}
	}
	for _, a := range args {
		pr := m[a]
		if pr == nil {
			return fmt.Errorf("no PipelineRun found for name %s", a)
		}
		pr, err = pipelines.Get(pr.Name, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "getting PipelineRun %s", pr.Name)
		}
		if tekton.PipelineRunIsComplete(pr) {
			log.Logger().Infof("PipelineRun %s has already completed", util.ColorInfo(pr.Name))
			continue
		}
		err = tekton.CancelPipelineRun(tektonClient, ns, pr)
		if err != nil {
			return errors.Wrapf(err, "failed to cancel pipeline %s in namespace %s", pr.Name, ns)
		}
		log.Logger().Infof("cancelled PipelineRun %s", util.ColorInfo(pr.Name))
	}
	return nil
}
