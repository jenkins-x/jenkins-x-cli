package builds

import (
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	numericStringRegex = regexp.MustCompile("[0-9]+")
)

// GetBuildNumber returns the build number using environment variables and/or pod Downward API files
func GetBuildNumber() string {
	buildNumber := os.Getenv("JX_BUILD_NUMBER")
	if buildNumber != "" {
		return buildNumber
	}
	buildNumber = os.Getenv("BUILD_NUMBER")
	if buildNumber != "" {
		return buildNumber
	}
	buildID := os.Getenv("BUILD_ID")
	if buildID != "" {
		return buildID
	}

	m := getDownwardAPILabelsMap()
	if m != nil {
		return GetBuildNumberFromLabels(m)
	}
	return ""
}

// GetBuildNumberFromLabels returns the
func GetBuildNumberFromLabels(m map[string]string) string {
	return GetBuildNumberFromLabelsWithKeys(m, LabelBuildName, "build-number", LabelOldBuildName, LabelPipelineRunName)
}

// getDownwardAPILabels returns the downward API labels from inside a pod or an empty string if they could not be found
func getDownwardAPILabelsMap() map[string]string {
	// if we are in a knative build pod we can discover it via the Downward API if the `/etc/podinfo/labels` file exists
	const podInfoLabelsFile = "/etc/podinfo/labels"
	exists, err := util.FileExists(podInfoLabelsFile)
	if err != nil {
		log.Logger().Warnf("failed to detect if the file %s exists: %s", podInfoLabelsFile, err)
	} else if exists {
		data, err := ioutil.ReadFile(podInfoLabelsFile)
		if err != nil {
			log.Logger().Warnf("failed to load downward API pod labels from %s due to: %s", podInfoLabelsFile, err)
		} else {
			text := strings.TrimSpace(string(data))
			if text != "" {
				return LoadDownwardAPILabels(text)
			}
		}
	}
	return nil
}

// GetBranchName returns the branch name using environment variables and/or pod Downward API
func GetBranchName() string {
	branch := os.Getenv(util.EnvVarBranchName)
	if branch == "" {
		m := getDownwardAPILabelsMap()
		if m != nil {
			branch = GetBranchNameFromLabels(m)
		}
	}
	return branch
}

// GetBranchNameFromLabels returns the branch name from the given pod labels
func GetBranchNameFromLabels(m map[string]string) string {
	return GetValueFromLabels(m, "branch")
}

// GetBuildNumberFromLabelsWithKeys returns the build number from the given Pod labels
func GetBuildNumberFromLabelsWithKeys(m map[string]string, keys ...string) string {
	if m == nil {
		return ""
	}

	answer := ""
	for _, key := range keys {
		answer = m[key]
		if answer != "" {
			break
		}
	}
	if answer != "" {
		return lastNumberFrom(answer)
	}
	return ""
}

// GetValueFromLabels returns the first label with the given key
func GetValueFromLabels(m map[string]string, keys ...string) string {
	if m == nil {
		return ""
	}
	answer := ""
	for _, key := range keys {
		answer = m[key]
		if answer != "" {
			break
		}
	}
	return answer
}

// lastNumberFrom splits a string such as "jstrachan-mynodething-master-1-build" via "-" and returns the last
// numeric string
func lastNumberFrom(text string) string {
	// lets remove any whilespace or double quotes
	paths := strings.Split(text, "-")
	for i := len(paths) - 1; i >= 0; i-- {
		path := paths[i]
		if numericStringRegex.MatchString(path) {
			return path
		}
	}
	return ""
}

func trimValue(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "\"")
	text = strings.TrimSuffix(text, "\"")
	return text
}

// LoadDownwardAPILabels parses the /etc/podinfo/labels text into a map of label values
func LoadDownwardAPILabels(text string) map[string]string {
	m := map[string]string{}
	if text != "" {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			l := strings.TrimSpace(line)
			paths := strings.SplitN(l, "=", 2)
			if len(paths) == 2 {
				m[paths[0]] = trimValue(paths[1])
			}
		}
	}
	return m
}
