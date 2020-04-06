package create

import (
	"os"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	createPullRequestLong = templates.LongDesc(`
		Creates a Pull Request in a the git project of the current directory. 

		If --push is specified the contents of the directory will be committed, pushed and used to create the pull request  
`)

	createPullRequestExample = templates.Examples(`
		# Create a Pull Request in the current project
		jx create pullrequest -t "my PR title"


		# Create a Pull Request with a title and a body
		jx create pullrequest -t "my PR title" --body "	
		some more
		text
		goes
		here
		""
"
	`)
)

// CreatePullRequestOptions the options for the create spring command
type CreatePullRequestOptions struct {
	options.CreateOptions

	Dir    string
	Title  string
	Body   string
	Labels []string
	Base   string
	Push   bool
	Fork   bool

	Results *gits.PullRequestInfo
}

// NewCmdCreatePullRequest creates a command object for the "create" command
func NewCmdCreatePullRequest(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreatePullRequestOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "pullrequest",
		Short:   "Create a Pull Request on the git project for the current directory",
		Aliases: []string{"pr", "pull request"},
		Long:    createPullRequestLong,
		Example: createPullRequestExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The source directory used to detect the Git repository. Defaults to the current directory")
	cmd.Flags().StringVarP(&options.Title, optionTitle, "t", "", "The title of the pullrequest to create")
	cmd.Flags().StringVarP(&options.Body, "body", "", "", "The body of the pullrequest")
	cmd.Flags().StringVarP(&options.Base, "base", "", "master", "The base branch to create the pull request into")
	cmd.Flags().StringArrayVarP(&options.Labels, "label", "l", []string{}, "The labels to add to the pullrequest")
	cmd.Flags().BoolVarP(&options.Push, "push", "", false, "If true the contents of the source directory will be committed, pushed, and used to create the pull request")
	cmd.Flags().BoolVarP(&options.Fork, "fork", "", false, "If true, and the username configured to push the repo is different from the org name a PR is being created against, assume that this is a fork")

	return cmd
}

// Run implements the command
func (o *CreatePullRequestOptions) Run() error {
	// lets discover the git dir
	if o.Dir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return errors.Wrapf(err, "getting working directory")
		}
		o.Dir = dir
	}
	gitInfo, provider, _, err := o.CreateGitProvider(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "creating git provider for directory %s", o.Dir)
	}
	// Rebuild the gitInfo so that we get all the info we need
	gitInfo, err = provider.GetRepository(gitInfo.Organisation, gitInfo.Name)
	if err != nil {
		return errors.Wrapf(err, "getting repository for %s/%s", gitInfo.Organisation, gitInfo.Name)
	}
	var forkInfo *gits.GitRepository
	if o.Fork && provider.CurrentUsername() != gitInfo.Organisation {
		forkInfo, err = provider.GetRepository(provider.CurrentUsername(), gitInfo.Name)
		if err != nil {
			return errors.Wrapf(err, "unable to get %s/%s, does the fork exist? Try running without --fork", provider.CurrentUsername(), gitInfo.Name)
		}
	}

	details, err := o.createPullRequestDetails(gitInfo)
	if err != nil {
		return errors.WithStack(err)
	}

	o.Results, err = gits.PushRepoAndCreatePullRequest(o.Dir, gitInfo, forkInfo, o.Base, details, nil, o.Push, details.Message, o.Push, false, o.Git(), provider)
	if err != nil {
		return errors.Wrapf(err, "failed to create PR for base %s and head branch %s", o.Base, details.BranchName)
	}
	return nil
}

func (o *CreatePullRequestOptions) createPullRequestDetails(gitInfo *gits.GitRepository) (*gits.PullRequestDetails, error) {
	title := o.Title
	if title == "" {
		if o.BatchMode {
			return nil, util.MissingOption(optionTitle)
		}
		defaultValue, body, err := o.findLastCommitTitle()
		if err != nil {
			log.Logger().Warnf("Failed to find last git commit title: %s", err)
		}
		if o.Body == "" {
			o.Body = body
		}
		title, err = util.PickValue("PullRequest title:", defaultValue, true, "", o.GetIOFileHandles())
		if err != nil {
			return nil, err
		}
	}
	if title == "" {
		return nil, fmt.Errorf("no title specified")
	}
	branchName, err := o.Git().Branch(o.Dir)
	if err != nil {
		return nil, err
	}
	return &gits.PullRequestDetails{
		Title:      title,
		Message:    o.Body,
		BranchName: branchName,
		Labels:     o.Labels,
	}, nil

}

func (o *CreatePullRequestOptions) findLastCommitTitle() (string, string, error) {
	title := ""
	body := ""
	dir := o.Dir
	gitDir, gitConfDir, err := o.Git().FindGitConfigDir(dir)
	if err != nil {
		return title, body, err
	}
	if gitDir == "" || gitConfDir == "" {
		log.Logger().Warnf("No git directory could be found from dir %s", dir)
		return title, body, err
	}
	message, err := o.Git().GetLatestCommitMessage(dir)
	if err != nil {
		return title, body, err
	}
	lines := strings.SplitN(message, "\n", 2)
	if len(lines) < 2 {
		return message, "", nil
	}
	return lines[0], lines[1], nil
}
