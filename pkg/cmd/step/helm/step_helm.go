package helm

import (
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/jenkins-x/jx/pkg/cmd/step/git/credentials"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"k8s.io/helm/pkg/chartutil"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/spf13/cobra"

	"os"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

const (
	PROW_JOB_ID   = "PROW_JOB_ID"
	REPO_OWNER    = "REPO_OWNER"
	REPO_NAME     = "REPO_NAME"
	PULL_PULL_SHA = "PULL_PULL_SHA"
)

// StepHelmOptions contains the command line flags
type StepHelmOptions struct {
	step.StepOptions

	Dir         string
	https       bool
	GitProvider string

	versionResolver *versionstream.VersionResolver
}

// NewCmdStepHelm Steps a command object for the "step" command
func NewCmdStepHelm(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepHelmOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "helm",
		Short: "helm [command]",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdStepHelmApply(commonOpts))
	cmd.AddCommand(NewCmdStepHelmBuild(commonOpts))
	cmd.AddCommand(NewCmdStepHelmDelete(commonOpts))
	cmd.AddCommand(NewCmdStepHelmEnv(commonOpts))
	cmd.AddCommand(NewCmdStepHelmInstall(commonOpts))
	cmd.AddCommand(NewCmdStepHelmList(commonOpts))
	cmd.AddCommand(NewCmdStepHelmRelease(commonOpts))
	cmd.AddCommand(NewCmdStepHelmVersion(commonOpts))
	return cmd
}

// Run implements this command
func (o *StepHelmOptions) Run() error {
	return o.Cmd.Help()
}

func (o *StepHelmOptions) addStepHelmFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", ".", "The directory containing the helm chart to apply")
	cmd.Flags().BoolVarP(&o.https, "clone-https", "", true, "Clone the environment Git repo over https rather than ssh which uses `git@foo/bar.git`")
	cmd.Flags().BoolVarP(&o.RemoteCluster, "remote", "", false, "If enabled assume we are in a remote cluster such as a stand alone Staging/Production cluster")
	cmd.Flags().StringVarP(&o.GitProvider, "git-provider", "", "github.com", "The Git provider for the environment Git repository")
}

func (o *StepHelmOptions) dropRepositories(repoIds []string, message string) error {
	var answer error
	for _, repoId := range repoIds {
		err := o.dropRepository(repoId, message)
		if err != nil {
			log.Logger().Warnf("Failed to drop repository %s: %s", util.ColorInfo(repoIds), util.ColorError(err))
			if answer == nil {
				answer = err
			}
		}
	}
	return answer
}

func (o *StepHelmOptions) dropRepository(repoId string, message string) error {
	if repoId == "" {
		return nil
	}
	log.Logger().Infof("Dropping helm release repository %s", util.ColorInfo(repoId))
	err := o.RunCommand("mvn",
		"org.sonatype.plugins:helm-staging-maven-plugin:1.6.5:rc-drop",
		"-DserverId=oss-sonatype-staging",
		"-DhelmUrl=https://oss.sonatype.org",
		"-DstagingRepositoryId="+repoId,
		"-Ddescription=\""+message+"\" -DstagingProgressTimeoutMinutes=60")
	if err != nil {
		log.Logger().Warnf("Failed to drop repository %s due to: %s", repoId, err)
	} else {
		log.Logger().Infof("Dropped repository %s", util.ColorInfo(repoId))
	}
	return err
}

func (o *StepHelmOptions) releaseRepository(repoId string) error {
	if repoId == "" {
		return nil
	}
	log.Logger().Infof("Releasing helm release repository %s", util.ColorInfo(repoId))
	options := o
	err := options.RunCommand("mvn",
		"org.sonatype.plugins:helm-staging-maven-plugin:1.6.5:rc-release",
		"-DserverId=oss-sonatype-staging",
		"-DhelmUrl=https://oss.sonatype.org",
		"-DstagingRepositoryId="+repoId,
		"-Ddescription=\"Next release is ready\" -DstagingProgressTimeoutMinutes=60")
	if err != nil {
		log.Logger().Infof("Failed to release repository %s due to: %s", repoId, err)
	} else {
		log.Logger().Infof("Released repository %s", util.ColorInfo(repoId))
	}
	return err
}

func (o *StepHelmOptions) cloneProwPullRequest(dir, gitProvider string) (string, error) {

	stepOpts := step.StepOptions{
		CommonOptions: o.CommonOptions,
	}
	gitOpts := credentials.StepGitCredentialsOptions{
		StepOptions: stepOpts,
	}

	err := gitOpts.Run()
	if err != nil {
		return "", fmt.Errorf("failed to create Git credentials file: %v", err)
	}

	org := os.Getenv(REPO_OWNER)
	if org == "" {
		return "", fmt.Errorf("no %s env var found", REPO_OWNER)
	}

	repo := os.Getenv(REPO_NAME)
	if org == "" {
		return "", fmt.Errorf("no %s env var found", REPO_NAME)
	}

	var gitURL string
	if o.https {
		gitURL = fmt.Sprintf("https://%s/%s/%s.git", gitProvider, org, repo)
	} else {
		gitURL = fmt.Sprintf("git@%s:%s/%s.git", gitProvider, org, repo)
	}

	err = o.RunCommand("git", "clone", gitURL)
	if err != nil {
		return "", err
	}

	prCommit := os.Getenv(PULL_PULL_SHA)
	if org == "" {
		return "", fmt.Errorf("no %s env var found", PULL_PULL_SHA)
	}

	if dir != "" {
		dir = filepath.Join(dir, repo)
	} else {
		pwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(pwd, repo)
	}

	err = o.RunCommandFromDir(dir, "git", "checkout", prCommit)
	if err != nil {
		return "", err
	}

	err = o.RunCommand("cd", repo)
	if err != nil {
		return "", err
	}

	err = o.RunCommandFromDir(dir, "git", "checkout", "-b", "pr")
	if err != nil {
		return "", err
	}
	exists, err := util.DirExists(filepath.Join(dir, "env"))
	if err != nil {
		return "", err
	}
	if exists {
		dir = filepath.Join(dir, "env")
	}
	return dir, nil
}

func (o *StepHelmOptions) discoverValuesFiles(dir string) ([]string, error) {
	valuesFiles := []string{}
	for _, name := range []string{"values.yaml", helm.SecretsFileName, "myvalues.yaml"} {
		path := filepath.Join(dir, name)
		exists, err := util.FileExists(path)
		if err != nil {
			return valuesFiles, err
		}
		if exists {
			valuesFiles = append(valuesFiles, path)
		}
	}
	return valuesFiles, nil
}

func (o *StepHelmOptions) getOrCreateVersionResolver(requirementsConfig *config.RequirementsConfig) (*versionstream.VersionResolver, error) {
	if o.versionResolver == nil {
		vs := requirementsConfig.VersionStream

		var err error
		o.versionResolver, err = o.CreateVersionResolver(vs.URL, vs.Ref)
		if err != nil {
			return o.versionResolver, errors.Wrapf(err, "failed to create version resolver")
		}
	}
	return o.versionResolver, nil
}

func (o *StepHelmOptions) verifyRequirementsYAML(resolver *versionstream.VersionResolver, prefixes *versionstream.RepositoryPrefixes, fileName string) error {
	req, err := helm.LoadRequirementsFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to load %s", fileName)
	}

	modified := false
	for _, dep := range req.Dependencies {
		if dep.Version == "" {
			name := dep.Alias
			if name == "" {
				name = dep.Name
			}
			repo := dep.Repository
			if repo == "" {
				return fmt.Errorf("cannot to find a version for dependency %s in file %s as there is no 'repository'", name, fileName)
			}

			prefix := prefixes.PrefixForURL(repo)
			if prefix == "" {
				return fmt.Errorf("the helm repository %s does not have an associated prefix in in the 'charts/repositories.yml' file the version stream, so we cannot default the version in file %s", repo, fileName)
			}
			newVersion := ""
			fullChartName := prefix + "/" + dep.Name
			newVersion, err := resolver.StableVersionNumber(versionstream.KindChart, fullChartName)
			if err != nil {
				return errors.Wrapf(err, "failed to find version of chart %s in file %s", fullChartName, fileName)
			}
			if newVersion == "" {
				return fmt.Errorf("failed to find a version for dependency %s in file %s in the current version stream - please either add an explicit version to this file or add chart %s to the version stream", name, fileName, fullChartName)
			}
			dep.Version = newVersion
			modified = true
			log.Logger().Debugf("adding version %s to dependency %s in file %s", newVersion, name, fileName)
		}
	}

	if modified {
		err = helm.SaveFile(fileName, req)
		if err != nil {
			return errors.Wrapf(err, "failed to save %s", fileName)
		}
		log.Logger().Debugf("adding dependency versions to file %s", fileName)
	}
	return nil
}

func (o *StepHelmOptions) replaceMissingVersionsFromVersionStream(requirementsConfig *config.RequirementsConfig, dir string) error {
	fileName := filepath.Join(dir, helm.RequirementsFileName)
	exists, err := util.FileExists(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to check for file %s", fileName)
	}
	if !exists {
		log.Logger().Infof("No requirements file: %s so not checking for missing versions\n", fileName)
		return nil
	}

	vs := requirementsConfig.VersionStream

	log.Logger().Infof("Verifying the helm requirements versions in dir: %s using version stream URL: %s and git ref: %s\n", o.Dir, vs.URL, vs.Ref)

	resolver, err := o.getOrCreateVersionResolver(requirementsConfig)
	if err != nil {
		return errors.Wrapf(err, "failed to create version resolver")
	}

	prefixes, err := resolver.GetRepositoryPrefixes()
	if err != nil {
		return errors.Wrapf(err, "failed to load repository prefixes")
	}

	err = o.verifyRequirementsYAML(resolver, prefixes, fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to replace missing versions in file %s", fileName)
	}
	return nil
}

func (o *StepHelmOptions) createFuncMap(requirementsConfig *config.RequirementsConfig) (template.FuncMap, error) {
	funcMap := helm.NewFunctionMap()
	resolver, err := o.getOrCreateVersionResolver(requirementsConfig)

	if err != nil {
		return funcMap, err
	}

	// represents the helm template function
	// which can be used like: `{{ versionStream "chart" "foo/bar" }}
	funcMap["versionStream"] = func(kindString, name string) string {
		kind := versionstream.VersionKind(kindString)
		version, err := resolver.StableVersionNumber(kind, name)
		if err != nil {
			log.Logger().Errorf("failed to find %s version for %s in the version stream due to: %s\n", kindString, name, err.Error())
		}
		return version
	}
	return funcMap, nil
}

func (o *StepHelmOptions) overwriteProviderValues(requirements *config.RequirementsConfig, requirementsFileName string, valuesData []byte, params chartutil.Values, providersValuesDir string) ([]byte, error) {
	provider := requirements.Cluster.Provider
	if provider == "" {
		log.Logger().Warnf("No provider in the requirements file %s\n", requirementsFileName)
		return valuesData, nil
	}
	valuesTmplYamlFile := filepath.Join(providersValuesDir, provider, "values.tmpl.yaml")
	exists, err := util.FileExists(valuesTmplYamlFile)
	if err != nil {
		return valuesData, errors.Wrapf(err, "failed to check if file exists: %s", valuesTmplYamlFile)
	}
	log.Logger().Infof("Applying the kubernetes overrides at %s\n", util.ColorInfo(valuesTmplYamlFile))

	if !exists {
		log.Logger().Warnf("No provider specific values overrides exist in file %s\n", valuesTmplYamlFile)
		return valuesData, nil

	}
	funcMap, err := o.createFuncMap(requirements)
	if err != nil {
		return valuesData, err
	}

	overrideData, err := helm.ReadValuesYamlFileTemplateOutput(valuesTmplYamlFile, params, funcMap, requirements)
	if err != nil {
		return valuesData, errors.Wrapf(err, "failed to load provider specific helm value overrides %s", valuesTmplYamlFile)
	}
	if len(overrideData) == 0 {
		return valuesData, nil
	}

	// now lets apply the overrides
	values, err := helm.LoadValues(valuesData)
	if err != nil {
		return valuesData, errors.Wrapf(err, "failed to unmarshal the default helm values")
	}

	overrides, err := helm.LoadValues(overrideData)
	if err != nil {
		return valuesData, errors.Wrapf(err, "failed to unmarshal the default helm values")
	}

	util.CombineMapTrees(values, overrides)

	data, err := yaml.Marshal(values)
	return data, err
}

func (o *StepHelmOptions) getChartValues(targetNS string) ([]string, []string) {
	return []string{
			fmt.Sprintf("tags.jx-ns-%s=true", targetNS),
			fmt.Sprintf("global.jxNs%s=true", util.ToCamelCase(targetNS)),
		}, []string{
			fmt.Sprintf("global.jxNs=%s", targetNS),
		}
}
