package git

import (
	"os"

	"github.com/jenkins-x/jx/pkg/jx/cmd/helper"

	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/jx/cmd/opts"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	// StepGitMergeLong command long description
	StepGitMergeLong = templates.LongDesc(`
		This pipeline step merges any SHAs specified into the HEAD of master. 

If no SHAs are specified then the PULL_REFS environment variable will be parsed for a branch:sha comma separated list of
shas to merge. For example:

master:ef08a6cd194c2687d4bc12df6bb8a86f53c348ba,2739:5b351f4eae3c4afbb90dd7787f8bf2f8c454723f,2822:bac2a1f34fd54811fb767f69543f59eb3949b2a5

`)
	// StepGitMergeExample command example
	StepGitMergeExample = templates.Examples(`
		# Merge the SHAs from the PULL_REFS environment variable
		jx step git merge

		# Merge the SHA into the HEAD of master
		jx step git merge --sha 123456a

		# Merge a number of SHAs into the HEAD of master
		jx step git merge --sha 123456a --sha 789012b
`)
)

type StepGitMergeCommand struct {
	opts.StepGitMergeOptions
}

// NewCmdStepGitMerge create the 'step git envs' command
func NewCmdStepGitMerge(commonOpts *opts.CommonOptions) *cobra.Command {
	options := opts.StepGitMergeOptions{
		StepOptions: opts.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "merge",
		Short:   "Merge a number of SHAs into the HEAD of master",
		Long:    StepGitMergeLong,
		Example: StepGitMergeExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringArrayVarP(&options.SHAs, "sha", "", make([]string, 0), "The SHA(s) to merge, "+
		"if not specified then the value of the env var PULL_REFS is used")
	cmd.Flags().StringVarP(&options.Remote, "remote", "", "origin", "The name of the remote")
	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The directory in which the git repo is checked out")
	cmd.Flags().StringVarP(&options.BaseBranch, "baseBranch", "", "", "The branch to merge to, "+
		"if not specified then the  first entry in PULL_REFS is used ")
	cmd.Flags().StringVarP(&options.BaseSHA, "baseSHA", "", "", "The SHA to use on the base branch, "+
		"if not specified then the first entry in PULL_REFS is used")

	return cmd
}

// Run implements the command
func (o *StepGitMergeCommand) Run() error {
	if o.Remote == "" {
		o.Remote = "origin"
	}

	// set dummy git config details if none set so we can do a local commit when merging
	err := o.setGitConfig()
	if err != nil {
		return errors.Wrapf(err, "failed to set git config")
	}
	if len(o.SHAs) == 0 || o.BaseBranch == "" || o.BaseSHA == "" {
		// Try to look in the env vars
		if pullRefs := os.Getenv("PULL_REFS"); pullRefs != "" {
			log.Logger().Infof("Using SHAs from PULL_REFS=%s\n", pullRefs)
			pullRefs, err := prow.ParsePullRefs(pullRefs)
			if err != nil {
				return errors.Wrapf(err, "parsing PULL_REFS=%s", pullRefs)
			}
			if len(o.SHAs) == 0 {
				o.SHAs = make([]string, 0)
				for _, sha := range pullRefs.ToMerge {
					o.SHAs = append(o.SHAs, sha)
				}
			}
			if o.BaseBranch == "" {
				o.BaseBranch = pullRefs.BaseBranch
			}
			if o.BaseSHA == "" {
				o.BaseSHA = pullRefs.BaseSha
			}
		}
	}
	if len(o.SHAs) == 0 {
		log.Logger().Warnf("no SHAs to merge, falling back to initial cloned commit")
		return nil
	}
	return gits.FetchAndMergeSHAs(o.SHAs, o.BaseBranch, o.BaseSHA, o.Remote, o.Dir, o.Git(), o.Verbose)
}

func (o *StepGitMergeCommand) setGitConfig() error {
	user, err := o.GetCommandOutput(o.Dir, "git", "config", "user.name")
	if err != nil || user == "" {
		err := o.RunCommandFromDir(o.Dir, "git", "config", "user.name", "jenkins-x")
		if err != nil {
			return err
		}
	}
	email, err := o.GetCommandOutput(o.Dir, "git", "config", "user.email")
	if email == "" || err != nil {
		err := o.RunCommandFromDir(o.Dir, "git", "config", "user.email", "jenkins-x@googlegroups.com")
		if err != nil {
			return err
		}
	}
	return nil
}
