package git

import (
	"fmt"
	"os"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/spf13/cobra"
)

// StepGitForkAndCloneOptions contains the command line flags
type StepGitForkAndCloneOptions struct {
	step.StepOptions
	Dir         string
	BaseRef     string
	PrintOutDir bool
	OutputDir   string
}

var (
	// StepGitForkAndCloneLong command long description
	StepGitForkAndCloneLong = templates.LongDesc(`
		This pipeline step will clone a git repo, creating a fork if required. The fork is created if the owner of the 
		repo is not the current git user (and that forking the git repo is allowed).

`)
	// StepGitForkAndCloneExample command example
	StepGitForkAndCloneExample = templates.Examples(`
		# Fork and clone the jx repo
		jx step git fork-and-clone https://github.com/jenkins-x/jx.git

		# Duplicate and clone the jx repo. This will create a new repo and mirror the contents of the source repo into,
		# but it won't mark it as a fork in the git provider
		jx step git fork-and-clone https://github.com/jenkins-x/jx.git --duplicate


`)
)

// NewCmdStepGitForkAndClone create the 'step git envs' command
func NewCmdStepGitForkAndClone(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGitForkAndCloneOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "fork-and-clone",
		Short:   "Forks and clones a git repo",
		Long:    StepGitForkAndCloneLong,
		Example: StepGitForkAndCloneExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The directory in which the git repo is checked out, by default the working directory")
	cmd.Flags().StringVarP(&options.BaseRef, "base", "", "master", "The base ref to start from")
	cmd.Flags().BoolVarP(&options.BatchMode, opts.OptionBatchMode, "b", false, "Enable batch mode")
	cmd.Flags().BoolVarP(&options.PrintOutDir, "print-out-dir", "", false, "prints the directory the fork has been cloned to on stdout")
	return cmd
}

// Run implements the command
func (o *StepGitForkAndCloneOptions) Run() error {
	if o.Dir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return err
		}
		o.Dir = dir
	}
	gitURL := ""
	if len(o.Args) > 1 {
		return errors.Errorf("Must specify exactly one git url but was %v", o.Args)
	} else if len(o.Args) == 0 {
		if os.Getenv("REPO_URL") != "" {
			gitURL = os.Getenv("REPO_URL")
		}
	} else {
		gitURL = o.Args[0]
	}

	if gitURL == "" {
		return errors.Errorf("Must specify a git url on the CLI or using the environment variable REPO_URL")
	}
	provider, err := o.GitProviderForURL(gitURL, "git username")
	if err != nil {
		return errors.Wrapf(err, "getting git provider for %s", gitURL)
	}
	dir, baseRef, upstreamInfo, forkInfo, err := gits.ForkAndPullRepo(gitURL, o.Dir, o.BaseRef, "master", provider, o.Git(), "")
	if err != nil {
		return errors.Wrapf(err, "forking and pulling %s", gitURL)
	}
	o.OutDir = dir
	if o.PrintOutDir {
		// Output the directory so it can be used in a script
		// Must use fmt.Print() as we need to write to stdout
		fmt.Print(dir)
	}
	if forkInfo != nil {
		log.Logger().Infof("Forked %s to %s, pulled it into %s and checked out %s", util.ColorInfo(upstreamInfo.HTMLURL), util.ColorInfo(forkInfo.HTMLURL), util.ColorInfo(dir), util.ColorInfo(baseRef))
	} else {
		log.Logger().Infof("Pulled %s (%s) into %s", upstreamInfo.URL, baseRef, dir)
	}

	return nil
}
