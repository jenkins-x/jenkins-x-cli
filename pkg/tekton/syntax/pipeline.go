package syntax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/knative/pkg/apis"
	"github.com/pkg/errors"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	tektonv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

const (
	// GitMergeImage is the default image name that is used in the git merge step of a pipeline
	GitMergeImage = "gcr.io/jenkinsxio/builder-jx"

	// WorkingDirRoot is the root directory for working directories.
	WorkingDirRoot = "/workspace"

	// braceMatchingRegex matches "${inputs.params.foo}" so we can replace it with "$(inputs.params.foo)"
	braceMatchingRegex = "(\\$(\\{(?P<var>inputs\\.params\\.[_a-zA-Z][_a-zA-Z0-9.-]*)\\}))"
)

var (
	ipAddressRegistryRegex = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+.\d+(:\d+)?`)

	commandIsSkaffoldRegex = regexp.MustCompile(`export VERSION=.*? && skaffold build.*`)
)

// ParsedPipeline is the internal representation of the Pipeline, used to validate and create CRDs
type ParsedPipeline struct {
	Agent      *Agent          `json:"agent,omitempty"`
	Env        []corev1.EnvVar `json:"env,omitempty"`
	Options    *RootOptions    `json:"options,omitempty"`
	Stages     []Stage         `json:"stages"`
	Post       []Post          `json:"post,omitempty"`
	WorkingDir *string         `json:"dir,omitempty"`

	// Replaced by Env, retained for backwards compatibility
	Environment []corev1.EnvVar `json:"environment,omitempty"`
}

// Agent defines where the pipeline, stage, or step should run.
type Agent struct {
	// One of label or image is required.
	Label string `json:"label,omitempty"`
	Image string `json:"image,omitempty"`

	// Legacy fields from jenkinsfile.PipelineAgent
	Container string `json:"container,omitempty"`
	Dir       string `json:"dir,omitempty"`
}

// TimeoutUnit is used for calculating timeout duration
type TimeoutUnit string

// The available time units.
const (
	TimeoutUnitSeconds TimeoutUnit = "seconds"
	TimeoutUnitMinutes TimeoutUnit = "minutes"
	TimeoutUnitHours   TimeoutUnit = "hours"
	TimeoutUnitDays    TimeoutUnit = "days"
)

// All possible time units, used for validation
var allTimeoutUnits = []TimeoutUnit{TimeoutUnitSeconds, TimeoutUnitMinutes, TimeoutUnitHours, TimeoutUnitDays}

func allTimeoutUnitsAsStrings() []string {
	tu := make([]string, len(allTimeoutUnits))

	for i, u := range allTimeoutUnits {
		tu[i] = string(u)
	}

	return tu
}

// Timeout defines how long a stage or pipeline can run before timing out.
type Timeout struct {
	Time int64 `json:"time"`
	// Has some sane default - probably seconds
	Unit TimeoutUnit `json:"unit,omitempty"`
}

// ToDuration generates a duration struct from a Timeout
func (t *Timeout) ToDuration() (*metav1.Duration, error) {
	durationStr := ""
	// TODO: Populate a default timeout unit, most likely seconds.
	if t.Unit != "" {
		durationStr = fmt.Sprintf("%d%c", t.Time, t.Unit[0])
	} else {
		durationStr = fmt.Sprintf("%ds", t.Time)
	}

	d, err := time.ParseDuration(durationStr)
	if err != nil {
		return nil, err
	}
	return &metav1.Duration{Duration: d}, nil
}

// RootOptions contains options that can be configured on either a pipeline or a stage
type RootOptions struct {
	Timeout *Timeout `json:"timeout,omitempty"`
	Retry   int8     `json:"retry,omitempty"`
	// ContainerOptions allows for advanced configuration of containers for a single stage or the whole
	// pipeline, adding to configuration that can be configured through the syntax already. This includes things
	// like CPU/RAM requests/limits, secrets, ports, etc. Some of these things will end up with native syntax approaches
	// down the road.
	ContainerOptions              *corev1.Container   `json:"containerOptions,omitempty"`
	Volumes                       []*corev1.Volume    `json:"volumes,omitempty"`
	DistributeParallelAcrossNodes bool                `json:"distributeParallelAcrossNodes,omitempty"`
	Tolerations                   []corev1.Toleration `json:"tolerations,omitempty"`
	PodLabels                     map[string]string   `json:"podLabels,omitempty"`
}

// Stash defines files to be saved for use in a later stage, marked with a name
type Stash struct {
	Name string `json:"name"`
	// Eventually make this optional so that you can do volumes instead
	Files string `json:"files"`
}

// Unstash defines a previously-defined stash to be copied into this stage's workspace
type Unstash struct {
	Name string `json:"name"`
	Dir  string `json:"dir,omitempty"`
}

// StageOptions contains both options that can be configured on either a pipeline or a stage, via
// RootOptions, or stage-specific options.
type StageOptions struct {
	*RootOptions `json:",inline"`

	// TODO: Not yet implemented in build-pipeline
	Stash   *Stash   `json:"stash,omitempty"`
	Unstash *Unstash `json:"unstash,omitempty"`

	Workspace *string `json:"workspace,omitempty"`
}

// Step defines a single step, from the author's perspective, to be executed within a stage.
type Step struct {
	// An optional name to give the step for reporting purposes
	Name string `json:"name,omitempty"`

	// One of command, step, or loop is required.
	Command string `json:"command,omitempty"`
	// args is optional, but only allowed with command
	Arguments []string `json:"args,omitempty"`
	// dir is optional, but only allowed with command. Refers to subdirectory of workspace
	Dir string `json:"dir,omitempty"`

	Step string `json:"step,omitempty"`
	// options is optional, but only allowed with step
	// Also, we'll need to do some magic to do type verification during translation - i.e., this step wants a number
	// for this option, so translate the string value for that option to a number.
	Options map[string]string `json:"options,omitempty"`

	Loop *Loop `json:"loop,omitempty"`

	// agent can be overridden on a step
	Agent *Agent `json:"agent,omitempty"`

	// Image alows the docker image for a step to be specified
	Image string `json:"image,omitempty"`

	// env allows defining per-step environment variables
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Legacy fields from jenkinsfile.PipelineStep before it was eliminated.
	Comment   string  `json:"comment,omitempty"`
	Groovy    string  `json:"groovy,omitempty"`
	Steps     []*Step `json:"steps,omitempty"`
	When      string  `json:"when,omitempty"`
	Container string  `json:"container,omitempty"`
	Sh        string  `json:"sh,omitempty"`
}

// Loop is a special step that defines a variable, a list of possible values for that variable, and a set of steps to
// repeat for each value for the variable, with the variable set with that value in the environment for the execution of
// those steps.
type Loop struct {
	// The variable name.
	Variable string `json:"variable"`
	// The list of values to iterate over
	Values []string `json:"values"`
	// The steps to run
	Steps []Step `json:"steps"`
}

// Stage is a unit of work in a pipeline, corresponding either to a Task or a set of Tasks to be run sequentially or in
// parallel with common configuration.
type Stage struct {
	Name       string          `json:"name"`
	Agent      *Agent          `json:"agent,omitempty"`
	Env        []corev1.EnvVar `json:"env,omitempty"`
	Options    *StageOptions   `json:"options,omitempty"`
	Steps      []Step          `json:"steps,omitempty"`
	Stages     []Stage         `json:"stages,omitempty"`
	Parallel   []Stage         `json:"parallel,omitempty"`
	Post       []Post          `json:"post,omitempty"`
	WorkingDir *string         `json:"dir,omitempty"`

	// Replaced by Env, retained for backwards compatibility
	Environment []corev1.EnvVar `json:"environment,omitempty"`
}

// PostCondition is used to specify under what condition a post action should be executed.
type PostCondition string

// Probably extensible down the road
const (
	PostConditionSuccess PostCondition = "success"
	PostConditionFailure PostCondition = "failure"
	PostConditionAlways  PostCondition = "always"
)

// Post contains a PostCondition and one more actions to be executed after a pipeline or stage if the condition is met.
type Post struct {
	// TODO: Conditional execution of something after a Task or Pipeline completes is not yet implemented
	Condition PostCondition `json:"condition"`
	Actions   []PostAction  `json:"actions"`
}

// PostAction contains the name of a built-in post action and options to pass to that action.
type PostAction struct {
	// TODO: Notifications are not yet supported in Build Pipeline per se.
	Name string `json:"name"`
	// Also, we'll need to do some magic to do type verification during translation - i.e., this action wants a number
	// for this option, so translate the string value for that option to a number.
	Options map[string]string `json:"options,omitempty"`
}

// StepOverrideType is used to specify whether the existing step should be replaced (default), new step(s) should be
// prepended before the existing step, or new step(s) should be appended after the existing step.
type StepOverrideType string

// The available override types
const (
	StepOverrideReplace StepOverrideType = "replace"
	StepOverrideBefore  StepOverrideType = "before"
	StepOverrideAfter   StepOverrideType = "after"
)

// PipelineOverride allows for overriding named steps, stages, or pipelines in the build pack or default pipeline
type PipelineOverride struct {
	Pipeline         string            `json:"pipeline,omitempty"`
	Stage            string            `json:"stage,omitempty"`
	Name             string            `json:"name,omitempty"`
	Step             *Step             `json:"step,omitempty"`
	Steps            []*Step           `json:"steps,omitempty"`
	Type             *StepOverrideType `json:"type,omitempty"`
	Agent            *Agent            `json:"agent,omitempty"`
	ContainerOptions *corev1.Container `json:"containerOptions,omitempty"`
	Volumes          []*corev1.Volume  `json:"volumes,omitempty"`
}

var _ apis.Validatable = (*ParsedPipeline)(nil)

func (s *Stage) taskName() string {
	return strings.ToLower(strings.NewReplacer(" ", "-").Replace(s.Name))
}

// stageLabelName replaces invalid characters in stage names for label usage.
func (s *Stage) stageLabelName() string {
	return MangleToRfc1035Label(s.Name, "")
}

// GroovyBlock returns the groovy expression for this step
// Legacy code for Jenkinsfile generation
func (s *Step) GroovyBlock(parentIndent string) string {
	var buffer bytes.Buffer
	indent := parentIndent
	if s.Comment != "" {
		buffer.WriteString(indent)
		buffer.WriteString("// ")
		buffer.WriteString(s.Comment)
		buffer.WriteString("\n")
	}
	if s.GetImage() != "" {
		buffer.WriteString(indent)
		buffer.WriteString("container('")
		buffer.WriteString(s.GetImage())
		buffer.WriteString("') {\n")
	} else if s.Dir != "" {
		buffer.WriteString(indent)
		buffer.WriteString("dir('")
		buffer.WriteString(s.Dir)
		buffer.WriteString("') {\n")
	} else if s.GetFullCommand() != "" {
		buffer.WriteString(indent)
		buffer.WriteString("sh \"")
		buffer.WriteString(s.GetFullCommand())
		buffer.WriteString("\"\n")
	} else if s.Groovy != "" {
		lines := strings.Split(s.Groovy, "\n")
		lastIdx := len(lines) - 1
		for i, line := range lines {
			buffer.WriteString(indent)
			buffer.WriteString(line)
			if i >= lastIdx && len(s.Steps) > 0 {
				buffer.WriteString(" {")
			}
			buffer.WriteString("\n")
		}
	}
	childIndent := indent + "  "
	for _, child := range s.Steps {
		buffer.WriteString(child.GroovyBlock(childIndent))
	}
	return buffer.String()
}

// ToJenkinsfileStatements converts the step to one or more jenkinsfile statements
// Legacy code for Jenkinsfile generation
func (s *Step) ToJenkinsfileStatements() []*util.Statement {
	statements := []*util.Statement{}
	if s.Comment != "" {
		statements = append(statements, &util.Statement{
			Statement: "",
		}, &util.Statement{
			Statement: "// " + s.Comment,
		})
	}
	if s.GetImage() != "" {
		statements = append(statements, &util.Statement{
			Function:  "container",
			Arguments: []string{s.GetImage()},
		})
	} else if s.Dir != "" {
		statements = append(statements, &util.Statement{
			Function:  "dir",
			Arguments: []string{s.Dir},
		})
	} else if s.GetFullCommand() != "" {
		statements = append(statements, &util.Statement{
			Statement: "sh \"" + s.GetFullCommand() + "\"",
		})
	} else if s.Groovy != "" {
		lines := strings.Split(s.Groovy, "\n")
		for _, line := range lines {
			statements = append(statements, &util.Statement{
				Statement: line,
			})
		}
	}
	if len(statements) > 0 {
		last := statements[len(statements)-1]
		for _, c := range s.Steps {
			last.Children = append(last.Children, c.ToJenkinsfileStatements()...)
		}
	}
	return statements
}

// Validate validates the step is populated correctly
// Legacy code for Jenkinsfile generation
func (s *Step) Validate() error {
	if len(s.Steps) > 0 || s.GetCommand() != "" {
		return nil
	}
	return fmt.Errorf("invalid step %#v as no child steps or command", s)
}

// PutAllEnvVars puts all the defined environment variables in the given map
// Legacy code for Jenkinsfile generation
func (s *Step) PutAllEnvVars(m map[string]string) {
	for _, step := range s.Steps {
		step.PutAllEnvVars(m)
	}
}

// GetCommand gets the step's command to execute, opting for Command if set, then Sh.
func (s *Step) GetCommand() string {
	if s.Command != "" {
		return s.Command
	}

	return s.Sh
}

// GetFullCommand gets the full command to execute, including arguments.
func (s *Step) GetFullCommand() string {
	cmd := s.GetCommand()

	// If GetCommand() was an empty string, don't deal with arguments, just return.
	if len(s.Arguments) > 0 && cmd != "" {
		cmd = fmt.Sprintf("%s %s", cmd, strings.Join(s.Arguments, " "))
	}

	return cmd
}

// GetImage gets the step's image to run on, opting for Image if set, then Container.
func (s *Step) GetImage() string {
	if s.Image != "" {
		return s.Image
	}
	if s.Agent != nil && s.Agent.Image != "" {
		return s.Agent.Image
	}

	return s.Container
}

// DeepCopyForParsedPipeline returns a copy of the Agent with deprecated fields migrated to current ones.
func (a *Agent) DeepCopyForParsedPipeline() *Agent {
	agent := a.DeepCopy()
	if agent.Container != "" {
		agent.Image = agent.GetImage()
		agent.Container = ""
		agent.Label = ""
	}

	return agent
}

// Groovy returns the agent groovy expression for the agent or `any` if its blank
// Legacy code for Jenkinsfile generation
func (a *Agent) Groovy() string {
	if a.Label != "" {
		return fmt.Sprintf(`{
    label "%s"
  }`, a.Label)
	}
	// lets use any for Prow
	return "any"
}

// GetImage gets the agent's image to run on, opting for Image if set, then Container.
func (a *Agent) GetImage() string {
	if a.Image != "" {
		return a.Image
	}

	return a.Container
}

// MangleToRfc1035Label - Task/Step names need to be RFC 1035/1123 compliant DNS labels, so we mangle
// them to make them compliant. Results should match the following regex and be
// no more than 63 characters long:
//     [a-z]([-a-z0-9]*[a-z0-9])?
// cf. https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
// body is assumed to have at least one ASCII letter.
// suffix is assumed to be alphanumeric and non-empty.
// TODO: Combine with kube.ToValidName (that function needs to handle lengths)
func MangleToRfc1035Label(body string, suffix string) string {
	const maxLabelLength = 63
	maxBodyLength := maxLabelLength
	if len(suffix) > 0 {
		maxBodyLength = maxLabelLength - len(suffix) - 1 // Add an extra hyphen before the suffix
	}
	var sb strings.Builder
	bufferedHyphen := false // Used to make sure we don't output consecutive hyphens.
	for _, codepoint := range body {
		toWrite := 0
		if sb.Len() != 0 { // Digits and hyphens aren't allowed to be the first character
			if codepoint == ' ' || codepoint == '-' || codepoint == '.' {
				bufferedHyphen = true
			} else if codepoint >= '0' && codepoint <= '9' {
				toWrite = 1
			}
		}

		if codepoint >= 'A' && codepoint <= 'Z' {
			codepoint += ('a' - 'A') // Offset to make character lowercase
			toWrite = 1
		} else if codepoint >= 'a' && codepoint <= 'z' {
			toWrite = 1
		}

		if toWrite > 0 {
			if bufferedHyphen {
				toWrite++
			}
			if sb.Len()+toWrite > maxBodyLength {
				break
			}
			if bufferedHyphen {
				sb.WriteRune('-')
				bufferedHyphen = false
			}
			sb.WriteRune(codepoint)
		}
	}

	if suffix != "" {
		sb.WriteRune('-')
		sb.WriteString(suffix)
	}
	return sb.String()
}

// GetEnv gets the environment for the ParsedPipeline, returning Env first and Environment if Env isn't populated.
func (j *ParsedPipeline) GetEnv() []corev1.EnvVar {
	if j != nil {
		if len(j.Env) > 0 {
			return j.Env
		}

		return j.Environment
	}
	return []corev1.EnvVar{}
}

// GetEnv gets the environment for the Stage, returning Env first and Environment if Env isn't populated.
func (s *Stage) GetEnv() []corev1.EnvVar {
	if len(s.Env) > 0 {
		return s.Env
	}

	return s.Environment
}

// Validate checks the ParsedPipeline to find any errors in it, without validating against the cluster.
func (j *ParsedPipeline) Validate(context context.Context) *apis.FieldError {
	return j.ValidateInCluster(context, nil, "")
}

// ValidateInCluster checks the parsed ParsedPipeline to find any errors in it, including validation against the cluster.
func (j *ParsedPipeline) ValidateInCluster(context context.Context, kubeClient kubernetes.Interface, ns string) *apis.FieldError {
	if err := validateAgent(j.Agent).ViaField("agent"); err != nil {
		return err
	}

	var volumes []*corev1.Volume
	if j.Options != nil && len(j.Options.Volumes) > 0 {
		volumes = append(volumes, j.Options.Volumes...)
	}
	if err := validateStages(j.Stages, j.Agent, volumes, kubeClient, ns); err != nil {
		return err
	}

	if err := validateStageNames(j); err != nil {
		return err
	}

	if err := validateRootOptions(j.Options, volumes, kubeClient, ns).ViaField("options"); err != nil {
		return err
	}

	return nil
}

func validateAgent(a *Agent) *apis.FieldError {
	// TODO: This is the same whether you specify an agent without label or image, or if you don't specify an agent
	// at all, which is nonoptimal.
	if a != nil {
		if a.Container != "" {
			return &apis.FieldError{
				Message: "the container field is deprecated - please use image instead",
				Paths:   []string{"container"},
			}
		}
		if a.Dir != "" {
			return &apis.FieldError{
				Message: "the dir field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
				Paths:   []string{"dir"},
			}
		}

		if a.Image != "" && a.Label != "" {
			return apis.ErrMultipleOneOf("label", "image")
		}

		if a.Image == "" && a.Label == "" {
			return apis.ErrMissingOneOf("label", "image")
		}
	}

	return nil
}

var containsASCIILetter = regexp.MustCompile(`[a-zA-Z]`).MatchString

func validateStage(s Stage, parentAgent *Agent, parentVolumes []*corev1.Volume, kubeClient kubernetes.Interface, ns string) *apis.FieldError {
	if len(s.Steps) == 0 && len(s.Stages) == 0 && len(s.Parallel) == 0 {
		return apis.ErrMissingOneOf("steps", "stages", "parallel")
	}

	if !containsASCIILetter(s.Name) {
		return &apis.FieldError{
			Message: "Stage name must contain at least one ASCII letter",
			Paths:   []string{"name"},
		}
	}

	var volumes []*corev1.Volume

	volumes = append(volumes, parentVolumes...)
	if s.Options != nil && s.Options.RootOptions != nil && len(s.Options.Volumes) > 0 {
		volumes = append(volumes, s.Options.Volumes...)
	}

	stageAgent := s.Agent.DeepCopy()
	if stageAgent == nil {
		stageAgent = parentAgent.DeepCopy()
	}

	if stageAgent == nil {
		return &apis.FieldError{
			Message: "No agent specified for stage or for its parent(s)",
			Paths:   []string{"agent"},
		}
	}

	if len(s.Steps) > 0 {
		if len(s.Stages) > 0 || len(s.Parallel) > 0 {
			return apis.ErrMultipleOneOf("steps", "stages", "parallel")
		}
		seenStepNames := make(map[string]int)
		for i, step := range s.Steps {
			if err := validateStep(step).ViaFieldIndex("steps", i); err != nil {
				return err
			}
			if step.Name != "" {
				if count, exists := seenStepNames[step.Name]; exists {
					seenStepNames[step.Name] = count + 1
				} else {
					seenStepNames[step.Name] = 1
				}
			}
		}

		var duplicateSteps []string
		for k, v := range seenStepNames {
			if v > 1 {
				duplicateSteps = append(duplicateSteps, k)
			}
		}
		if len(duplicateSteps) > 0 {
			sort.Strings(duplicateSteps)
			return &apis.FieldError{
				Message: "step names within a stage must be unique",
				Details: fmt.Sprintf("The following step names in the stage %s are used more than once: %s", s.Name, strings.Join(duplicateSteps, ", ")),
				Paths:   []string{"steps"},
			}
		}
	}

	if len(s.Stages) > 0 {
		if len(s.Parallel) > 0 {
			return apis.ErrMultipleOneOf("steps", "stages", "parallel")
		}
		for i, stage := range s.Stages {
			if err := validateStage(stage, parentAgent, volumes, kubeClient, ns).ViaFieldIndex("stages", i); err != nil {
				return err
			}
		}
	}

	if len(s.Parallel) > 0 {
		for i, stage := range s.Parallel {
			if err := validateStage(stage, parentAgent, volumes, kubeClient, ns).ViaFieldIndex("parallel", i); err != nil {
				return nil
			}
		}
	}

	return validateStageOptions(s.Options, volumes, kubeClient, ns).ViaField("options")
}

func moreThanOneAreTrue(vals ...bool) bool {
	count := 0

	for _, v := range vals {
		if v {
			count++
		}
	}

	return count > 1
}

func validateStep(s Step) *apis.FieldError {
	// Special cases for when you use legacy build pack syntax inside a pipeline definition
	if s.Container != "" {
		return &apis.FieldError{
			Message: "the container field is deprecated - please use image instead",
			Paths:   []string{"container"},
		}
	}
	if s.Groovy != "" {
		return &apis.FieldError{
			Message: "the groovy field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
			Paths:   []string{"groovy"},
		}
	}
	if s.Comment != "" {
		return &apis.FieldError{
			Message: "the comment field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
			Paths:   []string{"comment"},
		}
	}
	if s.When != "" {
		return &apis.FieldError{
			Message: "the when field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
			Paths:   []string{"when"},
		}
	}
	if len(s.Steps) > 0 {
		return &apis.FieldError{
			Message: "the steps field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it and list the nested stages sequentially instead.",
			Paths:   []string{"steps"},
		}
	}

	if s.GetCommand() == "" && s.Step == "" && s.Loop == nil {
		return apis.ErrMissingOneOf("command", "step", "loop")
	}

	if moreThanOneAreTrue(s.GetCommand() != "", s.Step != "", s.Loop != nil) {
		return apis.ErrMultipleOneOf("command", "step", "loop")
	}

	if (s.GetCommand() != "" || s.Loop != nil) && len(s.Options) != 0 {
		return &apis.FieldError{
			Message: "Cannot set options for a command or a loop",
			Paths:   []string{"options"},
		}
	}

	if (s.Step != "" || s.Loop != nil) && len(s.Arguments) != 0 {
		return &apis.FieldError{
			Message: "Cannot set command-line arguments for a step or a loop",
			Paths:   []string{"args"},
		}
	}

	if err := validateLoop(s.Loop); err != nil {
		return err.ViaField("loop")
	}

	if s.Agent != nil {
		return validateAgent(s.Agent).ViaField("agent")
	}
	return nil
}

func validateLoop(l *Loop) *apis.FieldError {
	if l != nil {
		if l.Variable == "" {
			return apis.ErrMissingField("variable")
		}

		if len(l.Steps) == 0 {
			return apis.ErrMissingField("steps")
		}

		if len(l.Values) == 0 {
			return apis.ErrMissingField("values")
		}

		for i, step := range l.Steps {
			if err := validateStep(step).ViaFieldIndex("steps", i); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateStages(stages []Stage, parentAgent *Agent, parentVolumes []*corev1.Volume, kubeClient kubernetes.Interface, ns string) *apis.FieldError {
	if len(stages) == 0 {
		return apis.ErrMissingField("stages")
	}

	for i, s := range stages {
		if err := validateStage(s, parentAgent, parentVolumes, kubeClient, ns).ViaFieldIndex("stages", i); err != nil {
			return err
		}
	}

	return nil
}

func validateRootOptions(o *RootOptions, volumes []*corev1.Volume, kubeClient kubernetes.Interface, ns string) *apis.FieldError {
	if o != nil {
		if o.Timeout != nil {
			if err := validateTimeout(o.Timeout); err != nil {
				return err.ViaField("timeout")
			}
		}

		// TODO: retry will default to 0, so we're kinda stuck checking if it's less than zero here.
		if o.Retry < 0 {
			return &apis.FieldError{
				Message: "Retry count cannot be negative",
				Paths:   []string{"retry"},
			}
		}

		for i, v := range o.Volumes {
			if err := validateVolume(v, kubeClient, ns).ViaFieldIndex("volumes", i); err != nil {
				return err
			}
		}

		return validateContainerOptions(o.ContainerOptions, volumes).ViaField("containerOptions")
	}

	return nil
}

func validateVolume(v *corev1.Volume, kubeClient kubernetes.Interface, ns string) *apis.FieldError {
	if v != nil {
		if v.Name == "" {
			return apis.ErrMissingField("name")
		}
		if kubeClient != nil {
			if v.Secret != nil {
				_, err := kubeClient.CoreV1().Secrets(ns).Get(v.Secret.SecretName, metav1.GetOptions{})
				if err != nil {
					return &apis.FieldError{
						Message: fmt.Sprintf("Secret %s does not exist, so cannot be used as a volume", v.Secret.SecretName),
						Paths:   []string{"secretName"},
					}
				}
			} else if v.PersistentVolumeClaim != nil {
				_, err := kubeClient.CoreV1().PersistentVolumeClaims(ns).Get(v.PersistentVolumeClaim.ClaimName, metav1.GetOptions{})
				if err != nil {
					return &apis.FieldError{
						Message: fmt.Sprintf("PVC %s does not exist, so cannot be used as a volume", v.PersistentVolumeClaim.ClaimName),
						Paths:   []string{"claimName"},
					}
				}
			}
		}
	}

	return nil
}

func validateContainerOptions(c *corev1.Container, volumes []*corev1.Volume) *apis.FieldError {
	if c != nil {
		if len(c.Command) != 0 {
			return &apis.FieldError{
				Message: "Command cannot be specified in containerOptions",
				Paths:   []string{"command"},
			}
		}
		if len(c.Args) != 0 {
			return &apis.FieldError{
				Message: "Arguments cannot be specified in containerOptions",
				Paths:   []string{"args"},
			}
		}
		if c.Image != "" {
			return &apis.FieldError{
				Message: "Image cannot be specified in containerOptions",
				Paths:   []string{"image"},
			}
		}
		if c.WorkingDir != "" {
			return &apis.FieldError{
				Message: "WorkingDir cannot be specified in containerOptions",
				Paths:   []string{"workingDir"},
			}
		}
		if c.Name != "" {
			return &apis.FieldError{
				Message: "Name cannot be specified in containerOptions",
				Paths:   []string{"name"},
			}
		}
		if c.Stdin {
			return &apis.FieldError{
				Message: "Stdin cannot be specified in containerOptions",
				Paths:   []string{"stdin"},
			}
		}
		if c.TTY {
			return &apis.FieldError{
				Message: "TTY cannot be specified in containerOptions",
				Paths:   []string{"tty"},
			}
		}
		if len(c.VolumeMounts) > 0 {
			for i, m := range c.VolumeMounts {
				if !isVolumeMountValid(m, volumes) {
					fieldErr := &apis.FieldError{
						Message: fmt.Sprintf("Volume mount name %s not found in volumes for stage or pipeline", m.Name),
						Paths:   []string{"name"},
					}

					return fieldErr.ViaFieldIndex("volumeMounts", i)
				}
			}
		}
	}

	return nil
}

func isVolumeMountValid(mount corev1.VolumeMount, volumes []*corev1.Volume) bool {
	foundVolume := false

	for _, v := range volumes {
		if v.Name == mount.Name {
			foundVolume = true
			break
		}
	}

	return foundVolume
}

func validateStageOptions(o *StageOptions, volumes []*corev1.Volume, kubeClient kubernetes.Interface, ns string) *apis.FieldError {
	if o != nil {
		if err := validateStash(o.Stash); err != nil {
			return err.ViaField("stash")
		}

		if o.Unstash != nil {
			if err := validateUnstash(o.Unstash); err != nil {
				return err.ViaField("unstash")
			}
		}

		if o.Workspace != nil {
			if err := validateWorkspace(*o.Workspace); err != nil {
				return err
			}
		}

		if o.RootOptions != nil && o.RootOptions.DistributeParallelAcrossNodes {
			return &apis.FieldError{
				Message: "distributeParallelAcrossNodes cannot be used in a stage",
				Paths:   []string{"distributeParallelAcrossNodes"},
			}
		}

		return validateRootOptions(o.RootOptions, volumes, kubeClient, ns)
	}

	return nil
}

func validateTimeout(t *Timeout) *apis.FieldError {
	if t != nil {
		isAllowed := false
		for _, allowed := range allTimeoutUnits {
			if t.Unit == allowed {
				isAllowed = true
			}
		}

		if !isAllowed {
			return &apis.FieldError{
				Message: fmt.Sprintf("%s is not a valid time unit. Valid time units are %s", string(t.Unit),
					strings.Join(allTimeoutUnitsAsStrings(), ", ")),
				Paths: []string{"unit"},
			}
		}

		if t.Time < 1 {
			return &apis.FieldError{
				Message: "Timeout must be greater than zero",
				Paths:   []string{"time"},
			}
		}
	}

	return nil
}

func validateUnstash(u *Unstash) *apis.FieldError {
	if u != nil {
		// TODO: Check to make sure the corresponding stash is defined somewhere
		if u.Name == "" {
			return &apis.FieldError{
				Message: "The unstash name must be provided",
				Paths:   []string{"name"},
			}
		}
	}

	return nil
}

func validateStash(s *Stash) *apis.FieldError {
	if s != nil {
		if s.Name == "" {
			return &apis.FieldError{
				Message: "The stash name must be provided",
				Paths:   []string{"name"},
			}
		}
		if s.Files == "" {
			return &apis.FieldError{
				Message: "files to stash must be provided",
				Paths:   []string{"files"},
			}
		}
	}

	return nil
}

func validateWorkspace(w string) *apis.FieldError {
	if w == "" {
		return &apis.FieldError{
			Message: "The workspace name must be unspecified or non-empty",
			Paths:   []string{"workspace"},
		}
	}

	return nil
}

// EnvMapToSlice transforms a map of environment variables into a slice that can be used in container configuration
func EnvMapToSlice(envMap map[string]corev1.EnvVar) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(envMap))

	// Avoid nondeterministic results by sorting the keys and appending vars in that order.
	var envVars []string
	for k := range envMap {
		envVars = append(envVars, k)
	}
	sort.Strings(envVars)

	for _, envVar := range envVars {
		env = append(env, envMap[envVar])
	}

	return env
}

// GetPodLabels returns the optional additional labels to apply to all pods for this pipeline. The labels and their values
// will be converted to RFC1035-compliant strings.
func (j *ParsedPipeline) GetPodLabels() map[string]string {
	sanitizedLabels := make(map[string]string)
	if j.Options != nil {
		for k, v := range j.Options.PodLabels {
			sanitizedKey := MangleToRfc1035Label(k, "")
			sanitizedValue := MangleToRfc1035Label(v, "")
			if sanitizedKey != k || sanitizedValue != v {
				log.Logger().Infof("Converted custom label/value '%s' to '%s' to conform to Kubernetes label requirements",
					util.ColorInfo(k+"="+v), util.ColorInfo(sanitizedKey+"="+sanitizedValue))
			}
			sanitizedLabels[sanitizedKey] = sanitizedValue
		}
	}
	return sanitizedLabels
}

// GetTolerations returns the tolerations configured in the root options for this pipeline, if any.
func (j *ParsedPipeline) GetTolerations() []corev1.Toleration {
	if j.Options != nil {
		return j.Options.Tolerations
	}
	return nil
}

// GetPossibleAffinityPolicy takes the pipeline name and returns the appropriate affinity policy for pods in this
// pipeline given its configuration, specifically of options.distributeParallelAcrossNodes.
func (j *ParsedPipeline) GetPossibleAffinityPolicy(name string) *corev1.Affinity {
	if j.Options != nil && j.Options.DistributeParallelAcrossNodes {

		antiAffinityLabels := make(map[string]string)
		if len(j.Options.PodLabels) > 0 {
			antiAffinityLabels = util.MergeMaps(j.GetPodLabels())
		} else {
			antiAffinityLabels[pipeline.GroupName+pipeline.PipelineRunLabelKey] = name
		}
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: antiAffinityLabels,
					},
					TopologyKey: "kubernetes.io/hostname",
				}},
			},
		}
	}
	return nil
}

// StepPlaceholderReplacementArgs specifies the arguments required for replacing placeholders in build pack directories.
type StepPlaceholderReplacementArgs struct {
	WorkspaceDir      string
	GitName           string
	GitOrg            string
	GitHost           string
	DockerRegistry    string
	DockerRegistryOrg string
	ProjectID         string
	KanikoImage       string
	UseKaniko         bool
}

func (p *StepPlaceholderReplacementArgs) workingDirAsPointer() *string {
	// TODO: Is there a better way to ensure that we're creating a pointer of a copy of the string?
	copyOfWorkingDir := p.WorkspaceDir
	return &copyOfWorkingDir
}

// ReplacePlaceholdersInStepAndStageDirs traverses this pipeline's stages and any nested stages for any steps (and any nested steps)
// within the stages, and replaces "REPLACE_ME_..." placeholders in those steps' directories.
func (j *ParsedPipeline) ReplacePlaceholdersInStepAndStageDirs(args StepPlaceholderReplacementArgs) {
	var stages []Stage
	for _, s := range j.Stages {
		s.replacePlaceholdersInStage(j.WorkingDir, args)
		stages = append(stages, s)
	}
	j.Stages = stages
}

func (s *Stage) replacePlaceholdersInStage(parentDir *string, args StepPlaceholderReplacementArgs) {
	var steps []Step
	var stages []Stage
	var parallel []Stage
	// If there's no working directory and this stage contains steps, we should set a stage directory
	if s.WorkingDir == nil && len(s.Steps) > 0 {
		// If there's no parent working directory, use the default provided.
		if parentDir == nil {
			s.WorkingDir = args.workingDirAsPointer()
		} else {
			s.WorkingDir = parentDir
		}
	}
	s.WorkingDir = replacePlaceholdersInDir(s.WorkingDir, args)
	for _, step := range s.Steps {
		step.replacePlaceholdersInStep(args)
		steps = append(steps, step)
	}
	for _, nested := range s.Stages {
		nested.replacePlaceholdersInStage(s.WorkingDir, args)
		stages = append(stages, nested)
	}
	for _, p := range s.Parallel {
		p.replacePlaceholdersInStage(s.WorkingDir, args)
		parallel = append(parallel, p)
	}
	s.Steps = steps
	s.Stages = stages
	s.Parallel = parallel
}

func replacePlaceholdersInDir(originalDir *string, args StepPlaceholderReplacementArgs) *string {
	if originalDir == nil || *originalDir == "" {
		return originalDir
	}
	dir := *originalDir
	// Replace the Go buildpack path with the correct location for Tekton builds.
	dir = strings.Replace(dir, "/home/jenkins/go/src/REPLACE_ME_GIT_PROVIDER/REPLACE_ME_ORG/REPLACE_ME_APP_NAME", args.WorkspaceDir, -1)

	dir = strings.Replace(dir, util.PlaceHolderAppName, strings.ToLower(args.GitName), -1)
	dir = strings.Replace(dir, util.PlaceHolderOrg, strings.ToLower(args.GitOrg), -1)
	dir = strings.Replace(dir, util.PlaceHolderGitProvider, strings.ToLower(args.GitHost), -1)
	dir = strings.Replace(dir, util.PlaceHolderDockerRegistryOrg, strings.ToLower(args.DockerRegistryOrg), -1)

	if strings.HasPrefix(dir, "./") {
		dir = args.WorkspaceDir + strings.TrimPrefix(dir, ".")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(args.WorkspaceDir, dir)
	}
	return &dir
}

func (s *Step) replacePlaceholdersInStep(args StepPlaceholderReplacementArgs) {
	if s.GetCommand() != "" {
		s.modifyStep(args)
		s.Dir = *replacePlaceholdersInDir(&s.Dir, args)
	}
	var steps []*Step
	for _, nested := range s.Steps {
		nested.replacePlaceholdersInStep(args)
		steps = append(steps, nested)
	}
	s.Steps = steps
	if s.Loop != nil {
		var loopSteps []Step
		for _, nested := range s.Loop.Steps {
			nested.replacePlaceholdersInStep(args)
			loopSteps = append(loopSteps, nested)
		}
		s.Loop.Steps = loopSteps
	}
}

// modifyStep allows a container step to be modified to do something different
func (s *Step) modifyStep(params StepPlaceholderReplacementArgs) {
	if params.UseKaniko {
		if strings.HasPrefix(s.GetCommand(), "skaffold build") ||
			(len(s.Arguments) > 0 && strings.HasPrefix(strings.Join(s.Arguments[1:], " "), "skaffold build")) ||
			commandIsSkaffoldRegex.MatchString(s.GetCommand()) {

			sourceDir := params.WorkspaceDir
			dockerfile := filepath.Join(sourceDir, "Dockerfile")
			localRepo := params.DockerRegistry
			destination := params.DockerRegistry + "/" + params.DockerRegistryOrg + "/" + naming.ToValidName(params.GitName)

			args := []string{"--cache=true", "--cache-dir=/workspace",
				"--context=" + sourceDir,
				"--dockerfile=" + dockerfile,
				"--destination=" + destination + ":${inputs.params.version}",
				"--cache-repo=" + localRepo + "/" + params.ProjectID + "/cache",
			}
			if localRepo != "gcr.io" {
				args = append(args, "--skip-tls-verify-registry="+localRepo)
			}

			if ipAddressRegistryRegex.MatchString(localRepo) {
				args = append(args, "--insecure")
			}

			s.Command = "/kaniko/executor"
			s.Arguments = args

			s.Image = params.KanikoImage
		}
	}
}

// AddContainerEnvVarsToPipeline allows for adding a slice of container environment variables directly to the
// pipeline, if they're not already defined.
func (j *ParsedPipeline) AddContainerEnvVarsToPipeline(origEnv []corev1.EnvVar) {
	if len(origEnv) > 0 {
		envMap := make(map[string]corev1.EnvVar)

		// Add the container env vars first.
		for _, e := range origEnv {
			if e.ValueFrom == nil {
				envMap[e.Name] = corev1.EnvVar{
					Name:  e.Name,
					Value: e.Value,
				}
			}
		}

		// Overwrite with the existing pipeline environment, if it exists
		for _, e := range j.GetEnv() {
			envMap[e.Name] = e
		}

		env := make([]corev1.EnvVar, 0, len(envMap))

		// Avoid nondeterministic results by sorting the keys and appending vars in that order.
		var envVars []string
		for k := range envMap {
			envVars = append(envVars, k)
		}
		sort.Strings(envVars)

		for _, envVar := range envVars {
			env = append(env, envMap[envVar])
		}

		j.Env = env
	}
}

func scopedEnv(newEnv []corev1.EnvVar, parentEnv []corev1.EnvVar) []corev1.EnvVar {
	if len(parentEnv) == 0 && len(newEnv) == 0 {
		return nil
	}
	return CombineEnv(newEnv, parentEnv)
}

// CombineEnv combines the two environments into a single unified slice where
// the `newEnv` overrides anything in the `parentEnv`
func CombineEnv(newEnv []corev1.EnvVar, parentEnv []corev1.EnvVar) []corev1.EnvVar {
	envMap := make(map[string]corev1.EnvVar)

	for _, e := range parentEnv {
		envMap[e.Name] = e
	}

	for _, e := range newEnv {
		envMap[e.Name] = e
	}

	return EnvMapToSlice(envMap)
}

type transformedStage struct {
	Stage Stage
	// Only one of Sequential, Parallel, and Task is non-empty
	Sequential []*transformedStage
	Parallel   []*transformedStage
	Task       *tektonv1alpha1.Task
	// PipelineTask is non-empty only if Task is non-empty, but it is populated
	// after Task is populated so the reverse is not true.
	PipelineTask *tektonv1alpha1.PipelineTask
	// The depth of this stage in the full tree of stages
	Depth int8
	// The parallel or sequntial stage enclosing this stage, or nil if this stage is at top level
	EnclosingStage *transformedStage
	// The stage immediately before this stage at the same depth, or nil if there is no such stage
	PreviousSiblingStage *transformedStage
	// TODO: Add the equivalent reverse relationship
}

func (ts transformedStage) toPipelineStructureStage() v1.PipelineStructureStage {
	s := v1.PipelineStructureStage{
		Name:  ts.Stage.Name,
		Depth: ts.Depth,
	}

	if ts.EnclosingStage != nil {
		s.Parent = &ts.EnclosingStage.Stage.Name
	}

	if ts.PreviousSiblingStage != nil {
		s.Previous = &ts.PreviousSiblingStage.Stage.Name
	}
	// TODO: Add the equivalent reverse relationship

	if ts.PipelineTask != nil {
		s.TaskRef = &ts.PipelineTask.TaskRef.Name
	}

	if len(ts.Parallel) > 0 {
		for _, n := range ts.Parallel {
			s.Parallel = append(s.Parallel, n.Stage.Name)
		}
	}

	if len(ts.Sequential) > 0 {
		for _, n := range ts.Sequential {
			s.Stages = append(s.Stages, n.Stage.Name)
		}
	}

	return s
}

func (ts transformedStage) getAllAsPipelineStructureStages() []v1.PipelineStructureStage {
	var stages []v1.PipelineStructureStage

	stages = append(stages, ts.toPipelineStructureStage())

	if len(ts.Parallel) > 0 {
		for _, n := range ts.Parallel {
			stages = append(stages, n.getAllAsPipelineStructureStages()...)
		}
	}

	if len(ts.Sequential) > 0 {
		for _, n := range ts.Sequential {
			stages = append(stages, n.getAllAsPipelineStructureStages()...)
		}
	}

	return stages
}

func (ts transformedStage) isSequential() bool {
	return len(ts.Sequential) > 0
}

func (ts transformedStage) isParallel() bool {
	return len(ts.Parallel) > 0
}

func (ts transformedStage) getLinearTasks() []*tektonv1alpha1.Task {
	if ts.isSequential() {
		var tasks []*tektonv1alpha1.Task
		for _, seqTs := range ts.Sequential {
			tasks = append(tasks, seqTs.getLinearTasks()...)
		}
		return tasks
	} else if ts.isParallel() {
		var tasks []*tektonv1alpha1.Task
		for _, parTs := range ts.Parallel {
			tasks = append(tasks, parTs.getLinearTasks()...)
		}
		return tasks
	} else {
		return []*tektonv1alpha1.Task{ts.Task}
	}
}

// If the workspace is nil, sets it to the parent's workspace
func (ts *transformedStage) computeWorkspace(parentWorkspace string) {
	if ts.Stage.Options == nil {
		ts.Stage.Options = &StageOptions{
			RootOptions: &RootOptions{},
		}
	}
	if ts.Stage.Options.Workspace == nil {
		ts.Stage.Options.Workspace = &parentWorkspace
	}
}

type stageToTaskParams struct {
	parentParams         CRDsFromPipelineParams
	stage                Stage
	baseWorkingDir       *string
	parentEnv            []corev1.EnvVar
	parentAgent          *Agent
	parentWorkspace      string
	parentContainer      *corev1.Container
	parentVolumes        []*corev1.Volume
	depth                int8
	enclosingStage       *transformedStage
	previousSiblingStage *transformedStage
}

func stageToTask(params stageToTaskParams) (*transformedStage, error) {
	if len(params.stage.Post) != 0 {
		return nil, errors.New("post on stages not yet supported")
	}

	stageContainer := &corev1.Container{}
	var stageVolumes []*corev1.Volume

	if params.stage.Options != nil {
		o := params.stage.Options
		if o.RootOptions == nil {
			o.RootOptions = &RootOptions{}
		} else {
			if o.Timeout != nil {
				return nil, errors.New("Timeout on stage not yet supported")
			}
			if o.ContainerOptions != nil {
				stageContainer = o.ContainerOptions
			}
			stageVolumes = o.Volumes
		}
		if o.Stash != nil {
			return nil, errors.New("Stash on stage not yet supported")
		}
		if o.Unstash != nil {
			return nil, errors.New("Unstash on stage not yet supported")
		}
	}

	// Don't overwrite the inherited working dir if we don't have one specified here.
	if params.stage.WorkingDir != nil {
		params.baseWorkingDir = params.stage.WorkingDir
	}

	if params.parentContainer != nil {
		merged, err := MergeContainers(params.parentContainer, stageContainer)
		if err != nil {
			return nil, errors.Wrapf(err, "Error merging stage and parent container overrides: %s", err)
		}
		stageContainer = merged
	}
	stageVolumes = append(stageVolumes, params.parentVolumes...)

	env := scopedEnv(params.stage.GetEnv(), params.parentEnv)

	agent := params.stage.Agent.DeepCopy()

	if agent == nil {
		agent = params.parentAgent.DeepCopy()
	}

	stepCounter := 0
	defaultTaskSpec, err := getDefaultTaskSpec(env, stageContainer, params.parentParams.DefaultImage, params.parentParams.VersionsDir)
	if err != nil {
		return nil, err
	}

	if len(params.stage.Steps) > 0 {
		t := &tektonv1alpha1.Task{
			TypeMeta: metav1.TypeMeta{
				APIVersion: TektonAPIVersion,
				Kind:       "Task",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: params.parentParams.Namespace,
				Name:      MangleToRfc1035Label(fmt.Sprintf("%s-%s", params.parentParams.PipelineIdentifier, params.stage.Name), params.parentParams.BuildIdentifier),
				Labels:    util.MergeMaps(params.parentParams.Labels, map[string]string{LabelStageName: params.stage.stageLabelName()}),
			},
		}
		// Only add the default git merge step if this is the first actual step stage - including if the stage is one of
		// N stages within a parallel stage, and that parallel stage is the first stage in the pipeline
		if params.previousSiblingStage == nil && isNestedFirstStepsStage(params.enclosingStage) {
			t.Spec = defaultTaskSpec
		}

		t.SetDefaults(context.Background())

		ws := &tektonv1alpha1.TaskResource{
			ResourceDeclaration: tektonv1alpha1.ResourceDeclaration{
				Name:       "workspace",
				TargetPath: params.parentParams.SourceDir,
				Type:       tektonv1alpha1.PipelineResourceTypeGit,
			},
		}

		t.Spec.Inputs = &tektonv1alpha1.Inputs{
			Resources: []tektonv1alpha1.TaskResource{*ws},
		}

		t.Spec.Outputs = &tektonv1alpha1.Outputs{
			Resources: []tektonv1alpha1.TaskResource{*ws},
		}

		// We don't want to dupe volumes for the Task if there are multiple steps
		volumes := make(map[string]corev1.Volume)

		for _, v := range stageVolumes {
			volumes[v.Name] = *v
		}

		for _, step := range params.stage.Steps {
			actualSteps, stepVolumes, newCounter, err := generateSteps(generateStepsParams{
				stageParams:     params,
				step:            step,
				inheritedAgent:  agent.Image,
				env:             env,
				parentContainer: stageContainer,
				stepCounter:     stepCounter,
			})
			if err != nil {
				return nil, err
			}

			stepCounter = newCounter

			t.Spec.Steps = append(t.Spec.Steps, actualSteps...)
			for k, v := range stepVolumes {
				volumes[k] = v
			}
		}

		// Avoid nondeterministic results by sorting the keys and appending volumes in that order.
		var volNames []string
		for k := range volumes {
			volNames = append(volNames, k)
		}
		sort.Strings(volNames)

		for _, v := range volNames {
			t.Spec.Volumes = append(t.Spec.Volumes, volumes[v])
		}

		ts := transformedStage{Stage: params.stage, Task: t, Depth: params.depth, EnclosingStage: params.enclosingStage, PreviousSiblingStage: params.previousSiblingStage}
		ts.computeWorkspace(params.parentWorkspace)
		return &ts, nil
	}
	if len(params.stage.Stages) > 0 {
		var tasks []*transformedStage
		ts := transformedStage{Stage: params.stage, Depth: params.depth, EnclosingStage: params.enclosingStage, PreviousSiblingStage: params.previousSiblingStage}
		ts.computeWorkspace(params.parentWorkspace)

		for i, nested := range params.stage.Stages {
			var nestedPreviousSibling *transformedStage
			if i > 0 {
				nestedPreviousSibling = tasks[i-1]
			}
			nestedTask, err := stageToTask(stageToTaskParams{
				parentParams:         params.parentParams,
				stage:                nested,
				baseWorkingDir:       params.baseWorkingDir,
				parentEnv:            env,
				parentAgent:          agent,
				parentWorkspace:      *ts.Stage.Options.Workspace,
				parentContainer:      stageContainer,
				parentVolumes:        stageVolumes,
				depth:                params.depth + 1,
				enclosingStage:       &ts,
				previousSiblingStage: nestedPreviousSibling,
			})
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, nestedTask)
		}
		ts.Sequential = tasks

		return &ts, nil
	}

	if len(params.stage.Parallel) > 0 {
		var tasks []*transformedStage
		ts := transformedStage{Stage: params.stage, Depth: params.depth, EnclosingStage: params.enclosingStage, PreviousSiblingStage: params.previousSiblingStage}
		ts.computeWorkspace(params.parentWorkspace)

		for _, nested := range params.stage.Parallel {
			nestedTask, err := stageToTask(stageToTaskParams{
				parentParams:    params.parentParams,
				stage:           nested,
				baseWorkingDir:  params.baseWorkingDir,
				parentEnv:       env,
				parentAgent:     agent,
				parentWorkspace: *ts.Stage.Options.Workspace,
				parentContainer: stageContainer,
				parentVolumes:   stageVolumes,
				depth:           params.depth + 1,
				enclosingStage:  &ts,
			})
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, nestedTask)
		}
		ts.Parallel = tasks

		return &ts, nil
	}
	return nil, errors.New("no steps, sequential stages, or parallel stages")
}

// MergeContainers combines parent and child container structs, with the child overriding the parent.
func MergeContainers(parentContainer, childContainer *corev1.Container) (*corev1.Container, error) {
	if parentContainer == nil {
		return childContainer, nil
	} else if childContainer == nil {
		return parentContainer, nil
	}

	// We need JSON bytes to generate a patch to merge the child containers onto the parent container, so marshal the parent.
	parentAsJSON, err := json.Marshal(parentContainer)
	if err != nil {
		return nil, err
	}
	// We need to do a three-way merge to actually combine the parent and child containers, so we need an empty container
	// as the "original"
	emptyAsJSON, err := json.Marshal(&corev1.Container{})
	if err != nil {
		return nil, err
	}
	// Marshal the child to JSON
	childAsJSON, err := json.Marshal(childContainer)
	if err != nil {
		return nil, err
	}

	// Get the patch meta for Container, which is needed for generating and applying the merge patch.
	patchSchema, err := strategicpatch.NewPatchMetaFromStruct(parentContainer)

	if err != nil {
		return nil, err
	}

	// Create a merge patch, with the empty JSON as the original, the child JSON as the modified, and the parent
	// JSON as the current - this lets us do a deep merge of the parent and child containers, with awareness of
	// the "patchMerge" tags.
	patch, err := strategicpatch.CreateThreeWayMergePatch(emptyAsJSON, childAsJSON, parentAsJSON, patchSchema, true)
	if err != nil {
		return nil, err
	}

	// Actually apply the merge patch to the parent JSON.
	mergedAsJSON, err := strategicpatch.StrategicMergePatchUsingLookupPatchMeta(parentAsJSON, patch, patchSchema)
	if err != nil {
		return nil, err
	}

	// Unmarshal the merged JSON to a Container pointer, and return it.
	merged := &corev1.Container{}
	err = json.Unmarshal(mergedAsJSON, merged)
	if err != nil {
		return nil, err
	}

	return merged, nil
}

func isNestedFirstStepsStage(enclosingStage *transformedStage) bool {
	if enclosingStage != nil {
		if enclosingStage.PreviousSiblingStage != nil {
			return false
		}
		return isNestedFirstStepsStage(enclosingStage.EnclosingStage)
	}
	return true
}

type generateStepsParams struct {
	stageParams     stageToTaskParams
	step            Step
	inheritedAgent  string
	env             []corev1.EnvVar
	parentContainer *corev1.Container
	stepCounter     int
}

func generateSteps(params generateStepsParams) ([]tektonv1alpha1.Step, map[string]corev1.Volume, int, error) {
	volumes := make(map[string]corev1.Volume)
	var steps []tektonv1alpha1.Step

	stepImage := params.inheritedAgent
	if params.step.GetImage() != "" {
		stepImage = params.step.GetImage()
	}

	// Default to ${WorkingDirRoot}/${sourceDir}
	workingDir := filepath.Join(WorkingDirRoot, params.stageParams.parentParams.SourceDir)

	// Directory we will cd to if it differs from the working dir.
	targetDir := workingDir

	if params.step.Dir != "" {
		targetDir = params.step.Dir
	} else if params.stageParams.baseWorkingDir != nil {
		targetDir = *(params.stageParams.baseWorkingDir)
	}
	// Relative working directories are always just added to /workspace/source, e.g.
	if !filepath.IsAbs(targetDir) {
		targetDir = filepath.Join(WorkingDirRoot, params.stageParams.parentParams.SourceDir, targetDir)
	}

	if params.step.GetCommand() != "" {
		var targetDirPrefix []string
		if targetDir != workingDir && !params.stageParams.parentParams.InterpretMode {
			targetDirPrefix = append(targetDirPrefix, "cd", targetDir, "&&")
		}
		c := &corev1.Container{}
		if params.parentContainer != nil {
			c = params.parentContainer.DeepCopy()
		}
		if params.stageParams.parentParams.PodTemplates != nil && params.stageParams.parentParams.PodTemplates[stepImage] != nil {
			podTemplate := params.stageParams.parentParams.PodTemplates[stepImage]
			containers := podTemplate.Spec.Containers
			for _, volume := range podTemplate.Spec.Volumes {
				volumes[volume.Name] = volume
			}
			if !equality.Semantic.DeepEqual(c, &corev1.Container{}) {
				merged, err := MergeContainers(&containers[0], c)
				if err != nil {
					return nil, nil, params.stepCounter, errors.Wrapf(err, "Error merging pod template and parent container: %s", err)
				}
				c = merged
			} else {
				c = &containers[0]
			}
		} else {
			c.Image = stepImage
			c.Command = []string{"/bin/sh", "-c"}
		}

		resolvedImage, err := versionstream.ResolveDockerImage(params.stageParams.parentParams.VersionsDir, c.Image)
		if err != nil {
			log.Logger().Warnf("failed to resolve step image version: %s due to %s", c.Image, err.Error())
		} else {
			c.Image = resolvedImage
		}
		// Special-casing for commands starting with /kaniko/warmer, which doesn't have sh at all
		if strings.HasPrefix(params.step.GetCommand(), "/kaniko/warmer") {
			c.Command = append(targetDirPrefix, params.step.GetCommand())
			c.Args = params.step.Arguments
		} else {
			// If it's /kaniko/executor, use /busybox/sh instead of /bin/sh, and use the debug image
			if strings.HasPrefix(params.step.GetCommand(), "/kaniko/executor") && strings.Contains(c.Image, "gcr.io/kaniko-project") {
				if !strings.Contains(c.Image, "debug") {
					c.Image = strings.Replace(c.Image, "/executor:", "/executor:debug-", 1)
				}
				c.Command = []string{"/busybox/sh", "-c"}
			}
			cmdStr := params.step.GetCommand()
			if len(params.step.Arguments) > 0 {
				cmdStr += " " + strings.Join(params.step.Arguments, " ")
			}
			if len(targetDirPrefix) > 0 {
				cmdStr = strings.Join(targetDirPrefix, " ") + " " + cmdStr
			}
			c.Args = []string{cmdStr}
		}
		if params.stageParams.parentParams.InterpretMode {
			c.WorkingDir = targetDir
		} else {
			var newCmd []string
			var newArgs []string
			for _, c := range c.Command {
				newCmd = append(newCmd, ReplaceCurlyWithParen(c))
			}
			c.Command = newCmd
			for _, a := range c.Args {
				newArgs = append(newArgs, ReplaceCurlyWithParen(a))
			}
			c.Args = newArgs
			c.WorkingDir = workingDir
		}
		params.stepCounter++
		if params.step.Name != "" {
			c.Name = MangleToRfc1035Label(params.step.Name, "")
		} else {
			c.Name = "step" + strconv.Itoa(1+params.stepCounter)
		}

		c.Stdin = false
		c.TTY = false
		c.Env = scopedEnv(params.step.Env, scopedEnv(params.env, c.Env))

		steps = append(steps, tektonv1alpha1.Step{
			Container: *c,
		})
	} else if params.step.Loop != nil {
		for i, v := range params.step.Loop.Values {
			loopEnv := scopedEnv([]corev1.EnvVar{{Name: params.step.Loop.Variable, Value: v}}, params.env)

			for _, s := range params.step.Loop.Steps {
				if s.Name != "" {
					s.Name = s.Name + strconv.Itoa(1+i)
				}
				loopSteps, loopVolumes, loopCounter, loopErr := generateSteps(generateStepsParams{
					stageParams:     params.stageParams,
					step:            s,
					inheritedAgent:  stepImage,
					env:             loopEnv,
					parentContainer: params.parentContainer,
					stepCounter:     params.stepCounter,
				})
				if loopErr != nil {
					return nil, nil, loopCounter, loopErr
				}

				// Bump the step counter to what we got from the loop
				params.stepCounter = loopCounter

				// Add the loop-generated steps
				steps = append(steps, loopSteps...)

				// Add any new volumes that may have shown up
				for k, v := range loopVolumes {
					volumes[k] = v
				}
			}
		}
	} else {
		return nil, nil, params.stepCounter, errors.New("syntactic sugar steps not yet supported")
	}

	// lets make sure if we've overloaded any environment variables we remove any remaining valueFrom structs
	// to avoid creating bad Tasks
	for i, step := range steps {
		for j, e := range step.Env {
			if e.Value != "" {
				steps[i].Env[j].ValueFrom = nil
			}
		}
	}

	return steps, volumes, params.stepCounter, nil
}

// PipelineRunName returns the pipeline name given the pipeline and build identifier
func PipelineRunName(pipelineIdentifier string, buildIdentifier string) string {
	return MangleToRfc1035Label(fmt.Sprintf("%s", pipelineIdentifier), buildIdentifier)
}

// CRDsFromPipelineParams is how the parameters to GenerateCRDs are specified
type CRDsFromPipelineParams struct {
	PipelineIdentifier string
	BuildIdentifier    string
	Namespace          string
	PodTemplates       map[string]*corev1.Pod
	VersionsDir        string
	TaskParams         []tektonv1alpha1.ParamSpec
	SourceDir          string
	Labels             map[string]string
	DefaultImage       string
	InterpretMode      bool
}

// GenerateCRDs translates the Pipeline structure into the corresponding Pipeline and Task CRDs
func (j *ParsedPipeline) GenerateCRDs(params CRDsFromPipelineParams) (*tektonv1alpha1.Pipeline, []*tektonv1alpha1.Task, *v1.PipelineStructure, error) {
	if len(j.Post) != 0 {
		return nil, nil, nil, errors.New("Post at top level not yet supported")
	}

	var parentContainer *corev1.Container
	var parentVolumes []*corev1.Volume

	baseWorkingDir := j.WorkingDir

	if j.Options != nil {
		o := j.Options
		if o.Retry > 0 {
			return nil, nil, nil, errors.New("Retry at top level not yet supported")
		}
		parentContainer = o.ContainerOptions
		parentVolumes = o.Volumes
	}

	p := &tektonv1alpha1.Pipeline{
		TypeMeta: metav1.TypeMeta{
			APIVersion: TektonAPIVersion,
			Kind:       "Pipeline",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: params.Namespace,
			Name:      PipelineRunName(params.PipelineIdentifier, params.BuildIdentifier),
		},
		Spec: tektonv1alpha1.PipelineSpec{
			Resources: []tektonv1alpha1.PipelineDeclaredResource{
				{
					Name: params.PipelineIdentifier,
					Type: tektonv1alpha1.PipelineResourceTypeGit,
				},
			},
		},
	}

	p.SetDefaults(context.Background())

	structure := &v1.PipelineStructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: p.Name,
		},
	}

	if len(params.Labels) > 0 {
		p.Labels = util.MergeMaps(params.Labels)
		structure.Labels = util.MergeMaps(params.Labels)
	}

	var previousStage *transformedStage

	var tasks []*tektonv1alpha1.Task

	baseEnv := j.GetEnv()

	for i, s := range j.Stages {
		isLastStage := i == len(j.Stages)-1

		stage, err := stageToTask(stageToTaskParams{
			parentParams:         params,
			stage:                s,
			baseWorkingDir:       baseWorkingDir,
			parentEnv:            baseEnv,
			parentAgent:          j.Agent,
			parentWorkspace:      "default",
			parentContainer:      parentContainer,
			parentVolumes:        parentVolumes,
			depth:                0,
			previousSiblingStage: previousStage,
		})
		if err != nil {
			return nil, nil, nil, err
		}

		o := stage.Stage.Options
		if o.RootOptions != nil {
			if o.Retry > 0 {
				stage.Stage.Options.Retry = s.Options.Retry
				log.Logger().Infof("setting retries to %d for stage %s", stage.Stage.Options.Retry, stage.Stage.Name)
			}
		}
		previousStage = stage

		pipelineTasks := createPipelineTasks(stage, p.Spec.Resources[0].Name)

		linearTasks := stage.getLinearTasks()

		for index, lt := range linearTasks {
			if shouldRemoveWorkspaceOutput(stage, lt.Name, index, len(linearTasks), isLastStage) {
				pipelineTasks[index].Resources.Outputs = nil
				lt.Spec.Outputs = nil
			}
			if len(lt.Spec.Inputs.Params) == 0 {
				lt.Spec.Inputs.Params = params.TaskParams
			}
		}

		tasks = append(tasks, linearTasks...)
		p.Spec.Tasks = append(p.Spec.Tasks, pipelineTasks...)
		structure.Stages = append(structure.Stages, stage.getAllAsPipelineStructureStages()...)
	}

	return p, tasks, structure, nil
}

func shouldRemoveWorkspaceOutput(stage *transformedStage, taskName string, index int, tasksLen int, isLastStage bool) bool {
	if stage.isParallel() {
		parallelStages := stage.Parallel
		for _, ps := range parallelStages {
			if ps.Task != nil && ps.Task.Name == taskName {
				return true
			}
			seq := ps.Sequential
			if len(seq) > 0 {
				lastSeq := seq[len(seq)-1]
				if lastSeq.Task.Name == taskName {
					return true
				}
			}

		}
	} else if index == tasksLen-1 && isLastStage {
		return true
	}
	return false
}

func createPipelineTasks(stage *transformedStage, resourceName string) []tektonv1alpha1.PipelineTask {
	if stage.isSequential() {
		var pTasks []tektonv1alpha1.PipelineTask
		for _, nestedStage := range stage.Sequential {
			pTasks = append(pTasks, createPipelineTasks(nestedStage, resourceName)...)
		}
		return pTasks
	} else if stage.isParallel() {
		var pTasks []tektonv1alpha1.PipelineTask
		for _, nestedStage := range stage.Parallel {
			pTasks = append(pTasks, createPipelineTasks(nestedStage, resourceName)...)
		}
		return pTasks
	} else {
		pTask := tektonv1alpha1.PipelineTask{
			Name: stage.Stage.stageLabelName(),
			TaskRef: tektonv1alpha1.TaskRef{
				Name: stage.Task.Name,
			},
			Retries: int(stage.Stage.Options.Retry),
		}

		_, provider := findWorkspaceProvider(stage, stage.getEnclosing(0))
		var previousStageNames []string
		for _, previousStage := range findPreviousNonBlockStages(*stage) {
			previousStageNames = append(previousStageNames, previousStage.PipelineTask.Name)
		}
		pTask.Resources = &tektonv1alpha1.PipelineTaskResources{
			Inputs: []tektonv1alpha1.PipelineTaskInputResource{
				{
					Name:     "workspace",
					Resource: resourceName,
					From:     provider,
				},
			},
			Outputs: []tektonv1alpha1.PipelineTaskOutputResource{
				{
					Name:     "workspace",
					Resource: resourceName,
				},
			},
		}
		pTask.RunAfter = previousStageNames
		stage.PipelineTask = &pTask

		return []tektonv1alpha1.PipelineTask{pTask}
	}
}

// Looks for the most recent Task using the desired workspace that was not in the
// same parallel stage and returns the name of the corresponding Task.
func findWorkspaceProvider(stage, sibling *transformedStage) (bool, []string) {
	if *stage.Stage.Options.Workspace == "empty" {
		return true, nil
	}

	for sibling != nil {
		if sibling.isSequential() {
			found, provider := findWorkspaceProvider(stage, sibling.Sequential[len(sibling.Sequential)-1])
			if found {
				return true, provider
			}
		} else if sibling.isParallel() {
			// We don't want to use a workspace from a parallel stage outside of that stage,
			// but we do need to descend inwards in case stage is in that same stage.
			if stage.getEnclosing(sibling.Depth) == sibling {
				for _, nested := range sibling.Parallel {
					// Pick the parallel branch that has stage
					if stage.getEnclosing(nested.Depth) == nested {
						found, provider := findWorkspaceProvider(stage, nested)
						if found {
							return true, provider
						}
					}
				}
			}
			// TODO: What to do about custom workspaces? Check for erroneous uses specially?
			// Allow them if only one of the parallel tasks uses the same resource?
		} else if sibling.PipelineTask != nil {
			if *sibling.Stage.Options.Workspace == *stage.Stage.Options.Workspace {
				return true, []string{sibling.PipelineTask.Name}
			}
		} else {
			// We are in a sequential stage and sibling has not had its PipelineTask created.
			// Check the task before it so we don't use a workspace of a later task.
		}
		sibling = sibling.PreviousSiblingStage
	}

	return false, nil
}

// Find the end tasks for this stage, traversing down to the end stages of any
// nested sequential or parallel stages as well.
func findEndStages(stage transformedStage) []*transformedStage {
	if stage.isSequential() {
		return findEndStages(*stage.Sequential[len(stage.Sequential)-1])
	} else if stage.isParallel() {
		var endTasks []*transformedStage
		for _, pStage := range stage.Parallel {
			endTasks = append(endTasks, findEndStages(*pStage)...)
		}
		return endTasks
	} else {
		return []*transformedStage{&stage}
	}
}

// Find the tasks that run immediately before this stage, not including
// sequential or parallel wrapper stages.
func findPreviousNonBlockStages(stage transformedStage) []*transformedStage {
	if stage.PreviousSiblingStage != nil {
		return findEndStages(*stage.PreviousSiblingStage)
	} else if stage.EnclosingStage != nil {
		return findPreviousNonBlockStages(*stage.EnclosingStage)
	} else {
		return []*transformedStage{}
	}
}

// Return the stage that encloses this stage at the given depth, or nil if there is no such stage.
// Depth must be >= 0. Returns the stage itself if depth == stage.Depth
func (ts *transformedStage) getEnclosing(depth int8) *transformedStage {
	if ts.Depth == depth {
		return ts
	} else if ts.EnclosingStage == nil {
		return nil
	} else {
		return ts.EnclosingStage.getEnclosing(depth)
	}
}

// Return the first stage that will execute before this stage
// Depth must be >= 0
func (ts transformedStage) getClosestAncestor() *transformedStage {
	if ts.PreviousSiblingStage != nil {
		return ts.PreviousSiblingStage
	} else if ts.EnclosingStage == nil {
		return nil
	} else {
		return ts.EnclosingStage.getClosestAncestor()
	}
}

func findDuplicates(names []string) *apis.FieldError {
	// Count members
	counts := make(map[string]int)
	mangled := make(map[string]string)
	for _, v := range names {
		counts[MangleToRfc1035Label(v, "")]++
		mangled[v] = MangleToRfc1035Label(v, "")
	}

	var duplicateNames []string
	for k, v := range mangled {
		if counts[v] > 1 {
			duplicateNames = append(duplicateNames, "'"+k+"'")
		}
	}

	if len(duplicateNames) > 0 {
		// Avoid nondeterminism in error messages
		sort.Strings(duplicateNames)
		return &apis.FieldError{
			Message: "Stage names must be unique",
			Details: "The following stage names are used more than once: " + strings.Join(duplicateNames, ", "),
		}
	}
	return nil
}

func validateStageNames(j *ParsedPipeline) (err *apis.FieldError) {
	var validate func(stages []Stage, stageNames *[]string)
	validate = func(stages []Stage, stageNames *[]string) {

		for _, stage := range stages {
			*stageNames = append(*stageNames, stage.Name)
			if len(stage.Stages) > 0 {
				validate(stage.Stages, stageNames)
			}
		}

	}
	var names []string

	validate(j.Stages, &names)

	err = findDuplicates(names)

	return
}

// todo JR lets remove this when we switch tekton to using git merge type pipelineresources
func getDefaultTaskSpec(envs []corev1.EnvVar, parentContainer *corev1.Container, defaultImage string, versionsDir string) (tektonv1alpha1.TaskSpec, error) {
	var err error
	image := defaultImage
	if image == "" {
		image = os.Getenv("BUILDER_JX_IMAGE")
		if image == "" {
			image, err = versionstream.ResolveDockerImage(versionsDir, GitMergeImage)
			if err != nil {
				return tektonv1alpha1.TaskSpec{}, err
			}
		}
	}

	childContainer := &corev1.Container{
		Name:       "git-merge",
		Image:      image,
		Command:    []string{"jx"},
		Args:       []string{"step", "git", "merge", "--verbose"},
		WorkingDir: "/workspace/source",
		Env:        envs,
	}

	if parentContainer != nil {
		merged, err := MergeContainers(parentContainer, childContainer)
		if err != nil {
			return tektonv1alpha1.TaskSpec{}, err
		}
		childContainer = merged
	}

	return tektonv1alpha1.TaskSpec{
		Steps: []tektonv1alpha1.Step{{Container: *childContainer}},
	}, nil
}

// HasNonStepOverrides returns true if this override contains configuration like agent, containerOptions, or volumes.
func (p *PipelineOverride) HasNonStepOverrides() bool {
	return p.ContainerOptions != nil || p.Agent != nil || len(p.Volumes) > 0
}

// AsStepsSlice returns a possibly empty slice of the step or steps in this override
func (p *PipelineOverride) AsStepsSlice() []*Step {
	if p.Step != nil {
		return []*Step{p.Step}
	}
	if len(p.Steps) > 0 {
		return p.Steps
	}
	return []*Step{}
}

// MatchesPipeline returns true if the pipeline name is specified in the override or no pipeline is specified at all in the override
func (p *PipelineOverride) MatchesPipeline(name string) bool {
	if p.Pipeline == "" || strings.EqualFold(p.Pipeline, name) {
		return true
	}
	return false
}

// MatchesStage returns true if the stage/lifecycle name is specified in the override or no stage/lifecycle is specified at all in the override
func (p *PipelineOverride) MatchesStage(name string) bool {
	if p.Stage == "" || p.Stage == name {
		return true
	}
	return false
}

// ApplyStepOverridesToPipeline applies an individual override to the pipeline, replacing named steps in specified stages (or all stages if
// no stage name is specified).
func ApplyStepOverridesToPipeline(pipeline *ParsedPipeline, override *PipelineOverride) *ParsedPipeline {
	if pipeline == nil || override == nil {
		return pipeline
	}

	var newStages []Stage
	for _, s := range pipeline.Stages {
		overriddenStage := ApplyStepOverridesToStage(s, override)
		if !equality.Semantic.DeepEqual(overriddenStage, Stage{}) {
			newStages = append(newStages, overriddenStage)
		}
	}
	pipeline.Stages = newStages

	return pipeline
}

func stepPointerSliceToStepSlice(orig []*Step) []Step {
	var newSteps []Step
	for _, s := range orig {
		if s != nil {
			newSteps = append(newSteps, *s)
		}
	}

	return newSteps
}

// ApplyNonStepOverridesToPipeline applies the non-step configuration from an individual override to the pipeline.
func ApplyNonStepOverridesToPipeline(pipeline *ParsedPipeline, override *PipelineOverride) *ParsedPipeline {
	if pipeline == nil || override == nil {
		return pipeline
	}

	// Only apply this override to the top-level pipeline if no stage is specified.
	if override.Stage == "" {
		if override.Agent != nil {
			pipeline.Agent = override.Agent
		}
		if override.ContainerOptions != nil {
			containerOptionsCopy := *override.ContainerOptions
			if pipeline.Options == nil {
				pipeline.Options = &RootOptions{}
			}
			if pipeline.Options.ContainerOptions == nil {
				pipeline.Options.ContainerOptions = &containerOptionsCopy
			} else {
				mergedContainer, err := MergeContainers(pipeline.Options.ContainerOptions, &containerOptionsCopy)
				if err != nil {
					log.Logger().Warnf("couldn't merge override container options: %s", err)
				} else {
					pipeline.Options.ContainerOptions = mergedContainer
				}
			}
		}
		if len(override.Volumes) > 0 {
			if pipeline.Options == nil {
				pipeline.Options = &RootOptions{}
			}
			pipeline.Options.Volumes = append(pipeline.Options.Volumes, override.Volumes...)
		}
	}

	var newStages []Stage
	for _, s := range pipeline.Stages {
		overriddenStage := ApplyNonStepOverridesToStage(s, override)
		if !equality.Semantic.DeepEqual(overriddenStage, Stage{}) {
			newStages = append(newStages, overriddenStage)
		}
	}
	pipeline.Stages = newStages

	return pipeline
}

// ApplyNonStepOverridesToStage applies non-step overrides, such as stage agent, containerOptions, and volumes, to this
// stage and its children.
func ApplyNonStepOverridesToStage(stage Stage, override *PipelineOverride) Stage {
	if override == nil {
		return stage
	}

	// Since a traditional build pack only has one stage at this point, treat anything that's stage-specific as valid here.
	if (override.MatchesStage(stage.Name) || stage.Name == DefaultStageNameForBuildPack) && override.Stage != "" {
		if override.Agent != nil {
			stage.Agent = override.Agent
		}
		if override.ContainerOptions != nil {
			containerOptionsCopy := *override.ContainerOptions
			if stage.Options == nil {
				stage.Options = &StageOptions{
					RootOptions: &RootOptions{},
				}
			}
			if stage.Options.ContainerOptions == nil {
				stage.Options.ContainerOptions = &containerOptionsCopy
			} else {
				mergedContainer, err := MergeContainers(stage.Options.ContainerOptions, &containerOptionsCopy)
				if err != nil {
					log.Logger().Warnf("couldn't merge override container options: %s", err)
				} else {
					stage.Options.ContainerOptions = mergedContainer
				}
			}
		}

		if len(override.Volumes) > 0 {
			if stage.Options == nil {
				stage.Options = &StageOptions{
					RootOptions: &RootOptions{},
				}
			}
			stage.Options.Volumes = append(stage.Options.Volumes, override.Volumes...)
		}
	}
	if len(stage.Stages) > 0 {
		var newStages []Stage
		for _, s := range stage.Stages {
			newStages = append(newStages, ApplyNonStepOverridesToStage(s, override))
		}
		stage.Stages = newStages
	}
	if len(stage.Parallel) > 0 {
		var newParallel []Stage
		for _, s := range stage.Parallel {
			newParallel = append(newParallel, ApplyNonStepOverridesToStage(s, override))
		}
		stage.Parallel = newParallel
	}

	return stage
}

// ApplyStepOverridesToStage applies a set of overrides to named steps in this stage and its children
func ApplyStepOverridesToStage(stage Stage, override *PipelineOverride) Stage {
	if override == nil {
		return stage
	}

	if override.MatchesStage(stage.Name) {
		if len(stage.Steps) > 0 {
			var newSteps []Step
			if override.Name != "" {
				for _, s := range stage.Steps {
					newSteps = append(newSteps, OverrideStep(s, override)...)
				}
			} else {
				// If no step name was specified but there are steps, just replace all steps in the stage/lifecycle,
				// or add the new steps before/after the existing steps in the stage/lifecycle
				if steps := override.AsStepsSlice(); len(steps) > 0 {
					if override.Type == nil || *override.Type == StepOverrideReplace {
						newSteps = append(newSteps, stepPointerSliceToStepSlice(steps)...)
					} else if *override.Type == StepOverrideBefore {
						newSteps = append(newSteps, stepPointerSliceToStepSlice(steps)...)
						newSteps = append(newSteps, stage.Steps...)
					} else if *override.Type == StepOverrideAfter {
						newSteps = append(newSteps, stage.Steps...)
						newSteps = append(newSteps, stepPointerSliceToStepSlice(steps)...)
					}
				}
				// If there aren't any steps as well as no step name, then we're removing all steps from this stage/lifecycle,
				// so just don't add anything to newSteps, and we'll end up returning an empty stage
			}

			// If newSteps isn't empty, use it for the stage's steps list. Otherwise, if no agent override is specified,
			// we're removing this stage, so return an empty stage.
			if len(newSteps) > 0 {
				stage.Steps = newSteps
			} else if !override.HasNonStepOverrides() {
				return Stage{}
			}
		}
	}
	if len(stage.Stages) > 0 {
		var newStages []Stage
		for _, s := range stage.Stages {
			newStages = append(newStages, ApplyStepOverridesToStage(s, override))
		}
		stage.Stages = newStages
	}
	if len(stage.Parallel) > 0 {
		var newParallel []Stage
		for _, s := range stage.Parallel {
			newParallel = append(newParallel, ApplyStepOverridesToStage(s, override))
		}
		stage.Parallel = newParallel
	}

	return stage
}

// OverrideStep overrides an existing step, if it matches the override's name, with the contents of the override. It also
// recurses into child steps.
func OverrideStep(step Step, override *PipelineOverride) []Step {
	if override != nil {
		if step.Name == override.Name {
			var newSteps []Step

			if override.Step != nil {
				if override.Step.Name == "" {
					override.Step.Name = step.Name
				}
				newSteps = append(newSteps, *override.Step)
			}
			if override.Steps != nil {
				for _, s := range override.Steps {
					newSteps = append(newSteps, *s)
				}
			}

			if override.Type == nil || *override.Type == StepOverrideReplace {
				return newSteps
			} else if *override.Type == StepOverrideBefore {
				return append(newSteps, step)
			} else if *override.Type == StepOverrideAfter {
				return append([]Step{step}, newSteps...)
			}

			// Fall back on just returning the original. We shouldn't ever get here.
			return []Step{step}
		}

		if len(step.Steps) > 0 {
			var newSteps []*Step
			for _, s := range step.Steps {
				for _, o := range OverrideStep(*s, override) {
					stepCopy := o
					newSteps = append(newSteps, &stepCopy)
				}
			}
			step.Steps = newSteps
		}
	}

	return []Step{step}
}

// StringParamValue generates a Tekton ArrayOrString value for the given string
func StringParamValue(val string) tektonv1alpha1.ArrayOrString {
	return tektonv1alpha1.ArrayOrString{
		Type:      tektonv1alpha1.ParamTypeString,
		StringVal: val,
	}
}

// ReplaceCurlyWithParen replaces legacy "${inputs.params.foo}" with "$(inputs.params.foo)"
func ReplaceCurlyWithParen(input string) string {
	re := regexp.MustCompile(braceMatchingRegex)
	matches := re.FindAllStringSubmatch(input, -1)
	for _, m := range matches {
		if len(m) >= 3 {
			input = strings.ReplaceAll(input, m[0], "$("+m[3]+")")
		}
	}
	return input
}
