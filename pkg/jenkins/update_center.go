package jenkins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

const (
	// DefaultUpdateCenterURL the LTS update center metadata URL
	DefaultUpdateCenterURL = "https://updates.jenkins.io/current/update-center.json"
)

// CoreVersion represents the version of Core
type CoreVersion struct {
	BuildDate string `json:"buildDate"`
	Name      string `json:"name"`
	Sha1      string `json:"sha1"`
	Sha256    string `json:"sha256"`
	URL       string `json:"url"`
	Version   string `json:"version"`
}

// Dependency a dependency of a plugin
type Dependency struct {
	Name     string `json:"name"`
	Optional bool   `json:"optional"`
	Version  string `json:"version"`
}

// Developer a developer on a plugin
type Developer struct {
	DeveloperID string `json:"developerId"`
	Email       string `json:"email"`
	Name        string `json:"name"`
}

// PluginRelease represets the data for a plugin
type PluginRelease struct {
	BuildDate    string       `json:"buildDate"`
	Dependencies []Dependency `json:"dependencies"`

	Developers        []Developer `json:"developers"`
	Excerpt           string      `json:"excerpt"`
	Gav               string      `json:"gav"`
	Labels            []string    `json:"labels"`
	Name              string      `json:"name"`
	PreviousTimestamp string      `json:"previousTimestamp"`
	PreviousVersion   string      `json:"previousVersion"`
	ReleaseTimestamp  string      `json:"releaseTimestamp"`
	RequiredCore      string      `json:"requiredCore"`
	Scm               string      `json:"scm"`
	Sha1              string      `json:"sha1"`
	Sha256            string      `json:"sha256"`
	Title             string      `json:"title"`
	URL               string      `json:"url"`
	Version           string      `json:"version"`
	Wiki              string      `json:"wiki"`
}

// Signature signature metadata
type Signature struct {
	Certificates        []string `json:"certificates"`
	CorrectDigest       string   `json:"correct_digest"`
	CorrectDigest512    string   `json:"correct_digest512"`
	CorrectSignature    string   `json:"correct_signature"`
	CorrectSignature512 string   `json:"correct_signature512"`
	Digest              string   `json:"digest"`
	Digest512           string   `json:"digest512"`
	Signature           string   `json:"signature"`
	Signature512        string   `json:"signature512"`
}

// Warning a warning message
type Warning struct {
	ID       string           `json:"id"`
	Message  string           `json:"message"`
	Name     string           `json:"name"`
	Type     string           `json:"type"`
	URL      string           `json:"url"`
	Versions []WarningVersion `json:"versions"`
}

// WarningVersion warning versions
type WarningVersion struct {
	FirstVersion string `json:"firstVersion"`
	LastVersion  string `json:"lastVersion"`
	Pattern      string `json:"pattern"`
}

// UpdateCenter represents the Update Center metadata returned from URLs
// like https://updates.jenkins.io/current/update-center.json
type UpdateCenter struct {
	ConnectionCheckURL  string                   `json:"connectionCheckUrl"`
	Core                CoreVersion              `json:"core"`
	ID                  string                   `json:"id"`
	Plugins             map[string]PluginRelease `json:"plugins"`
	Signature           Signature                `json:"signature"`
	UpdateCenterVersion string                   `json:"updateCenterVersion"`
	Warnings            []Warning                `json:"warnings"`
}

// LoadUpdateCenterFile loads the given UpdateCenter JSON file
func LoadUpdateCenterFile(fileName string) (*UpdateCenter, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load UpdateCenter file %s", fileName)
	}
	answer, err := LoadUpdateCenterData(data)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to unmarshal JSON from UpdateCenter file %s", fileName)
	}
	return answer, nil
}

// LoadUpdateCenterURL loads the given UpdateCenter URL
func LoadUpdateCenterURL(u string) (*UpdateCenter, error) {
	httpClient := util.GetClient()
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to invoke GET on %s", u)
	}
	stream := resp.Body
	defer stream.Close()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GET data from %s", u)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("status %s when performing GET on %s", resp.Status, u)
	}
	answer, err := LoadUpdateCenterData(data)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to unmarshal JSON from UpdateCenter URL %s", u)
	}
	return answer, nil
}

// LoadUpdateCenterData loads the given UpdateCenter data
func LoadUpdateCenterData(data []byte) (*UpdateCenter, error) {
	answer := &UpdateCenter{}
	// lets check for JSONP and strip it
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "{") && !strings.HasPrefix(text, "[") {
		// we have JSONP of the form 'foo.bar(....)' so lets grab the contents
		i := strings.Index(text, "(")
		if i < 0 {
			lines := strings.SplitN(text, "\n", 2)
			return answer, fmt.Errorf("invalid JSON and JSONP document. Was expecting '(' but got: %s", lines[0])
		}
		text = text[i+1:]
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)
		text = strings.TrimSuffix(text, ")")
		text = strings.TrimSpace(text)
		data = []byte(text)
	}
	err := json.Unmarshal(data, answer)
	return answer, err
}

// PickPlugins provides the user with a list of plugins that can be added to a Jenkins App
func (u *UpdateCenter) PickPlugins(currentValues []string, handles util.IOFileHandles) ([]string, error) {
	names := []string{}
	pluginMap := map[string]string{}
	maxLen := 1
	for _, plugin := range u.Plugins {
		l := utf8.RuneCountInString(plugin.Name)
		if l > maxLen {
			maxLen = l
		}
	}
	defaults := []string{}

	for _, plugin := range u.Plugins {
		name := util.PadRight(plugin.Name, " ", maxLen) + " " + plugin.Title
		namePrefix := plugin.Name + ":"
		current := false
		for _, cv := range currentValues {
			if strings.HasPrefix(cv, namePrefix) {
				current = true
				break
			}
		}
		if current {
			defaults = append(defaults, name)
		}
		value := namePrefix + plugin.Version
		names = append(names, name)
		pluginMap[name] = value
	}
	sort.Strings(names)
	help := "select the Jenkins plugins you wish to include inside your Jenkins App"
	message := "pick Jenkins plugins to include in your Jenkins App: "
	selection, err := util.PickNamesWithDefaults(names, defaults, message, help, handles)
	if err != nil {
		return nil, err
	}
	answer := []string{}
	for _, sel := range selection {
		value := pluginMap[sel]
		if value == "" {
			log.Logger().Warnf("Could not find value for %s in map!", value)
		} else {
			answer = append(answer, value)
		}
	}
	return answer, nil
}
