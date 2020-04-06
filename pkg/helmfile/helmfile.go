package helmfile

// Originally authored in the roboll/helmfile repo https://github.com/roboll/helmfile/blob/fc75f25293055003d8159a841940313e56a164c6/pkg/state/state.go
// copied here to avoid issues go module dependency issues
// changed from yaml: to json: annotations so we can marshal struct and omit unset values

// HelmState structure for the helmfile
type HelmState struct {
	FilePath string `json:"filePath,omitempty"`

	// DefaultValues is the default values to be overrode by environment values and command-line overrides
	DefaultValues []interface{} `json:"values,omitempty"`

	Environments map[string]EnvironmentSpec `json:"environments,omitempty"`

	Bases              []string          `json:"bases,omitempty"`
	HelmDefaults       HelmSpec          `json:"helmDefaults,omitempty"`
	Helmfiles          []SubHelmfileSpec `json:"helmfiles,omitempty"`
	DeprecatedContext  string            `json:"context,omitempty"`
	DeprecatedReleases []ReleaseSpec     `json:"charts,omitempty"`
	OverrideNamespace  string            `json:"namespace,omitempty"`
	Repositories       []RepositorySpec  `json:"repositories,omitempty"`
	Releases           []ReleaseSpec     `json:"releases,omitempty"`
	Selectors          []string          `json:"-"`
	APIVersions        []string          `json:"apiVersions,omitempty"`

	Templates map[string]TemplateSpec `json:"templates,omitempty"`

	Env Environment `json:"-"`
}

// SubHelmfileSpec defines the subhelmfile path and options
type SubHelmfileSpec struct {
	//path or glob pattern for the sub helmfiles
	Path string `json:"path,omitempty"`
	//chosen selectors for the sub helmfiles
	Selectors []string `json:"selectors,omitempty"`
	//do the sub helmfiles inherits from parent selectors
	SelectorsInherited bool `json:"selectorsInherited,omitempty"`

	Environment SubhelmfileEnvironmentSpec
}

// SubhelmfileEnvironmentSpec overrides
type SubhelmfileEnvironmentSpec struct {
	OverrideValues []interface{} `json:"values,omitempty"`
}

// HelmSpec to defines helmDefault values
type HelmSpec struct {
	KubeContext     string   `json:"kubeContext,omitempty"`
	TillerNamespace string   `json:"tillerNamespace,omitempty"`
	Tillerless      bool     `json:"tillerless"`
	Args            []string `json:"args,omitempty"`
	Verify          bool     `json:"verify"`
	// Devel, when set to true, use development versions, too. Equivalent to version '>0.0.0-0'
	Devel bool `json:"devel"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait bool `json:"wait"`
	// Timeout is the time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
	Timeout int `json:"timeout"`
	// RecreatePods, when set to true, instruct helmfile to perform pods restart for the resource if applicable
	RecreatePods bool `json:"recreatePods"`
	// Force, when set to true, forces resource update through delete/recreate if needed
	Force bool `json:"force"`
	// Atomic, when set to true, restore previous state in case of a failed install/upgrade attempt
	Atomic bool `json:"atomic"`
	// CleanupOnFail, when set to true, the --cleanup-on-fail helm flag is passed to the upgrade command
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`
	// HistoryMax, limit the maximum number of revisions saved per release. Use 0 for no limit (default 10)
	HistoryMax *int `json:"historyMax,omitempty"`

	TLS       bool   `json:"tls"`
	TLSCACert string `json:"tlsCACert,omitempty"`
	TLSKey    string `json:"tlsKey,omitempty"`
	TLSCert   string `json:"tlsCert,omitempty"`
}

// RepositorySpec that defines values for a helm repo
type RepositorySpec struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	CaFile   string `json:"caFile,omitempty"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ReleaseSpec defines the structure of a helm release
type ReleaseSpec struct {
	// Chart is the name of the chart being installed to create this release
	Chart   string `json:"chart,omitempty"`
	Version string `json:"version,omitempty"`
	Verify  *bool  `json:"verify,omitempty"`
	// Devel, when set to true, use development versions, too. Equivalent to version '>0.0.0-0'
	Devel *bool `json:"devel,omitempty"`
	// Wait, if set to true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment are in a ready state before marking the release as successful
	Wait *bool `json:"wait,omitempty"`
	// Timeout is the time in seconds to wait for any individual Kubernetes operation (like Jobs for hooks, and waits on pod/pvc/svc/deployment readiness) (default 300)
	Timeout *int `json:"timeout,omitempty"`
	// RecreatePods, when set to true, instruct helmfile to perform pods restart for the resource if applicable
	RecreatePods *bool `json:"recreatePods,omitempty"`
	// Force, when set to true, forces resource update through delete/recreate if needed
	Force *bool `json:"force,omitempty"`
	// Installed, when set to true, `delete --purge` the release
	Installed *bool `json:"installed,omitempty"`
	// Atomic, when set to true, restore previous state in case of a failed install/upgrade attempt
	Atomic *bool `json:"atomic,omitempty"`
	// CleanupOnFail, when set to true, the --cleanup-on-fail helm flag is passed to the upgrade command
	CleanupOnFail *bool `json:"cleanupOnFail,omitempty"`
	// HistoryMax, limit the maximum number of revisions saved per release. Use 0 for no limit (default 10)
	HistoryMax *int `json:"historyMax,omitempty"`

	// MissingFileHandler is set to either "Error" or "Warn". "Error" instructs helmfile to fail when unable to find a values or secrets file. When "Warn", it prints the file and continues.
	// The default value for MissingFileHandler is "Error".
	MissingFileHandler *string `json:"missingFileHandler,omitempty"`
	// Needs is the [TILLER_NS/][NS/]NAME representations of releases that this release depends on.
	Needs []string `json:"needs,omitempty"`

	// Hooks is a list of extension points paired with operations, that are executed in specific points of the lifecycle of releases defined in helmfile
	Hooks []Hook `json:"hooks,omitempty"`

	// Name is the name of this release
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	// JENKINS X comment - changed from the original []interface{} so we can unmarshall string array
	Values    []string   `json:"values,omitempty"`
	Secrets   []string   `json:"secrets,omitempty"`
	SetValues []SetValue `json:"set,omitempty"`

	ValuesTemplate    []interface{} `json:"valuesTemplate,omitempty"`
	SetValuesTemplate []SetValue    `json:"setTemplate,omitempty"`

	// The 'env' section is not really necessary any longer, as 'set' would now provide the same functionality
	EnvValues []SetValue `json:"env,omitempty"`

	ValuesPathPrefix string `json:"valuesPathPrefix,omitempty"`

	TillerNamespace string `json:"tillerNamespace,omitempty"`
	Tillerless      *bool  `json:"tillerless,omitempty"`

	KubeContext string `json:"kubeContext,omitempty"`

	TLS       *bool  `json:"tls,omitempty"`
	TLSCACert string `json:"tlsCACert,omitempty"`
	TLSKey    string `json:"tlsKey,omitempty"`
	TLSCert   string `json:"tlsCert,omitempty"`

	// These values are used in templating
	TillerlessTemplate *string `json:"tillerlessTemplate,omitempty"`
	VerifyTemplate     *string `json:"verifyTemplate,omitempty"`
	WaitTemplate       *string `json:"waitTemplate,omitempty"`
	InstalledTemplate  *string `json:"installedTemplate,omitempty"`

	// These settings requires helm-x integration to work
	Dependencies          []Dependency  `json:"dependencies,omitempty"`
	JSONPatches           []interface{} `json:"jsonPatches,omitempty"`
	StrategicMergePatches []interface{} `json:"strategicMergePatches,omitempty"`
	Adopt                 []string      `json:"adopt,omitempty"`
}

// Release for helm
type Release struct {
	ReleaseSpec

	Filtered bool
}

// SetValue are the key values to set on a helm release
type SetValue struct {
	Name   string   `json:"name,omitempty"`
	Value  string   `json:"value,omitempty"`
	File   string   `json:"file,omitempty"`
	Values []string `json:"values,omitempty"`
}

// AffectedReleases hold the list of released that where updated, deleted, or in error
type AffectedReleases struct {
	Upgraded []*ReleaseSpec
	Deleted  []*ReleaseSpec
	Failed   []*ReleaseSpec
}

// EnvironmentSpec envs
type EnvironmentSpec struct {
	Values  []interface{} `json:"values,omitempty"`
	Secrets []string      `json:"secrets,omitempty"`

	// MissingFileHandler instructs helmfile to fail when unable to find a environment values file listed
	// under `environments.NAME.values`.
	//
	// Possible values are  "Error", "Warn", "Info", "Debug". The default is "Error".
	//
	// Use "Warn", "Info", or "Debug" if you want helmfile to not fail when a values file is missing, while just leaving
	// a message about the missing file at the log-level.
	MissingFileHandler *string `json:"missingFileHandler,omitempty"`
}

// TemplateSpec defines the structure of a reusable and composable template for helm releases.
type TemplateSpec struct {
	ReleaseSpec `json:",inline"`
}

// EnvironmentTemplateData provides variables accessible while executing golang text/template expressions in helmfile and values YAML files
type EnvironmentTemplateData struct {
	// Environment is accessible as `.Environment` from any template executed by the renderer
	Environment Environment
	// Namespace is accessible as `.Namespace` from any non-values template executed by the renderer
	Namespace string
	// Values is accessible as `.Values` and it contains default state values overrode by environment values and override values.
	Values map[string]interface{}
}

// Dependency deps
type Dependency struct {
	Chart   string `json:"chart"`
	Version string `json:"version"`
	Alias   string `json:"alias"`
}

// Hook hooks
type Hook struct {
	Name     string   `json:"name"`
	Events   []string `json:"events"`
	Command  string   `json:"command"`
	Args     []string `json:"args"`
	ShowLogs bool     `json:"showlogs"`
}

// Environment vars
type Environment struct {
	Name     string
	Values   map[string]interface{}
	Defaults map[string]interface{}
}
