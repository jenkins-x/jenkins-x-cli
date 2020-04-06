package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

const (
	optionTitle = "title"
)

var (
	create_issue_long = templates.LongDesc(`
		Creates an issue in a the git project of the current directory
`)

	create_issue_example = templates.Examples(`
		# Create an issue in the current project
		jx create issue -t "something we should do"


		# Create an issue with a title and a body
		jx create issue -t "something we should do" --body "	
		some more
		text
		goes
		here
		""
"
	`)
)

// CreateIssueOptions the options for the create spring command
type CreateIssueOptions struct {
	options.CreateOptions

	Dir    string
	Title  string
	Body   string
	Labels []string
}

// NewCmdCreateIssue creates a command object for the "create" command
func NewCmdCreateIssue(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateIssueOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "issue",
		Short:   "Create an issue on the git project for the current directory",
		Aliases: []string{"env"},
		Long:    create_issue_long,
		Example: create_issue_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The source directory used to detect the Git repository. Defaults to the current directory")
	cmd.Flags().StringVarP(&options.Title, optionTitle, "t", "", "The title of the issue to create")
	cmd.Flags().StringVarP(&options.Body, "body", "", "", "The body of the issue")
	cmd.Flags().StringArrayVarP(&options.Labels, "label", "l", []string{}, "The labels to add to the issue")

	return cmd
}

// Run implements the command
func (o *CreateIssueOptions) Run() error {
	tracker, err := o.CreateIssueProvider(o.Dir)
	if err != nil {
		return err
	}
	issue := &gits.GitIssue{}

	err = o.PopulateIssue(issue)
	if err != nil {
		return err
	}

	createdIssue, err := tracker.CreateIssue(issue)
	if err != nil {
		return err
	}
	if createdIssue == nil {
		return fmt.Errorf("Failed to create issue: %s", issue.Title)
	}
	log.Logger().Infof("\nCreated issue %s at %s", util.ColorInfo(createdIssue.Name()), util.ColorInfo(createdIssue.URL))
	return nil
}

func (o *CreateIssueOptions) PopulateIssue(issue *gits.GitIssue) error {
	title := o.Title
	body := o.Body
	var err error
	if title == "" {
		if o.BatchMode {
			return util.MissingOption(optionTitle)
		}
		title, err = util.PickValue("Issue title:", "", true, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}
	issue.Title = title
	issue.Body = body

	labels := []gits.GitLabel{}
	for _, label := range o.Labels {
		labels = append(labels, gits.GitLabel{
			Name: label,
		})
	}
	issue.Labels = labels

	if title == "" {
		return fmt.Errorf("No title specified!")
	}
	return nil
}
