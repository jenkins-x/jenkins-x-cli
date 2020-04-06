package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/pkg/errors"
)

var (
	dockerfileNameRegex = regexp.MustCompile(`^(Dockerfile|Dockerfile\..*)$`)
)

//UpdateVersions scans the directory structure rooted in dir for files that match dockerfileNameRegex and replaces any
// lines starting with FROM <name>:, ENV <name> or ARG=<name> with the newVersion
func UpdateVersions(dir string, newVersion string, name string) ([]string, error) {
	linePrefixes := []string{
		fmt.Sprintf("FROM %s:", name),
		fmt.Sprintf("ENV %s ", name),
		fmt.Sprintf("ARG %s=", name),
	} // #nosec
	oldVersions := make(map[string]bool)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Logger().Warnf("looking for Dockerfiles in %s %v", path, err)
		} else {
			if dockerfileNameRegex.MatchString(filepath.Base(path)) {
				bytes, err := ioutil.ReadFile(path)
				if err != nil {
					return errors.Wrapf(err, "reading %s", path)
				}
				dockerfile := string(bytes)
				answer := make([]string, 0)
				for _, line := range strings.Split(dockerfile, "\n") {
					found := false
					for _, prefix := range linePrefixes {
						if strings.HasPrefix(line, prefix) {
							oldVersions[strings.TrimPrefix(line, prefix)] = true
							answer = append(answer, fmt.Sprintf("%s%s", prefix, newVersion))
							found = true
							continue
						}
					}
					if !found {
						answer = append(answer, line)
					}
				}
				err = ioutil.WriteFile(path, []byte(strings.Join(answer, "\n")), info.Mode())
				if err != nil {
					return errors.Wrapf(err, "writing %s", path)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "walking %s to file Dockerfiles", dir)
	}
	answer := make([]string, 0)
	for oldVersion := range oldVersions {
		answer = append(answer, oldVersion)
	}
	sort.Strings(answer)
	return answer, nil
}
