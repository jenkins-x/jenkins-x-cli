package dependencymatrix

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/log"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/pkg/errors"
)

const (
	// DependencyMatrixDirName is the name of the directory in which the dependency matrix is stored in git
	DependencyMatrixDirName = "dependency-matrix"
	// DependencyMatrixYamlFileName is the name of file in which the dependency matrix will be stored
	DependencyMatrixYamlFileName = "matrix.yaml"
	// DependencyMatrixAssetName is the name of the asset when the dependency matrix is added to the release on the git provider
	DependencyMatrixAssetName = "dependency-matrix.yaml"
)

// DependencyMatrix is the dependency matrix for a git repo
type DependencyMatrix struct {
	Dependencies []*Dependency `json:"dependencies"`
}

// Dependency represents a dependency in the DependencyMatrix
type Dependency struct {
	DependencyDetails `json:",inline"`
	Sources           []DependencySource `json:"sources,omitempty"`
}

// DependencySource is describes the source of a dependency, including the Path to it and the Version of the dependency
type DependencySource struct {
	Path       DependencyPath `json:"path"`
	Version    string         `json:"version"`
	VersionURL string         `json:"versionURL"`
}

type DependencyPath []*DependencyDetails

// PathEquals returns true if a dependency path is equal to a dependency update path
func (d DependencyPath) PathEquals(o v1.DependencyUpdatePath) bool {
	if len(d) != len(o) {
		return false
	}
	for i, j := range d {
		if !j.KeyEquals(o[i]) {
			return false
		}
	}
	return true
}

func (d DependencyPath) String() string {
	answer := make([]string, 0)
	for _, e := range d {
		answer = append(answer, fmt.Sprintf("%s/%s/%s:%s", e.Host, e.Owner, e.Repo, e.Component))
	}
	return strings.Join(answer, ";")
}

func (d DependencyDetails) String() string {
	componentStr := ""
	if d.Component != "" {
		componentStr = fmt.Sprintf(":%s", d.Component)
	}
	return fmt.Sprintf("%s/%s/%s%s", d.Host, d.Owner, d.Repo, componentStr)
}

// Markdown returns the dependency path as markdown
func (d DependencyPath) Markdown() string {
	answer := make([]string, 0)
	for _, e := range d {
		componentStr := ""
		if e.Component != "" {
			componentStr = fmt.Sprintf(":%s", e.Component)
		}
		answer = append(answer, fmt.Sprintf("[%s/%s/%s%s](%s)", e.Host, e.Owner, e.Repo, componentStr, e.URL))
	}
	return strings.Join(answer, ";")
}

func asDependencyPath(path v1.DependencyUpdatePath) DependencyPath {
	answer := DependencyPath{}
	for _, e := range path {
		answer = append(answer, &DependencyDetails{
			Host:       e.Host,
			Component:  e.Component,
			Repo:       e.Repo,
			Owner:      e.Owner,
			Version:    e.ToVersion,
			URL:        e.URL,
			VersionURL: e.ToReleaseHTMLURL,
		})
	}
	return answer
}

// DependencyDetails are the details of a dependency
type DependencyDetails struct {
	Host       string `json:"host"'`
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	Component  string `json:"component,omitempty"`
	URL        string `json:"url"`
	Version    string `json:"version"`
	VersionURL string `json:"versionURL"`
}

// KeyEquals validates that the key (Host, Owner, Repo and Component) are all the same
func (d *DependencyDetails) KeyEquals(o v1.DependencyUpdateDetails) bool {
	return d.Host == o.Host && d.Owner == o.Owner && d.Repo == o.Repo && d.Component == o.Component
}

// LoadDependencyMatrix loads the dependency matrix file from the given project directory
func LoadDependencyMatrix(dir string) (*DependencyMatrix, error) {
	dependencyMatrix := &DependencyMatrix{}
	path := filepath.Join(dir, DependencyMatrixDirName, "matrix.yaml")
	exists, err := util.FileExists(path)
	if err != nil {
		return dependencyMatrix, errors.Wrapf(err, "checking %s exists", path)
	}
	if exists {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return dependencyMatrix, errors.Wrapf(err, "reading %s", path)
		}
		err = yaml.Unmarshal(data, &dependencyMatrix)
		if err != nil {
			return dependencyMatrix, errors.Wrapf(err, "unmarshaling %s", path)
		}
	}
	return dependencyMatrix, nil
}

// UpdateDependencyMatrix updates the dependency matrix in dir/dependency-matrix using update
func UpdateDependencyMatrix(dir string, update *v1.DependencyUpdate) error {
	// Only create/update the dependency matrix if the directory already exists
	dependencyMatrixDirPath := filepath.Join(dir, DependencyMatrixDirName)
	if info, err := os.Stat(dependencyMatrixDirPath); err == nil && !info.IsDir() {
		log.Logger().Warnf("%s is not a directory, please remove it in order to generate a depedendency matrix", dependencyMatrixDirPath)
		return nil
	} else if os.IsNotExist(err) {
		err := os.MkdirAll(dependencyMatrixDirPath, 0700)
		if err != nil {
			return errors.Wrapf(err, "making directory %s", dependencyMatrixDirPath)
		}
	}
	var data []byte
	path := filepath.Join(dir, DependencyMatrixDirName, "matrix.yaml")
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return errors.Errorf("%s is a directory", path)
	} else if err == nil {
		data, err = ioutil.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "reading %s", path)
		}
	}
	var dependencyMatrix DependencyMatrix
	err = yaml.Unmarshal(data, &dependencyMatrix)
	if err != nil {
		return errors.Wrapf(err, "unmarshaling %s", path)
	}
	found := false
	for _, d := range dependencyMatrix.Dependencies {
		if d.KeyEquals(update.DependencyUpdateDetails) {
			d.Version = update.ToVersion
			found = true
			d.URL = update.URL
			d.VersionURL = update.ToReleaseHTMLURL
			for _, p := range update.Paths {
				pathFound := false
				for i, q := range d.Sources {
					if q.Path.PathEquals(p) {
						d.Sources[i].Path = asDependencyPath(p)
						d.Sources[i].Version = update.ToVersion
						d.Sources[i].VersionURL = update.ToReleaseHTMLURL
						pathFound = true
						break
					}
				}
				if !pathFound {
					d.Sources = append(d.Sources, DependencySource{
						Path:       asDependencyPath(p),
						Version:    update.ToVersion,
						VersionURL: update.ToReleaseHTMLURL,
					})
				}
			}
		}
	}
	if !found {
		paths := make([]DependencySource, 0)
		for _, p := range update.Paths {
			paths = append(paths, DependencySource{
				Path:       asDependencyPath(p),
				Version:    update.ToVersion,
				VersionURL: update.ToReleaseHTMLURL,
			})
		}
		dependencyMatrix.Dependencies = append(dependencyMatrix.Dependencies, &Dependency{
			DependencyDetails: DependencyDetails{
				Owner:      update.Owner,
				Repo:       update.Repo,
				Component:  update.Component,
				Version:    update.ToVersion,
				URL:        update.URL,
				VersionURL: update.ToReleaseHTMLURL,
				Host:       update.Host,
			},
			Sources: paths,
		})
	}
	for _, d := range dependencyMatrix.Dependencies {
		sort.Slice(d.Sources, func(i, j int) bool {
			return d.Sources[i].Path.String() < d.Sources[j].Path.String()
		})
	}
	data, err = yaml.Marshal(dependencyMatrix)
	if err != nil {
		return errors.Wrapf(err, "marshaling %s", path)
	}
	err = ioutil.WriteFile(path, data, 0600)
	if err != nil {
		return errors.Wrapf(err, "writing %s", path)
	}
	err = GenerateMarkdownDependencyMatrix(filepath.Join(dir, DependencyMatrixDirName, "matrix.md"), dependencyMatrix)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

//GenerateMarkdownDependencyMatrix will generate a markdown version of the dependency matrix at path
func GenerateMarkdownDependencyMatrix(path string, matrix DependencyMatrix) error {
	var md bytes.Buffer
	md.WriteString("# Dependency Matrix\n")
	md.WriteString("\n")
	md.WriteString("Dependency | Sources | Version | Mismatched versions\n")
	md.WriteString("---------- | ------- | ------- | -------------------\n")
	for _, d := range matrix.Dependencies {
		sources := make([]string, 0)
		mismatchedVersionsMap := make(map[string][]string, 0)
		for _, source := range d.Sources {
			if source.Version != d.Version {
				if _, ok := mismatchedVersionsMap[source.Version]; !ok {
					mismatchedVersionsMap[source.Version] = make([]string, 0)
				}
				mismatchedVersionsMap[source.Version] = append(mismatchedVersionsMap[source.Version], source.Path.Markdown())
			}
			sources = append(sources, source.Path.Markdown())
		}
		mismatchedVersions := make([]string, 0)
		for k, v := range mismatchedVersionsMap {
			mismatchedVersions = append(mismatchedVersions, fmt.Sprintf("**%s**: %s", k, strings.Join(v, ";")))
		}
		sort.Strings(mismatchedVersions)
		componentStr := ""
		if d.Component != "" {
			componentStr = fmt.Sprintf(":%s", d.Component)
		}
		md.WriteString(fmt.Sprintf("[%s/%s](%s)%s | %s | [%s](%s) | %s\n", d.Owner, d.Repo, d.URL, componentStr, strings.Join(sources, ";"), d.Version, d.VersionURL, strings.Join(mismatchedVersions, "<br>")))
	}
	err := ioutil.WriteFile(path, md.Bytes(), 0600)
	if err != nil {
		return errors.Wrapf(err, "writing %s", path)
	}
	return nil
}

// FindVersionForDependency searches the matrix for a dependency matching the given host, owner, and repo, and if found,
// returns its version
func (d *DependencyMatrix) FindVersionForDependency(host, owner, repo string) (string, error) {
	for _, dep := range d.Dependencies {
		if dep.Host == host && dep.Owner == owner && dep.Repo == repo {
			return dep.Version, nil
		}
	}
	return "", fmt.Errorf("could not find a dependency on host %s, owner %s, repo %s in the dependency matrix", host, owner, repo)
}
