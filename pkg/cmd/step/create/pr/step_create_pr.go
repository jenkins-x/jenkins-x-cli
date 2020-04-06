package pr

import (
	"os"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/gits/operations"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

//StepCreatePrOptions are the common options for all PR creation steps
type StepCreatePrOptions struct {
	step.StepCreateOptions
	Results       *gits.PullRequestInfo
	BranchName    string
	GitURLs       []string
	Base          string
	Fork          bool
	SrcGitURL     string
	Component     string
	Version       string
	DryRun        bool
	SkipCommit    bool
	SkipAutoMerge bool
	Labels        []string
}

// NewCmdStepCreatePr Steps a command object for the "step" command
func NewCmdStepCreatePr(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePrOptions{
		StepCreateOptions: step.StepCreateOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "pullrequest",
		Aliases: []string{"pr"},
		Short:   "create pullrequest [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepCreatePullRequestBrew(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestChart(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestDocker(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestGo(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestMake(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestQuickStarts(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestRegex(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestRepositories(commonOpts))
	cmd.AddCommand(NewCmdStepCreatePullRequestVersion(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepCreatePrOptions) Run() error {
	return o.Cmd.Help()
}

//AddStepCreatePrFlags adds the common flags for all PR creation steps to the cmd and stores them in o
func AddStepCreatePrFlags(cmd *cobra.Command, o *StepCreatePrOptions) {
	cmd.Flags().StringArrayVarP(&o.GitURLs, "repo", "r", []string{}, "Git repo to update")
	cmd.Flags().StringVarP(&o.BranchName, "branch", "", "master", "Branch to clone and generate a pull request from")
	cmd.Flags().StringVarP(&o.Base, "base", "", "master", "The branch to create the pull request into")
	cmd.Flags().StringVarP(&o.SrcGitURL, "src-repo", "", "", "The git repo which caused this change; if this is a dependency update this will cause commit messages to be generated which can be parsed by jx step changelog. By default this will be read from the environment variable REPO_URL")
	cmd.Flags().StringVarP(&o.Component, "component", "", "", "The component of the git repo which caused this change; useful if you have a complex or monorepo setup and want to differentiate between different components from the same repo")
	cmd.Flags().StringVarP(&o.Version, "version", "v", "", "The version to change. If no version is supplied the latest version is found")
	cmd.Flags().BoolVarP(&o.DryRun, "dry-run", "", false, "Perform a dry run, the change will be generated and committed, but not pushed or have a PR created")
	cmd.Flags().BoolVarP(&o.SkipAutoMerge, "skip-auto-merge", "", false, "Disable auto merge of the PR if status checks pass")
	cmd.Flags().StringArrayVarP(&o.Labels, "labels", "", []string{}, "Labels to add to the created PR")
}

// ValidateOptions validates the common options for all PR creation steps
func (o *StepCreatePrOptions) ValidateOptions(allowEmptyVersion bool) error {
	if o.SrcGitURL == "" {
		o.SrcGitURL = os.Getenv("REPO_URL")
		if o.SrcGitURL != "" {
			log.Logger().Infof("Using %s as source for change discovered from env var REPO_URL", o.SrcGitURL)
		} else {
			// see if we're in a git repo and use it
			wd, err := os.Getwd()
			if err != nil {
				return errors.Wrapf(err, "getting working directory")
			}
			gitInfo, err := o.FindGitInfo(wd)
			if err != nil {
				log.Logger().Debugf("Unable to discover git info from current directory because %v", err)
			} else {
				o.SrcGitURL = gitInfo.HttpsURL()
				log.Logger().Infof("Using %s as source for change discovered from git repo in %s", o.SrcGitURL, wd)
			}
		}

	}
	if o.SrcGitURL == "" {
		return errors.Errorf("unable to determine source url, no argument provided, env var REPO_URL is empty and working directory is not a git repo")
	}
	if !allowEmptyVersion && o.Version == "" {
		return util.MissingOption("version")
	}
	if len(o.GitURLs) == 0 {
		return util.MissingOption("repo")
	}
	return nil
}

// CreatePullRequest will fork (if needed) and pull a git repo, then perform the update, and finally create or update a
// PR for the change. Any open PR on the repo with the `updatebot` label will be updated.
func (o *StepCreatePrOptions) CreatePullRequest(kind string, update operations.ChangeFilesFn) error {
	if o.DryRun {
		log.Logger().Infof("--dry-run specified. Change will be created and committed to local git repo, but not pushed. No pull request will be created or updated. A fork will still be created.")
	}
	op := o.createPullRequestOperation()
	var err error
	o.Results, err = op.CreatePullRequest(kind, update)
	if err != nil {
		return errors.Wrap(err, "unable to create pull request")
	}
	return nil
}

func (o *StepCreatePrOptions) createPullRequestOperation() operations.PullRequestOperation {
	op := operations.PullRequestOperation{
		CommonOptions: o.CommonOptions,
		GitURLs:       o.GitURLs,
		BranchName:    o.BranchName,
		SrcGitURL:     o.SrcGitURL,
		Base:          o.Base,
		Version:       o.Version,
		Component:     o.Component,
		DryRun:        o.DryRun,
		SkipCommit:    o.SkipCommit,
		SkipAutoMerge: o.SkipAutoMerge,
		Labels:        o.Labels,
	}
	authorName, authorEmail, err := gits.EnsureUserAndEmailSetup(o.Git())
	if err != nil {
		op.AuthorName = authorName
		op.AuthorEmail = authorEmail
	}
	return op
}
