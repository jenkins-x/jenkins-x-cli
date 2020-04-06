package v1

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	v1 "k8s.io/api/rbac/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ghodss/yaml"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/stoewer/go-strcase"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// Disable openapi-gen as this is not an API we want to promote
// +k8s:openapi-gen=false

// Extension represents an extension available to this Jenkins X install
type Extension struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec ExtensionSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionList is a list of Extensions available for a team
type ExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Extension `json:"items"`
}

// ExtensionSpec provides details of an extension available for a team
type ExtensionSpec struct {
	Name        string               `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	Description string               `json:"description,omitempty"  protobuf:"bytes,2,opt,name=description"`
	Version     string               `json:"version,omitempty"  protobuf:"bytes,3,opt,name=version"`
	Script      string               `json:"script,omitempty"  protobuf:"bytes,4,opt,name=script"`
	When        []ExtensionWhen      `json:"when,omitempty"  protobuf:"bytes,5,opt,name=when"`
	Given       ExtensionGiven       `json:"given,omitempty"  protobuf:"bytes,6,opt,name=given"`
	Parameters  []ExtensionParameter `json:"parameters,omitempty"  protobuf:"bytes,8,opt,name=parameters"`
	Namespace   string               `json:"namespace,omitempty"  protobuf:"bytes,9,opt,name=namespace"`
	UUID        string               `json:"uuid,omitempty"  protobuf:"bytes,10,opt,name=uuid"`
	Children    []string             `json:"children,omitempty"  protobuf:"bytes,11,opt,name=children"`
}

// ExtensionWhen specifies when in the lifecycle an extension should execute. By default Post.
type ExtensionWhen string

const (
	// Executed before a pipeline starts
	ExtensionWhenPre ExtensionWhen = "pre"
	// Executed after a pipeline completes
	ExtensionWhenPost ExtensionWhen = "post"
	// Executed when an extension installs
	ExtensionWhenInstall ExtensionWhen = "onInstall"
	// Executed when an extension uninstalls
	ExtensionWhenUninstall ExtensionWhen = "onUninstall"
	// Executed when an extension upgrades
	ExtensionWhenUpgrade ExtensionWhen = "onUpgrade"
)

// ExtensionGiven specifies the condition (if the extension is executing in a pipeline on which the extension should execute. By default Always.
type ExtensionGiven string

const (
	ExtensionGivenAlways  ExtensionGiven = "Always"
	ExtensionGivenFailure ExtensionGiven = "Failure"
	ExtensionGivenSuccess ExtensionGiven = "Success"
)

// ExtensionParameter describes a parameter definition for an extension
type ExtensionParameter struct {
	Name                    string `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	Description             string `json:"description,omitempty"  protobuf:"bytes,2,opt,name=description"`
	EnvironmentVariableName string `json:"environmentVariableName,omitempty"  protobuf:"bytes,3,opt,name=environmentVariableName"`
	DefaultValue            string `json:"defaultValue,omitempty"  protobuf:"bytes,3,opt,name=defaultValue"`
}

// ExtensionExecution is an executable instance of an extension which can be attached into a pipeline for later execution.
// It differs from an Extension as it cannot have children and parameters have been resolved to environment variables
type ExtensionExecution struct {
	Name                 string                `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	Description          string                `json:"description,omitempty"  protobuf:"bytes,2,opt,name=description"`
	Script               string                `json:"script,omitempty"  protobuf:"bytes,3,opt,name=script"`
	EnvironmentVariables []EnvironmentVariable `json:"environmentVariables,omitempty" protobuf:"bytes,4,opt,name=environmentvariables"`
	Given                ExtensionGiven        `json:"given,omitempty"  protobuf:"bytes,5,opt,name=given"`
	Namespace            string                `json:"namespace,omitempty"  protobuf:"bytes,7,opt,name=namespace"`
	UUID                 string                `json:"uuid,omitempty"  protobuf:"bytes,8,opt,name=uuid"`
}

// ExtensionRepositoryLockList contains a list of ExtensionRepositoryLock items
// +k8s:openapi-gen=false
type ExtensionRepositoryLockList struct {
	Version    string          `json:"version"`
	Extensions []ExtensionSpec `json:"extensions"`
}

// ExtensionDefinitionReferenceList contains a list of ExtensionRepository items
type ExtensionDefinitionReferenceList struct {
	Remotes []ExtensionDefinitionReference `json:"remotes"`
}

// ExtensionRepositoryReference references a GitHub repo that contains extension definitions
type ExtensionDefinitionReference struct {
	Remote string `json:"remote"`
	Tag    string `json:"tag"`
}

// ExtensionDefinitionList contains a list of ExtensionDefinition items
type ExtensionDefinitionList struct {
	Version    string                `json:"version,omitempty"`
	Extensions []ExtensionDefinition `json:"extensions"`
}

// ExtensionDefinition defines an Extension
type ExtensionDefinition struct {
	Name        string                              `json:"name"`
	Namespace   string                              `json:"namespace"`
	UUID        string                              `json:"uuid"`
	Description string                              `json:"description,omitempty"`
	When        []ExtensionWhen                     `json:"when,omitempty"`
	Given       ExtensionGiven                      `json:"given,omitempty"`
	Children    []ExtensionDefinitionChildReference `json:"children,omitempty"`
	ScriptFile  string                              `json:"scriptFile,omitempty"`
	Parameters  []ExtensionParameter                `json:"parameters,omitempty"`
}

// ExtensionDefinitionChildReference provides a reference to a child
type ExtensionDefinitionChildReference struct {
	UUID      string `json:"uuid,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Remote    string `json:"remote,omitempty"`
	Tag       string `json:"tag,omitempty"`
}

type EnvironmentVariable struct {
	Name  string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
}

// ExtensionsConfigList contains a list of ExtensionConfig items
type ExtensionConfigList struct {
	Extensions []ExtensionConfig `json:"extensions"`
}

// ExtensionConfig is the configuration and enablement for an extension inside an app
type ExtensionConfig struct {
	Name       string                    `json:"name"`
	Namespace  string                    `json:"namespace"`
	Parameters []ExtensionParameterValue `json:"parameters"`
}

const (
	ExtensionsConfigKnownRepositories = "knownRepositories"
	ExtensionsConfigRepository        = "repository"
)

type ExtensionRepositoryReferenceList struct {
	Repositories []ExtensionRepositoryReference `json:"repositories,omitempty"`
}

type ExtensionRepositoryReference struct {
	Url    string   `json:"url,omitempty"`
	GitHub string   `json:"github,omitempty"`
	Chart  ChartRef `json:"chart,omitempty"`
}

type ChartRef struct {
	Repo     string `json:"repo,omitempty"`
	RepoName string `json:"repoName,omitempty"`
	Name     string `json:"name,omitempty"`
}

type ExtensionParameterValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

const (
	VersionGlobalParameterName        string = "extVersion"
	TeamNamespaceGlobalParameterName  string = "extTeamNamespace"
	OwnerReferenceGlobalParameterName string = "extOwnerReference"
)

func (e *ExtensionExecution) Execute() (err error) {
	scriptFile, err := ioutil.TempFile("", fmt.Sprintf("%s-*", e.Name))
	if err != nil {
		return err
	}
	_, err = scriptFile.Write([]byte(e.Script))
	if err != nil {
		return err
	}
	err = scriptFile.Chmod(0755)
	if err != nil {
		return err
	}
	err = scriptFile.Close()
	if err != nil {
		return err
	}
	log.Logger().Debugf("Environment Variables:\n %s", e.EnvironmentVariables)
	log.Logger().Debugf("Script:\n %s", e.Script)
	envVars := make(map[string]string, 0)
	for _, v := range e.EnvironmentVariables {
		envVars[v.Name] = v.Value
	}
	cmd := util.Command{
		Name: scriptFile.Name(),
		Env:  envVars,
	}
	out, err := cmd.RunWithoutRetry()
	log.Logger().Info(out)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Error executing script %s", e.Name))
	}
	return nil
}

func (constraints *ExtensionDefinitionReferenceList) LoadFromFile(inputFile string) (err error) {
	path, err := filepath.Abs(inputFile)
	if err != nil {
		return err
	}
	y, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(y, constraints)
	if err != nil {
		return err
	}
	return nil
}

func (lock *ExtensionRepositoryLockList) LoadFromFile(inputFile string) (err error) {
	path, err := filepath.Abs(inputFile)
	if err != nil {
		return err
	}
	y, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(y, lock)
	if err != nil {
		return err
	}
	return nil
}

func (e *ExtensionDefinition) FullyQualifiedName() (fqn string) {
	return fmt.Sprintf("%s.%s", e.Namespace, e.Name)
}

func (e *ExtensionDefinition) FullyQualifiedKebabName() (fqn string) {
	return fmt.Sprintf("%s.%s", strcase.KebabCase(e.Namespace), strcase.KebabCase(e.Name))
}

func (e *ExtensionDefinitionChildReference) FullyQualifiedName() (fqn string) {
	return fmt.Sprintf("%s.%s", e.Namespace, e.Name)
}

func (e *ExtensionDefinitionChildReference) FullyQualifiedKebabName() (fqn string) {
	return fmt.Sprintf("%s.%s", strcase.KebabCase(e.Namespace), strcase.KebabCase(e.Name))
}

func (e *ExtensionSpec) FullyQualifiedName() (fqn string) {
	return fmt.Sprintf("%s.%s", e.Namespace, e.Name)
}

func (e *ExtensionSpec) FullyQualifiedKebabName() (fqn string) {
	return fmt.Sprintf("%s.%s", strcase.KebabCase(e.Namespace), strcase.KebabCase(e.Name))
}

func (e *ExtensionConfig) FullyQualifiedName() (fqn string) {
	return fmt.Sprintf("%s.%s", e.Namespace, e.Name)
}

func (e *ExtensionConfig) FullyQualifiedKebabName() (fqn string) {
	return fmt.Sprintf("%s.%s", strcase.KebabCase(e.Namespace), strcase.KebabCase(e.Name))
}

func (e *ExtensionExecution) FullyQualifiedName() (fqn string) {
	return fmt.Sprintf("%s.%s", e.Namespace, e.Name)
}

func (e *ExtensionExecution) FullyQualifiedKebabName() (fqn string) {
	return fmt.Sprintf("%s.%s", strcase.KebabCase(e.Namespace), strcase.KebabCase(e.Name))
}

func (extensionsConfig *ExtensionConfigList) LoadFromFile(inputFile string) (cfg *ExtensionConfigList, err error) {
	extensionsYamlPath, err := filepath.Abs(inputFile)
	if err != nil {
		return nil, err
	}
	extensionsYaml, err := ioutil.ReadFile(extensionsYamlPath)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(extensionsYaml, extensionsConfig)
	if err != nil {
		return nil, err
	}
	return extensionsConfig, nil
}

func (extensionsConfig *ExtensionConfigList) LoadFromConfigMap(configMapName string, client kubernetes.Interface, namespace string) (cfg *ExtensionConfigList, err error) {
	cm, err := client.CoreV1().ConfigMaps(namespace).Get(configMapName, metav1.GetOptions{})
	if err != nil {
		// CM doesn't exist, create it
		cm, err = client.CoreV1().ConfigMaps(namespace).Create(&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configMapName,
			},
		})
		if err != nil {
			return nil, err
		}
	}
	extensionsConfig.Extensions = make([]ExtensionConfig, 0)

	extensionConfigList := ExtensionConfigList{}
	err = yaml.Unmarshal([]byte(cm.Data["extensions"]), &extensionConfigList.Extensions)
	if err != nil {
		return nil, err
	}
	return &extensionConfigList, nil
}

func (e *ExtensionSpec) IsPost() bool {
	return e.Contains(ExtensionWhenPost) || len(e.When) == 0
}

func (e *ExtensionSpec) IsOnUninstall() bool {
	return e.Contains(ExtensionWhenUninstall)
}

func (e *ExtensionSpec) Contains(when ExtensionWhen) bool {
	for _, w := range e.When {
		if when == w {
			return true
		}
	}
	return false
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=false

// CommitStatus represents the commit statuses for a particular pull request
type CommitStatus struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec CommitStatusSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CommitStatusList is a structure used by k8s to store lists of commit statuses
type CommitStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CommitStatus `json:"Items"`
}

// CommitStatusSpec provides details of a particular commit status
type CommitStatusSpec struct {
	Items []CommitStatusDetails `json:"items"  protobuf:"bytes,1,opt,name=items"`
}

type CommitStatusDetails struct {
	PipelineActivity ResourceReference           `json:"pipelineActivity"  protobuf:"bytes,1,opt,name=pipelineActivity"`
	Items            []CommitStatusItem          `json:"Items,omitempty"  protobuf:"bytes,2,opt,name=Items"`
	Checked          bool                        `json:"checked"  protobuf:"bytes,3,opt,name=checked"`
	Commit           CommitStatusCommitReference `json:"commit"  protobuf:"bytes,4,opt,name=commit"`
	Context          string                      `json:"context"  protobuf:"bytes,5,opt,name=context"`
}

type CommitStatusCommitReference struct {
	GitURL      string `json:"gitUrl,omitempty"  protobuf:"bytes,1,opt,name=gitUrl"`
	PullRequest string `json:"pullRequest,omitempty"  protobuf:"bytes,2,opt,name=pullRequest"`
	SHA         string `json:"sha,omitempty"  protobuf:"bytes,3,opt,name=sha"`
}

type CommitStatusItem struct {
	Name        string `json:"name,omitempty"  protobuf:"bytes,1,opt,name=name"`
	Description string `json:"description,omitempty"  protobuf:"bytes,2,opt,name=description"`
	Pass        bool   `json:"pass"  protobuf:"bytes,3,opt,name=pass"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// App is the metadata for an App
type App struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec AppSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AppList is a structure used by k8s to store lists of apps
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []App `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// SourceRepositoryGroup is the metadata for an Application/Project/SourceRepository
type SourceRepositoryGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              SourceRepositoryGroupSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceRepositoryGroupList is a structure used by k8s to store lists of apps
type SourceRepositoryGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SourceRepositoryGroup `json:"items"`
}

// SourceRepositoryGroupSpec is the metadata for an Application/Project/SourceRepository
type SourceRepositoryGroupSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	SourceRepositorySpec []ResourceReference `json:"repositories" protobuf:"bytes,2,opt,name=repositories`
	Scheduler            ResourceReference   `json:"scheduler" protobuf:"bytes,3,opt,name=scheduler`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceRepository is the metadata for an Application/Project/SourceRepository
type SourceRepository struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec SourceRepositorySpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// Sanitize sanitizes the source repository URL
func (repo *SourceRepository) Sanitize() {
	repo.Spec.URL = util.SanitizeURL(repo.Spec.URL)
	repo.Spec.HTTPCloneURL = util.SanitizeURL(repo.Spec.HTTPCloneURL)
	repo.Spec.SSHCloneURL = util.SanitizeURL(repo.Spec.SSHCloneURL)
	// The URL is stored sometimes in the provider and provider name
	repo.Spec.Provider = util.SanitizeURL(repo.Spec.Provider)
	repo.Spec.ProviderName = util.SanitizeURL(repo.Spec.ProviderName)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceRepositoryList is a structure used by k8s to store lists of apps
type SourceRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SourceRepository `json:"items"`
}

// SourceRepositorySpec provides details of the metadata for an App
type SourceRepositorySpec struct {
	Description string `json:"description,omitempty" protobuf:"bytes,1,opt,name=description"`
	// Provider stores the URL of the git provider such as https://github.com
	Provider string `json:"provider,omitempty" protobuf:"bytes,2,opt,name=provider"`
	Org      string `json:"org,omitempty" protobuf:"bytes,3,opt,name=org"`
	Repo     string `json:"repo,omitempty" protobuf:"bytes,4,opt,name=repo"`
	// ProviderName is a logical name for the provider without any URL scheme which can be used in a label selector
	ProviderName string `json:"providerName,omitempty" protobuf:"bytes,5,opt,name=providerName"`
	// ProviderKind is the kind of provider (github / bitbucketcloud / bitbucketserver etc)
	ProviderKind string `json:"providerKind,omitempty" protobuf:"bytes,6,opt,name=providerKind"`
	// URL is the web URL of the project page
	URL string `json:"url,omitempty" protobuf:"bytes,7,opt,name=url"`
	// SSHCloneURL is the git URL to clone this repository using SSH
	SSHCloneURL string `json:"sshCloneURL,omitempty" protobuf:"bytes,8,opt,name=sshCloneURL"`
	// HTTPCloneURL is the git URL to clone this repository using HTTP/HTTPS
	HTTPCloneURL string `json:"httpCloneURL,omitempty" protobuf:"bytes,9,opt,name=httpCloneURL"`
	// Scheduler a reference to a custom scheduler otherwise we default to the Team's Scededuler
	Scheduler ResourceReference `json:"scheduler,omitempty" protobuf:"bytes,10,opt,name=scheduler"`
}

// AppSpec provides details of the metadata for an App
type AppSpec struct {
	SchemaPreprocessor     *corev1.Container `json:"schemaPreprocessor,omitempty" protobuf:"bytes,1,opt,name=schemaPreprocessor"`
	SchemaPreprocessorRole *v1.Role          `json:"schemaPreprocessorRole,omitempty" protobuf:"bytes,2,opt,name=schemaPreprocessorRole"`

	PipelineExtension *PipelineExtension `json:"pipelineExtension,omitempty" protobuf:"bytes,3,opt,name=pipelineExtension"`
}

// PipelineExtension defines the image and command of an app which wants to modify/extend the pipeline
type PipelineExtension struct {
	// Name of the container specified as a DNS_LABEL.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Docker image name.
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`
	// Entrypoint array. Not executed within a shell.
	Command string `json:"command,omitempty" protobuf:"bytes,3,rep,name=command"`
	// Arguments to the entrypoint.
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PluginList is a list of Plugins available for a team
type PluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Plugin `json:"items"`
}

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// Plugin represents a binary plugin installed into this Jenkins X team
type Plugin struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec PluginSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// PluginSpec provides details of a binary plugin available for a team
type PluginSpec struct {
	SubCommand  string   `json:"subCommand,omitempty"  protobuf:"bytes,3,opt,name=subCommand"`
	Group       string   `json:"group,omitempty"  protobuf:"bytes,4,opt,name=group"`
	Binaries    []Binary `json:"binaries,omitempty" protobuf:"bytes,7opt,name=binaries"`
	Description string   `json:"description,omitempty"  protobuf:"bytes,2,opt,name=description"`
	Name        string   `json:"name,omitempty"  protobuf:"bytes,5,opt,name=name"`
	Version     string   `json:"version,omitempty"  protobuf:"bytes,6,opt,name=version"`
}

// Binary provies the details of a downloadable binary
type Binary struct {
	Goarch string `json:"goarch,omitempty"  protobuf:"bytes,1,opt,name=goarch"`
	Goos   string `json:"goos,omitempty"  protobuf:"bytes,2,opt,name=goos"`
	URL    string `json:"url,omitempty"  protobuf:"bytes,3,opt,name=url"`
}
