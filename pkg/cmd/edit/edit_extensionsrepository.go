package edit

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/ghodss/yaml"
	survey "gopkg.in/AlecAivazis/survey.v1"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/extensions"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	editExtensionsRepositoryLong = templates.LongDesc(`
		Sets Extensions Repository in use
`)

	editExtensionsRepositoryExample = templates.Examples(`
		# Prompt to select an Extensions Repository from a list or enter your own
		jx edit extensionsrepository

"
	`)
)

const (
	optionExtensionsRepositoryUrl          = "url"
	optionExtensionsRepositoryGitHub       = "github"
	optionExtensionsRepositoryHelmChart    = "helm-chart"
	optionExtensionsRepositoryHelmRepo     = "helm-repo"
	optionExtensionsRepositoryHelmRepoName = "helm-repo-name"
	optionExtensionsRepositoryHelmUsername = "helm-username"
	optionExtensionsRepositoryHelmPassword = "helm-password"
	other                                  = "Other"
)

type EditExtensionsRepositoryOptions struct {
	EditOptions

	ExtensionsRepositoryUrl          string
	ExtensionsRepositoryGitHub       string
	ExtensionsRepositoryHelmChart    string
	ExtensionsRepositoryHelmRepo     string
	ExtensionsRepositoryHelmRepoName string
	ExtensionsRepositoryHelmUsername string
	ExtensionsRepositoryHelmPassword string
}

func NewCmdEditExtensionsRepository(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &EditExtensionsRepositoryOptions{
		EditOptions: EditOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "extensionsrepository",
		Short:   editExtensionsRepositoryLong,
		Example: editExtensionsRepositoryExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryUrl, optionExtensionsRepositoryUrl, "", "", "The extensions repository URL to use")
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryGitHub, optionExtensionsRepositoryGitHub, "", "", "The extensions repository GitHub repo to use")
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryHelmChart, optionExtensionsRepositoryHelmChart, "", "", "The extensions repository Helm Chart to use")
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryHelmRepo, optionExtensionsRepositoryHelmRepo, "", "", "The extensions repository Helm Chart Repo to use")
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryHelmRepoName, optionExtensionsRepositoryHelmRepoName, "", "", "The extensions repository Helm Chart Repo Name to use")
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryHelmUsername, optionExtensionsRepositoryHelmUsername, "", "", "The extensions repository Helm Chart Username to use")
	cmd.Flags().StringVarP(&options.ExtensionsRepositoryHelmPassword, optionExtensionsRepositoryHelmPassword, "", "", "The extensions repository Helm Chart Password to use")

	return cmd
}

// Run implements the command
func (o *EditExtensionsRepositoryOptions) Run() error {
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	extensionsConfig, err := extensions.GetOrCreateExtensionsConfig(kubeClient, ns)
	if err != nil {
		return err
	}
	current := jenkinsv1.ExtensionRepositoryReference{}
	var userpass bool
	var username string
	var password string

	knownRepositories := jenkinsv1.ExtensionRepositoryReferenceList{}

	err = yaml.Unmarshal([]byte(extensionsConfig.Data[jenkinsv1.ExtensionsConfigKnownRepositories]), &knownRepositories)
	if err != nil {
		return err
	}

	if o.ExtensionsRepositoryGitHub != "" {
		current.GitHub = o.ExtensionsRepositoryGitHub
	} else if o.ExtensionsRepositoryUrl != "" {
		current.Url = o.ExtensionsRepositoryUrl
	} else if o.ExtensionsRepositoryHelmChart != "" && o.ExtensionsRepositoryHelmRepo != "" && o.ExtensionsRepositoryHelmRepoName != "" {
		if o.ExtensionsRepositoryHelmUsername != "" && o.ExtensionsRepositoryHelmPassword != "" {
			userpass = true
			username = o.ExtensionsRepositoryHelmUsername
			password = o.ExtensionsRepositoryHelmPassword
		}
		current.Chart.Repo = o.ExtensionsRepositoryHelmRepo
		current.Chart.RepoName = o.ExtensionsRepositoryHelmRepoName
		current.Chart.Name = o.ExtensionsRepositoryHelmChart
	} else {

		err = yaml.Unmarshal([]byte(extensionsConfig.Data[jenkinsv1.ExtensionsConfigRepository]), &current)
		if err != nil {
			return err
		}

		askMap := make(map[string]jenkinsv1.ExtensionRepositoryReference)
		asks := make([]string, 0)
		for _, k := range knownRepositories.Repositories {
			if k.Url != "" {
				asks = append(asks, k.Url)
				askMap[k.Url] = k
			} else if k.GitHub != "" {
				asks = append(asks, k.GitHub)
				askMap[k.GitHub] = k
			} else if k.Chart.Name != "" {
				msg := fmt.Sprintf("helm chart %s in repo %s", k.Chart.Name, k.Chart.Repo)
				asks = append(asks, msg)
				askMap[msg] = k
			}

		}
		asks = append(asks, other)
		r, err := util.PickName(asks, "Pick the repository to use", "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		if r == other {
			types := []string{
				"URL", "Helm", "GitHub",
			}
			current = jenkinsv1.ExtensionRepositoryReference{}
			t, err := util.PickName(types, "What type of repository?", "", o.GetIOFileHandles())
			if t == "URL" {
				prompt := &survey.Input{
					Message: "Extensions Repository URL",
					Help:    "Enter an Extensions Repository URL to use",
				}
				surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
				survey.AskOne(prompt, &current.Url, nil, surveyOpts)
			} else if t == "GitHub" {
				prompt := &survey.Input{
					Message: "GitHub org/repo",
					Help:    "Enter Github org and repo to use e.g. acme/myrepo",
				}
				surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
				survey.AskOne(prompt, &current.GitHub, nil, surveyOpts)
			} else if t == "Helm" {

				prompt := &survey.Input{
					Message: "Helm Chart Repo Name",
					Help:    "Enter the Helm Chart Repo Name to use e.g. acme-corp",
				}
				surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
				survey.AskOne(prompt, &current.Chart.RepoName, nil, surveyOpts)
				prompt = &survey.Input{
					Message: "Helm Chart Repo",
					Help:    "Enter the Helm Chart Repo to use e.g. storage.googleapis.com/jenkinsx-chartmuseum",
				}
				surveyOpts = survey.WithStdio(o.In, o.Out, o.Err)
				survey.AskOne(prompt, &current.Chart.Repo, nil, surveyOpts)
				confirmPrompt := &survey.Confirm{
					Message: "Username/Password required?",
					Help:    "Does the Chart Repo require a username and password?",
					Default: true,
				}
				surveyOpts = survey.WithStdio(o.In, o.Out, o.Err)
				survey.AskOne(confirmPrompt, &userpass, nil, surveyOpts)
				if userpass {
					prompt = &survey.Input{
						Message: "Username",
						Help:    "Enter the Helm Chart Name to use",
					}
					surveyOpts = survey.WithStdio(o.In, o.Out, o.Err)
					survey.AskOne(prompt, &username, nil, surveyOpts)

					promptPass := &survey.Password{
						Message: "Password",
						Help:    "Enter the Helm Chart Name to use",
					}
					surveyOpts = survey.WithStdio(o.In, o.Out, o.Err)
					survey.AskOne(promptPass, &password, nil, surveyOpts)
					if err != nil {
						return err
					}
				} else {
					_, err := o.AddHelmBinaryRepoIfMissing(current.Chart.Repo, current.Chart.RepoName, "", "")
					if err != nil {
						return err
					}
				}

				prompt = &survey.Input{
					Message: "Helm Chart Name",
					Help:    "Enter the Helm Chart Name to use",
				}
				surveyOpts = survey.WithStdio(o.In, o.Out, o.Err)
				survey.AskOne(prompt, &current.Chart.Name, nil, surveyOpts)
			}
		} else {
			current = askMap[r]
		}
	}
	cBytes, err := yaml.Marshal(&current)
	if err != nil {
		return err
	}
	extensionsConfig.Data[jenkinsv1.ExtensionsConfigRepository] = string(cBytes)

	found := false
	for _, r := range knownRepositories.Repositories {
		if (r.Chart.Name != "" && r.Chart.Name == current.Chart.Name) || (r.Url != "" && r.Url == current.Url) || (r.GitHub != "" && r.GitHub == current.GitHub) {
			found = true
		}
	}
	if !found {
		knownRepositories.Repositories = append(knownRepositories.Repositories, current)
		kRBytes, err := yaml.Marshal(&knownRepositories)
		if err != nil {
			return err
		}
		extensionsConfig.Data[jenkinsv1.ExtensionsConfigKnownRepositories] = string(kRBytes)
	}
	_, err = kubeClient.CoreV1().ConfigMaps(ns).Update(extensionsConfig)
	if err != nil {
		return err
	}
	var msg string
	if current.Url != "" {
		msg = current.Url
	} else if current.GitHub != "" {
		msg = fmt.Sprintf("GitHub repo at %s", util.ColorInfo(current.GitHub))
	} else if current.Chart.Name != "" {
		repoUrl := current.Chart.Repo
		if userpass {
			chartRepo := strings.TrimPrefix(strings.TrimPrefix(current.Chart.Repo, "https://"), "http://")
			repoUrl = fmt.Sprintf("https://%s:%s@%s", username, password, chartRepo)
		} else {
			repoUrl = fmt.Sprintf("https://%s", current.Chart.Repo)
		}
		_, err := o.AddHelmBinaryRepoIfMissing(repoUrl, current.Chart.RepoName, "", "")
		if err != nil {
			return err
		}
		msg = fmt.Sprintf("Chart %s in repo %s", util.ColorInfo(current.Chart.Name), util.ColorInfo(current.Chart.Repo))
	}
	log.Logger().Infof("Set Extensions Repository to %s", msg)

	return nil

}
