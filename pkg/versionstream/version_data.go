package versionstream

import (
	"fmt"
	"sort"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Callback a callback function for processing version information. Return true to continue processing
// or false to terminate the loop
type Callback func(kind VersionKind, name string, version *StableVersion) (bool, error)

// VersionKind represents the kind of version
type VersionKind string

const (
	// KindChart represents a chart version
	KindChart VersionKind = "charts"

	// KindPackage represents a package version
	KindPackage VersionKind = "packages"

	// KindDocker represents a docker resolveImage version
	KindDocker VersionKind = "docker"

	// KindGit represents a git repository (e.g. for jx boot configuration or a build pack)
	KindGit VersionKind = "git"
)

var (
	// Kinds all the version kinds
	Kinds = []VersionKind{
		KindChart,
		KindPackage,
		KindDocker,
		KindGit,
	}

	// KindStrings all the kinds as strings for validating CLI arguments
	KindStrings = []string{
		string(KindChart),
		string(KindPackage),
		string(KindDocker),
		string(KindGit),
	}
)

// StableVersion stores the stable version information
type StableVersion struct {
	// Version the default version to use
	Version string `json:"version,omitempty"`
	// VersionUpperLimit represents the upper limit which indicates a version which is too new.

	// e.g. for packages we could use: `{ version: "1.10.1", upperLimit: "1.14.0"}` which would mean these
	// versions are all valid `["1.11.5", "1.13.1234"]` but these are invalid `["1.14.0", "1.14.1"]`
	UpperLimit string `json:"upperLimit,omitempty"`
	// GitURL the URL to the source code
	GitURL string `json:"gitUrl,omitempty"`
	// Component is the component inside the git URL
	Component string `json:"component,omitempty"`
	// URL the URL for the documentation
	URL string `json:"url,omitempty"`
}

// VerifyPackage verifies the current version of the package is valid
func (data *StableVersion) VerifyPackage(name string, currentVersion string, workDir string) error {
	currentVersion = convertToVersion(currentVersion)
	if currentVersion == "" {
		return nil
	}
	version := convertToVersion(data.Version)
	if version == "" {
		log.Logger().Warnf("could not find a stable package version for %s from %s\nFor background see: https://jenkins-x.io/docs/concepts/version-stream/", name, workDir)
		log.Logger().Infof("Please lock this version down via the command: %s", util.ColorInfo(fmt.Sprintf("jx step create pr versions -k package -n %s", name)))
		return nil
	}

	currentSem, err := semver.Make(currentVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to parse semantic version for current version %s for package %s", currentVersion, name)
	}

	minSem, err := semver.Make(version)
	if err != nil {
		return errors.Wrapf(err, "failed to parse required semantic version %s for package %s", version, name)
	}

	upperLimitText := convertToVersion(data.UpperLimit)
	if upperLimitText == "" {
		if minSem.Equals(currentSem) {
			return nil
		}
		return verifyError(name, fmt.Errorf("package %s is on version %s but the version stream requires version %s", name, currentVersion, version))
	}

	// lets make sure the current version is in the range
	if currentSem.LT(minSem) {
		return verifyError(name, fmt.Errorf("package %s is an old version %s. The version stream requires at least %s", name, currentVersion, version))
	}

	limitSem, err := semver.Make(upperLimitText)
	if err != nil {
		return errors.Wrapf(err, "failed to parse upper limit version %s for package %s", upperLimitText, name)
	}

	if currentSem.GE(limitSem) {
		return verifyError(name, fmt.Errorf("package %s is using version %s which is too new. The version stream requires a version earlier than %s", name, currentVersion, upperLimitText))
	}
	return nil
}

// verifyError allows package verify errors to be disabled in development via environment variables
func verifyError(name string, err error) error {
	envVar := "JX_DISABLE_VERIFY_" + strings.ToUpper(name)
	value := os.Getenv(envVar)
	if strings.ToLower(value) == "true" {
		log.Logger().Warnf("$%s is true so disabling verify of %s: %s\n", envVar, name, err.Error())
		return nil
	}
	return err
}

// convertToVersion extracts a semantic version from the specified string.
// If no semantic version is contained in the specified string the string is returned unmodified.
func convertToVersion(text string) string {
	// Some apps might not exactly follow semver, like for example Git for Windows: 2.23.0.windows.1
	// we're trimming everything after a semver from the answer
	// to avoid error described in issue #6825
	r := regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`)
	if !r.Match([]byte(text)) {
		return text
	}
	return r.FindString(text)
}

// LoadStableVersion loads the stable version data from the version configuration directory returning an empty object if there is
// no specific stable version configuration available
func LoadStableVersion(wrkDir string, kind VersionKind, name string) (*StableVersion, error) {
	if kind == KindGit {
		name = GitURLToName(name)
	}
	path := filepath.Join(wrkDir, string(kind), name+".yml")
	return LoadStableVersionFile(path)
}

// GitURLToName lets trim any URL scheme and trailing .git or / from a git URL
func GitURLToName(name string) string {
	// lets trim the URL scheme
	idx := strings.Index(name, "://")
	if idx > 0 {
		name = name[idx+3:]
	}
	name = strings.TrimSuffix(name, ".git")
	name = strings.TrimSuffix(name, "/")
	return name
}

// LoadStableVersionFile loads the stable version data from the given file name
func LoadStableVersionFile(path string) (*StableVersion, error) {
	version := &StableVersion{}
	exists, err := util.FileExists(path)
	if err != nil {
		return version, errors.Wrapf(err, "failed to check if file exists %s", path)
	}
	if !exists {
		return version, nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return version, errors.Wrapf(err, "failed to load YAML file %s", path)
	}
	version, err = LoadStableVersionFromData(data)
	if err != nil {
		return version, errors.Wrapf(err, "failed to unmarshal YAML for file %s", path)
	}
	return version, err
}

// LoadStableVersionFromData loads the stable version data from the given the data
func LoadStableVersionFromData(data []byte) (*StableVersion, error) {
	version := &StableVersion{}
	err := yaml.Unmarshal(data, version)
	if err != nil {
		return version, errors.Wrapf(err, "failed to unmarshal YAML")
	}
	return version, err
}

// LoadStableVersionNumber loads just the stable version number for the given kind and name
func LoadStableVersionNumber(wrkDir string, kind VersionKind, name string) (string, error) {
	data, err := LoadStableVersion(wrkDir, kind, name)
	if err != nil {
		return "", err
	}
	version := data.Version
	if version != "" {
		log.Logger().Debugf("using stable version %s from %s of %s from %s", util.ColorInfo(version), string(kind), util.ColorInfo(name), wrkDir)
	} else {
		// lets not warn if building current dir chart
		if kind == KindChart && name == "." {
			return version, err
		}
		log.Logger().Warnf("could not find a stable version from %s of %s from %s\nFor background see: https://jenkins-x.io/docs/concepts/version-stream/", string(kind), name, wrkDir)
		log.Logger().Infof("Please lock this version down via the command: %s", util.ColorInfo(fmt.Sprintf("jx step create pr versions -k %s -n %s", string(kind), name)))
	}
	return version, err
}

// SaveStableVersion saves the version file
func SaveStableVersion(wrkDir string, kind VersionKind, name string, stableVersion *StableVersion) error {
	path := filepath.Join(wrkDir, string(kind), name+".yml")
	return SaveStableVersionFile(path, stableVersion)
}

// SaveStableVersionFile saves the stabe version to the given file name
func SaveStableVersionFile(path string, stableVersion *StableVersion) error {
	data, err := yaml.Marshal(stableVersion)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal data to YAML %#v", stableVersion)
	}
	dir, _ := filepath.Split(path)
	err = os.MkdirAll(dir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dir)
	}

	err = ioutil.WriteFile(path, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", path)
	}
	return nil
}

// ResolveDockerImage resolves the version of the specified image against the version stream defined in versionsDir.
// If there is a version defined for the image in the version stream 'image:<version>' is returned, otherwise the
// passed image name is returned as is.
func ResolveDockerImage(versionsDir, image string) (string, error) {
	// lets check if we already have a version
	path := strings.SplitN(image, ":", 2)
	if len(path) == 2 && path[1] != "" {
		return image, nil
	}
	info, err := LoadStableVersion(versionsDir, KindDocker, image)
	if err != nil {
		return image, err
	}
	if info.Version == "" {
		// lets check if there is a docker.io prefix and if so lets try fetch without the docker prefix
		prefix := "docker.io/"
		if strings.HasPrefix(image, prefix) {
			image = strings.TrimPrefix(image, prefix)
			info, err = LoadStableVersion(versionsDir, KindDocker, image)
			if err != nil {
				return image, err
			}
		}
	}
	if info.Version == "" {
		log.Logger().Warnf("could not find a stable version for Docker image: %s in %s", image, versionsDir)
		log.Logger().Warn("for background see: https://jenkins-x.io/docs/concepts/version-stream/")
		log.Logger().Infof("please lock this version down via the command: %s", util.ColorInfo(fmt.Sprintf("jx step create pr versions -k docker -n %s -v 1.2.3", image)))
		return image, nil
	}
	prefix := strings.TrimSuffix(strings.TrimSpace(image), ":")
	return prefix + ":" + info.Version, nil
}

// UpdateStableVersionFiles applies an update to the stable version files matched by globPattern, updating to version
func UpdateStableVersionFiles(globPattern string, version string, excludeFiles ...string) ([]string, error) {
	files, err := filepath.Glob(globPattern)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create glob from pattern %s", globPattern)
	}
	answer := make([]string, 0)

	for _, path := range files {
		_, name := filepath.Split(path)
		if util.StringArrayIndex(excludeFiles, name) >= 0 {
			continue
		}
		data, err := LoadStableVersionFile(path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load oldVersion info for %s", path)
		}
		if data.Version == "" || data.Version == version {
			continue
		}
		answer = append(answer, data.Version)
		data.Version = version
		err = SaveStableVersionFile(path, data)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to save oldVersion info for %s", path)
		}
	}
	return answer, nil
}

// UpdateStableVersion applies an update to the stable version file in dir/kindStr/name.yml, updating to version
func UpdateStableVersion(dir string, kindStr string, name string, version string) ([]string, error) {
	answer := make([]string, 0)
	kind := VersionKind(kindStr)
	data, err := LoadStableVersion(dir, kind, name)
	if err != nil {
		return nil, err
	}
	if data.Version == version {
		return nil, nil
	}
	answer = append(answer, data.Version)
	data.Version = version

	err = SaveStableVersion(dir, kind, name, data)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to save versionstream file")
	}
	return answer, nil
}

// GetRepositoryPrefixes loads the repository prefixes for the version stream
func GetRepositoryPrefixes(dir string) (*RepositoryPrefixes, error) {
	answer := &RepositoryPrefixes{}
	fileName := filepath.Join(dir, "charts", "repositories.yml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to find file %s", fileName)
	}
	if !exists {
		return answer, nil
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to load file %s", fileName)
	}
	err = yaml.Unmarshal(data, answer)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to unmarshal YAML in file %s", fileName)
	}
	return answer, nil
}

// GetQuickStarts loads the quickstarts from the version stream
func GetQuickStarts(dir string) (*QuickStarts, error) {
	answer := &QuickStarts{}
	fileName := filepath.Join(dir, "quickstarts.yml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to find file %s", fileName)
	}
	if !exists {
		return answer, nil
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to load file %s", fileName)
	}
	err = yaml.Unmarshal(data, &answer)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to unmarshal YAML in file %s", fileName)
	}
	return answer, nil
}

// SaveQuickStarts saves the modified quickstarts in the version stream dir
func SaveQuickStarts(dir string, qs *QuickStarts) error {
	data, err := yaml.Marshal(qs)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal quickstarts to YAML")
	}
	fileName := filepath.Join(dir, "quickstarts.yml")
	err = ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save file %s", fileName)
	}
	return nil
}

// RepositoryPrefixes maps repository prefixes to URLs
type RepositoryPrefixes struct {
	Repositories []RepositoryURLs    `json:"repositories"`
	urlToPrefix  map[string]string   `json:"-"`
	prefixToURLs map[string][]string `json:"-"`
}

// RepositoryURLs contains the prefix and URLS for a repository
type RepositoryURLs struct {
	Prefix string   `json:"prefix"`
	URLs   []string `json:"urls"`
}

// QuickStart the configuration of a quickstart in the version stream
type QuickStart struct {
	ID             string   `json:"id,omitempty"`
	Owner          string   `json:"owner,omitempty"`
	Name           string   `json:"name,omitempty"`
	Version        string   `json:"version,omitempty"`
	Language       string   `json:"language,omitempty"`
	Framework      string   `json:"framework,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	DownloadZipURL string   `json:"downloadZipURL,omitempty"`
}

// QuickStarts the configuration of a the quickstarts in the version stream
type QuickStarts struct {
	QuickStarts  []*QuickStart `json:"quickstarts"`
	DefaultOwner string        `json:"defaultOwner"`
}

// DefaultMissingValues defaults any missing values such as ID which is a combination of owner and name
func (qs *QuickStarts) DefaultMissingValues() {
	for _, q := range qs.QuickStarts {
		q.defaultMissingValues(qs)
	}
}

// Sort sorts the quickstarts into name order
func (qs *QuickStarts) Sort() {
	sort.Sort(quickStartOrder(qs.QuickStarts))
}

type quickStartOrder []*QuickStart

// Len returns the length of the order
func (a quickStartOrder) Len() int { return len(a) }

// Swap swaps 2 items in the slice
func (a quickStartOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// Less returns trtue if an itetm is less than the order
func (a quickStartOrder) Less(i, j int) bool {
	r1 := a[i]
	r2 := a[j]

	n1 := r1.Name
	n2 := r2.Name
	if n1 != n2 {
		return n1 < n2
	}
	o1 := r1.Owner
	o2 := r2.Owner
	return o1 < o2
}

func (q *QuickStart) defaultMissingValues(qs *QuickStarts) {
	if qs.DefaultOwner == "" {
		qs.DefaultOwner = "jenkins-x-quickstarts"
	}
	if q.Owner == "" {
		q.Owner = qs.DefaultOwner
	}
	if q.ID == "" {
		q.ID = fmt.Sprintf("%s/%s", q.Owner, q.Name)
	}
	if q.DownloadZipURL == "" {
		q.DownloadZipURL = fmt.Sprintf("https://codeload.github.com/%s/%s/zip/master", q.Owner, q.Name)
	}
}

// PrefixForURL returns the repository prefix for the given URL
func (p *RepositoryPrefixes) PrefixForURL(u string) string {
	if p.urlToPrefix == nil {
		p.urlToPrefix = map[string]string{}

		for _, repo := range p.Repositories {
			for _, url := range repo.URLs {
				p.urlToPrefix[url] = repo.Prefix
			}
		}
	}
	return p.urlToPrefix[u]
}

// URLsForPrefix returns the repository URLs for the given prefix
func (p *RepositoryPrefixes) URLsForPrefix(prefix string) []string {
	if p.prefixToURLs == nil {
		p.prefixToURLs = make(map[string][]string)
		for _, repo := range p.Repositories {
			p.prefixToURLs[repo.Prefix] = repo.URLs
		}
	}
	return p.prefixToURLs[prefix]
}

// NameFromPath converts a path into a name for use with stable versions
func NameFromPath(basepath string, path string) (string, error) {
	name, err := filepath.Rel(basepath, path)
	if err != nil {
		return "", errors.Wrapf(err, "failed to extract base path from %s", path)
	}
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	return name, nil
}
