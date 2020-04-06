package pr

import (
	"os"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/spf13/cobra"

	"strconv"

	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
)

// GetOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type StepPRCommentOptions struct {
	StepPROptions
	Flags StepPRCommentFlags
}

type StepPRCommentFlags struct {
	Comment    string
	URL        string
	Owner      string
	Repository string
	PR         string
	Code       bool
}

// NewCmdStepPRComment Steps a command object for the "step pr comment" command
func NewCmdStepPRComment(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepPRCommentOptions{
		StepPROptions: StepPROptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:   "comment",
		Short: "pipeline step pr comment",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Flags.Comment, "comment", "c", "", "comment to add to the Pull Request")
	cmd.Flags().StringVarP(&options.Flags.Owner, "owner", "o", "", "Git organisation / owner")
	cmd.Flags().StringVarP(&options.Flags.Repository, "repository", "r", "", "Git repository")
	cmd.Flags().StringVarP(&options.Flags.PR, "pull-request", "p", "", "Git Pull Request number")
	cmd.Flags().BoolVarP(&options.Flags.Code, "code", "", false, "Treat the comment as code")

	return cmd
}

// Run implements this command
func (o *StepPRCommentOptions) Run() error {
	if o.Flags.PR == "" {
		o.Flags.PR = os.Getenv("PULL_NUMBER")
	}
	if o.Flags.PR == "" {
		return fmt.Errorf("no Pull Request number provided")
	}

	if o.Flags.Owner == "" {
		o.Flags.Owner = os.Getenv("REPO_OWNER")
	}
	if o.Flags.Owner == "" {
		return fmt.Errorf("no Git owner provided")
	}

	if o.Flags.Repository == "" {
		o.Flags.Repository = os.Getenv("REPO_NAME")
	}
	if o.Flags.Repository == "" {
		return fmt.Errorf("no Git repository provided")
	}

	if o.Flags.Comment == "" {
		return fmt.Errorf("no comment provided")
	}

	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}

	gitInfo, err := o.Git().Info("")
	if err != nil {
		return err
	}
	gitKind, err := o.GitServerKind(gitInfo)
	if err != nil {
		return err
	}

	ghOwner, err := o.GetGitHubAppOwner(gitInfo)
	if err != nil {
		return err
	}
	provider, err := o.NewGitProvider(gitInfo.URL, "user name to submit comment as", authConfigSvc, gitKind, ghOwner, o.BatchMode, o.Git())
	if err != nil {
		return err
	}

	prNumber, err := strconv.Atoi(o.Flags.PR)
	if err != nil {
		return err
	}

	pr := gits.GitPullRequest{
		Repo:   o.Flags.Repository,
		Owner:  o.Flags.Owner,
		Number: &prNumber,
	}

	if o.Flags.Code {
		return provider.AddPRComment(&pr, escapeAsCode(o.Flags.Comment))
	}
	return provider.AddPRComment(&pr, o.Flags.Comment)
}

func escapeAsCode(comment string) string {
	return "```\n" + comment
}
