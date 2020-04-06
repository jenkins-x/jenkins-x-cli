package pr

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/gits/operations"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	createtPullRequestGoLong = templates.LongDesc(`
		Creates a Pull Request to change a go module dependency, updating the go.mod and go.sum files to use a new version

		Files named Makefile or Makefile.* will be updated
`)

	createtPullRequestGoExample = templates.Examples(`
		# update a go dependency 
		jx step create pr go --name github.com/myorg/myrepo --version v1.2.3 --repo https://github.com/jenkins-x/cloud-environments.git

		# update a go dependency using a custom build step (to update the 'go.sum' file) 
		jx step create pr go --name github.com/myorg/myrepo --version v1.2.3 --build "make something" --repo https://github.com/jenkins-x/cloud-environments.git
					`)
)

// StepCreatetPullRequestGoOptions contains the command line flags
type StepCreatetPullRequestGoOptions struct {
	StepCreatePrOptions

	Name         string
	BuildCommand string
	FailOnBuild  bool
}

// NewCmdStepCreatePullRequestGo Creates a new Command object
func NewCmdStepCreatePullRequestGo(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatetPullRequestGoOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "go",
		Short:   "Creates a Pull Request on a git repository updating a go module dependency",
		Long:    createtPullRequestGoLong,
		Example: createtPullRequestGoExample,
		Aliases: []string{"golang"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	cmd.Flags().StringVarP(&options.Name, "name", "", "", "The name of the go module dependency to use when doing updates")
	cmd.Flags().StringVarP(&options.BuildCommand, "build", "", "make build", "The build command to update the 'go.sum' file after the change to the source")
	cmd.Flags().BoolVarP(&options.FailOnBuild, "fail-on-build", "", false, "Should we fail to create the Pull Request if the build command fails. Its common for incompatible changes to the go code to fail to build so we usually want to go ahead with the Pull Request anyway")
	return cmd
}

// ValidateGoOptions validates the common options for make pr steps
func (o *StepCreatetPullRequestGoOptions) ValidateGoOptions() error {
	if err := o.ValidateOptions(false); err != nil {
		return errors.WithStack(err)
	}
	if o.Name == "" {
		return util.MissingOption("name")
	}
	if o.SrcGitURL == "" {
		log.Logger().Warnf("srcRepo is not provided so generated PR will not be correctly linked in release notesPR")
	}

	return nil
}

// Run implements this command
func (o *StepCreatetPullRequestGoOptions) Run() error {
	// lets make sure the version starts with a v for go style version tags
	if !strings.HasPrefix(o.Version, "v") {
		o.Version = "v" + o.Version
	}
	if err := o.ValidateGoOptions(); err != nil {
		return errors.WithStack(err)
	}
	regex := fmt.Sprintf(`(?m)^\s*\Q%s\E\s+(?P<version>.+)`, o.Name)
	regexFn, err := operations.CreatePullRequestRegexFn(o.Version, regex, "go.mod")
	if err != nil {
		return errors.WithStack(err)
	}
	fn := func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		answer, err := regexFn(dir, gitInfo)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		err = o.runGoBuild(dir)
		if err != nil {
			log.Logger().Errorf("failed to run build after modifying go.mod")
			return nil, errors.WithStack(err)
		}
		return answer, nil
	}
	err = o.CreatePullRequest("go", fn)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (o *StepCreatetPullRequestGoOptions) runGoBuild(dir string) error {
	build := o.BuildCommand
	if build == "" {
		log.Logger().Warn("no build command so we will not change the 'go.sum' command")
		return nil
	}
	log.Logger().Infof("running the build command: %s in the directory %s to update the 'go.sum' file\n", util.ColorInfo(build), dir)

	values := strings.Split(build, " ")
	cmd := util.Command{
		Dir:  dir,
		Name: values[0],
		Args: values[1:],
	}
	_, err := cmd.RunWithoutRetry()
	if err != nil {
		if o.FailOnBuild {
			return errors.Wrapf(err, "running %s", cmd.String())
		}
		log.Logger().Warnf("failed to run %s so the Pull Request will probably need some manual work to make it pass the CI tests. Failure: %s", cmd.String(), err.Error())
	}
	return nil
}
