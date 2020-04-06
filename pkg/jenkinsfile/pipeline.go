package jenkinsfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/yaml"
)

const (
	// PipelineConfigFileName is the name of the pipeline configuration file
	PipelineConfigFileName = "pipeline.yaml"

	// PipelineTemplateFileName defines the jenkisnfile template used to generate the pipeline
	PipelineTemplateFileName = "Jenkinsfile.tmpl"

	// PipelineKindRelease represents a release pipeline triggered on merge to master (or a release branch)
	PipelineKindRelease = "release"

	// PipelineKindPullRequest represents a Pull Request pipeline
	PipelineKindPullRequest = "pullrequest"

	// PipelineKindFeature represents a pipeline on a feature branch
	PipelineKindFeature = "feature"

	// the modes of adding a step

	// CreateStepModePre creates steps before any existing steps
	CreateStepModePre = "pre"

	// CreateStepModePost creates steps after the existing steps
	CreateStepModePost = "post"

	// CreateStepModeReplace replaces the existing steps with the new steps
	CreateStepModeReplace = "replace"
)

var (
	// PipelineKinds the possible values of pipeline
	PipelineKinds = []string{PipelineKindRelease, PipelineKindPullRequest, PipelineKindFeature}

	// PipelineLifecycleNames the possible names of lifecycles of pipeline
	PipelineLifecycleNames = []string{"setup", "setversion", "prebuild", "build", "postbuild", "promote"}

	// CreateStepModes the step creation modes
	CreateStepModes = []string{CreateStepModePre, CreateStepModePost, CreateStepModeReplace}
)

// Pipelines contains all the different kinds of pipeline for different branches
type Pipelines struct {
	PullRequest *PipelineLifecycles        `json:"pullRequest,omitempty"`
	Release     *PipelineLifecycles        `json:"release,omitempty"`
	Feature     *PipelineLifecycles        `json:"feature,omitempty"`
	Post        *PipelineLifecycle         `json:"post,omitempty"`
	Overrides   []*syntax.PipelineOverride `json:"overrides,omitempty"`
	Default     *syntax.ParsedPipeline     `json:"default,omitempty"`
}

// PipelineLifecycles defines the steps of a lifecycle section
type PipelineLifecycles struct {
	Setup      *PipelineLifecycle     `json:"setup,omitempty"`
	SetVersion *PipelineLifecycle     `json:"setVersion,omitempty"`
	PreBuild   *PipelineLifecycle     `json:"preBuild,omitempty"`
	Build      *PipelineLifecycle     `json:"build,omitempty"`
	PostBuild  *PipelineLifecycle     `json:"postBuild,omitempty"`
	Promote    *PipelineLifecycle     `json:"promote,omitempty"`
	Pipeline   *syntax.ParsedPipeline `json:"pipeline,omitempty"`
}

// PipelineLifecycle defines the steps of a lifecycle section
type PipelineLifecycle struct {
	Steps []*syntax.Step `json:"steps,omitempty"`

	// PreSteps if using inheritance then invoke these steps before the base steps
	PreSteps []*syntax.Step `json:"preSteps,omitempty"`

	// Replace if using inheritance then replace steps from the base pipeline
	Replace bool `json:"replace,omitempty"`
}

// NamedLifecycle a lifecycle and its name
type NamedLifecycle struct {
	Name      string
	Lifecycle *PipelineLifecycle
}

// PipelineLifecycleArray an array of named lifecycle pointers
type PipelineLifecycleArray []NamedLifecycle

// PipelineExtends defines the extension (e.g. parent pipeline which is overloaded
type PipelineExtends struct {
	Import string `json:"import,omitempty"`
	File   string `json:"file,omitempty"`
}

// ImportFile returns an ImportFile for the given extension
func (x *PipelineExtends) ImportFile() *ImportFile {
	return &ImportFile{
		Import: x.Import,
		File:   x.File,
	}
}

// PipelineConfig defines the pipeline configuration
type PipelineConfig struct {
	Extends          *PipelineExtends  `json:"extends,omitempty"`
	Agent            *syntax.Agent     `json:"agent,omitempty"`
	Env              []corev1.EnvVar   `json:"env,omitempty"`
	Environment      string            `json:"environment,omitempty"`
	Pipelines        Pipelines         `json:"pipelines,omitempty"`
	ContainerOptions *corev1.Container `json:"containerOptions,omitempty"`
}

// CreateJenkinsfileArguments contains the arguents to generate a Jenkinsfiles dynamically
type CreateJenkinsfileArguments struct {
	ConfigFile          string
	TemplateFile        string
	OutputFile          string
	IsTekton            bool
	ClearContainerNames bool
}

// +k8s:deepcopy-gen=false

// CreatePipelineArguments contains the arguments to translate a build pack into a pipeline
type CreatePipelineArguments struct {
	Lifecycles        *PipelineLifecycles
	PodTemplates      map[string]*corev1.Pod
	CustomImage       string
	DefaultImage      string
	WorkspaceDir      string
	GitHost           string
	GitName           string
	GitOrg            string
	ProjectID         string
	DockerRegistry    string
	DockerRegistryOrg string
	KanikoImage       string
	UseKaniko         bool
	NoReleasePrepare  bool
	StepCounter       int
}

// Validate validates all the arguments are set correctly
func (a *CreateJenkinsfileArguments) Validate() error {
	if a.ConfigFile == "" {
		return fmt.Errorf("Missing argument: ConfigFile")
	}
	if a.TemplateFile == "" {
		return fmt.Errorf("Missing argument: TemplateFile")
	}
	if a.OutputFile == "" {
		return fmt.Errorf("Missing argument: ReportName")
	}
	return nil
}

// Groovy returns the groovy expression for all of the lifecycles
func (a *PipelineLifecycles) Groovy() string {
	return a.All().Groovy()
}

// All returns all lifecycles in order
func (a *PipelineLifecycles) All() PipelineLifecycleArray {
	return []NamedLifecycle{
		{"setup", a.Setup},
		{"setversion", a.SetVersion},
		{"prebuild", a.PreBuild},
		{"build", a.Build},
		{"postbuild", a.PostBuild},
		{"promote", a.Promote},
	}
}

// AllButPromote returns all lifecycles but promote
func (a *PipelineLifecycles) AllButPromote() PipelineLifecycleArray {
	return []NamedLifecycle{
		{"setup", a.Setup},
		{"setversion", a.SetVersion},
		{"prebuild", a.PreBuild},
		{"build", a.Build},
		{"postbuild", a.PostBuild},
	}
}

// RemoveWhenStatements removes any when conditions
func (a *PipelineLifecycles) RemoveWhenStatements(prow bool) {
	for _, n := range a.All() {
		v := n.Lifecycle
		if v != nil {
			v.RemoveWhenStatements(prow)
		}
	}
}

// GetLifecycle returns the pipeline lifecycle of the given name lazy creating on the fly if required
// or returns an error if the name is not valid
func (a *PipelineLifecycles) GetLifecycle(name string, lazyCreate bool) (*PipelineLifecycle, error) {
	switch name {
	case "setup":
		if a.Setup == nil && lazyCreate {
			a.Setup = &PipelineLifecycle{}
		}
		return a.Setup, nil
	case "setversion":
		if a.SetVersion == nil && lazyCreate {
			a.SetVersion = &PipelineLifecycle{}
		}
		return a.SetVersion, nil
	case "prebuild":
		if a.PreBuild == nil && lazyCreate {
			a.PreBuild = &PipelineLifecycle{}
		}
		return a.PreBuild, nil
	case "build":
		if a.Build == nil && lazyCreate {
			a.Build = &PipelineLifecycle{}
		}
		return a.Build, nil
	case "postbuild":
		if a.PostBuild == nil && lazyCreate {
			a.PostBuild = &PipelineLifecycle{}
		}
		return a.PostBuild, nil
	case "promote":
		if a.Promote == nil && lazyCreate {
			a.Promote = &PipelineLifecycle{}
		}
		return a.Promote, nil
	default:
		return nil, fmt.Errorf("unknown pipeline lifecycle stage: %s", name)
	}
}

// Groovy returns the groovy string for the lifecycles
func (s PipelineLifecycleArray) Groovy() string {
	statements := []*util.Statement{}
	for _, n := range s {
		l := n.Lifecycle
		if l != nil {
			statements = append(statements, l.ToJenkinsfileStatements()...)
		}
	}
	text := util.WriteJenkinsfileStatements(4, statements)
	// lets remove the very last newline so its easier to compose in templates
	text = strings.TrimSuffix(text, "\n")
	return text
}

// Groovy returns the groovy expression for this lifecycle
func (l *NamedLifecycle) Groovy() string {
	lifecycles := PipelineLifecycleArray([]NamedLifecycle{*l})
	return lifecycles.Groovy()
}

// PutAllEnvVars puts all the defined environment variables in the given map
func (l *NamedLifecycle) PutAllEnvVars(m map[string]string) {
	if l.Lifecycle != nil {
		for _, step := range l.Lifecycle.Steps {
			step.PutAllEnvVars(m)
		}
	}
}

// Groovy returns the groovy expression for this lifecycle
func (l *PipelineLifecycle) Groovy() string {
	nl := &NamedLifecycle{Name: "", Lifecycle: l}
	return nl.Groovy()
}

// ToJenkinsfileStatements converts the lifecycle to one or more jenkinsfile statements
func (l *PipelineLifecycle) ToJenkinsfileStatements() []*util.Statement {
	statements := []*util.Statement{}
	for _, step := range l.Steps {
		statements = append(statements, step.ToJenkinsfileStatements()...)
	}
	return statements
}

// RemoveWhenStatements removes any when conditions
func (l *PipelineLifecycle) RemoveWhenStatements(prow bool) {
	l.PreSteps = removeWhenSteps(prow, l.PreSteps)
	l.Steps = removeWhenSteps(prow, l.Steps)
}

// CreateStep creates the given step using the mode
func (l *PipelineLifecycle) CreateStep(mode string, step *syntax.Step) error {
	err := step.Validate()
	if err != nil {
		return err
	}
	switch mode {
	case CreateStepModePre:
		l.PreSteps = append(l.PreSteps, step)
	case CreateStepModePost:
		l.Steps = append(l.Steps, step)
	case CreateStepModeReplace:
		l.Steps = []*syntax.Step{step}
		l.Replace = true
	default:
		return fmt.Errorf("uknown create mode: %s", mode)
	}
	return nil
}

func removeWhenSteps(prow bool, steps []*syntax.Step) []*syntax.Step {
	answer := []*syntax.Step{}
	for _, step := range steps {
		when := strings.TrimSpace(step.When)
		if prow && when == "!prow" {
			continue
		}
		if !prow && when == "prow" {
			continue
		}
		step.Steps = removeWhenSteps(prow, step.Steps)
		answer = append(answer, step)
	}
	return answer
}

// Extend extends these pipelines with the base pipeline
func (p *Pipelines) Extend(base *Pipelines) error {
	p.PullRequest = ExtendPipelines("pullRequest", p.PullRequest, base.PullRequest, p.Overrides)
	p.Release = ExtendPipelines("release", p.Release, base.Release, p.Overrides)
	p.Feature = ExtendPipelines("feature", p.Feature, base.Feature, p.Overrides)
	p.Post = ExtendLifecycle("", "post", p.Post, base.Post, p.Overrides)
	return nil
}

// All returns all the lifecycles in this pipeline, some may be null
func (p *Pipelines) All() []*PipelineLifecycles {
	return []*PipelineLifecycles{p.PullRequest, p.Feature, p.Release}
}

// AllMap returns all the lifecycles in this pipeline indexed by the pipeline name
func (p *Pipelines) AllMap() map[string]*PipelineLifecycles {
	m := map[string]*PipelineLifecycles{}
	if p.PullRequest != nil {
		m[PipelineKindPullRequest] = p.PullRequest
	}
	if p.Feature != nil {
		m[PipelineKindFeature] = p.Feature
	}
	if p.Release != nil {
		m[PipelineKindRelease] = p.Release
	}
	return m
}

// defaultContainerAndDir defaults the container if none is being used
func (p *Pipelines) defaultContainerAndDir(container string, dir string) {
	defaultContainerAndDir(container, dir, p.All()...)
}

// RemoveWhenStatements removes any prow or !prow statements
func (p *Pipelines) RemoveWhenStatements(prow bool) {
	for _, l := range p.All() {
		if l != nil {
			l.RemoveWhenStatements(prow)
		}
	}
	if p.Post != nil {
		p.Post.RemoveWhenStatements(prow)
	}
}

// GetPipeline returns the pipeline for the given name, creating if required if lazyCreate is true or returns an error if its not a valid name
func (p *Pipelines) GetPipeline(kind string, lazyCreate bool) (*PipelineLifecycles, error) {
	switch kind {
	case PipelineKindRelease:
		if p.Release == nil && lazyCreate {
			p.Release = &PipelineLifecycles{}
		}
		return p.Release, nil
	case PipelineKindPullRequest:
		if p.PullRequest == nil && lazyCreate {
			p.PullRequest = &PipelineLifecycles{}
		}
		return p.PullRequest, nil
	case PipelineKindFeature:
		if p.Feature == nil && lazyCreate {
			p.Feature = &PipelineLifecycles{}
		}
		return p.Feature, nil
	default:
		return nil, fmt.Errorf("no such pipeline kind: %s", kind)
	}
}

func defaultContainerAndDir(container string, dir string, lifecycles ...*PipelineLifecycles) {
	for _, l := range lifecycles {
		if l != nil {
			defaultLifecycleContainerAndDir(container, dir, l.All())
		}
	}
}

func defaultLifecycleContainerAndDir(container string, dir string, lifecycles PipelineLifecycleArray) {
	if container == "" && dir == "" {
		return
	}
	for _, n := range lifecycles {
		l := n.Lifecycle
		if l != nil {
			if dir != "" {
				l.PreSteps = defaultDirAroundSteps(dir, l.PreSteps)
				l.Steps = defaultDirAroundSteps(dir, l.Steps)
			}
			if container != "" {
				l.PreSteps = defaultContainerAroundSteps(container, l.PreSteps)
				l.Steps = defaultContainerAroundSteps(container, l.Steps)
			}
		}
	}
}

func defaultContainerAroundSteps(container string, steps []*syntax.Step) []*syntax.Step {
	if container == "" {
		return steps
	}
	var containerStep *syntax.Step
	result := []*syntax.Step{}
	for _, step := range steps {
		if step.GetImage() != "" {
			result = append(result, step)
		} else {
			if containerStep == nil {
				containerStep = &syntax.Step{
					Image: container,
				}
				result = append(result, containerStep)
			}
			containerStep.Steps = append(containerStep.Steps, step)
		}
	}
	return result
}

func defaultDirAroundSteps(dir string, steps []*syntax.Step) []*syntax.Step {
	if dir == "" {
		return steps
	}
	var dirStep *syntax.Step
	result := []*syntax.Step{}
	for _, step := range steps {
		if step.GetImage() != "" {
			step.Steps = defaultDirAroundSteps(dir, step.Steps)
			result = append(result, step)
		} else if step.Dir != "" {
			result = append(result, step)
		} else {
			if dirStep == nil {
				dirStep = &syntax.Step{
					Dir: dir,
				}
				result = append(result, dirStep)
			}
			dirStep.Steps = append(dirStep.Steps, step)
		}
	}
	return result
}

// LoadPipelineConfig returns the pipeline configuration
func LoadPipelineConfig(fileName string, resolver ImportFileResolver, isTekton bool, clearContainer bool) (*PipelineConfig, error) {
	return LoadPipelineConfigAndMaybeValidate(fileName, resolver, isTekton, clearContainer, true)
}

// LoadPipelineConfigAndMaybeValidate returns the pipeline configuration, optionally after validating the YAML.
func LoadPipelineConfigAndMaybeValidate(fileName string, resolver ImportFileResolver, isTekton bool, clearContainer bool, skipYamlValidation bool) (*PipelineConfig, error) {
	config := PipelineConfig{}
	exists, err := util.FileExists(fileName)
	if err != nil || !exists {
		return &config, err
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return &config, errors.Wrapf(err, "Failed to load file %s", fileName)
	}
	if !skipYamlValidation {
		validationErrors, err := util.ValidateYaml(&config, data)
		if err != nil {
			return &config, fmt.Errorf("failed to validate YAML file %s due to %s", fileName, err)
		}
		if len(validationErrors) > 0 {
			return &config, fmt.Errorf("Validation failures in YAML file %s:\n%s", fileName, strings.Join(validationErrors, "\n"))
		}
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return &config, errors.Wrapf(err, "Failed to unmarshal file %s", fileName)
	}
	pipelines := &config.Pipelines
	pipelines.RemoveWhenStatements(isTekton)
	if clearContainer {
		// lets force any agent for prow / jenkinsfile runner
		config.Agent = clearContainerAndLabel(config.Agent)
	}
	config.PopulatePipelinesFromDefault()
	if config.Extends == nil || config.Extends.File == "" {
		config.defaultContainerAndDir()
		return &config, nil
	}
	file := config.Extends.File
	importModule := config.Extends.Import
	if importModule != "" {
		file, err = resolver(config.Extends.ImportFile())
		if err != nil {
			return &config, errors.Wrapf(err, "Failed to resolve imports for file %s", fileName)
		}

	} else if !filepath.IsAbs(file) {
		dir, _ := filepath.Split(fileName)
		if dir != "" {
			file = filepath.Join(dir, file)
		}
	}
	exists, err = util.FileExists(file)
	if err != nil {
		return &config, errors.Wrapf(err, "base pipeline file does not exist %s", file)
	}
	if !exists {
		return &config, fmt.Errorf("base pipeline file does not exist %s", file)
	}
	basePipeline, err := LoadPipelineConfig(file, resolver, isTekton, clearContainer)
	if err != nil {
		return &config, errors.Wrapf(err, "Failed to base pipeline file %s", file)
	}
	err = config.ExtendPipeline(basePipeline, clearContainer)
	return &config, err
}

// PopulatePipelinesFromDefault sets the Release, PullRequest, and Feature pipelines, if unset, with the Default pipeline.
func (c *PipelineConfig) PopulatePipelinesFromDefault() {
	if c != nil && c.Pipelines.Default != nil {
		if c.Pipelines.Default.Agent == nil && c.Agent != nil {
			c.Pipelines.Default.Agent = c.Agent.DeepCopyForParsedPipeline()
		}
		if c.Pipelines.Release == nil {
			c.Pipelines.Release = &PipelineLifecycles{
				Pipeline: c.Pipelines.Default.DeepCopy(),
			}
		}
		if c.Pipelines.PullRequest == nil {
			c.Pipelines.PullRequest = &PipelineLifecycles{
				Pipeline: c.Pipelines.Default.DeepCopy(),
			}
		}
		if c.Pipelines.Feature == nil {
			c.Pipelines.Feature = &PipelineLifecycles{
				Pipeline: c.Pipelines.Default.DeepCopy(),
			}
		}
	}
}

// clearContainerAndLabel wipes the label and container from an Agent, preserving the Dir if it exists.
func clearContainerAndLabel(agent *syntax.Agent) *syntax.Agent {
	if agent != nil {
		agent.Container = ""
		agent.Image = ""
		agent.Label = ""

		return agent
	}
	return &syntax.Agent{}
}

// IsEmpty returns true if this configuration is empty
func (c *PipelineConfig) IsEmpty() bool {
	empty := &PipelineConfig{}
	return reflect.DeepEqual(empty, c)
}

// SaveConfig saves the configuration file to the given project directory
func (c *PipelineConfig) SaveConfig(fileName string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fileName, data, util.DefaultWritePermissions)
}

// ExtendPipeline inherits this pipeline from the given base pipeline
func (c *PipelineConfig) ExtendPipeline(base *PipelineConfig, clearContainer bool) error {
	if clearContainer {
		c.Agent = clearContainerAndLabel(c.Agent)
		base.Agent = clearContainerAndLabel(base.Agent)
	} else {
		if c.Agent == nil {
			c.Agent = &syntax.Agent{}
		}
		if base.Agent == nil {
			base.Agent = &syntax.Agent{}
		}
		if c.Agent.Label == "" {
			c.Agent.Label = base.Agent.Label
		} else if base.Agent.Label == "" && c.Agent.Label != "" {
			base.Agent.Label = c.Agent.Label
		}
		if c.Agent.GetImage() == "" {
			c.Agent.Image = base.Agent.GetImage()
		} else if base.Agent.GetImage() == "" && c.Agent.GetImage() != "" {
			base.Agent.Image = c.Agent.GetImage()
		}
	}
	if c.Agent.Dir == "" {
		c.Agent.Dir = base.Agent.Dir
	} else if base.Agent.Dir == "" && c.Agent.Dir != "" {
		base.Agent.Dir = c.Agent.Dir
	}
	mergedContainer, err := syntax.MergeContainers(base.ContainerOptions, c.ContainerOptions)
	if err != nil {
		return err
	}
	c.ContainerOptions = mergedContainer
	base.defaultContainerAndDir()
	c.defaultContainerAndDir()
	c.Env = syntax.CombineEnv(c.Env, base.Env)
	c.Pipelines.Extend(&base.Pipelines)
	return nil
}

func (c *PipelineConfig) defaultContainerAndDir() {
	if c.Agent != nil {
		c.Pipelines.defaultContainerAndDir(c.Agent.GetImage(), c.Agent.Dir)
	}
}

// GetAllEnvVars finds all the environment variables defined in all pipelines + steps with the first value we find
func (c *PipelineConfig) GetAllEnvVars() map[string]string {
	answer := map[string]string{}

	for _, pipeline := range c.Pipelines.All() {
		if pipeline != nil {
			for _, lifecycle := range pipeline.All() {
				lifecycle.PutAllEnvVars(answer)
			}
		}
	}
	for _, env := range c.Env {
		if env.Value != "" || answer[env.Name] == "" {
			answer[env.Name] = env.Value
		}
	}
	return answer

}

// ExtendPipelines extends the parent lifecycle with the base
func ExtendPipelines(pipelineName string, parent, base *PipelineLifecycles, overrides []*syntax.PipelineOverride) *PipelineLifecycles {
	if base == nil {
		return parent
	}
	if parent == nil {
		parent = &PipelineLifecycles{}
	}
	l := &PipelineLifecycles{
		Setup:      ExtendLifecycle(pipelineName, "setup", parent.Setup, base.Setup, overrides),
		SetVersion: ExtendLifecycle(pipelineName, "setVersion", parent.SetVersion, base.SetVersion, overrides),
		PreBuild:   ExtendLifecycle(pipelineName, "preBuild", parent.PreBuild, base.PreBuild, overrides),
		Build:      ExtendLifecycle(pipelineName, "build", parent.Build, base.Build, overrides),
		PostBuild:  ExtendLifecycle(pipelineName, "postBuild", parent.PostBuild, base.PostBuild, overrides),
		Promote:    ExtendLifecycle(pipelineName, "promote", parent.Promote, base.Promote, overrides),
	}
	if parent.Pipeline != nil {
		l.Pipeline = parent.Pipeline
	} else if base.Pipeline != nil {
		l.Pipeline = base.Pipeline
	}
	for _, override := range overrides {
		if override.MatchesPipeline(pipelineName) {
			// If no name, stage, or agent is specified, remove the whole pipeline.
			if override.Name == "" && override.Stage == "" && override.Agent == nil && override.ContainerOptions == nil && len(override.Volumes) == 0 {
				return &PipelineLifecycles{}
			}

			l.Pipeline = syntax.ApplyStepOverridesToPipeline(l.Pipeline, override)
		}
	}
	return l
}

// ExtendLifecycle extends the lifecycle with the inherited base lifecycle
func ExtendLifecycle(pipelineName, stageName string, parent *PipelineLifecycle, base *PipelineLifecycle, overrides []*syntax.PipelineOverride) *PipelineLifecycle {
	var lifecycle *PipelineLifecycle
	if parent == nil {
		lifecycle = base
	} else if base == nil {
		lifecycle = parent
	} else if parent.Replace {
		lifecycle = parent
	} else {
		steps := []*syntax.Step{}
		steps = append(steps, parent.PreSteps...)
		steps = append(steps, base.Steps...)
		steps = append(steps, parent.Steps...)
		lifecycle = &PipelineLifecycle{
			Steps: steps,
		}
	}

	if lifecycle != nil {
		for _, override := range overrides {
			if override.MatchesPipeline(pipelineName) && override.MatchesStage(stageName) {
				overriddenSteps := []*syntax.Step{}

				// If a step name is specified on this override, override looking for that step.
				if override.Name != "" {
					for _, s := range lifecycle.Steps {
						for _, o := range syntax.OverrideStep(*s, override) {
							overriddenSteps = append(overriddenSteps, &o)
						}
					}
				} else {
					// If no step name was specified but there are steps, just replace all steps in the stage/lifecycle,
					// or add the new steps before/after the existing steps in the stage/lifecycle
					if steps := override.AsStepsSlice(); len(steps) > 0 {
						if override.Type == nil || *override.Type == syntax.StepOverrideReplace {
							overriddenSteps = append(overriddenSteps, steps...)
						} else if *override.Type == syntax.StepOverrideBefore {
							overriddenSteps = append(overriddenSteps, steps...)
							overriddenSteps = append(overriddenSteps, lifecycle.Steps...)
						} else if *override.Type == syntax.StepOverrideAfter {
							overriddenSteps = append(overriddenSteps, lifecycle.Steps...)
							overriddenSteps = append(overriddenSteps, override.Steps...)
						}
					}
					// If there aren't any steps as well as no step name, then we're removing all steps from this stage/lifecycle,
					// so do nothing. =)
				}
				lifecycle.Steps = overriddenSteps
			}
		}
	}

	return lifecycle
}

// GenerateJenkinsfile generates the jenkinsfile
func (a *CreateJenkinsfileArguments) GenerateJenkinsfile(resolver ImportFileResolver) error {
	err := a.Validate()
	if err != nil {
		return err
	}
	config, err := LoadPipelineConfig(a.ConfigFile, resolver, a.IsTekton, a.ClearContainerNames)
	if err != nil {
		return err
	}

	templateFile := a.TemplateFile

	data, err := ioutil.ReadFile(templateFile)
	if err != nil {
		return errors.Wrapf(err, "failed to load template %s", templateFile)
	}

	t, err := template.New("myJenkinsfile").Parse(string(data))
	if err != nil {
		return errors.Wrapf(err, "failed to parse template %s", templateFile)
	}
	outFile := a.OutputFile
	outDir, _ := filepath.Split(outFile)
	if outDir != "" {
		err = os.MkdirAll(outDir, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to make directory %s when creating Jenkinsfile %s", outDir, outFile)
		}
	}
	file, err := os.Create(outFile)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %s", outFile)
	}
	defer file.Close()

	err = t.Execute(file, config)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", outFile)
	}
	return nil
}

// createPipelineSteps translates a step into one or more steps that can be used in jenkins-x.yml pipeline syntax.
func (c *PipelineConfig) createPipelineSteps(step *syntax.Step, prefixPath string, args CreatePipelineArguments) ([]syntax.Step, int) {
	steps := []syntax.Step{}

	containerName := c.Agent.GetImage()

	if step.GetImage() != "" {
		containerName = step.GetImage()
	}

	dir := args.WorkspaceDir

	if step.Dir != "" {
		dir = step.Dir
	}

	if step.GetCommand() != "" {
		if containerName == "" {
			containerName = args.DefaultImage
			log.Logger().Warnf("No 'agent.container' specified in the pipeline configuration so defaulting to use: %s", containerName)
		}

		s := syntax.Step{}
		args.StepCounter++
		prefix := prefixPath
		if prefix != "" {
			prefix += "-"
		}
		stepName := step.Name
		if stepName == "" {
			stepName = "step" + strconv.Itoa(1+args.StepCounter)
		}
		s.Name = prefix + stepName
		s.Command = replaceCommandText(step)
		if args.CustomImage != "" {
			s.Image = args.CustomImage
		} else {
			s.Image = containerName
		}

		s.Dir = dir
		s.Env = step.Env
		steps = append(steps, s)
	} else if step.Loop != nil {
		// Just copy in the loop step without altering it.
		// TODO: We don't get magic around image resolution etc, but we avoid naming collisions that result otherwise.
		steps = append(steps, *step)
	}
	for _, s := range step.Steps {
		// TODO add child prefix?
		childPrefixPath := prefixPath
		args.WorkspaceDir = dir
		nestedSteps, nestedCounter := c.createPipelineSteps(s, childPrefixPath, args)
		args.StepCounter = nestedCounter
		steps = append(steps, nestedSteps...)
	}
	return steps, args.StepCounter
}

// replaceCommandText lets remove any escaped "\$" stuff in the pipeline library
// and replace any use of the VERSION file with using the VERSION env var
func replaceCommandText(step *syntax.Step) string {
	answer := strings.Replace(step.GetFullCommand(), "\\$", "$", -1)

	// lets replace the old way of setting versions
	answer = strings.Replace(answer, "export VERSION=`cat VERSION` && ", "", 1)
	answer = strings.Replace(answer, "export VERSION=$PREVIEW_VERSION && ", "", 1)

	for _, text := range []string{"$(cat VERSION)", "$(cat ../VERSION)", "$(cat ../../VERSION)"} {
		answer = strings.Replace(answer, text, "${VERSION}", -1)
	}
	return answer
}

// createStageForBuildPack generates the Task for a build pack
func (c *PipelineConfig) createStageForBuildPack(args CreatePipelineArguments) (*syntax.Stage, int, error) {
	if args.Lifecycles == nil {
		return nil, args.StepCounter, errors.New("generatePipeline: no lifecycles")
	}

	// lets generate the pipeline using the build packs
	container := ""
	if c.Agent != nil {
		container = c.Agent.GetImage()

	}
	if args.CustomImage != "" {
		container = args.CustomImage
	}
	if container == "" {
		container = args.DefaultImage
	}

	steps := []syntax.Step{}
	for _, n := range args.Lifecycles.All() {
		l := n.Lifecycle
		if l == nil {
			continue
		}
		if !args.NoReleasePrepare && n.Name == "setversion" {
			continue
		}

		for _, s := range l.Steps {
			newSteps, newCounter := c.createPipelineSteps(s, n.Name, args)
			args.StepCounter = newCounter
			steps = append(steps, newSteps...)
		}
	}

	stage := &syntax.Stage{
		Name: syntax.DefaultStageNameForBuildPack,
		Agent: &syntax.Agent{
			Image: container,
		},
		Steps: steps,
	}

	return stage, args.StepCounter, nil
}

// CreatePipelineForBuildPack translates a set of lifecycles into a full pipeline.
func (c *PipelineConfig) CreatePipelineForBuildPack(args CreatePipelineArguments) (*syntax.ParsedPipeline, int, error) {
	args.GitOrg = naming.ToValidName(strings.ToLower(args.GitOrg))
	args.GitName = naming.ToValidName(strings.ToLower(args.GitName))
	args.DockerRegistryOrg = strings.ToLower(args.DockerRegistryOrg)

	stage, newCounter, err := c.createStageForBuildPack(args)
	if err != nil {
		return nil, args.StepCounter, errors.Wrapf(err, "Failed to generate stage from build pack")
	}

	parsed := &syntax.ParsedPipeline{
		Stages: []syntax.Stage{*stage},
	}

	// If agent.container is specified, use that for default container configuration for step images.
	containerName := c.Agent.GetImage()
	if containerName != "" {
		if args.PodTemplates != nil && args.PodTemplates[containerName] != nil {
			podTemplate := args.PodTemplates[containerName]
			container := podTemplate.Spec.Containers[0]
			if !equality.Semantic.DeepEqual(container, corev1.Container{}) {
				container.Name = ""
				container.Command = nil
				container.Args = nil
				container.Image = ""
				container.WorkingDir = ""
				container.Stdin = false
				container.TTY = false
				if parsed.Options == nil {
					parsed.Options = &syntax.RootOptions{}
				}
				parsed.Options.ContainerOptions = &container
				for _, v := range podTemplate.Spec.Volumes {
					parsed.Options.Volumes = append(parsed.Options.Volumes, &corev1.Volume{
						Name:         v.Name,
						VolumeSource: v.VolumeSource,
					})
				}
			}
		}
	}

	return parsed, newCounter, nil
}
