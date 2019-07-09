package pr

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	createPullRequestMakeLong = templates.LongDesc(`
		Creates a Pull Request updating a Makefile so that any variables defined as <name> := <value> will have the 
		value replaced with the new version

		Files named Makefile or Makefile.* will be updated
`)

	createPullRequestMakeExample = templates.Examples(`
		jx step create pr make --name CHART_VERSION --version 1.2.3 --repo https://github.com/jenkins-x/cloud-environments.git
					`)
)

// StepCreatePullRequestMakeOptions contains the command line flags
type StepCreatePullRequestMakeOptions struct {
	StepCreatePrOptions

	Name string
}

// NewCmdStepCreatePullRequestMake Creates a new Command object
func NewCmdStepCreatePullRequestMake(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestMakeOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: opts.StepCreateOptions{
				StepOptions: opts.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "make",
		Short:   "Creates a Pull Request on a git repository, doing an update to a Makefile",
		Long:    createPullRequestMakeLong,
		Example: createPullRequestMakeExample,
		Aliases: []string{"version pullrequest"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	cmd.Flags().StringVarP(&options.Name, "name", "", "", "The name of the variable to use when doing updates")
	return cmd
}

// Run implements this command
func (o *StepCreatePullRequestMakeOptions) Run() error {
	if err := o.ValidateOptions(); err != nil {
		return errors.WithStack(err)
	}
	if o.Name == "" {
		return util.MissingOption("name")
	}
	ro := StepCreatePullRequestRegexOptions{
		StepCreatePrOptions: o.StepCreatePrOptions,
		Files:               []string{"Makefile", "Makefile.*"},
		Regexp:              fmt.Sprintf(`^%s\s*:=\s*(?P<version>.+)`, o.Name),
		Kind:                "make",
	}
	err := ro.Run()
	if err != nil {
		return errors.Wrapf(err, "executing regex %s on globs %+v", ro.Regexp, ro.Files)
	}
	return nil
}
