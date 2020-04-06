package dependencymatrix

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
)

//VerifyDependencyMatrixHasConsistentVersions loads a dependency matrix from dir and verifies that there are no inconsistent versions in it
func VerifyDependencyMatrixHasConsistentVersions(dir string) error {
	path := filepath.Join(dir, DependencyMatrixDirName, DependencyMatrixYamlFileName)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Logger().Warnf("Unable to verify %s as it does not exist", path)
		return nil
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "reading %s", path)
	}
	matrix := DependencyMatrix{}
	err = yaml.Unmarshal(data, &matrix)
	if err != nil {
		return errors.Wrapf(err, "unmarshaling %s", path)
	}
	for _, d := range matrix.Dependencies {
		seen := make(map[string][]DependencySource)
		for _, s := range d.Sources {
			if _, ok := seen[s.Version]; !ok {
				seen[s.Version] = make([]DependencySource, 0)
			}
			seen[s.Version] = append(seen[s.Version], s)
		}
		if len(seen) == 0 {
			// This case is ok, there is no source
			continue
		} else if len(seen) == 1 {
			if _, ok := seen[d.Version]; ok {
				// This case is ok, there was only one version seen in the sources, and it was the same as the update version
				continue
			}
		}
		// Otherwise there is a problem
		return errors.Errorf("more than one version found for %s; dependency version is %s but found these paths %+v", d.String(), d.Version, d.Sources)
	}
	return nil
}
