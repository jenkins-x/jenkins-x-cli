package pr

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/gits/operations"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	createPullRequestRegexLong = templates.LongDesc(`
		Creates a Pull Request on a git repository updating files using a regex.
		
		Any named capturing group called "version" will be replaced. If there are no named capturing groups, then the
		all the capturing group will be used.
"
`)

	createPullRequestRegexExample = templates.Examples(`
		# Create a PR to change the value of release = <value> to $VERSION in the config.toml file
		./build/linux/jx step create pr regex --regex "\s*release = \"(.*)\"" --version $VERSION --files config.toml \
			--repo https://github.com/jenkins-x/jx-docs.git

		# Create a PR to change the value of the ImageTag: <value> to ${VERSION} where the previous line is Image: 
 	    # "jenkinsxio/jenkinsx" in the jenkins-x-platform/values.yaml file
		jx step create pr regex --regex "^(?m)\s+Image: \"jenkinsxio\/jenkinsx\"\s+ImageTag: \"(.*)\"$" \
			--version ${VERSION} --files values.yaml --repo https://github.com/jenkins-x/jenkins-x-platform.git

		# Create a PR to change the value of the named capture to $VERSION in the config.toml file
		./build/linux/jx step create pr regex --regex "\s*release = \"(?P<version>.*)\"" --version $VERSION --files config.toml \
			--repo https://github.com/jenkins-x/jx-docs.git

					`)
)

// StepCreatePullRequestRegexOptions contains the command line flags
type StepCreatePullRequestRegexOptions struct {
	StepCreatePrOptions

	Regexps []string
	Files   []string
	Kind    string
}

// NewCmdStepCreatePullRequestRegex Creates a new Command object
func NewCmdStepCreatePullRequestRegex(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestRegexOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "regex",
		Short:   "Creates a Pull Request on a git repository, doing an update using the provided regex",
		Long:    createPullRequestRegexLong,
		Example: createPullRequestRegexExample,
		Aliases: []string{"version pullrequest"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	cmd.Flags().StringArrayVarP(&options.Regexps, "regex", "", make([]string, 0), "The regex to use when doing updates")
	cmd.Flags().StringArrayVarP(&options.Files, "files", "", make([]string, 0), "A glob describing the files to change")
	return cmd
}

// ValidateRegexOptions validates the common options for regex pr steps
func (o *StepCreatePullRequestRegexOptions) ValidateRegexOptions() error {
	if err := o.ValidateOptions(false); err != nil {
		return errors.WithStack(err)
	}
	if len(o.Regexps) == 0 {
		return util.MissingOption("regex")
	}

	for i, regex := range o.Regexps {
		// ensure the regexp is multi-line
		if !strings.HasPrefix(regex, "(?m") {
			o.Regexps[i] = fmt.Sprintf("(?m)%s", regex)
		}
	}
	if o.SrcGitURL == "" {
		log.Logger().Warnf("srcRepo is not provided so generated PR will not be correctly linked in release notesPR")
	}
	if o.Kind == "" {
		o.Kind = "regex"
	}

	return nil
}

// Run implements this command
func (o *StepCreatePullRequestRegexOptions) Run() error {
	if err := o.ValidateRegexOptions(); err != nil {
		return errors.WithStack(err)
	}
	modifyFns := make([]operations.ChangeFilesFn, 0)
	for _, regex := range o.Regexps {
		fn, err := operations.CreatePullRequestRegexFn(o.Version, regex, o.Files...)
		if err != nil {
			return errors.WithStack(err)
		}
		modifyFns = append(modifyFns, fn)
	}
	err := o.CreatePullRequest(o.Kind, func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		var oldVersions []string
		for _, fn := range modifyFns {
			answer, err := fn(dir, gitInfo)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			oldVersions = append(oldVersions, answer...)
		}
		return oldVersions, nil
	})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
