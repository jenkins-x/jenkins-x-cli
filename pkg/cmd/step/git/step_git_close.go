package git

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepGitCloseOptions contains the command line flags
type StepGitCloseOptions struct {
	step.StepOptions
	Dir      string
	Orgs     []string
	Excludes []string
	Includes []string
	DryRun   bool
}

var (
	// StepGitCloseLong command long description
	StepGitCloseLong = templates.LongDesc(`
		This pipeline step will close git provider issue trackers, wikis and projects that are not in use 
		(no issues, no wiki pages, no projects). It will log any it can't close, indicating why.

`)
	// StepGitCloseExample command example
	StepGitCloseExample = templates.Examples(`
		# Close unused issue trackers, wikis and projects for organizations
		jx step git close --org https://github.com/jenkins-x --org https://github.com/jenkins-x

		# Close unused issue trackers, wikis and projects for an organization
		jx step git close --org https://github.com/jenkins-x --whitelist jenkins-x/jx

`)
)

// NewCmdStepGitClose create the 'step git envs' command
func NewCmdStepGitClose(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGitCloseOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "close",
		Short:   "Closes issue trackers, wikis and projects",
		Long:    StepGitCloseLong,
		Example: StepGitCloseExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The directory in which the git repo is checked out, by default the working directory")
	cmd.Flags().StringArrayVarP(&options.Orgs, "org", "", make([]string, 0), "An org to close issue trackers, wikis and projects for")
	cmd.Flags().StringArrayVarP(&options.Excludes, "exclude", "", make([]string, 0), "A repo to ignore when closing issue trackers, wikis and projects e.g. jenkins-x/jx")
	cmd.Flags().StringArrayVarP(&options.Includes, "include", "", make([]string, 0), "If any includes are specified then only those repos will have issue trackers, wikis and projects closed")
	cmd.Flags().BoolVarP(&options.DryRun, "dry-run", "", false, "execute as a dry run - print what would be done but exit before making any changes")
	cmd.Flags().BoolVarP(&options.BatchMode, opts.OptionBatchMode, "b", false, "execute in batch mode")
	return cmd
}

// Run implements the command
func (o *StepGitCloseOptions) Run() error {
	if len(o.Orgs) == 0 {
		return o.Cmd.Help()
	}
	err := o.DisableFeatures(o.Orgs, o.Includes, o.Excludes, o.DryRun)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
