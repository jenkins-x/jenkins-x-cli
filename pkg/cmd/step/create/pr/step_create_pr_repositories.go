package pr

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	createPullRequestRepositoriesLong = templates.LongDesc(`
		Creates a Pull Request on a 'jx boot' git repository to mirror all the SourceRepository CRDs into the repositories Chart
`)

	createPullRequestRepositoriesExample = templates.Examples(`
					`)
)

// StepCreatePullRequestRepositoriesOptions contains the command line flags
type StepCreatePullRequestRepositoriesOptions struct {
	StepCreatePrOptions
}

// NewCmdStepCreatePullRequestRepositories Creates a new Command object
func NewCmdStepCreatePullRequestRepositories(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestRepositoriesOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "repositories",
		Short:   "Creates a Pull Request on a 'jx boot' git repository to mirror all the SourceRepository CRDs into the repositories Chart",
		Long:    createPullRequestRepositoriesLong,
		Example: createPullRequestRepositoriesExample,
		Aliases: []string{"repos"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	return cmd
}

// ValidateRepositoriesOptions validates the common options for repositories pr steps
func (o *StepCreatePullRequestRepositoriesOptions) ValidateRepositoriesOptions() error {
	if len(o.GitURLs) == 0 {
		// lets default to the dev environment git repository
		devEnv, _, err := o.DevEnvAndTeamSettings()
		if err != nil {
			return errors.Wrapf(err, "no --repo specified so trying to find the 'dev' Environment to default the repository but cannot find it")
		}
		o.GitURLs = []string{devEnv.Spec.Source.URL}
	}
	if err := o.ValidateOptions(true); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Run implements this command
func (o *StepCreatePullRequestRepositoriesOptions) Run() error {
	if err := o.ValidateRepositoriesOptions(); err != nil {
		return errors.WithStack(err)
	}
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	srList, err := jxClient.JenkinsV1().SourceRepositories(ns).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to query the SourceRepository resources in namespace %s", ns)
	}

	err = o.CreatePullRequest("repositories",
		func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
			outDir := filepath.Join(dir, "repositories", "templates")
			exists, err := util.DirExists(outDir)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to check if output dir exists %s", outDir)
			}
			if !exists {
				return nil, fmt.Errorf("output dir %s does not exist", outDir)
			}

			for _, sr := range srList.Items {
				labels := sr.Labels
				if labels != nil {
					if strings.ToLower(labels[kube.LabelGitSync]) == "false" {
						continue
					}
				}
				sr.ObjectMeta = o.emptyObjectMeta(&sr.ObjectMeta)

				data, err := yaml.Marshal(&sr)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to marshal SourceRepository %s to YAML", sr.Name)
				}

				fileName := filepath.Join(outDir, sr.Name+".yaml")
				err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to write file %s for SourceRepository %s to YAML", fileName, sr.Name)
				}
			}
			return nil, nil
		})
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// emptyObjectMeta lets return a clean ObjectMeta without any cluster or transient specific values
func (o *StepCreatePullRequestRepositoriesOptions) emptyObjectMeta(md *metav1.ObjectMeta) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name: md.Name,
	}
}
