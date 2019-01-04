package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StopPipelineOptions contains the command line options
type StopPipelineOptions struct {
	GetOptions

	Build  int
	Filter string

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
func NewCmdStopPipeline(commonOpts *CommonOptions) *cobra.Command {
	options := &StopPipelineOptions{
		GetOptions: GetOptions{
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
			CheckErr(err)
		},
	}
	cmd.Flags().IntVarP(&options.Build, "build", "b", 0, "The build number to stop")
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filters all the available jobs by those that contain the given text")

	return cmd
}

// Run implements this command
func (o *StopPipelineOptions) Run() error {
	jobMap, err := o.getJobMap(o.Filter)
	if err != nil {
		return err
	}
	o.Jobs = jobMap
	args := o.Args
	names := []string{}
	for k, _ := range o.Jobs {
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
		name, err := util.PickNameWithDefault(names, "Which pipelines do you want to stop: ", defaultName, "", o.In, o.Out, o.Err)
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
