package get

import (
	"errors"
	"sort"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/jenkins"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/prow"

	"github.com/spf13/cobra"

	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/table"
)

// GetPipelineOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type GetPipelineOptions struct {
	GetOptions

	JenkinsSelector opts.JenkinsSelectorOptions

	ProwOptions prow.Options
}

var (
	getPipelineLong = templates.LongDesc(`
		Display one or more pipelines.

`)

	getPipelineExample = templates.Examples(`
		# list all pipelines
		jx get pipeline

		# Lists all the pipelines in a custom Jenkins App
		jx get pipeline -m
	`)
)

// NewCmdGetPipeline creates the command
func NewCmdGetPipeline(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetPipelineOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "pipelines [flags]",
		Short:   "Display one or more Pipelines",
		Long:    getPipelineLong,
		Example: getPipelineExample,
		Aliases: []string{"pipe", "pipes", "pipeline"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)

	cmd.Flags().BoolVarP(&options.JenkinsSelector.UseCustomJenkins, "custom", "m", false, "List the pipelines in custom Jenkins App instead of the default execution engine in Jenkins X")
	cmd.Flags().StringVarP(&options.JenkinsSelector.CustomJenkinsName, "name", "n", "", "The name of the custom Jenkins App if you don't wish to list the pipelines in the default execution engine in Jenkins X")

	return cmd
}

// Run implements this command
func (o *GetPipelineOptions) Run() error {
	jo := &o.JenkinsSelector
	if jo.CustomJenkinsName != "" {
		jo.UseCustomJenkins = true
	}

	_, _, err := o.JXClient()
	if err != nil {
		return err
	}

	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	isProw, err := o.IsProw()
	if err != nil {
		return err
	}

	if jo.UseCustomJenkins || !isProw {
		jenkins, err := o.CreateCustomJenkinsClient(jo)
		if err != nil {
			return err
		}
		jobs, err := jenkins.GetJobs()
		if err != nil {
			return err
		}
		if len(jobs) == 0 {
			return outputEmptyListWarning(o.Out)
		}

		if o.Output != "" {
			return o.renderResult(jobs, o.Output)
		}

		table := createTable(o)

		for _, j := range jobs {
			job, err := jenkins.GetJob(j.Name)
			if err != nil {
				return err
			}
			o.dump(jenkins, job.Name, &table)
		}
		table.Render()
		return nil
	}
	o.ProwOptions = prow.Options{
		KubeClient: client,
		NS:         currentNamespace,
	}
	names, err := o.ProwOptions.GetReleaseJobs()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.New("no pipelines found")
	}
	sort.Strings(names)

	if len(names) == 0 {
		return outputEmptyListWarning(o.Out)
	}

	if o.Output != "" {
		return o.renderResult(names, o.Output)
	}

	table := createTable(o)

	for _, j := range names {
		if err != nil {
			return err
		}
		table.AddRow(j, "N/A", "N/A", "N/A", "N/A")
	}
	table.Render()

	return nil
}

func createTable(o *GetPipelineOptions) table.Table {
	table := o.CreateTable()
	table.AddRow("Name", "URL", "LAST_BUILD", "STATUS", "DURATION")
	return table
}

func (o *GetPipelineOptions) dump(jenkinsClient gojenkins.JenkinsClient, name string, table *table.Table) error {
	job, err := jenkinsClient.GetJob(name)
	if err != nil {
		return err
	}

	if job.Jobs != nil {
		for _, child := range job.Jobs {
			o.dump(jenkinsClient, job.FullName+"/"+child.Name, table)
		}
		if len(job.Jobs) == 0 {
			log.Logger().Warnf("Job %s has no children!", job.Name)
		}
	} else {
		job.Url = jenkins.SwitchJenkinsBaseURL(job.Url, jenkinsClient.BaseURL())
		last, err := jenkinsClient.GetLastBuild(job)

		if err != nil {
			if jenkinsClient.IsErrNotFound(err) {
				if o.matchesFilter(&job) {
					table.AddRow(job.FullName, job.Url, "", "Never Built", "")
				}
			} else {
				log.Logger().Warnf("Failed to find last build for job %s: %s", job.Name, err.Error())
			}
			return nil
		}
		if o.matchesFilter(&job) {
			if last.Building {
				table.AddRow(job.FullName, job.Url, "#"+last.Id, "Building", time.Duration(last.EstimatedDuration).String()+"(est.)")
			} else {
				table.AddRow(job.FullName, job.Url, "#"+last.Id, last.Result, time.Duration(last.Duration).String())
			}
		}
	}
	return nil
}

func (o *GetPipelineOptions) matchesFilter(job *gojenkins.Job) bool {
	args := o.Args
	if len(args) == 0 {
		return true
	}
	name := job.FullName
	for _, arg := range args {
		if strings.Contains(name, arg) {
			return true
		}
	}
	return false
}
