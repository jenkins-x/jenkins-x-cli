package kube

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/jenkinsfile"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	survey "gopkg.in/AlecAivazis/survey.v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var useForkForEnvGitRepo = false

// ResolveChartMuseumURLFn used to resolve the chart repository URL if using remote environments
type ResolveChartMuseumURLFn func() (string, error)

// CreateEnvironmentSurvey creates a Survey on the given environment using the default options
// from the CLI
func CreateEnvironmentSurvey(batchMode bool, authConfigSvc auth.ConfigService, devEnv *v1.Environment, data *v1.Environment,
	config *v1.Environment, update bool, forkEnvGitURL string, ns string, jxClient versioned.Interface, kubeClient kubernetes.Interface, envDir string,
	gitRepoOptions *gits.GitRepositoryOptions, helmValues config.HelmValuesConfig, prefix string, git gits.Gitter, chartMusemFn ResolveChartMuseumURLFn, handles util.IOFileHandles) (gits.GitProvider, error) {
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	name := data.Name
	createMode := name == ""
	if createMode {
		if config.Name != "" {
			err := ValidNameOption(OptionName, config.Name)
			if err != nil {
				return nil, err
			}
			if !update {
				err = ValidateEnvironmentDoesNotExist(jxClient, ns, config.Name)
				if err != nil {
					return nil, err
				}
			}
			data.Name = config.Name
		} else {
			if batchMode {
				return nil, fmt.Errorf("environment name cannot be empty. Use --name option.")
			}

			validator := func(val interface{}) error {
				err := ValidateName(val)
				if err != nil {
					return err
				}
				str, ok := val.(string)
				if !ok {
					return fmt.Errorf("Expected string value")
				}
				v := ValidateEnvironmentDoesNotExist(jxClient, ns, str)
				return v
			}

			q := &survey.Input{
				Message: "Name:",
				Help:    "The Environment name must be unique, lower case and a valid DNS name",
			}
			err := survey.AskOne(q, &data.Name, validator, surveyOpts)
			if err != nil {
				return nil, err
			}
		}
	}
	if string(config.Spec.Kind) != "" {
		data.Spec.Kind = config.Spec.Kind
	} else {
		if string(data.Spec.Kind) == "" {
			data.Spec.Kind = v1.EnvironmentKindTypePermanent
		}
	}
	if config.Spec.Label != "" {
		data.Spec.Label = config.Spec.Label
	} else {
		defaultValue := data.Spec.Label
		if defaultValue == "" {
			defaultValue = strings.Title(data.Name)
		}
		q := &survey.Input{
			Message: "Label:",
			Default: defaultValue,
			Help:    "The Environment label is a person friendly descriptive text like 'Staging' or 'Production'",
		}
		err := survey.AskOne(q, &data.Spec.Label, survey.Required, surveyOpts)
		if err != nil {
			return nil, err
		}
	}
	if config.Spec.Namespace != "" {
		err := ValidNameOption(OptionNamespace, config.Spec.Namespace)
		if err != nil {
			return nil, err
		}
		data.Spec.Namespace = config.Spec.Namespace
	} else {
		defaultValue := data.Spec.Namespace
		if defaultValue == "" {
			// lets use the namespace as a team name
			defaultValue = data.Namespace
			if defaultValue == "" {
				defaultValue = ns
			}
			if data.Name != "" {
				if defaultValue == "" {
					defaultValue = data.Name
				} else {
					defaultValue += "-" + data.Name
				}
			}
		}
		if batchMode {
			data.Spec.Namespace = defaultValue
		} else {
			q := &survey.Input{
				Message: "Namespace:",
				Default: defaultValue,
				Help:    "The Kubernetes namespace name to use for this Environment",
			}
			err := survey.AskOne(q, &data.Spec.Namespace, ValidateName, surveyOpts)
			if err != nil {
				return nil, err
			}
		}
	}

	if helmValues.ExposeController.Config.Domain == "" {
		ic, err := GetIngressConfig(kubeClient, ns)
		if err != nil {
			return nil, err
		}

		if batchMode {
			log.Logger().Infof("Running in batch mode and no domain flag used so defaulting to team domain %s", ic.Domain)
			helmValues.ExposeController.Config.Domain = ic.Domain
		} else {
			q := &survey.Input{
				Message: "Domain:",
				Default: ic.Domain,
				Help:    "Domain to expose ingress endpoints.  Example: jenkinsx.io, leave blank if no appplications are to be exposed via ingress rules",
			}
			err := survey.AskOne(q, &helmValues.ExposeController.Config.Domain, nil, surveyOpts)
			if err != nil {
				return nil, err
			}
		}
	}

	data.Spec.RemoteCluster = config.Spec.RemoteCluster
	if !batchMode {
		var err error
		data.Spec.RemoteCluster, err = util.Confirm("Environment in separate cluster to Dev Environment:",
			data.Spec.RemoteCluster, " Is this Environment going to be in a different cluster to the Development environment. For help on Multi Cluster support see: https://jenkins-x.io/getting-started/multi-cluster/", handles)
		if err != nil {
			return nil, err
		}
	}
	if config.Spec.Cluster != "" {
		data.Spec.Cluster = config.Spec.Cluster
	} else {
		if data.Spec.RemoteCluster {
			// lets not show the UI for this if users specify the namespace via arguments
			if !createMode || config.Spec.Namespace == "" {
				defaultValue := data.Spec.Cluster
				if batchMode {
					data.Spec.Cluster = defaultValue
				} else {
					q := &survey.Input{
						Message: "Cluster URL:",
						Default: defaultValue,
						Help:    "The Kubernetes cluster URL to use to host this Environment. You can leave this blank for now.",
					}
					// TODO validate/transform to match valid kubnernetes cluster syntax
					err := survey.AskOne(q, &data.Spec.Cluster, nil, surveyOpts)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if string(config.Spec.PromotionStrategy) != "" {
		data.Spec.PromotionStrategy = config.Spec.PromotionStrategy
	} else {
		promoteValues := []string{
			string(v1.PromotionStrategyTypeAutomatic),
			string(v1.PromotionStrategyTypeManual),
			string(v1.PromotionStrategyTypeNever),
		}
		defaultValue := string(data.Spec.PromotionStrategy)
		if defaultValue == "" {
			defaultValue = string(v1.PromotionStrategyTypeAutomatic)
		}
		q := &survey.Select{
			Message: "Promotion Strategy:",
			Options: promoteValues,
			Default: defaultValue,
			Help:    "Whether we promote to this Environment automatically, manually or never",
		}
		textValue := ""
		err := survey.AskOne(q, &textValue, survey.Required, surveyOpts)
		if err != nil {
			return nil, err
		}
		if textValue != "" {
			data.Spec.PromotionStrategy = v1.PromotionStrategyType(textValue)
		}
	}
	if string(data.Spec.PromotionStrategy) == "" {
		data.Spec.PromotionStrategy = v1.PromotionStrategyTypeAutomatic
	}
	if config.Spec.Order != 0 {
		data.Spec.Order = config.Spec.Order
	} else {
		order := data.Spec.Order
		if order == 0 {
			// TODO should we generate an order to default to last one?
			order = 100
		}
		defaultValue := util.Int32ToA(order)
		q := &survey.Input{
			Message: "Order:",
			Default: defaultValue,
			Help:    "This number is used to sort Environments in sequential order, lowest first",
		}
		textValue := ""
		err := survey.AskOne(q, &textValue, survey.Required, surveyOpts)
		if err != nil {
			return nil, err
		}
		if textValue != "" {
			i, err := util.AtoInt32(textValue)
			if err != nil {
				return nil, fmt.Errorf("Failed to convert input '%s' to number: %s", textValue, err)
			}
			data.Spec.Order = i
		}
	}
	if batchMode && gitRepoOptions.Owner == "" {
		devEnvGitOwner, err := GetDevEnvGitOwner(jxClient)
		if err != nil {
			return nil, fmt.Errorf("Failed to get default Git owner for repos: %s", err)
		}
		if devEnvGitOwner != "" {
			gitRepoOptions.Owner = devEnvGitOwner
		} else {
			gitRepoOptions.Owner = gitRepoOptions.Username
		}
		log.Logger().Infof("Using %s environment git owner in batch mode.", util.ColorInfo(gitRepoOptions.Owner))
	}
	_, gitProvider, err := CreateEnvGitRepository(batchMode, authConfigSvc, devEnv, data, config, forkEnvGitURL, envDir, gitRepoOptions, helmValues, prefix, git, chartMusemFn, handles)
	return gitProvider, err
}

// CreateEnvGitRepository creates the git repository for the given Environment
func CreateEnvGitRepository(batchMode bool, authConfigSvc auth.ConfigService, devEnv *v1.Environment, data *v1.Environment, config *v1.Environment, forkEnvGitURL string, envDir string, gitRepoOptions *gits.GitRepositoryOptions, helmValues config.HelmValuesConfig, prefix string, git gits.Gitter, chartMusemFn ResolveChartMuseumURLFn, handles util.IOFileHandles) (*gits.GitRepository, gits.GitProvider, error) {
	var gitProvider gits.GitProvider
	var repo *gits.GitRepository
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	createRepo := false
	if config.Spec.Source.URL != "" {
		data.Spec.Source.URL = config.Spec.Source.URL
	} else {
		showURLEdit := devEnv.Spec.TeamSettings.UseGitOps
		if data.Spec.Source.URL == "" && !showURLEdit {
			if devEnv.Spec.TeamSettings.AskOnCreate {
				confirm := &survey.Confirm{
					Message: "Would you like to use GitOps to manage this environment? :",
					Default: false,
				}
				err := survey.AskOne(confirm, &showURLEdit, nil, surveyOpts)
				if err != nil {
					return repo, nil, errors.Wrap(err, "asking enable GitOps question")
				}
			} else {
				showURLEdit = true
			}
		}
		if showURLEdit {
			if data.Spec.Source.URL == "" {
				if batchMode {
					createRepo = true
				} else {
					confirm := &survey.Confirm{
						Message: fmt.Sprintf("We will now create a Git repository to store your %s environment, ok? :", data.Name),
						Default: true,
					}
					err := survey.AskOne(confirm, &createRepo, nil, surveyOpts)
					if err != nil {
						return repo, nil, errors.Wrapf(err, "asking to create the git repository %q", data.Name)
					}
				}

				if createRepo {
					showURLEdit = false
					var err error
					repo, gitProvider, err = DoCreateEnvironmentGitRepo(batchMode, authConfigSvc, data, forkEnvGitURL, envDir, gitRepoOptions, helmValues, prefix, git, chartMusemFn, handles)
					if err != nil {
						return repo, gitProvider, errors.Wrap(err, "creating environment git repository")
					}
					data.Spec.Source.URL = repo.CloneURL
				}
			} else {
				showURLEdit = true
			}
			if showURLEdit && !batchMode {
				q := &survey.Input{
					Message: "Git URL for the Environment source code:",
					Default: data.Spec.Source.URL,
					Help:    "The git clone URL for the Environment's Helm charts source code and custom configuration",
				}
				err := survey.AskOne(q, &data.Spec.Source.URL, survey.Required, surveyOpts)
				if err != nil {
					return repo, nil, errors.Wrap(err, "asking for environment git clone URL")
				}
			}
		}
	}
	if config.Spec.Source.Ref != "" {
		data.Spec.Source.Ref = config.Spec.Source.Ref
	} else {
		if data.Spec.Source.URL != "" || data.Spec.Source.Ref != "" {
			if batchMode {
				createRepo = true
			} else {
				defaultBranch := data.Spec.Source.Ref
				if defaultBranch == "" {
					defaultBranch = "master"
				}
				q := &survey.Input{
					Message: "Git branch for the Environment source code:",
					Default: defaultBranch,
					Help:    "The Git release branch in the Environments Git repository used to store Helm charts source code and custom configuration",
				}
				err := survey.AskOne(q, &data.Spec.Source.Ref, nil, surveyOpts)
				if err != nil {
					return repo, nil, errors.Wrap(err, "asking git branch for environment source")
				}
			}
		}
	}
	return repo, gitProvider, nil
}

// DoCreateEnvironmentGitRepo actually creates the git repository for the environment
func DoCreateEnvironmentGitRepo(batchMode bool, authConfigSvc auth.ConfigService, env *v1.Environment, forkEnvGitURL string,
	environmentsDir string, gitRepoOptions *gits.GitRepositoryOptions, helmValues config.HelmValuesConfig, prefix string,
	git gits.Gitter, chartMuseumFn ResolveChartMuseumURLFn, handles util.IOFileHandles) (*gits.GitRepository, gits.GitProvider, error) {
	defaultRepoName := fmt.Sprintf("environment-%s-%s", prefix, env.Name)
	details, err := gits.PickNewGitRepository(batchMode, authConfigSvc, defaultRepoName, gitRepoOptions, nil, nil, git, handles)
	if err != nil {
		return nil, nil, errors.Wrap(err, "picking new git repository for environment")
	}
	org := details.Organisation

	repoName := details.RepoName
	owner := org
	if owner == "" {
		owner = details.User.Username
	}
	envDir := filepath.Join(environmentsDir, owner)
	provider := details.GitProvider

	repo, err := provider.GetRepository(owner, repoName)
	if err == nil {
		log.Logger().Infof("Git repository %s/%s already exists", util.ColorInfo(owner), util.ColorInfo(repoName))
		// if the repo already exists then lets just modify it if required
		dir, err := util.CreateUniqueDirectory(envDir, details.RepoName, util.MaximumNewDirectoryAttempts)
		if err != nil {
			return nil, nil, errors.Wrap(err, "creating unique directory for environment repo")
		}
		pushGitURL, err := git.CreateAuthenticatedURL(repo.CloneURL, details.User)
		if err != nil {
			return nil, nil, errors.Wrap(err, "creating push URL for environment repo")
		}
		err = git.Clone(pushGitURL, dir)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "cloning environment from %q into %q", pushGitURL, dir)
		}
		err = ModifyNamespace(handles.Out, dir, env, git, chartMuseumFn)
		if err != nil {
			return nil, nil, errors.Wrap(err, "modifying environment namespace")
		}
		err = addValues(handles.Out, dir, helmValues, git)
		if err != nil {
			return nil, nil, errors.Wrap(err, "adding helm values to the environment")
		}
		err = git.PushMaster(dir)
		if err != nil {
			return nil, nil, errors.Wrap(err, "pushing environment master branch")
		}
		log.Logger().Infof("Pushed Git repository to %s\n\n", util.ColorInfo(repo.HTMLURL))
	} else {
		log.Logger().Infof("Creating Git repository %s/%s\n", util.ColorInfo(owner), util.ColorInfo(repoName))

		if forkEnvGitURL != "" {
			gitInfo, err := gits.ParseGitURL(forkEnvGitURL)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "parsing forked environment git URL %q", forkEnvGitURL)
			}
			originalOrg := gitInfo.Organisation
			originalRepo := gitInfo.Name
			if useForkForEnvGitRepo && gitInfo.IsGitHub() && provider.IsGitHub() && originalOrg != "" && originalRepo != "" {
				// lets try fork the repository and rename it
				repo, err := provider.ForkRepository(originalOrg, originalRepo, org)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to fork GitHub repo %s/%s to organisation %s due to %s",
						originalOrg, originalRepo, org, err)
				}
				if repoName != originalRepo {
					repo, err = provider.RenameRepository(owner, originalRepo, repoName)
					if err != nil {
						return nil, nil, fmt.Errorf("failed to rename GitHub repo %s/%s to organisation %s due to %s",
							originalOrg, originalRepo, repoName, err)
					}
				}
				log.Logger().Infof("Forked Git repository to %s\n\n", util.ColorInfo(repo.HTMLURL))

				dir, err := util.CreateUniqueDirectory(envDir, repoName, util.MaximumNewDirectoryAttempts)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "creating unique dir to fork environment repository %q", envDir)
				}
				err = git.Clone(repo.CloneURL, dir)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "cloning the environment %q", repo.CloneURL)
				}
				err = git.SetRemoteURL(dir, "upstream", forkEnvGitURL)
				if err != nil {
					return nil, nil, errors.Wrapf(err, "setting remote upstream %q in forked environment repo", forkEnvGitURL)
				}
				err = git.PullUpstream(dir)
				if err != nil {
					return nil, nil, errors.Wrap(err, "pulling upstream of forked environment repository")
				}
				err = ModifyNamespace(handles.Out, dir, env, git, chartMuseumFn)
				if err != nil {
					return nil, nil, errors.Wrap(err, "modifying namespace of forked environment")
				}
				err = addValues(handles.Out, dir, helmValues, git)
				if err != nil {
					return nil, nil, errors.Wrap(err, "adding helm values to the forked environment repo")
				}
				err = git.Push(dir, "origin", false, "HEAD")
				if err != nil {
					return nil, nil, errors.Wrapf(err, "pushing forked environment dir %q", dir)
				}
				return repo, provider, nil
			}
		}

		// default to forking the URL if possible...
		repo, err = details.CreateRepository()
		if err != nil {
			return nil, nil, errors.Wrap(err, "creating the repository")
		}

		if forkEnvGitURL != "" {
			// now lets clone the fork and push it...
			dir, err := util.CreateUniqueDirectory(envDir, details.RepoName, util.MaximumNewDirectoryAttempts)
			if err != nil {
				return nil, nil, errors.Wrap(err, "create unique directory for environment fork clone")
			}
			err = git.Clone(forkEnvGitURL, dir)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "cloning the forked environment %q into %q", forkEnvGitURL, dir)
			}
			pushGitURL, err := git.CreateAuthenticatedURL(repo.CloneURL, details.User)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "creating the push URL for %q", repo.CloneURL)
			}
			err = git.AddRemote(dir, "upstream", forkEnvGitURL)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "adding remote %q to forked env clone", forkEnvGitURL)
			}
			err = git.UpdateRemote(dir, pushGitURL)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "updating remote %q", pushGitURL)
			}
			err = ModifyNamespace(handles.Out, dir, env, git, chartMuseumFn)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "modifying dev environment namespace")
			}
			err = addValues(handles.Out, dir, helmValues, git)
			if err != nil {
				return nil, nil, errors.Wrap(err, "adding helm values into environment git repository")
			}
			err = git.PushMaster(dir)
			if err != nil {
				return nil, nil, errors.Wrap(err, "push forked environment git repository")
			}
			log.Logger().Infof("Pushed Git repository to %s\n\n", util.ColorInfo(repo.HTMLURL))
		}
	}
	return repo, provider, nil
}

// GetDevEnvTeamSettings gets the team settings from the specified namespace.
func GetDevEnvTeamSettings(jxClient versioned.Interface, ns string) (*v1.TeamSettings, error) {
	devEnv, err := GetDevEnvironment(jxClient, ns)
	if err != nil {
		log.Logger().Errorf("Error loading team settings. %v", err)
		return nil, err
	}
	if devEnv != nil {
		return &devEnv.Spec.TeamSettings, nil
	}
	return nil, fmt.Errorf("unable to find development environment in %s to get team settings", ns)
}

// GetDevEnvGitOwner gets the default GitHub owner/organisation to use for Environment repos. This takes the setting
// from the 'jx' Dev Env to get the one that was selected at installation time.
func GetDevEnvGitOwner(jxClient versioned.Interface) (string, error) {
	adminDevEnv, err := GetDevEnvironment(jxClient, "jx")
	if err != nil {
		log.Logger().Errorf("Error loading team settings. %v", err)
		return "", err
	}
	if adminDevEnv != nil {
		return adminDevEnv.Spec.TeamSettings.EnvOrganisation, nil
	}
	return "", errors.New("Unable to find development environment in 'jx' to take git owner from")
}

// ModifyNamespace modifies the namespace
func ModifyNamespace(out io.Writer, dir string, env *v1.Environment, git gits.Gitter, chartMusemFn ResolveChartMuseumURLFn) error {
	ns := env.Spec.Namespace
	if ns == "" {
		return fmt.Errorf("No Namespace is defined for Environment %s", env.Name)
	}

	// makefile changes
	file := filepath.Join(dir, "Makefile")
	exists, err := util.FileExists(file)
	if err != nil {
		return err
	}
	if !exists {
		log.Logger().Warnf("WARNING: Could not find a Makefile in %s", dir)
		return nil
	}
	input, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	lines := strings.Split(string(input), "\n")
	err = ReplaceMakeVariable(lines, "NAMESPACE", "\""+ns+"\"")
	if err != nil {
		return err
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(file, []byte(output), 0644)
	if err != nil {
		return err
	}

	// Jenkinsfile changes
	file = filepath.Join(dir, "Jenkinsfile")
	exists, err = util.FileExists(file)
	if err != nil {
		return err
	}
	if exists {
		input, err := ioutil.ReadFile(file)
		if err != nil {
			return err
		}
		lines := strings.Split(string(input), "\n")
		err = replaceEnvVar(lines, "DEPLOY_NAMESPACE", ns)
		if err != nil {
			return err
		}
		output := strings.Join(lines, "\n")
		err = ioutil.WriteFile(file, []byte(output), 0644)
		if err != nil {
			return err
		}
	}

	// lets ensure the namespace is set in a jenkins-x.yml file for tekton
	projectConfig, projectConfigFile, err := config.LoadProjectConfig(dir)
	if err != nil {
		return err
	}
	foundEnv := false
	for i := range projectConfig.Env {
		if projectConfig.Env[i].Name == "DEPLOY_NAMESPACE" {
			projectConfig.Env[i].Value = ns
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		projectConfig.Env = append(projectConfig.Env, corev1.EnvVar{
			Name:  "DEPLOY_NAMESPACE",
			Value: ns,
		})
	}
	foundEnv = false
	pipelineConfig := projectConfig.PipelineConfig
	if pipelineConfig == nil {
		projectConfig.PipelineConfig = &jenkinsfile.PipelineConfig{}
		pipelineConfig = projectConfig.PipelineConfig
	}
	for i := range pipelineConfig.Env {
		if pipelineConfig.Env[i].Name == "DEPLOY_NAMESPACE" {
			pipelineConfig.Env[i].Value = ns
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		pipelineConfig.Env = append(pipelineConfig.Env, corev1.EnvVar{
			Name:  "DEPLOY_NAMESPACE",
			Value: ns,
		})
	}

	if env.Spec.RemoteCluster && chartMusemFn != nil {
		// lets ensure we have a chart museum env var
		u, err := chartMusemFn()
		if err != nil {
			return errors.Wrapf(err, "failed to resolve Chart Museum URL for remote Environment %s", env.Name)
		}
		if u != "" {
			pipelineConfig.Env = SetEnvVar(pipelineConfig.Env, "CHART_REPOSITORY", u)
		}
	}

	err = projectConfig.SaveConfig(projectConfigFile)
	if err != nil {
		return err
	}

	err = git.Add(dir, "*")
	if err != nil {
		return err
	}
	changes, err := git.HasChanges(dir)
	if err != nil {
		return err
	}
	if changes {
		return git.CommitDir(dir, "Use correct namespace for environment")
	}
	return nil
}

func addValues(out io.Writer, dir string, values config.HelmValuesConfig, git gits.Gitter) error {
	file := filepath.Join(dir, "env", "values.yaml")
	exists, err := util.FileExists(file)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("could not find a values.yaml in %s", dir)
	}

	oldText, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	text, err := values.String()
	if err != nil {
		return err
	}

	sourceMap := map[string]interface{}{}
	overrideMap := map[string]interface{}{}
	err = yaml.Unmarshal(oldText, &sourceMap)
	if err != nil {
		return errors.Wrapf(err, "failed to parse YAML for file %s", file)
	}
	err = yaml.Unmarshal([]byte(text), &overrideMap)
	if err != nil {
		return errors.Wrapf(err, "failed to parse YAML for file %s", file)
	}

	if sourceMap != nil {
		// now lets merge together the 2 blobs of YAML
		util.CombineMapTrees(sourceMap, overrideMap)
	} else {
		sourceMap = overrideMap
	}

	output, err := yaml.Marshal(sourceMap)
	if err != nil {
		return errors.Wrap(err, "Failed to marshal the combined values YAML files back to YAML")
	}
	err = ioutil.WriteFile(file, output, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "Failed to save YAML file %s", file)
	}

	err = git.Add(dir, "*")
	if err != nil {
		return err
	}
	changes, err := git.HasChanges(dir)
	if err != nil {
		return err
	}
	if changes {
		return git.CommitDir(dir, "Add environment configuration")
	}
	return nil
}

// ReplaceMakeVariable needs a description
func ReplaceMakeVariable(lines []string, name string, value string) error {
	re, err := regexp.Compile(name + "\\s*:?=\\s*(.*)")
	if err != nil {
		return err
	}
	replaceValue := name + " := " + value
	for i, line := range lines {
		lines[i] = re.ReplaceAllString(line, replaceValue)
	}
	return nil
}

func replaceEnvVar(lines []string, name string, value string) error {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, name) {
			remain := strings.TrimSpace(strings.TrimPrefix(trimmed, name))
			if strings.HasPrefix(remain, "=") {
				// lets preserve whitespace
				idx := strings.Index(line, name)
				lines[i] = line[0:idx] + name + ` = "` + value + `"`
			}
		}
	}
	return nil
}

// GetEnvironmentNames returns the sorted list of environment names
func GetEnvironmentNames(jxClient versioned.Interface, ns string) ([]string, error) {
	envNames := []string{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return envNames, err
	}
	SortEnvironments(envs.Items)
	for _, env := range envs.Items {
		n := env.Name
		if n != "" {
			envNames = append(envNames, n)
		}
	}
	sort.Strings(envNames)
	return envNames, nil
}

func IsPreviewEnvironment(env *v1.Environment) bool {
	return env != nil && env.Spec.Kind == v1.EnvironmentKindTypePreview
}

// GetFilteredEnvironmentNames returns the sorted list of environment names
func GetFilteredEnvironmentNames(jxClient versioned.Interface, ns string, fn func(environment *v1.Environment) bool) ([]string, error) {
	envNames := []string{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return envNames, err
	}
	SortEnvironments(envs.Items)
	for _, env := range envs.Items {
		n := env.Name
		if n != "" && fn(&env) {
			envNames = append(envNames, n)
		}
	}
	sort.Strings(envNames)
	return envNames, nil
}

// GetOrderedEnvironments returns a map of the environments along with the correctly ordered  names
func GetOrderedEnvironments(jxClient versioned.Interface, ns string) (map[string]*v1.Environment, []string, error) {
	m := map[string]*v1.Environment{}

	envNames := []string{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, envNames, err
	}
	SortEnvironments(envs.Items)
	for _, env := range envs.Items {
		n := env.Name
		copy := env
		m[n] = &copy
		if n != "" {
			envNames = append(envNames, n)
		}
	}
	return m, envNames, nil
}

// GetEnvironments returns a map of the environments along with a sorted list of names
func GetEnvironments(jxClient versioned.Interface, ns string) (map[string]*v1.Environment, []string, error) {
	m := map[string]*v1.Environment{}

	envNames := []string{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, envNames, err
	}
	for _, env := range envs.Items {
		n := env.Name
		copy := env
		m[n] = &copy
		if n != "" {
			envNames = append(envNames, n)
		}
	}
	sort.Strings(envNames)
	return m, envNames, nil
}

// GetEnvironment find an environment by name
func GetEnvironment(jxClient versioned.Interface, ns string, name string) (*v1.Environment, error) {
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, env := range envs.Items {
		if env.GetName() == name {
			return &env, nil
		}
	}
	return nil, fmt.Errorf("no environment with name '%s' found", name)
}

// GetEnvironmentsByPrURL find an environment by a pull request URL
func GetEnvironmentsByPrURL(jxClient versioned.Interface, ns string, prURL string) (*v1.Environment, error) {
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, env := range envs.Items {
		if env.Spec.PullRequestURL == prURL {
			return &env, nil
		}
	}
	return nil, fmt.Errorf("no environment found for PR '%s'", prURL)
}

// GetEnvironments returns the namespace name for a given environment
func GetEnvironmentNamespace(jxClient versioned.Interface, ns, environment string) (string, error) {
	env, err := jxClient.JenkinsV1().Environments(ns).Get(environment, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	if env == nil {
		return "", fmt.Errorf("no environment found called %s, try running `jx get env`", environment)
	}
	return env.Spec.Namespace, nil
}

// GetEditEnvironmentNamespace returns the namespace of the current users edit environment
func GetEditEnvironmentNamespace(jxClient versioned.Interface, ns string) (string, error) {
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	for _, env := range envs.Items {
		if env.Spec.Kind == v1.EnvironmentKindTypeEdit && env.Spec.PreviewGitSpec.User.Username == u.Username {
			return env.Spec.Namespace, nil
		}
	}
	return "", fmt.Errorf("The user %s does not have an Edit environment in home namespace %s", u.Username, ns)
}

// GetDevNamespace returns the developer environment namespace
// which is the namespace that contains the Environments and the developer tools like Jenkins
func GetDevNamespace(kubeClient kubernetes.Interface, ns string) (string, string, error) {
	env := ""
	namespace, err := kubeClient.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
	if err != nil {
		return ns, env, err
	}
	if namespace == nil {
		return ns, env, fmt.Errorf("No namespace found for %s", ns)
	}
	if namespace.Labels != nil {
		answer := namespace.Labels[LabelTeam]
		if answer != "" {
			ns = answer
		}
		env = namespace.Labels[LabelEnvironment]
	}
	return ns, env, nil
}

// GetTeams returns the Teams the user is a member of
func GetTeams(kubeClient kubernetes.Interface) ([]*corev1.Namespace, []string, error) {
	names := []string{}
	answer := []*corev1.Namespace{}
	namespaceList, err := kubeClient.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return answer, names, err
	}
	for idx, namespace := range namespaceList.Items {
		if namespace.Labels[LabelEnvironment] == LabelValueDevEnvironment {
			answer = append(answer, &namespaceList.Items[idx])
			names = append(names, namespace.Name)
		}
	}
	sort.Strings(names)
	return answer, names, nil
}

func PickEnvironment(envNames []string, defaultEnv string, handles util.IOFileHandles) (string, error) {
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	name := ""
	if len(envNames) == 0 {
		return "", nil
	} else if len(envNames) == 1 {
		name = envNames[0]
	} else {
		prompt := &survey.Select{
			Message: "Pick environment:",
			Options: envNames,
			Default: defaultEnv,
		}
		err := survey.AskOne(prompt, &name, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}
	return name, nil
}

type ByOrder []v1.Environment

func (a ByOrder) Len() int      { return len(a) }
func (a ByOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByOrder) Less(i, j int) bool {
	env1 := a[i]
	env2 := a[j]
	o1 := env1.Spec.Order
	o2 := env2.Spec.Order
	if o1 == o2 {
		return env1.Name < env2.Name
	}
	return o1 < o2
}

func SortEnvironments(environments []v1.Environment) {
	sort.Sort(ByOrder(environments))
}

// ByTimestamp is used to fileter a list of PipelineActivities by their given timestamp
type ByTimestamp []v1.PipelineActivity

func (a ByTimestamp) Len() int      { return len(a) }
func (a ByTimestamp) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByTimestamp) Less(i, j int) bool {
	act1 := a[i]
	act2 := a[j]
	t1 := act1.Spec.StartedTimestamp
	if t1 == nil {
		return false
	}
	t2 := act2.Spec.StartedTimestamp
	if t2 == nil {
		return true
	}

	return t1.Before(t2)
}

// SortActivities sorts a list of PipelineActivities
func SortActivities(activities []v1.PipelineActivity) {
	sort.Sort(ByTimestamp(activities))
}

// NewPermanentEnvironment creates a new permanent environment for testing
func NewPermanentEnvironment(name string) *v1.Environment {
	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "jx",
		},
		Spec: v1.EnvironmentSpec{
			Label:             strings.Title(name),
			Namespace:         "jx-" + name,
			PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
			Kind:              v1.EnvironmentKindTypePermanent,
		},
	}
}

// NewPermanentEnvironment creates a new permanent environment for testing
func NewPermanentEnvironmentWithGit(name string, gitUrl string) *v1.Environment {
	env := NewPermanentEnvironment(name)
	env.Spec.Source.URL = gitUrl
	env.Spec.Source.Ref = "master"
	return env
}

// NewPreviewEnvironment creates a new preview environment for testing
func NewPreviewEnvironment(name string) *v1.Environment {
	return &v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "jx",
		},
		Spec: v1.EnvironmentSpec{
			Label:             strings.Title(name),
			Namespace:         "jx-preview-" + name,
			PromotionStrategy: v1.PromotionStrategyTypeAutomatic,
			Kind:              v1.EnvironmentKindTypePreview,
		},
	}
}

// GetDevEnvironment returns the current development environment using the jxClient for the given ns.
// If the Dev Environment cannot be found, returns nil Environment (rather than an error). A non-nil error is only
// returned if there is an error fetching the Dev Environment.
func GetDevEnvironment(jxClient versioned.Interface, ns string) (*v1.Environment, error) {
	//Find the settings for the team
	environmentInterface := jxClient.JenkinsV1().Environments(ns)
	name := LabelValueDevEnvironment
	answer, err := environmentInterface.Get(name, metav1.GetOptions{})
	if err == nil {
		return answer, nil
	}
	selector := "env=dev"
	envList, err := environmentInterface.List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, err
	}
	if len(envList.Items) == 1 {
		return &envList.Items[0], nil
	}
	if len(envList.Items) == 0 {
		return nil, nil
	}
	return nil, fmt.Errorf("Error fetching dev environment resource definition in namespace %s, No Environment called: %s or with selector: %s found %d entries: %v",
		ns, name, selector, len(envList.Items), envList.Items)
}

// GetPreviewEnvironmentReleaseName returns the (helm) release name for the given (preview) environment
// or the empty string is the environment is not a preview environment, or has no release name associated with it
func GetPreviewEnvironmentReleaseName(env *v1.Environment) string {
	if !IsPreviewEnvironment(env) {
		return ""
	}
	return env.Annotations[AnnotationReleaseName]
}

// IsPermanentEnvironment indicates if an environment is permanent
func IsPermanentEnvironment(env *v1.Environment) bool {
	return env.Spec.Kind == v1.EnvironmentKindTypePermanent
}

// GetPermanentEnvironments returns a list with the current permanent environments
func GetPermanentEnvironments(jxClient versioned.Interface, ns string) ([]*v1.Environment, error) {
	result := []*v1.Environment{}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return result, errors.Wrapf(err, "listing the environments in namespace %q", ns)
	}
	for i := range envs.Items {
		env := &envs.Items[i]
		if IsPermanentEnvironment(env) {
			result = append(result, env)
		}
	}
	return result, nil
}
