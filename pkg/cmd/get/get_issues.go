package get

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/spf13/cobra"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// GetIssuesOptions contains the command line options
type GetIssuesOptions struct {
	GetOptions
	Dir    string
	Filter string
}

var (
	GetIssuesLong = templates.LongDesc(`
		Display one or more issues for a project.

`)

	GetIssuesExample = templates.Examples(`
		# List open issues on the current project
		jx get issues
	`)
)

// NewCmdGetIssues creates the command
func NewCmdGetIssues(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetIssuesOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "issues [flags]",
		Short:   "Display one or more issues",
		Long:    GetIssuesLong,
		Example: GetIssuesExample,
		Aliases: []string{"jira"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The root project directory")
	cmd.Flags().StringVarP(&options.Filter, "filter", "", "open", "The filter to use")
	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetIssuesOptions) Run() error {
	tracker, err := o.CreateIssueProvider(o.Dir)
	if err != nil {
		return err
	}

	issues, err := tracker.SearchIssues(o.Filter)
	if err != nil {
		return err
	}

	table := o.CreateTable()
	table.AddRow("ISSUE", "TITLE")
	for _, i := range issues {
		table.AddRow(i.URL, i.Title)
	}
	table.Render()
	return nil
}

func (o *GetIssuesOptions) matchesFilter(job *gojenkins.Job) bool {
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
