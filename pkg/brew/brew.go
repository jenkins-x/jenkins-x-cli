package brew

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/pkg/errors"
)

var (
	brewFilePattern = "**/*.rb"
	versionRegex    = regexp.MustCompile(`(?m)^\s*version \"(.*)\"$`)
	shaRegex        = regexp.MustCompile(`(?m)^\s*sha256 \"(.*)\"$`)
)

//UpdateVersion scans the directory structure rooted in dir for files that match brewNameRegex and replaces any
// version and sha with their new values
func UpdateVersionAndSha(dir string, newVersion string, newSha string) ([]string, []string, error) {
	oldVersions := make(map[string]bool)
	oldShas := make(map[string]bool)
	files, err := filepath.Glob(filepath.Join(dir, brewFilePattern))
	if err != nil {
		log.Logger().Warnf("looking for homebrew ruby files in %s %v", dir, err)
	}
	for _, path := range files {
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "reading %s", path)
		}
		brewFile := string(bytes)
		updatedBrewFile := util.ReplaceAllStringSubmatchFunc(versionRegex, brewFile, func(groups []util.Group) []string {
			answer := make([]string, 0)
			for _, group := range groups {
				oldVersions[group.Value] = true
			}
			answer = append(answer, newVersion)
			return answer
		})
		updatedBrewFile = util.ReplaceAllStringSubmatchFunc(shaRegex, updatedBrewFile, func(groups []util.Group) []string {
			answer := make([]string, 0)
			for _, group := range groups {
				oldShas[group.Value] = true
			}
			answer = append(answer, newSha)
			return answer
		})
		info, err := os.Stat(path)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "reading mode of %s file", path)
		}
		err = ioutil.WriteFile(path, []byte(updatedBrewFile), info.Mode())
		if err != nil {
			return nil, nil, errors.Wrapf(err, "writing %s", path)
		}
	}
	versionAnswer := make([]string, 0)
	for oldVersion := range oldVersions {
		versionAnswer = append(versionAnswer, oldVersion)
	}
	sort.Strings(versionAnswer)

	shaAnswer := make([]string, 0)
	for oldSha := range oldShas {
		shaAnswer = append(shaAnswer, oldSha)
	}
	sort.Strings(shaAnswer)
	return versionAnswer, shaAnswer, nil
}

// BrewInfo contains some of the `brew info` data.
type brewInfo struct {
	Name     string
	Outdated bool
	Versions struct {
		Stable string
	}
}

func LatestJxBrewVersion(jsonInfo string) (string, error) {
	var brewInfo []brewInfo
	err := json.Unmarshal([]byte(jsonInfo), &brewInfo)
	if err != nil {
		return "", err
	}
	return brewInfo[0].Versions.Stable, nil
}
