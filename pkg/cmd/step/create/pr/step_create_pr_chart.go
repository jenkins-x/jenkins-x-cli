package pr

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	createPullRequestChartLong = templates.LongDesc(`
		Creates a Pull Request on a git repository updating the requirements.yaml and values.yaml with the new version
`)

	createPullRequestChartExample = templates.Examples(`
					`)
)

// StepCreatePullRequestChartsOptions contains the command line flags
type StepCreatePullRequestChartsOptions struct {
	StepCreatePrOptions

	Names []string
}

// NewCmdStepCreatePullRequestChart Creates a new Command object
func NewCmdStepCreatePullRequestChart(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestChartsOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "chart",
		Short:   "Creates a Pull Request on a git repository updating the Chart",
		Long:    createPullRequestChartLong,
		Example: createPullRequestChartExample,
		Aliases: []string{"version pullrequest"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	cmd.Flags().StringArrayVarP(&options.Names, "name", "n", make([]string, 0), "The name of the property to update")
	return cmd
}

// ValidateChartsOptions validates the common options for chart pr steps
func (o *StepCreatePullRequestChartsOptions) ValidateChartsOptions() error {
	if err := o.ValidateOptions(false); err != nil {
		return errors.WithStack(err)
	}
	if len(o.Names) == 0 {
		return util.MissingOption("name")
	}
	if o.SrcGitURL == "" {
		log.Logger().Warnf("srcRepo is not provided so generated PR will not be correctly linked in release notesPR")
	}

	return nil
}

// Run implements this command
func (o *StepCreatePullRequestChartsOptions) Run() error {
	if err := o.ValidateChartsOptions(); err != nil {
		return errors.WithStack(err)
	}
	err := o.CreatePullRequest("chart",
		func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
			oldVersions := make([]string, 0)
			// walk the filepath, looking for values.yaml and requirements.yaml
			err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				base := filepath.Base(path)
				if base == helm.RequirementsFileName {
					requirements, err := helm.LoadRequirementsFile(path)
					if err != nil {
						return errors.Wrapf(err, "loading %s", path)
					}
					for _, name := range o.Names {
						oldVersions = append(oldVersions, helm.UpdateRequirementsToNewVersion(requirements, name, o.Version)...)
					}
					err = helm.SaveFile(path, *requirements)
					if err != nil {
						return errors.Wrapf(err, "saving %s", path)
					}
				} else if base == helm.ValuesFileName {
					values, err := ioutil.ReadFile(path)
					if err != nil {
						return errors.Wrapf(err, "reading %s", path)
					}
					newValues := string(values)
					for _, name := range o.Names {
						re, err := regexp.Compile(fmt.Sprintf(`(?m)^\s*Image: %s:(.*)$`, name))
						if err != nil {
							return errors.WithStack(err)
						}
						newValues = util.ReplaceAllStringSubmatchFunc(re, newValues, func(groups []util.Group) []string {
							answer := make([]string, 0)
							for i := range groups {
								oldVersions = append(oldVersions, groups[i].Value)
								answer = append(answer, o.Version)
							}
							return answer
						})
					}
					err = ioutil.WriteFile(path, []byte(newValues), info.Mode())
					if err != nil {
						return errors.Wrapf(err, "writing %s", path)
					}
				}
				return nil
			})
			if err != nil {
				return nil, errors.WithStack(err)
			}
			return oldVersions, nil
		})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}
