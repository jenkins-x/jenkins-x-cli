package apps

import (
	"fmt"

	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"io/ioutil"

	"github.com/jenkins-x/jx/pkg/environments"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
)

// GitOpsOptions is the options used for Git Operations for apps
type GitOpsOptions struct {
	*InstallOptions
}

// AddApp adds the app with version rooted in dir from the repository. An alias can be specified.
func (o *GitOpsOptions) AddApp(app string, dir string, version string, repository string, alias string, autoMerge bool) error {
	details := gits.PullRequestDetails{
		BranchName: "add-app-" + app + "-" + version,
		Title:      fmt.Sprintf("Add %s %s", app, version),
		Message:    fmt.Sprintf("Add app %s %s", app, version),
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter: o.Gitter,
		ModifyChartFn: environments.CreateAddRequirementFn(app, alias, version,
			repository, o.valuesFiles, dir, o.Verbose, o.Helmer),
		GitProvider: o.GitProvider,
	}

	info, err := options.Create(o.DevEnv, o.EnvironmentCloneDir, &details, nil, "", autoMerge)
	if err != nil {
		return errors.Wrapf(err, "creating pr for %s", app)
	}

	if info != nil {
		log.Logger().Infof("Added app via Pull Request %s", info.PullRequest.URL)
	} else {
		log.Logger().Infof("Already up to date")
	}
	return nil
}

// UpgradeApp upgrades the app (or all apps if empty) to a version (
// or latest if empty) from a repository with username and password.
// If one app is being upgraded an alias can be specified.
func (o *GitOpsOptions) UpgradeApp(app string, version string, repository string, username string, password string,
	alias string, interrogateChartFunc func(dir string, existing map[string]interface{}) (*ChartDetails,
		error), autoMerge bool) error {
	all := true
	details := gits.PullRequestDetails{}

	// use a random string in the branch name to ensure we use a unique git branch and fail to push
	rand, err := util.RandStringBytesMaskImprSrc(5)
	if err != nil {
		return errors.Wrapf(err, "failed to generate a random string")
	}

	if app != "" {
		all = false
		versionBranchName := version
		if versionBranchName == "" {
			versionBranchName = "latest"
		}
		details.BranchName = fmt.Sprintf("upgrade-app-%s-%s-%s", app, versionBranchName, rand)
	} else {
		details.BranchName = fmt.Sprintf("upgrade-all-apps-%s", rand)
		details.Title = fmt.Sprintf("Upgrade all apps")
		details.Message = fmt.Sprintf("Upgrade all apps:\n")
	}

	var interrogateCleanup func()
	defer func() {
		if interrogateCleanup != nil {
			interrogateCleanup()
		}
	}()
	inspectChartFunc := func(chartDir string, values map[string]interface{}) error {
		chartDetails, err := interrogateChartFunc(chartDir, values)
		interrogateCleanup = chartDetails.Cleanup
		if err != nil {
			return errors.Wrapf(err, "asking questions for %s", chartDir)
		}
		return nil
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter: o.Gitter,
		ModifyChartFn: environments.CreateUpgradeRequirementsFn(all, app, alias, version, username, password,
			o.Helmer, inspectChartFunc, o.Verbose, o.valuesFiles),
		GitProvider: o.GitProvider,
	}

	_, err = options.Create(o.DevEnv, o.EnvironmentCloneDir, &details, nil, app, autoMerge)
	if err != nil {
		return err
	}
	return nil
}

// DeleteApp deletes the app with alias
func (o *GitOpsOptions) DeleteApp(app string, alias string, autoMerge bool) error {

	modifyChartFn := func(requirements *helm.Requirements, metadata *chart.Metadata, values map[string]interface{},
		templates map[string]string, dir string, details *gits.PullRequestDetails) error {
		// See if the app already exists in requirements
		found := false
		for i, d := range requirements.Dependencies {
			if d.Name == app && d.Alias == alias {
				found = true
				requirements.Dependencies = append(requirements.Dependencies[:i], requirements.Dependencies[i+1:]...)
			}
		}
		// If app not found, add it
		if !found {
			a := app
			if alias != "" {
				a = fmt.Sprintf("%s with alias %s", a, alias)
			}
			return fmt.Errorf("unable to delete app %s as not installed", app)
		}
		if info, err := os.Stat(filepath.Join(dir, app)); err == nil {
			if info.IsDir() {
				err := util.DeleteFile(info.Name())
				if err != nil {
					return err
				}
			} else {
				log.Logger().Warnf("Not removing %s for %s because it is not a directory", info.Name(), app)
			}
		}
		return nil
	}
	details := gits.PullRequestDetails{
		BranchName: "delete-app-" + app,
		Title:      fmt.Sprintf("Delete %s", app),
		Message:    fmt.Sprintf("Delete app %s", app),
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter:        o.Gitter,
		ModifyChartFn: modifyChartFn,
		GitProvider:   o.GitProvider,
	}

	info, err := options.Create(o.DevEnv, o.EnvironmentCloneDir, &details, nil, "", autoMerge)
	if err != nil {
		return err
	}
	log.Logger().Infof("Delete app via Pull Request %s", info.PullRequest.URL)
	return nil
}

// GetApps retrieves all the apps information for the given appNames from the repository and / or the CRD API
func (o *GitOpsOptions) GetApps(appNames map[string]bool, expandFn func([]string) (*v1.AppList, error)) (*v1.AppList, error) {
	// AddApp, DeleteApp, and UpgradeApps delegate selecting/creating the directory to clone in to environments/gitops.go's
	// Create function, but here we need to create the directory explicitly. since we aren't calling Create, because we're
	// not creating a pull request.
	dir, err := ioutil.TempDir("", "get-apps-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	gitInfo, err := gits.ParseGitURL(o.DevEnv.Spec.Source.URL)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing dev env repo URL %s", o.DevEnv.Spec.Source.URL)
	}

	providerInfo, err := o.GitProvider.GetRepository(gitInfo.Organisation, gitInfo.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "determining git provider information for %s", o.DevEnv.Spec.Source.URL)
	}
	cloneUrl := providerInfo.CloneURL
	userDetails := o.GitProvider.UserAuth()
	originFetchURL, err := o.Gitter.CreateAuthenticatedURL(cloneUrl, &userDetails)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create authenticated fetch URL for %s", cloneUrl)
	}
	err = o.Gitter.Clone(originFetchURL, dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to clone %s to dir %s", cloneUrl, dir)
	}
	err = o.Gitter.Checkout(dir, o.DevEnv.Spec.Source.Ref)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to checkout %s to dir %s", o.DevEnv.Spec.Source.Ref, dir)
	}

	envDir := filepath.Join(dir, helm.DefaultEnvironmentChartDir)
	if err != nil {
		return nil, err
	}
	exists, err := util.DirExists(envDir)
	if err != nil {
		return nil, err
	}

	if !exists {
		envDir = dir
	}

	requirementsFile, err := ioutil.ReadFile(filepath.Join(envDir, helm.RequirementsFileName))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't read the environment's requirements.yaml file")
	}
	reqs := helm.Requirements{}
	err = yaml.Unmarshal(requirementsFile, &reqs)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't unmarshal the environment's requirements.yaml file")
	}

	appsList := v1.AppList{}
	for _, d := range reqs.Dependencies {
		if appNames[d.Name] == true || len(appNames) == 0 {
			//Make sure we ignore the jenkins-x-platform requirement
			if d.Name != "jenkins-x-platform" {
				resourcesInCRD, _ := expandFn([]string{d.Name})
				if len(resourcesInCRD.Items) != 0 {
					appsList.Items = append(appsList.Items, resourcesInCRD.Items...)
				} else {
					appPath := filepath.Join(envDir, d.Name, "templates", "app.yaml")
					exists, err := util.FileExists(appPath)
					if err != nil {
						return nil, errors.Wrapf(err, "there was a problem checking if %s exists", appPath)
					}
					if exists {
						appFile, err := ioutil.ReadFile(appPath)
						if err != nil {
							return nil, errors.Wrapf(err, "there was a problem reading the app.yaml file of %s", d.Name)
						}
						app := v1.App{}
						err = yaml.Unmarshal(appFile, &app)
						if err != nil {
							return nil, errors.Wrapf(err, "there was a problem unmarshalling the app.yaml file of %s", d.Name)
						}
						appsList.Items = append(appsList.Items, app)
					}
				}
			}
		}
	}
	return &appsList, nil
}
