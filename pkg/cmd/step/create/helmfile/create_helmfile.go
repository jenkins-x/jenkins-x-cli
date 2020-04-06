package helmfile

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path"

	"github.com/jenkins-x/jx/pkg/config"
	helmfile2 "github.com/jenkins-x/jx/pkg/helmfile"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/uuid"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/ghodss/yaml"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	helmfile = "helmfile.yaml"
)

var (
	createHelmfileLong = templates.LongDesc(`
		Creates a new helmfile.yaml from a jx-apps.yaml
`)

	createHelmfileExample = templates.Examples(`
		# Create a new helmfile.yaml from a jx-apps.yaml
		jx create helmfile
	`)
)

// GeneratedValues is a struct that gets marshalled into helm values for creating namespaces via helm
type GeneratedValues struct {
	Namespaces []string `json:"namespaces"`
}

// CreateHelmfileOptions the options for the create helmfile command
type CreateHelmfileOptions struct {
	options.CreateOptions

	dir        string
	outputDir  string
	valueFiles []string
}

// NewCmdCreateHelmfile  creates a command object for the "create" command
func NewCmdCreateHelmfile(commonOpts *opts.CommonOptions) *cobra.Command {
	o := &CreateHelmfileOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "helmfile",
		Short:   "Create a new helmfile",
		Long:    createHelmfileLong,
		Example: createHelmfileExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&o.dir, "dir", "", ".", "the directory to look for a 'jx-apps.yml' file")
	cmd.Flags().StringVarP(&o.outputDir, "outputDir", "", "", "The directory to write the helmfile.yaml file")
	cmd.Flags().StringArrayVarP(&o.valueFiles, "values", "", []string{""}, "specify values in a YAML file or a URL(can specify multiple)")

	return cmd
}

// Run implements the command
func (o *CreateHelmfileOptions) Run() error {

	apps, err := config.LoadApplicationsConfig(o.dir)
	if err != nil {
		return errors.Wrap(err, "failed to load applications")
	}

	helm := o.Helm()
	localHelmRepos, err := helm.ListRepos()
	if err != nil {
		return errors.Wrap(err, "failed listing helm repos")
	}

	// iterate over all apps and split them into phases to generate separate helmfiles for each
	var applications []config.Application
	var systemApplications []config.Application
	for _, app := range apps.Applications {
		// default phase is apps so set it in if empty
		if app.Phase == "" || app.Phase == config.PhaseApps {
			applications = append(applications, app)
		}
		if app.Phase == config.PhaseSystem {
			systemApplications = append(systemApplications, app)
		}
	}

	err = o.generateHelmFile(applications, err, localHelmRepos, apps, string(config.PhaseApps))
	if err != nil {
		return errors.Wrap(err, "failed to generate apps helmfile")
	}
	err = o.generateHelmFile(systemApplications, err, localHelmRepos, apps, string(config.PhaseSystem))
	if err != nil {
		return errors.Wrap(err, "failed to generate system helmfile")
	}

	return nil
}

func (o *CreateHelmfileOptions) generateHelmFile(applications []config.Application, err error, localHelmRepos map[string]string, apps *config.ApplicationConfig, phase string) error {
	// contains the repo url and name to reference it by in the release spec
	// use a map to dedupe repositories
	repos := make(map[string]string)
	for _, app := range applications {
		_, err = url.ParseRequestURI(app.Repository)
		if err != nil {
			// if the repository isn't a valid URL lets just use whatever was supplied in the application repository field, probably it is a directory path
			repos[app.Repository] = app.Repository
		} else {
			matched := false
			// check if URL matches a repo in helms local list
			for key, value := range localHelmRepos {
				if app.Repository == value {
					repos[app.Repository] = key
					matched = true
				}
			}
			if !matched {
				repos[app.Repository] = uuid.New().String()
			}
		}
	}
	var repositories []helmfile2.RepositorySpec
	var releases []helmfile2.ReleaseSpec
	for repoURL, name := range repos {
		_, err = url.ParseRequestURI(repoURL)
		// skip non URLs as they're probably local directories which don't need to be in the helmfile.repository section
		if err == nil {
			repository := helmfile2.RepositorySpec{
				Name: name,
				URL:  repoURL,
			}
			repositories = append(repositories, repository)
		}
	}
	for _, app := range applications {

		if app.Namespace == "" {
			app.Namespace = apps.DefaultNamespace
		}

		// check if a local directory and values file exists for the app
		extraValuesFiles := o.valueFiles
		extraValuesFiles = o.addExtraAppValues(app, extraValuesFiles, "values.yaml", phase)
		extraValuesFiles = o.addExtraAppValues(app, extraValuesFiles, "values.yaml.gotmpl", phase)

		chartName := fmt.Sprintf("%s/%s", repos[app.Repository], app.Name)
		release := helmfile2.ReleaseSpec{
			Name:      app.Name,
			Namespace: app.Namespace,
			Chart:     chartName,
			Values:    extraValuesFiles,
		}
		releases = append(releases, release)
	}

	// ensure any namespaces referenced are created first, do this via an extra chart that creates namespaces
	// so that helm manages the k8s resources, useful when cleaning up, this is a workaround for a helm 3 limitation
	// which is expected to be fixed
	repositories, releases, err = o.ensureNamespaceExist(repositories, releases, phase)
	if err != nil {
		return errors.Wrapf(err, "failed to check namespaces exists")
	}
	h := helmfile2.HelmState{
		Bases: []string{"../environments.yaml"},
		HelmDefaults: helmfile2.HelmSpec{
			Atomic:  true,
			Verify:  false,
			Wait:    true,
			Timeout: 180,
			// need Force to be false https://github.com/helm/helm/issues/6378
			Force: false,
		},
		Repositories: repositories,
		Releases:     releases,
	}
	data, err := yaml.Marshal(h)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal helmfile data")
	}

	err = o.writeHelmfile(err, phase, data)
	if err != nil {
		return errors.Wrapf(err, "failed to write helmfile")
	}
	return nil
}

func (o *CreateHelmfileOptions) writeHelmfile(err error, phase string, data []byte) error {
	exists, err := util.DirExists(path.Join(o.outputDir, phase))
	if err != nil || !exists {
		err = os.MkdirAll(path.Join(o.outputDir, phase), os.ModePerm)
		if err != nil {
			return errors.Wrapf(err, "cannot create phase directory %s ", path.Join(o.outputDir, phase))
		}
	}
	err = ioutil.WriteFile(path.Join(o.outputDir, phase, helmfile), data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", helmfile)
	}
	return nil
}

func (o *CreateHelmfileOptions) addExtraAppValues(app config.Application, newValuesFiles []string, valuesFilename, phase string) []string {
	fileName := path.Join(o.dir, phase, app.Name, valuesFilename)
	exists, _ := util.FileExists(fileName)
	if exists {
		newValuesFiles = append(newValuesFiles, path.Join(app.Name, valuesFilename))
	}
	return newValuesFiles
}

// this is a temporary function that wont be needed once helm 3 supports creating namespaces
func (o *CreateHelmfileOptions) ensureNamespaceExist(helmfileRepos []helmfile2.RepositorySpec, helmfileReleases []helmfile2.ReleaseSpec, phase string) ([]helmfile2.RepositorySpec, []helmfile2.ReleaseSpec, error) {

	// start by deleting the existing generated directory
	err := os.RemoveAll(path.Join(o.outputDir, phase, "generated"))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "cannot delete generated values directory %s ", path.Join(phase, "generated"))
	}

	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create kube client")
	}

	namespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to list namespaces")
	}

	namespaceMatched := false
	// loop over each application and check if the namespace it references exists, if not add the namespace creator chart to the helmfile
	for k, release := range helmfileReleases {
		for _, ns := range namespaces.Items {
			if ns.Name == release.Namespace {
				namespaceMatched = true
			}
		}
		if !namespaceMatched {
			existingCreateNamespaceChartFound := false
			for _, release := range helmfileReleases {
				if release.Name == "namespace-"+release.Namespace {
					existingCreateNamespaceChartFound = true
				}
			}
			if !existingCreateNamespaceChartFound {

				err := o.writeGeneratedNamespaceValues(release.Namespace, phase)
				if err != nil {
					errors.Wrapf(err, "failed to write generated namespace values file")
				}

				repository := helmfile2.RepositorySpec{
					Name: "zloeber",
					URL:  "git+https://github.com/zloeber/helm-namespace@chart",
				}
				helmfileRepos = append(helmfileRepos, repository)

				createNamespaceChart := helmfile2.ReleaseSpec{
					Name:      "namespace-" + release.Namespace,
					Namespace: currentNamespace,
					Chart:     "zloeber/namespace",

					Values: []string{path.Join("generated", release.Namespace, "values.yaml")},
				}

				// add a dependency so that the create namespace chart is installed before the app chart
				helmfileReleases[k].Needs = []string{fmt.Sprintf("%s/namespace-%s", currentNamespace, release.Namespace)}

				helmfileReleases = append(helmfileReleases, createNamespaceChart)
			}
		}
	}

	return helmfileRepos, helmfileReleases, nil
}

func (o *CreateHelmfileOptions) writeGeneratedNamespaceValues(namespace, phase string) error {
	// workaround with using []interface{} for values, this causes problems with (un)marshalling so lets write a file and
	// add the file path to the []string values
	err := os.MkdirAll(path.Join(o.outputDir, phase, "generated", namespace), os.ModePerm)
	if err != nil {
		return errors.Wrapf(err, "cannot create generated values directory %s ", path.Join(phase, "generated", namespace))
	}
	value := GeneratedValues{
		Namespaces: []string{namespace},
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(path.Join(o.outputDir, phase, "generated", namespace, "values.yaml"), data, util.DefaultWritePermissions)
	return nil
}
