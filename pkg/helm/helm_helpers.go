package helm

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jenkins-x/jx/pkg/vault"

	"github.com/jenkins-x/jx/pkg/version"

	"github.com/jenkins-x/jx/pkg/kube"
	"k8s.io/client-go/kubernetes"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

const (
	// ChartFileName file name for a chart
	ChartFileName = "Chart.yaml"
	// RequirementsFileName the file name for helm requirements
	RequirementsFileName = "requirements.yaml"
	// SecretsFileName the file name for secrets
	SecretsFileName = "secrets.yaml"
	// ValuesFileName the file name for values
	ValuesFileName = "values.yaml"
	// TemplatesDirName is the default name for the templates directory
	TemplatesDirName = "templates"

	// DefaultHelmRepositoryURL is the default cluster local helm repo
	DefaultHelmRepositoryURL = "http://jenkins-x-chartmuseum:8080"

	defaultEnvironmentChartDir = "env"

	//RepoVaultPath is the path to the repo credentials in Vault
	RepoVaultPath = "helm/repos"
)

// copied from helm to minimise dependencies...

// Dependency describes a chart upon which another chart depends.
//
// Dependencies can be used to express developer intent, or to capture the state
// of a chart.
type Dependency struct {
	// Name is the name of the dependency.
	//
	// This must mach the name in the dependency's Chart.yaml.
	Name string `json:"name"`
	// Version is the version (range) of this chart.
	//
	// A lock file will always produce a single version, while a dependency
	// may contain a semantic version range.
	Version string `json:"version,omitempty"`
	// The URL to the repository.
	//
	// Appending `index.yaml` to this string should result in a URL that can be
	// used to fetch the repository index.
	Repository string `json:"repository"`
	// A yaml path that resolves to a boolean, used for enabling/disabling charts (e.g. subchart1.enabled )
	Condition string `json:"condition,omitempty"`
	// Tags can be used to group charts for enabling/disabling together
	Tags []string `json:"tags,omitempty"`
	// Enabled bool determines if chart should be loaded
	Enabled bool `json:"enabled,omitempty"`
	// ImportValues holds the mapping of source values to parent key to be imported. Each item can be a
	// string or pair of child/parent sublist items.
	ImportValues []interface{} `json:"import-values,omitempty"`
	// Alias usable alias to be used for the chart
	Alias string `json:"alias,omitempty"`
}

// ErrNoRequirementsFile to detect error condition
type ErrNoRequirementsFile error

// Requirements is a list of requirements for a chart.
//
// Requirements are charts upon which this chart depends. This expresses
// developer intent.
type Requirements struct {
	Dependencies []*Dependency `json:"dependencies"`
}

// DepSorter Used to avoid merge conflicts by sorting deps by name
type DepSorter []*Dependency

func (a DepSorter) Len() int           { return len(a) }
func (a DepSorter) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a DepSorter) Less(i, j int) bool { return a[i].Name < a[j].Name }

// SetAppVersion sets the version of the app to use
func (r *Requirements) SetAppVersion(app string, version string, repository string, alias string) {
	if r.Dependencies == nil {
		r.Dependencies = []*Dependency{}
	}
	for _, dep := range r.Dependencies {
		if dep != nil && dep.Name == app {
			dep.Version = version
			dep.Repository = repository
			dep.Alias = alias
			return
		}
	}
	r.Dependencies = append(r.Dependencies, &Dependency{
		Name:       app,
		Version:    version,
		Repository: repository,
		Alias:      alias,
	})
	sort.Sort(DepSorter(r.Dependencies))
}

// RemoveApplication removes the given app name. Returns true if a dependency was removed
func (r *Requirements) RemoveApplication(app string) bool {
	for i, dep := range r.Dependencies {
		if dep != nil && dep.Name == app {
			r.Dependencies = append(r.Dependencies[:i], r.Dependencies[i+1:]...)
			sort.Sort(DepSorter(r.Dependencies))
			return true
		}
	}
	return false
}

// FindRequirementsFileName returns the default requirements.yaml file name
func FindRequirementsFileName(dir string) (string, error) {
	return findFileName(dir, RequirementsFileName)
}

// FindChartFileName returns the default chart.yaml file name
func FindChartFileName(dir string) (string, error) {
	return findFileName(dir, ChartFileName)
}

// FindValuesFileName returns the default values.yaml file name
func FindValuesFileName(dir string) (string, error) {
	return findFileName(dir, ValuesFileName)
}

// FindTemplatesDirName returns the default templates/ dir name
func FindTemplatesDirName(dir string) (string, error) {
	return findFileName(dir, TemplatesDirName)
}

func findFileName(dir string, fileName string) (string, error) {
	names := []string{
		filepath.Join(dir, defaultEnvironmentChartDir, fileName),
		filepath.Join(dir, fileName),
	}
	for _, name := range names {
		exists, err := util.FileExists(name)
		if err != nil {
			return "", err
		}
		if exists {
			return name, nil
		}
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.IsDir() {
			name := filepath.Join(dir, f.Name(), fileName)
			exists, err := util.FileExists(name)
			if err != nil {
				return "", err
			}
			if exists {
				return name, nil
			}
		}
	}
	dirs := []string{
		filepath.Join(dir, defaultEnvironmentChartDir),
		dir,
	}
	for _, d := range dirs {
		name := filepath.Join(d, fileName)
		exists, err := util.FileExists(d)
		if err != nil {
			return "", err
		}
		if exists {
			return name, nil
		}
	}
	return "", fmt.Errorf("Could not deduce the default requirements.yaml file name")
}

// LoadRequirementsFile loads the requirements file or creates empty requirements if the file does not exist
func LoadRequirementsFile(fileName string) (*Requirements, error) {
	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, err
	}
	if exists {
		data, err := ioutil.ReadFile(fileName)
		if err != nil {
			return nil, err
		}
		return LoadRequirements(data)
	}
	r := &Requirements{}
	return r, nil
}

// LoadChartFile loads the chart file or creates empty chart if the file does not exist
func LoadChartFile(fileName string) (*chart.Metadata, error) {
	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, err
	}
	if exists {
		data, err := ioutil.ReadFile(fileName)
		if err != nil {
			return nil, err
		}
		return LoadChart(data)
	}
	return &chart.Metadata{}, nil
}

// LoadValuesFile loads the values file or creates empty map if the file does not exist
func LoadValuesFile(fileName string) (map[string]interface{}, error) {
	exists, err := util.FileExists(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "checking %s exists", fileName)
	}
	if exists {
		data, err := ioutil.ReadFile(fileName)
		if err != nil {
			return nil, errors.Wrapf(err, "reading %s", fileName)
		}
		v, err := LoadValues(data)
		if err != nil {
			return nil, errors.Wrapf(err, "unmarshaling %s", fileName)
		}
		return v, nil
	}
	return make(map[string]interface{}), nil
}

// LoadTemplatesDir loads the files in the templates dir or creates empty map if none exist
func LoadTemplatesDir(dirName string) (map[string]string, error) {
	exists, err := util.DirExists(dirName)
	if err != nil {
		return nil, err
	}
	answer := make(map[string]string)
	if exists {
		files, err := ioutil.ReadDir(dirName)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			filename, _ := filepath.Split(f.Name())
			answer[filename] = f.Name()
		}
	}
	return answer, nil
}

// LoadRequirements loads the requirements from some data
func LoadRequirements(data []byte) (*Requirements, error) {
	r := &Requirements{}
	return r, yaml.Unmarshal(data, r)
}

// LoadChart loads the requirements from some data
func LoadChart(data []byte) (*chart.Metadata, error) {
	r := &chart.Metadata{}
	return r, yaml.Unmarshal(data, r)
}

// LoadValues loads the values from some data
func LoadValues(data []byte) (map[string]interface{}, error) {
	r := make(map[string]interface{})

	return r, yaml.Unmarshal(data, &r)
}

// SaveFile saves contents (a pointer to a data structure) to a file
func SaveFile(fileName string, contents interface{}) error {
	data, err := yaml.Marshal(contents)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal helm values file %s", fileName)
	}
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save helm values file %s", fileName)
	}
	return nil
}

func LoadChartName(chartFile string) (string, error) {
	chart, err := chartutil.LoadChartfile(chartFile)
	if err != nil {
		return "", err
	}
	return chart.Name, nil
}

func LoadChartNameAndVersion(chartFile string) (string, string, error) {
	chart, err := chartutil.LoadChartfile(chartFile)
	if err != nil {
		return "", "", err
	}
	return chart.Name, chart.Version, nil
}

// ModifyChart modifies the given chart using a callback
func ModifyChart(chartFile string, fn func(chart *chart.Metadata) error) error {
	chart, err := chartutil.LoadChartfile(chartFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to load chart file %s", chartFile)
	}
	err = fn(chart)
	if err != nil {
		return errors.Wrapf(err, "Failed to modify chart for file %s", chartFile)
	}
	err = chartutil.SaveChartfile(chartFile, chart)
	if err != nil {
		return errors.Wrapf(err, "Failed to save modified chart file %s", chartFile)
	}
	return nil
}

// SetChartVersion modifies the given chart file to update the version
func SetChartVersion(chartFile string, version string) error {
	callback := func(chart *chart.Metadata) error {
		chart.Version = version
		return nil
	}
	return ModifyChart(chartFile, callback)
}

func AppendMyValues(valueFiles []string) ([]string, error) {
	// Overwrite the values with the content of myvalues.yaml files from the current folder if exists, otherwise
	// from ~/.jx folder also only if it's present
	curDir, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the current working directory")
	}
	myValuesFile := filepath.Join(curDir, "myvalues.yaml")
	exists, err := util.FileExists(myValuesFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check if the myvaules.yaml file exists in the current directory")
	}
	if exists {
		valueFiles = append(valueFiles, myValuesFile)
		logrus.Infof("Using local value overrides file %s\n", util.ColorInfo(myValuesFile))
	} else {
		configDir, err := util.ConfigDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to read the config directory")
		}
		myValuesFile = filepath.Join(configDir, "myvalues.yaml")
		exists, err = util.FileExists(myValuesFile)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if the myvaules.yaml file exists in the .jx directory")
		}
		if exists {
			valueFiles = append(valueFiles, myValuesFile)
			logrus.Infof("Using local value overrides file %s\n", util.ColorInfo(myValuesFile))
		}
	}
	return valueFiles, nil
}

// CombineValueFilesToFile iterates through the input files and combines them into a single Values object and then
// write it to the output file nested inside the chartName
func CombineValueFilesToFile(outFile string, inputFiles []string, chartName string, extraValues map[string]interface{}) error {
	answerMap := map[string]interface{}{}

	// lets load any previous values if they exist
	exists, err := util.FileExists(outFile)
	if err != nil {
		return err
	}
	if exists {
		answerMap, err = LoadValuesFile(outFile)
		if err != nil {
			return err
		}
	}

	// now lets merge any given input files
	answer := chartutil.Values{}
	for _, input := range inputFiles {
		values, err := chartutil.ReadValuesFile(input)
		if err != nil {
			return errors.Wrapf(err, "Failed to read helm values YAML file %s", input)
		}
		sourceMap := answer.AsMap()
		util.CombineMapTrees(sourceMap, values.AsMap())
		answer = chartutil.Values(sourceMap)
	}
	m := answer.AsMap()
	for k, v := range extraValues {
		m[k] = v
	}
	answerMap[chartName] = m
	answer = chartutil.Values(answerMap)
	text, err := answer.YAML()
	if err != nil {
		return errors.Wrap(err, "Failed to marshal the combined values YAML files back to YAML")
	}
	err = ioutil.WriteFile(outFile, []byte(text), util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "Failed to save combined helm values YAML file %s", outFile)
	}
	return nil
}

// GetLatestVersion get's the latest version of a chart in a repo using helmer
func GetLatestVersion(chart string, repo string, username string, password string, helmer Helmer) (string, error) {
	latest := ""
	version := ""
	err := InspectChart(chart, version, repo, username, password, helmer, func(dir string) error {
		var err error
		_, latest, err = LoadChartNameAndVersion(filepath.Join(dir, "Chart.yaml"))
		return err
	})
	return latest, err
}

// InspectChart fetches the specified chart in a repo using helmer, and then calls the closure on it, before cleaning up
func InspectChart(chart string, version string, repo string, username string, password string,
	helmer Helmer, inspector func(dir string) error) error {
	isLocal := false
	dirPrefix := fmt.Sprintf("jx-helm-fetch-%s-", chart)
	if strings.HasPrefix(chart, "/") || strings.HasPrefix(chart, ".") || strings.Count(chart, "/") > 1 {
		isLocal = true
		dirPrefix = "jx-helm-fetch"
	}

	dir, err := ioutil.TempDir("", dirPrefix)
	defer func() {
		err1 := os.RemoveAll(dir)
		if err1 != nil {
			logrus.Warnf("Error removing %s %v\n", dir, err1)
		}
	}()
	inspectPath := filepath.Join(dir, chart)
	if isLocal {
		// This is a local path
		err := util.CopyDir(chart, dir, true)
		if err != nil {
			return errors.Wrapf(err, "copying %s to %s", chart, dir)
		}
		// We need to manually build the dependencies
		err = helmer.BuildDependency()
		if err != nil {
			return errors.Wrapf(err, "building dependencies for %s", chart)
		}
		inspectPath = dir
	} else {
		err = helmer.FetchChart(chart, version, true, dir, repo, username, password)
		if err != nil {
			return err
		}
	}

	return inspector(inspectPath)
}

type InstallChartOptions struct {
	Dir         string
	ReleaseName string
	Chart       string
	Version     string
	Ns          string
	HelmUpdate  bool
	SetValues   []string
	ValueFiles  []string
	Repository  string
	Username    string
	Password    string
	VersionsDir string
	InstallOnly bool
	NoForce     bool
	Wait        bool
	UpgradeOnly bool
}

// InstallFromChartOptions uses the helmer and kubeClient interfaces to install the chart from the options,
// respecting the installTimeout, looking up or updating Vault with the username and password for the repo.
// If vaultClient is nil then username and passwords for repos will not be looked up in Vault.
func InstallFromChartOptions(options InstallChartOptions, helmer Helmer, kubeClient kubernetes.Interface,
	installTimeout string, vaultClient vault.Client) error {
	chart := options.Chart
	if options.Version == "" {
		versionsDir := options.VersionsDir
		if versionsDir == "" {
			return errors.Errorf("no VersionsDir specified when trying to install a chart")
		}
		var err error
		options.Version, err = version.LoadStableVersionNumber(versionsDir, version.KindChart, chart)
		if err != nil {
			return errors.Wrapf(err, "failed to load stable version in dir %s for chart %s", versionsDir, chart)
		}
	}
	if options.HelmUpdate {
		logrus.Infoln("Updating Helm repository...")
		err := helmer.UpdateRepo()
		if err != nil {
			return errors.Wrap(err, "failed to update repository")
		}
		logrus.Infoln("Helm repository update done.")
	}
	err := DecorateWithCredentials(&options, vaultClient)
	if err != nil {
		return errors.WithStack(err)
	}
	cleanup, err := DecorateWithSecrets(&options, vaultClient)
	defer cleanup()
	if err != nil {
		return errors.WithStack(err)
	}
	if options.Ns != "" {
		annotations := map[string]string{"jenkins-x.io/created-by": "Jenkins X"}
		kube.EnsureNamespaceCreated(kubeClient, options.Ns, nil, annotations)
	}
	timeout, err := strconv.Atoi(installTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to convert the timeout to an int")
	}
	helmer.SetCWD(options.Dir)
	if options.InstallOnly {
		return helmer.InstallChart(chart, options.ReleaseName, options.Ns, options.Version, timeout,
			options.SetValues, options.ValueFiles, options.Repository, options.Username, options.Password)
	}
	return helmer.UpgradeChart(chart, options.ReleaseName, options.Ns, options.Version, !options.UpgradeOnly, timeout,
		!options.NoForce, options.Wait, options.SetValues, options.ValueFiles, options.Repository,
		options.Username, options.Password)
}

// HelmRepoCredentials is a map of repositories to HelmRepoCredential that stores all the helm repo credentials for
// the cluster
type HelmRepoCredentials map[string]HelmRepoCredential

// HelmRepoCredential is a username and password pair that can ben used to authenticated against a Helm repo
type HelmRepoCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DecorateWithSecrets will replace any vault: URIs with the secret from vault. Safe to call with a nil client (
// no replacement will take place).
func DecorateWithSecrets(options *InstallChartOptions, vaultClient vault.Client) (func(), error) {
	cleanup := func() {
	}
	if vaultClient != nil {
		newValuesFiles := make([]string, 0)
		cleanup = func() {
			for _, f := range newValuesFiles {
				err := util.DeleteFile(f)
				if err != nil {
					log.Errorf("Deleting temp file %s\n", f)
				}
			}
		}
		for _, valueFile := range options.ValueFiles {
			newValuesFile, err := ioutil.TempFile("", "values.yaml")
			if err != nil {
				return cleanup, errors.Wrapf(err, "creating temp file for %s", valueFile)
			}
			bytes, err := ioutil.ReadFile(valueFile)
			if err != nil {
				return cleanup, errors.Wrapf(err, "reading file %s", valueFile)
			}
			newValues, err := vault.ReplaceURIs(string(bytes), vaultClient)
			if err != nil {
				return cleanup, errors.Wrapf(err, "replacing vault URIs")
			}
			err = ioutil.WriteFile(newValuesFile.Name(), []byte(newValues), 0600)
			if err != nil {
				return cleanup, errors.Wrapf(err, "writing new values file %s", newValuesFile.Name())
			}
			newValuesFiles = append(newValuesFiles, newValuesFile.Name())
		}
		options.ValueFiles = newValuesFiles
	}
	return cleanup, nil
}

// DecorateWithCredentials will, if vault is installed, attach a username and password to the options
func DecorateWithCredentials(options *InstallChartOptions, vaultClient vault.Client) error {
	if options.Repository != "" && vaultClient != nil {
		vaultPath := fmt.Sprintf("%s", RepoVaultPath)
		creds := HelmRepoCredentials{}
		if err := vaultClient.ReadObject(vaultPath, &creds); err != nil {
			return errors.Wrapf(err, "reading repo credentials from vault %s", vaultPath)
		}
		write := false
		cred := HelmRepoCredential{}
		if options.Username != "" {
			// If a username is passed in then we should update vault
			write = true
			cred.Username = options.Username
		} else if c, ok := creds[options.Repository]; ok {
			// Otherwise check if vault has a username
			options.Username = c.Username
		}

		if options.Password != "" {
			// If a password is passed in then we should update vault
			write = true
			cred.Password = options.Password
		} else if c, ok := creds[options.Repository]; ok {
			// Otherwise check if vault has a password
			options.Password = c.Password
		}

		if write {
			creds[options.Repository] = cred
			_, err := vaultClient.WriteObject(vaultPath, creds)
			if err != nil {
				return errors.Wrapf(err, "updating repo credentials in vault %s", vaultPath)
			}
		}
	}
	return nil
}

// GenerateReadmeForChart generates a string that can be used as a README.MD,
// and includes info on the chart.
func GenerateReadmeForChart(name string, version string, description string, chartRepo string,
	gitRepo string, releaseNotesURL string, appReadme string) string {
	var readme strings.Builder
	readme.WriteString(fmt.Sprintf("# %s\n\n|App Metadata||\n", unknownZeroValue(name)))
	readme.WriteString("|---|---|\n")
	if version != "" {
		readme.WriteString(fmt.Sprintf("| **Version** | %s |\n", version))
	}
	if description != "" {
		readme.WriteString(fmt.Sprintf("| **Description** | %s |\n", description))
	}
	if chartRepo != "" {
		readme.WriteString(fmt.Sprintf("| **Chart Repository** | %s |\n", chartRepo))
	}
	if gitRepo != "" {
		readme.WriteString(fmt.Sprintf("| **Git Repository** | %s |\n", gitRepo))
	}
	if releaseNotesURL != "" {
		readme.WriteString(fmt.Sprintf("| **Release Notes** | %s |\n", releaseNotesURL))
	}

	if appReadme != "" {
		readme.WriteString(fmt.Sprintf("\n## App README.MD\n\n%s\n", appReadme))
	}
	return readme.String()
}

func unknownZeroValue(value string) string {
	if value == "" {
		return "unknown"
	}
	return value

}

// SetValuesToMap converts the set of values of the form "foo.bar=123" into a helm values.yaml map structure
func SetValuesToMap(setValues []string) map[string]interface{} {
	answer := map[string]interface{}{}
	for _, setValue := range setValues {
		tokens := strings.SplitN(setValue, "=", 2)
		if len(tokens) > 1 {
			path := tokens[0]
			value := tokens[1]

			// lets assume false is a boolean
			if value == "false" {
				util.SetMapValueViaPath(answer, path, false)

			} else {
				util.SetMapValueViaPath(answer, path, value)
			}
		}
	}
	return answer
}
