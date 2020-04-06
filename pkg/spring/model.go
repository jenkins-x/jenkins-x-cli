package spring

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/version"
	survey "gopkg.in/AlecAivazis/survey.v1"
)

const (
	OptionGroupId        = "group"
	OptionArtifactId     = "artifact"
	OptionLanguage       = "language"
	OptionJavaVersion    = "java-version"
	OptionBootVersion    = "boot-version"
	OptionPackaging      = "packaging"
	OptionDependency     = "dep"
	OptionDependencyKind = "kind"
	OptionType           = "type"

	startSpringURL = "https://start.spring.io"
)

var (
	DefaultDependencyKinds = []string{"Core", "Web", "Template Engines", "SQL", "I/O", "Ops", "Spring Cloud GCP", "Azure", "Cloud Contract", "Cloud AWS", "Cloud Messaging", "Cloud Tracing"}
)

type SpringValue struct {
	Type    string
	Default string
}

type SpringOption struct {
	ID           string
	Name         string
	Description  string
	VersionRange string
}

type SpringOptions struct {
	Type    string
	Default string
	Values  []SpringOption
}

type SpringTreeGroup struct {
	Name   string
	Values []SpringOption
}

type SpringTreeSelect struct {
	Type   string
	Values []SpringTreeGroup
}

type SpringBootModel struct {
	Packaging    SpringOptions
	Language     SpringOptions
	JavaVersion  SpringOptions
	BootVersion  SpringOptions
	Type         SpringOptions
	GroupId      SpringValue
	ArtifactId   SpringValue
	Version      SpringValue
	Name         SpringValue
	Description  SpringValue
	PackageName  SpringValue
	Dependencies SpringTreeSelect
}

type SpringBootForm struct {
	Packaging       string
	Language        string
	JavaVersion     string
	BootVersion     string
	GroupId         string
	ArtifactId      string
	Version         string
	Name            string
	PackageName     string
	Dependencies    []string
	DependencyKinds []string
	Type            string
}

type errorResponse struct {
	Timestamp string `json:"timestamp,omitempty"`
	Status    int    `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
	Message   string `json:"message,omitempty"`
	Path      string `json:"path,omitempty"`
}

func LoadSpringBoot(cacheDir string) (*SpringBootModel, error) {
	loader := func() ([]byte, error) {
		client := http.Client{}
		req, err := http.NewRequest(http.MethodGet, startSpringURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		addClientHeader(req)

		res, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		return ioutil.ReadAll(res.Body)
	}

	cacheFileName := ""
	if cacheDir != "" {
		cacheFileName = filepath.Join(cacheDir, "start_spring_io.json")
	}
	body, err := util.LoadCacheData(cacheFileName, loader)
	if err != nil {
		return nil, err
	}

	model := SpringBootModel{}
	err = json.Unmarshal(body, &model)
	if err != nil {
		return nil, err
	}
	// default the build tool
	if model.Type.Default == "" {
		model.Type.Default = "maven"
	}
	if len(model.Type.Values) == 0 {
		model.Type.Values = []SpringOption{
			{
				ID:          "gradle",
				Name:        "Gradle",
				Description: "Build with the gradle build tool",
			},
			{
				ID:          "maven",
				Name:        "Maven",
				Description: "Build with the maven build tool",
			},
		}
	}
	return &model, nil
}

func (model *SpringBootModel) CreateSurvey(data *SpringBootForm, advanced bool, batchMode bool) error {
	err := model.ValidateInput(OptionLanguage, &model.Language, data.Language)
	if err != nil {
		return err
	}
	err = model.ValidateInput(OptionBootVersion, &model.BootVersion, data.BootVersion)
	if err != nil {
		return err
	}
	err = model.ValidateInput(OptionJavaVersion, &model.JavaVersion, data.JavaVersion)
	if err != nil {
		return err
	}
	err = model.ValidateInput(OptionPackaging, &model.Packaging, data.Packaging)
	if err != nil {
		return err
	}
	err = model.ValidateTreeInput(OptionDependency, &model.Dependencies, data.Dependencies)
	if err != nil {
		return err
	}

	var qs = []*survey.Question{}
	if batchMode {
		return nil
	}
	if data.Language == "" {
		qs = append(qs, CreateValueSelect("Language", "language", &model.Language, data))
	}
	if data.BootVersion == "" && advanced {
		qs = append(qs, CreateValueSelect("Spring Boot version", "bootVersion", &model.BootVersion, data))
	}
	if data.JavaVersion == "" && advanced {
		qs = append(qs, CreateValueSelect("Java version", "javaVersion", &model.JavaVersion, data))
	}
	if data.Packaging == "" && advanced {
		qs = append(qs, CreateValueSelect("Packaging", "packaging", &model.Packaging, data))
	}
	if data.Type == "" && advanced {
		qs = append(qs, CreateValueSelect("Build Tool", "type", &model.Type, data))
	}
	if data.GroupId == "" {
		qs = append(qs, CreateValueInput("Group", "groupId", &model.GroupId, data))
	}
	if data.ArtifactId == "" {
		qs = append(qs, CreateValueInput("Artifact", "artifactId", &model.ArtifactId, data))
	}
	if emptyArray(data.Dependencies) {
		qs = append(qs, CreateSpringTreeSelect("Dependencies", "dependencies", &model.Dependencies, data))
	}
	return survey.Ask(qs, data)
}

func (options *SpringOptions) StringArray() []string {
	values := []string{}
	for _, o := range options.Values {
		id := o.ID
		if id != "" {
			values = append(values, id)
		}
	}
	sort.Strings(values)
	return values
}

func (options *SpringTreeSelect) StringArray() []string {
	values := []string{}
	for _, g := range options.Values {
		for _, o := range g.Values {
			id := o.ID
			if id != "" {
				values = append(values, id)
			}
		}
	}
	sort.Strings(values)
	return values
}

func (model *SpringBootModel) ValidateInput(name string, options *SpringOptions, value string) error {
	if value != "" && options != nil {
		for _, v := range options.Values {
			if v.ID == value {
				return nil
			}
		}
		return util.InvalidOption(name, value, options.StringArray())
	}
	return nil
}

func (model *SpringBootModel) ValidateTreeInput(name string, options *SpringTreeSelect, values []string) error {
	if values != nil && len(values) > 0 && options != nil {
		for _, value := range values {
			if value != "" {
				valid := false
				for _, g := range options.Values {
					for _, o := range g.Values {
						if o.ID == value {
							valid = true
							break
						}
					}
				}
				if !valid {
					return util.InvalidOption(name, value, options.StringArray())
				}
			}
		}
	}
	return nil
}

func CreateValueSelect(message string, name string, options *SpringOptions, data *SpringBootForm) *survey.Question {
	values := options.StringArray()
	return &survey.Question{
		Name: name,
		Prompt: &survey.Select{
			Message: message + ":",
			Options: values,
			Default: options.Default,
		},
		Validate: survey.Required,
	}
}

func CreateValueInput(message string, name string, value *SpringValue, data *SpringBootForm) *survey.Question {
	return &survey.Question{
		Name: name,
		Prompt: &survey.Input{
			Message: message + ":",
			Default: value.Default,
		},
		Validate: survey.Required,
	}
}

func CreateSpringTreeSelect(message string, name string, tree *SpringTreeSelect, data *SpringBootForm) *survey.Question {
	dependencyKinds := []string{}
	if data.DependencyKinds != nil {
		dependencyKinds = data.DependencyKinds
	}
	if len(dependencyKinds) == 0 {
		dependencyKinds = DefaultDependencyKinds
	}
	values := []string{}
	for _, t := range tree.Values {
		name := t.Name
		if util.StringArrayIndex(dependencyKinds, name) >= 0 {
			for _, v := range t.Values {
				id := v.ID
				if id != "" {
					values = append(values, id)
				}
			}
		}
	}
	sort.Strings(values)
	return &survey.Question{
		Name: name,
		Prompt: &survey.MultiSelect{
			Message: message + ":",
			Options: values,
		},
		Validate: survey.Required,
	}
}

func (data *SpringBootForm) CreateProject(workDir string) (string, error) {
	dirName := data.ArtifactId
	if dirName == "" {
		dirName = "project"
	}
	answer := filepath.Join(workDir, dirName)

	client := http.Client{}

	form := url.Values{}
	data.AddFormValues(&form)

	parameters := form.Encode()
	if parameters != "" {
		parameters = "?" + parameters
	}
	u := "http://start.spring.io/starter.zip" + parameters
	req, err := http.NewRequest(http.MethodGet, u, strings.NewReader(""))
	if err != nil {
		return answer, err
	}
	addClientHeader(req)
	res, err := client.Do(req)
	if err != nil {
		return answer, err
	}

	if res.StatusCode == 400 {
		errorBody, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return answer, err
		}

		errorResponse := errorResponse{}
		json.Unmarshal(errorBody, &errorResponse)

		log.Logger().Infof("%s", util.ColorError(errorResponse.Message))
		return answer, errors.New("unable to create spring quickstart")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return answer, err
	}

	dir := filepath.Join(workDir, dirName)
	zipFile := dir + ".zip"
	err = ioutil.WriteFile(zipFile, body, util.DefaultWritePermissions)
	if err != nil {
		return answer, fmt.Errorf("Failed to download file %s due to %s", zipFile, err)
	}
	err = util.Unzip(zipFile, dir)
	if err != nil {
		return answer, fmt.Errorf("Failed to unzip new project file %s due to %s", zipFile, err)
	}
	err = os.Remove(zipFile)
	if err != nil {
		return answer, err
	}
	return answer, nil
}

func (data *SpringBootForm) AddFormValues(form *url.Values) {
	AddFormValue(form, "packaging", data.Packaging)
	AddFormValue(form, "language", data.Language)
	AddFormValue(form, "javaVersion", data.JavaVersion)
	AddFormValue(form, "bootVersion", data.BootVersion)
	AddFormValue(form, "groupId", data.GroupId)
	AddFormValue(form, "artifactId", data.ArtifactId)
	AddFormValue(form, "version", data.Version)
	AddFormValue(form, "name", data.Name)
	AddFormValue(form, "type", data.Type)
	AddFormValues(form, "dependencies", data.Dependencies)
}

func AddFormValues(form *url.Values, key string, values []string) {
	for _, v := range values {
		if v != "" {
			form.Add(key, v)
		}
	}
}

func AddFormValue(form *url.Values, key string, v string) {
	if v != "" {
		form.Add(key, v)
	}
}

func emptyArray(values []string) bool {
	return values == nil || len(values) == 0
}

func addClientHeader(req *http.Request) {
	userAgent := "jx/" + version.GetVersion()
	req.Header.Set("User-Agent", userAgent)
}
