// +build unit

package syntax_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/google/go-cmp/cmp"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	sh "github.com/jenkins-x/jx/pkg/tekton/syntax/syntax_helpers_test"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/kmp"
	tektonv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

var (
	// Needed to take address of strings since workspace is *string. Is there a better way to handle optional values?
	defaultWorkspace = "default"
	customWorkspace  = "custom"
)

// TODO: Try to write some helper functions to make Pipeline and Task expect building less bloody verbose.
func TestParseJenkinsfileYaml(t *testing.T) {
	testVersionsDir := filepath.Join("test_data", "stable_versions")
	resolvedGitMergeImage, err := versionstream.ResolveDockerImage(testVersionsDir, syntax.GitMergeImage)
	assert.NoError(t, err)

	ctx := context.Background()
	tests := []struct {
		name               string
		expected           *syntax.ParsedPipeline
		pipeline           *tektonv1alpha1.Pipeline
		tasks              []*tektonv1alpha1.Task
		expectedErrorMsg   string
		validationErrorMsg string
		structure          *v1.PipelineStructure
	}{
		{
			name: "simple_jenkinsfile",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
						sh.StepName("A Step With Spaces And Such"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
						tb.Step("a-step-with-spaces-and-such", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "multiple_stages",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("another-stage", "somepipeline-another-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("a-working-stage")),
					tb.RunAfter("a-working-stage")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-another-stage-1", "jx", sh.TaskStageLabel("Another stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo again"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
				sh.StructureStage("Another stage", sh.StructureStageTaskRef("somepipeline-another-stage-1"),
					sh.StructureStagePrevious("A Working Stage")),
			),
		},
		{
			name: "nested_stages",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("Parent Stage",
					sh.StageSequential("A Working Stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world"))),
					sh.StageSequential("Another stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("again"))),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("another-stage", "somepipeline-another-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("a-working-stage")),
					tb.RunAfter("a-working-stage")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
						tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
				tb.Task("somepipeline-another-stage-1", "jx", sh.TaskStageLabel("Another stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo again"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("Parent Stage",
					sh.StructureStageStages("A Working Stage", "Another stage")),
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage")),
				sh.StructureStage("Another stage", sh.StructureStageTaskRef("somepipeline-another-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage"),
					sh.StructureStagePrevious("A Working Stage")),
			),
		},
		{
			name: "parallel_stages",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("First Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("first"))),
				sh.PipelineStage("Parent Stage",
					sh.StageParallel("A Working Stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world"))),
					sh.StageParallel("Another stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("again"))),
				),
				sh.PipelineStage("Last Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("last"))),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("first-stage", "somepipeline-first-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline", tb.From("first-stage")),
					tb.RunAfter("first-stage")),
				tb.PipelineTask("another-stage", "somepipeline-another-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("first-stage")),
					tb.RunAfter("first-stage")),
				tb.PipelineTask("last-stage", "somepipeline-last-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline", tb.From("first-stage")),
					tb.RunAfter("a-working-stage", "another-stage")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-first-stage-1", "jx", sh.TaskStageLabel("First Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo first"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
				tb.Task("somepipeline-another-stage-1", "jx", sh.TaskStageLabel("Another stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo again"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-last-stage-1", "jx", sh.TaskStageLabel("Last Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo last"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("First Stage", sh.StructureStageTaskRef("somepipeline-first-stage-1")),
				sh.StructureStage("Parent Stage",
					sh.StructureStageParallel("A Working Stage", "Another stage"),
					sh.StructureStagePrevious("First Stage"),
				),
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage"),
				),
				sh.StructureStage("Another stage", sh.StructureStageTaskRef("somepipeline-another-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage"),
				),
				sh.StructureStage("Last Stage", sh.StructureStageTaskRef("somepipeline-last-stage-1"),
					sh.StructureStagePrevious("Parent Stage")),
			),
		},
		{
			name: "parallel_and_nested_stages",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("First Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("first"))),
				sh.PipelineStage("Parent Stage",
					sh.StageParallel("A Working Stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world"))),
					sh.StageParallel("Nested In Parallel",
						sh.StageSequential("Another stage",
							sh.StageStep(sh.StepCmd("echo"), sh.StepArg("again"))),
						sh.StageSequential("Some other stage",
							sh.StageStep(sh.StepCmd("echo"), sh.StepArg("otherwise"))),
					),
				),
				sh.PipelineStage("Last Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("last"))),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("first-stage", "somepipeline-first-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("first-stage")),
					tb.RunAfter("first-stage")),
				tb.PipelineTask("another-stage", "somepipeline-another-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("first-stage")),
					tb.PipelineTaskOutputResource("workspace", "somepipeline"),
					tb.RunAfter("first-stage")),
				tb.PipelineTask("some-other-stage", "somepipeline-some-other-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("another-stage")),
					tb.RunAfter("another-stage")),
				tb.PipelineTask("last-stage", "somepipeline-last-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("first-stage")),
					tb.RunAfter("a-working-stage", "some-other-stage")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-first-stage-1", "jx", sh.TaskStageLabel("First Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo first"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
				tb.Task("somepipeline-another-stage-1", "jx", sh.TaskStageLabel("Another stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo again"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-some-other-stage-1", "jx", sh.TaskStageLabel("Some other stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo otherwise"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-last-stage-1", "jx", sh.TaskStageLabel("Last Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo last"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("First Stage", sh.StructureStageTaskRef("somepipeline-first-stage-1")),
				sh.StructureStage("Parent Stage",
					sh.StructureStageParallel("A Working Stage", "Nested In Parallel"),
					sh.StructureStagePrevious("First Stage"),
				),
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage"),
				),
				sh.StructureStage("Nested In Parallel",
					sh.StructureStageParent("Parent Stage"),
					sh.StructureStageDepth(1),
					sh.StructureStageStages("Another stage", "Some other stage"),
				),
				sh.StructureStage("Another stage", sh.StructureStageTaskRef("somepipeline-another-stage-1"),
					sh.StructureStageDepth(2),
					sh.StructureStageParent("Nested In Parallel"),
				),
				sh.StructureStage("Some other stage", sh.StructureStageTaskRef("somepipeline-some-other-stage-1"),
					sh.StructureStageDepth(2),
					sh.StructureStageParent("Nested In Parallel"),
					sh.StructureStagePrevious("Another stage"),
				),
				sh.StructureStage("Last Stage", sh.StructureStageTaskRef("somepipeline-last-stage-1"),
					sh.StructureStagePrevious("Parent Stage")),
			),
		},
		{
			name: "custom_workspaces",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("stage1",
					sh.StageStep(sh.StepCmd("ls")),
				),
				sh.PipelineStage("stage2",
					sh.StageOptions(
						sh.StageOptionsWorkspace(customWorkspace),
					),
					sh.StageStep(sh.StepCmd("ls")),
				),
				sh.PipelineStage("stage3",
					sh.StageOptions(
						sh.StageOptionsWorkspace(defaultWorkspace),
					),
					sh.StageStep(sh.StepCmd("ls")),
				),
				sh.PipelineStage("stage4",
					sh.StageOptions(
						sh.StageOptionsWorkspace(customWorkspace),
					),
					sh.StageStep(sh.StepCmd("ls")),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("stage1", "somepipeline-stage1-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("stage2", "somepipeline-stage2-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline"),
					tb.RunAfter("stage1")),
				tb.PipelineTask("stage3", "somepipeline-stage3-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline", tb.From("stage1")),
					tb.PipelineTaskOutputResource("workspace", "somepipeline"),
					tb.RunAfter("stage2")),
				tb.PipelineTask("stage4", "somepipeline-stage4-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline", tb.From("stage2")),
					tb.RunAfter("stage3")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-stage1-1", "jx", sh.TaskStageLabel("stage1"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-stage2-1", "jx", sh.TaskStageLabel("stage2"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-stage3-1", "jx", sh.TaskStageLabel("stage3"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-stage4-1", "jx", sh.TaskStageLabel("stage4"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("stage1", sh.StructureStageTaskRef("somepipeline-stage1-1")),
				sh.StructureStage("stage2", sh.StructureStageTaskRef("somepipeline-stage2-1"), sh.StructureStagePrevious("stage1")),
				sh.StructureStage("stage3", sh.StructureStageTaskRef("somepipeline-stage3-1"), sh.StructureStagePrevious("stage2")),
				sh.StructureStage("stage4", sh.StructureStageTaskRef("somepipeline-stage4-1"), sh.StructureStagePrevious("stage3")),
			),
		},
		{
			name: "inherited_custom_workspaces",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("stage1",
					sh.StageStep(sh.StepCmd("ls")),
				),
				sh.PipelineStage("stage2",
					sh.StageOptions(
						sh.StageOptionsWorkspace(customWorkspace),
					),
					sh.StageSequential("stage3",
						sh.StageStep(sh.StepCmd("ls")),
					),
					sh.StageSequential("stage4",
						sh.StageOptions(
							sh.StageOptionsWorkspace(defaultWorkspace),
						),
						sh.StageStep(sh.StepCmd("ls")),
					),
					sh.StageSequential("stage5",
						sh.StageStep(sh.StepCmd("ls")),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("stage1", "somepipeline-stage1-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("stage3", "somepipeline-stage3-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline"),
					tb.RunAfter("stage1")),
				tb.PipelineTask("stage4", "somepipeline-stage4-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("stage1")),
					tb.PipelineTaskOutputResource("workspace", "somepipeline"),
					tb.RunAfter("stage3")),
				tb.PipelineTask("stage5", "somepipeline-stage5-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("stage3")),
					tb.RunAfter("stage4")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-stage1-1", "jx", sh.TaskStageLabel("stage1"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-stage3-1", "jx", sh.TaskStageLabel("stage3"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-stage4-1", "jx", sh.TaskStageLabel("stage4"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-stage5-1", "jx", sh.TaskStageLabel("stage5"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("stage1", sh.StructureStageTaskRef("somepipeline-stage1-1")),
				sh.StructureStage("stage2",
					sh.StructureStagePrevious("stage1"),
					sh.StructureStageStages("stage3", "stage4", "stage5"),
				),
				sh.StructureStage("stage3", sh.StructureStageTaskRef("somepipeline-stage3-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("stage2")),
				sh.StructureStage("stage4", sh.StructureStageTaskRef("somepipeline-stage4-1"),
					sh.StructureStagePrevious("stage3"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("stage2")),
				sh.StructureStage("stage5", sh.StructureStageTaskRef("somepipeline-stage5-1"),
					sh.StructureStagePrevious("stage4"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("stage2")),
			),
		},
		{
			name: "environment_at_top_and_in_stage",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineEnvVar("SOME_VAR", "A value for the env var"),
				sh.PipelineStage("A stage with environment",
					sh.StageEnvVar("SOME_OTHER_VAR", "A value for the other env var"),
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("${SOME_OTHER_VAR}")),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("goodbye"), sh.StepArg("${SOME_VAR} and ${ANOTHER_VAR}"),
						sh.StepEnvVar("SOME_VAR", "An overriding value"),
						sh.StepEnvVar("ANOTHER_VAR", "Yet another variable"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-stage-with-environment", "somepipeline-a-stage-with-environment-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-stage-with-environment-1", "jx",
					sh.TaskStageLabel("A stage with environment"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("SOME_OTHER_VAR", "A value for the other env var"), tb.StepEnvVar("SOME_VAR", "A value for the env var")),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${SOME_OTHER_VAR}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("SOME_OTHER_VAR", "A value for the other env var"), tb.StepEnvVar("SOME_VAR", "A value for the env var")),
						tb.Step("step3", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo goodbye ${SOME_VAR} and ${ANOTHER_VAR}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("ANOTHER_VAR", "Yet another variable"),
							tb.StepEnvVar("SOME_OTHER_VAR", "A value for the other env var"),
							tb.StepEnvVar("SOME_VAR", "An overriding value"),
						),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A stage with environment", sh.StructureStageTaskRef("somepipeline-a-stage-with-environment-1")),
			),
		},
		{
			name: "syntactic_sugar_step_and_a_command",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world")),
					sh.StageStep(sh.StepStep("some-step"),
						sh.StepOptions(map[string]string{"firstParam": "some value", "secondParam": "some other value"})),
				),
			),
			expectedErrorMsg: "syntactic sugar steps not yet supported",
		},
		{
			name: "post",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world")),
					sh.StagePost(syntax.PostConditionSuccess,
						sh.PostAction("mail", map[string]string{
							"to":      "foo@bar.com",
							"subject": "Yay, it passed",
						})),
					sh.StagePost(syntax.PostConditionFailure,
						sh.PostAction("slack", map[string]string{
							"whatever": "the",
							"slack":    "config",
							"actually": "is. =)",
						})),
					sh.StagePost(syntax.PostConditionAlways,
						sh.PostAction("junit", map[string]string{
							"pattern": "target/surefire-reports/**/*.xml",
						}),
					),
				),
			),
			expectedErrorMsg: "post on stages not yet supported",
		},
		{
			name: "top_level_and_stage_options",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineOptions(
					sh.PipelineOptionsTimeout(50, "minutes"),
					sh.PipelineOptionsRetry(3),
				),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageOptionsTimeout(5, "seconds"),
						sh.StageOptionsRetry(4),
						sh.StageOptionsStash("Some Files", "somedir/**/*"),
						sh.StageOptionsUnstash("Earlier Files", "some/sub/dir"),
					),
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world")),
				),
			),
			expectedErrorMsg: "Retry at top level not yet supported",
		},
		{
			name: "stage_and_step_agent",
			expected: sh.ParsedPipeline(
				sh.PipelineStage("A Working Stage",
					sh.StageAgent("some-image"),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
						sh.StepAgent("some-other-image"),
					),
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("goodbye")),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
						tb.Step("step2", "some-other-image", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
						tb.Step("step3", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo goodbye"), tb.StepWorkingDir("/workspace/source")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "mangled_task_names",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage(". -a- .",
					sh.StageStep(sh.StepCmd("ls")),
				),
				sh.PipelineStage("Wööh!!!! - This is cool.",
					sh.StageStep(sh.StepCmd("ls")),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a", "somepipeline-a-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("wh-this-is-cool", "somepipeline-wh-this-is-cool-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("a")),
					tb.RunAfter("a")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-1", "jx", sh.TaskStageLabel("a"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-wh-this-is-cool-1", "jx", sh.TaskStageLabel("wh-this-is-cool"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("ls"), tb.StepWorkingDir("/workspace/source")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage(". -a- .", sh.StructureStageTaskRef("somepipeline-a-1")),
				sh.StructureStage("Wööh!!!! - This is cool.", sh.StructureStageTaskRef("somepipeline-wh-this-is-cool-1"), sh.StructureStagePrevious(". -a- .")),
			),
		},
		{
			name: "stage_timeout",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageOptionsTimeout(50, "minutes"),
					),
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world")),
				),
			),
			/* TODO: Stop erroring out once we figure out how to handle task timeouts again
															pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
																tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
																	tb.PipelineTaskInputResource("workspace", "somepipeline"),
																	tb.PipelineTaskInputResource("temp-ordering-resource", "temp-ordering-resource"),
																	tb.PipelineTaskOutputResource("workspace", "somepipeline")),
									tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
																					tb.Task("somepipeline-a-working-stage-1", "jx", TaskStageLabel("A Working Stage"),
																	tb.TaskSpec(
												tb.TaskTimeout(50*time.Minute),
																	tb.TaskInputs(
																		tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
																			tb.ResourceTargetPath("source"))),
						tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
																	tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
																)),
															},*/
			expectedErrorMsg: "Timeout on stage not yet supported",
		},
		{
			name: "top_level_timeout",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineOptions(
					sh.PipelineOptionsTimeout(50, "minutes"),
				),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world")),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "loop_step",
			expected: sh.ParsedPipeline(
				sh.PipelineEnvVar("LANGUAGE", "rust"),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageEnvVar("DISTRO", "gentoo"),
					sh.StageStep(
						sh.StepLoop("LANGUAGE", []string{"maven", "gradle", "nodejs"},
							sh.LoopStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("${LANGUAGE}")),
							sh.LoopStep(sh.StepLoop("DISTRO", []string{"fedora", "ubuntu", "debian"},
								sh.LoopStep(sh.StepCmd("echo"),
									sh.StepArg("running"), sh.StepArg("${LANGUAGE}"),
									sh.StepArg("on"), sh.StepArg("${DISTRO}")),
							)),
						),
					),
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("after")),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "rust")),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${LANGUAGE}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "maven")),
						tb.Step("step3", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "fedora"), tb.StepEnvVar("LANGUAGE", "maven")),
						tb.Step("step4", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "ubuntu"), tb.StepEnvVar("LANGUAGE", "maven")),
						tb.Step("step5", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "debian"), tb.StepEnvVar("LANGUAGE", "maven")),
						tb.Step("step6", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${LANGUAGE}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "gradle")),
						tb.Step("step7", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "fedora"), tb.StepEnvVar("LANGUAGE", "gradle")),
						tb.Step("step8", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "ubuntu"), tb.StepEnvVar("LANGUAGE", "gradle")),
						tb.Step("step9", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "debian"), tb.StepEnvVar("LANGUAGE", "gradle")),
						tb.Step("step10", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${LANGUAGE}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "nodejs")),
						tb.Step("step11", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "fedora"), tb.StepEnvVar("LANGUAGE", "nodejs")),
						tb.Step("step12", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "ubuntu"), tb.StepEnvVar("LANGUAGE", "nodejs")),
						tb.Step("step13", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo running ${LANGUAGE} on ${DISTRO}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "debian"), tb.StepEnvVar("LANGUAGE", "nodejs")),
						tb.Step("step14", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello after"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "rust")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "loop_step_with_name",
			expected: sh.ParsedPipeline(
				sh.PipelineEnvVar("LANGUAGE", "rust"),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageEnvVar("DISTRO", "gentoo"),
					sh.StageStep(
						sh.StepLoop("LANGUAGE", []string{"maven", "gradle", "nodejs"},
							sh.LoopStep(
								sh.StepName("echo-step"),
								sh.StepCmd("echo"),
								sh.StepArg("hello"),
								sh.StepArg("${LANGUAGE}")),
						),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "rust")),
						tb.Step("echo-step1", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${LANGUAGE}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "maven")),
						tb.Step("echo-step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${LANGUAGE}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "gradle")),
						tb.Step("echo-step3", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello ${LANGUAGE}"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("DISTRO", "gentoo"), tb.StepEnvVar("LANGUAGE", "nodejs")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage",
					sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "loop_with_syntactic_sugar_step",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepLoop("LANGUAGE", []string{"maven", "gradle", "nodejs"},
							sh.LoopStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("${LANGUAGE}")),
							sh.LoopStep(sh.StepStep("some-step"),
								sh.StepOptions(map[string]string{"firstParam": "some value", "secondParam": "some other value"})),
						),
					),
				),
			),
			expectedErrorMsg: "syntactic sugar steps not yet supported",
		},
		{
			name: "top_level_container_options",
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(
					sh.PipelineContainerOptions(
						sh.ContainerResourceLimits("0.2", "128Mi"),
						sh.ContainerResourceRequests("0.1", "64Mi"),
					),
				),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.2", "128Mi"),
							sh.StepResourceRequests("0.1", "64Mi"),
						),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.2", "128Mi"),
							sh.StepResourceRequests("0.1", "64Mi"),
						),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "stage_overrides_top_level_container_options",
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(
					sh.PipelineContainerOptions(
						sh.ContainerResourceLimits("0.2", "128Mi"),
						sh.ContainerResourceRequests("0.1", "64Mi"),
					),
				),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageContainerOptions(
							sh.ContainerResourceLimits("0.4", "256Mi"),
							sh.ContainerResourceRequests("0.2", "128Mi"),
						),
					),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.4", "256Mi"),
							sh.StepResourceRequests("0.2", "128Mi"),
						),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.4", "256Mi"),
							sh.StepResourceRequests("0.2", "128Mi"),
						),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "merge_container_options",
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(
					sh.PipelineContainerOptions(
						sh.ContainerResourceRequests("0.1", "64Mi"),
					),
				),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageContainerOptions(
							sh.ContainerResourceLimits("0.4", "256Mi"),
						),
					),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.4", "256Mi"),
							sh.StepResourceRequests("0.1", "64Mi"),
						),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.4", "256Mi"),
							sh.StepResourceRequests("0.1", "64Mi"),
						),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "stage_level_container_options",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageContainerOptions(
							sh.ContainerResourceLimits("0.2", "128Mi"),
							sh.ContainerResourceRequests("0.1", "64Mi"),
						),
					),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.2", "128Mi"),
							sh.StepResourceRequests("0.1", "64Mi"),
						),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source"),
							sh.StepResourceLimits("0.2", "128Mi"),
							sh.StepResourceRequests("0.1", "64Mi"),
						),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "container_options_env_merge",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineOptions(
					sh.PipelineContainerOptions(
						tb.EnvVar("SOME_VAR", "A value for the env var"),
						tb.EnvVar("OVERRIDE_ENV", "Original value"),
						tb.EnvVar("OVERRIDE_STAGE_ENV", "Original value"),
					),
				),
				sh.PipelineEnvVar("SOME_OTHER_VAR", "A value for the other env var"),
				sh.PipelineEnvVar("OVERRIDE_ENV", "New value"),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageContainerOptions(
							tb.EnvVar("ANOTHER_OVERRIDE_STAGE_ENV", "Original value"),
						),
					),
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world")),
					sh.StageEnvVar("OVERRIDE_STAGE_ENV", "New value"),
					sh.StageEnvVar("ANOTHER_OVERRIDE_STAGE_ENV", "New value"),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx",
					sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("ANOTHER_OVERRIDE_STAGE_ENV", "New value"),
							tb.StepEnvVar("SOME_VAR", "A value for the env var"),
							tb.StepEnvVar("OVERRIDE_ENV", "New value"),
							tb.StepEnvVar("OVERRIDE_STAGE_ENV", "New value"),
							tb.StepEnvVar("SOME_OTHER_VAR", "A value for the other env var"),
						),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source"),
							tb.StepEnvVar("ANOTHER_OVERRIDE_STAGE_ENV", "New value"),
							tb.StepEnvVar("OVERRIDE_ENV", "New value"),
							tb.StepEnvVar("OVERRIDE_STAGE_ENV", "New value"),
							tb.StepEnvVar("SOME_OTHER_VAR", "A value for the other env var"),
							tb.StepEnvVar("SOME_VAR", "A value for the env var"),
						),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "dir_on_pipeline_and_stage",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineDir("a-relative-dir"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageDir("/an/absolute/dir"),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again")),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("in another dir"),
						sh.StepDir("another-relative-dir/with/a/subdir"))),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("another-stage", "somepipeline-another-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("a-working-stage")),
					tb.RunAfter("a-working-stage")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"),
						tb.StepArgs("cd /workspace/source/a-relative-dir && echo hello world"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-another-stage-1", "jx", sh.TaskStageLabel("Another stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"),
						tb.StepArgs("cd /an/absolute/dir && echo again"),
						tb.StepWorkingDir("/workspace/source")),
					tb.Step("step3", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"),
						tb.StepArgs("cd /workspace/source/another-relative-dir/with/a/subdir && echo in another dir"),
						tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
				sh.StructureStage("Another stage", sh.StructureStageTaskRef("somepipeline-another-stage-1"),
					sh.StructureStagePrevious("A Working Stage")),
			),
		},
		{
			name: "volumes",
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(
					sh.PipelineVolume(&corev1.Volume{
						Name: "top-level-volume",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "top-level-volume",
								ReadOnly:  true,
							},
						},
					}),
				),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageOptions(
						sh.StageVolume(&corev1.Volume{
							Name: "stage-level-volume",
							VolumeSource: corev1.VolumeSource{
								GCEPersistentDisk: &corev1.GCEPersistentDiskVolumeSource{
									PDName: "stage-level-volume",
								},
							},
						}),
						sh.StageContainerOptions(
							sh.ContainerVolumeMount("top-level-volume", "/mnt/top-level-volume"),
							sh.ContainerVolumeMount("stage-level-volume", "/mnt/stage-level-volume"),
						),
					),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
						sh.StepName("A Step With Spaces And Such"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage,
							tb.StepCommand("jx"),
							tb.StepArgs("step", "git", "merge", "--verbose"),
							tb.StepWorkingDir("/workspace/source"),
							sh.StepVolumeMount("top-level-volume", "/mnt/top-level-volume"),
							sh.StepVolumeMount("stage-level-volume", "/mnt/stage-level-volume"),
						),
						tb.Step("a-step-with-spaces-and-such", "some-image:0.0.1",
							tb.StepCommand("/bin/sh", "-c"),
							tb.StepArgs("echo hello world"),
							tb.StepWorkingDir("/workspace/source"),
							sh.StepVolumeMount("top-level-volume", "/mnt/top-level-volume"),
							sh.StepVolumeMount("stage-level-volume", "/mnt/stage-level-volume"),
						),
						tb.TaskVolume("stage-level-volume", tb.VolumeSource(corev1.VolumeSource{
							GCEPersistentDisk: &corev1.GCEPersistentDiskVolumeSource{
								PDName: "stage-level-volume",
							},
						})),
						tb.TaskVolume("top-level-volume", tb.VolumeSource(corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "top-level-volume",
								ReadOnly:  true,
							},
						})),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
		{
			name: "node_distributed_parallel_stages",
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(sh.PipelineOptionsDistributeParallelAcrossNodes(true)),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("First Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("first"))),
				sh.PipelineStage("Parent Stage",
					sh.StageParallel("A Working Stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("hello"), sh.StepArg("world"))),
					sh.StageParallel("Another stage",
						sh.StageStep(sh.StepCmd("echo"), sh.StepArg("again"))),
				),
				sh.PipelineStage("Last Stage",
					sh.StageStep(sh.StepCmd("echo"), sh.StepArg("last"))),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("first-stage", "somepipeline-first-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
					tb.PipelineTaskOutputResource("workspace", "somepipeline")),
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline", tb.From("first-stage")),
					tb.RunAfter("first-stage")),
				tb.PipelineTask("another-stage", "somepipeline-another-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline",
						tb.From("first-stage")),
					tb.RunAfter("first-stage")),
				tb.PipelineTask("last-stage", "somepipeline-last-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline", tb.From("first-stage")),
					tb.RunAfter("a-working-stage", "another-stage")),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-first-stage-1", "jx", sh.TaskStageLabel("First Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.TaskOutputs(sh.OutputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit, tb.ResourceTargetPath("source"))),
					tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo first"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
				tb.Task("somepipeline-another-stage-1", "jx", sh.TaskStageLabel("Another stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo again"), tb.StepWorkingDir("/workspace/source")),
				)),
				tb.Task("somepipeline-last-stage-1", "jx", sh.TaskStageLabel("Last Stage"), tb.TaskSpec(
					tb.TaskInputs(
						tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
							tb.ResourceTargetPath("source"))),
					tb.Step("step2", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo last"), tb.StepWorkingDir("/workspace/source")),
				)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("First Stage", sh.StructureStageTaskRef("somepipeline-first-stage-1")),
				sh.StructureStage("Parent Stage",
					sh.StructureStageParallel("A Working Stage", "Another stage"),
					sh.StructureStagePrevious("First Stage"),
				),
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage"),
				),
				sh.StructureStage("Another stage", sh.StructureStageTaskRef("somepipeline-another-stage-1"),
					sh.StructureStageDepth(1),
					sh.StructureStageParent("Parent Stage"),
				),
				sh.StructureStage("Last Stage", sh.StructureStageTaskRef("somepipeline-last-stage-1"),
					sh.StructureStagePrevious("Parent Stage")),
			),
		},
		{
			name: "tolerations",
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineOptions(
					sh.PipelineTolerations([]corev1.Toleration{{
						Key:      "some-key",
						Operator: "Exists",
						Effect:   "NoSchedule",
					}}),
					sh.PipelinePodLabels(map[string]string{
						"foo":   "bar",
						"fruit": "apple",
					}),
				),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world"),
						sh.StepName("A Step With Spaces And Such"),
					),
				),
			),
			pipeline: tb.Pipeline("somepipeline-1", "jx", tb.PipelineSpec(
				tb.PipelineTask("a-working-stage", "somepipeline-a-working-stage-1",
					tb.PipelineTaskInputResource("workspace", "somepipeline"),
				),
				tb.PipelineDeclaredResource("somepipeline", tektonv1alpha1.PipelineResourceTypeGit))),
			tasks: []*tektonv1alpha1.Task{
				tb.Task("somepipeline-a-working-stage-1", "jx", sh.TaskStageLabel("A Working Stage"),
					tb.TaskSpec(
						tb.TaskInputs(
							tb.InputsResource("workspace", tektonv1alpha1.PipelineResourceTypeGit,
								tb.ResourceTargetPath("source"))),
						tb.Step("git-merge", resolvedGitMergeImage, tb.StepCommand("jx"), tb.StepArgs("step", "git", "merge", "--verbose"), tb.StepWorkingDir("/workspace/source")),
						tb.Step("a-step-with-spaces-and-such", "some-image:0.0.1", tb.StepCommand("/bin/sh", "-c"), tb.StepArgs("echo hello world"), tb.StepWorkingDir("/workspace/source")),
					)),
			},
			structure: sh.PipelineStructure("somepipeline-1",
				sh.StructureStage("A Working Stage", sh.StructureStageTaskRef("somepipeline-a-working-stage-1")),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testVersionsDir := filepath.Join("test_data", "stable_versions")
			projectConfig, fn, err := config.LoadProjectConfig(filepath.Join("test_data", tt.name))
			if err != nil {
				t.Fatalf("Failed to parse YAML for %s: %q", tt.name, err)
			}

			if projectConfig.PipelineConfig == nil {
				t.Fatalf("PipelineConfig at %s is nil: %+v", fn, projectConfig)
			}
			if &projectConfig.PipelineConfig.Pipelines == nil {
				t.Fatalf("Pipelines at %s is nil: %+v", fn, projectConfig.PipelineConfig)
			}
			if projectConfig.PipelineConfig.Pipelines.Release == nil {
				t.Fatalf("Release at %s is nil: %+v", fn, projectConfig.PipelineConfig.Pipelines)
			}
			if projectConfig.PipelineConfig.Pipelines.Release.Pipeline == nil {
				t.Fatalf("Pipeline at %s is nil: %+v", fn, projectConfig.PipelineConfig.Pipelines.Release)
			}
			parsed := projectConfig.PipelineConfig.Pipelines.Release.Pipeline

			if d, _ := kmp.SafeDiff(tt.expected, parsed); d != "" && tt.expected != nil {
				t.Errorf("Parsed ParsedPipeline did not match expected: %s", d)
			}

			validateErr := parsed.Validate(ctx)
			if validateErr != nil && tt.validationErrorMsg == "" {
				t.Errorf("Validation failed: %s", validateErr)
			}

			if validateErr != nil && tt.validationErrorMsg != "" {
				if tt.validationErrorMsg != validateErr.Details {
					t.Errorf("Validation Error failed: '%s', '%s'", validateErr.Details, tt.validationErrorMsg)
				}
			}

			crdParams := syntax.CRDsFromPipelineParams{
				PipelineIdentifier: "somepipeline",
				BuildIdentifier:    "1",
				Namespace:          "jx",
				VersionsDir:        testVersionsDir,
				SourceDir:          "source",
				DefaultImage:       "",
				InterpretMode:      false,
			}
			pipeline, tasks, structure, err := parsed.GenerateCRDs(crdParams)

			if err != nil {
				if tt.expectedErrorMsg != "" {
					if d := cmp.Diff(tt.expectedErrorMsg, err.Error()); d != "" {
						t.Fatalf("CRD generation error did not meet expectation: %s", d)
					}
				} else {
					t.Fatalf("Error generating CRDs: %s", err)
				}
			}

			if tt.expectedErrorMsg == "" && tt.pipeline != nil {
				pipeline.TypeMeta = metav1.TypeMeta{}
				if d := cmp.Diff(tt.pipeline, pipeline); d != "" {
					t.Errorf("Generated Pipeline did not match expected: %s", d)
				}

				if err := pipeline.Spec.Validate(ctx); err != nil {
					t.Errorf("PipelineSpec.Validate(ctx) = %v", err)
				}

				for _, task := range tasks {
					task.TypeMeta = metav1.TypeMeta{}
				}
				if d, _ := kmp.SafeDiff(tt.tasks, tasks); d != "" {
					t.Errorf("Generated Tasks did not match expected: %s", d)
				}

				for _, task := range tasks {
					if err := task.Spec.Validate(ctx); err != nil {
						t.Errorf("TaskSpec.Validate(ctx) = %v", err)
					}
				}

				if tt.structure != nil {
					if d := cmp.Diff(tt.structure, structure); d != "" {
						t.Errorf("Generated PipelineStructure did not match expected: %s", d)
					}
				}
			}
		})
	}
}

func TestFailedValidation(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name          string
		expectedError error
	}{
		/* TODO: Once we figure out how to differentiate between an empty agent and no agent specified...
		{
			name: "empty_agent",
			expectedError: &apis.FieldError{
				Message: "Invalid apiVersion format: must be 'v(digits).(digits)",
				Paths:   []string{"apiVersion"},
			},
		},
		*/
		{
			name: "agent_with_both_image_and_label",
			expectedError: apis.ErrMultipleOneOf("label", "image").
				ViaField("agent"),
		},
		{
			name:          "no_stages",
			expectedError: apis.ErrMissingField("stages"),
		},
		{
			name:          "no_steps_stages_or_parallel",
			expectedError: apis.ErrMissingOneOf("steps", "stages", "parallel").ViaFieldIndex("stages", 0),
		},
		{
			name:          "steps_and_stages",
			expectedError: apis.ErrMultipleOneOf("steps", "stages", "parallel").ViaFieldIndex("stages", 0),
		},
		{
			name:          "steps_and_parallel",
			expectedError: apis.ErrMultipleOneOf("steps", "stages", "parallel").ViaFieldIndex("stages", 0),
		},
		{
			name:          "stages_and_parallel",
			expectedError: apis.ErrMultipleOneOf("steps", "stages", "parallel").ViaFieldIndex("stages", 0),
		},
		{
			name:          "step_without_command_step_or_loop",
			expectedError: apis.ErrMissingOneOf("command", "step", "loop").ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name:          "step_with_both_command_and_step",
			expectedError: apis.ErrMultipleOneOf("command", "step", "loop").ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name:          "step_with_both_command_and_loop",
			expectedError: apis.ErrMultipleOneOf("command", "step", "loop").ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "step_with_command_and_options",
			expectedError: (&apis.FieldError{
				Message: "Cannot set options for a command or a loop",
				Paths:   []string{"options"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "step_with_step_and_arguments",
			expectedError: (&apis.FieldError{
				Message: "Cannot set command-line arguments for a step or a loop",
				Paths:   []string{"args"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "step_with_loop_and_options",
			expectedError: (&apis.FieldError{
				Message: "Cannot set options for a command or a loop",
				Paths:   []string{"options"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "step_with_loop_and_arguments",
			expectedError: (&apis.FieldError{
				Message: "Cannot set command-line arguments for a step or a loop",
				Paths:   []string{"args"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "no_parent_or_stage_agent",
			expectedError: (&apis.FieldError{
				Message: "No agent specified for stage or for its parent(s)",
				Paths:   []string{"agent"},
			}).ViaFieldIndex("stages", 0),
		},
		{
			name: "top_level_timeout_without_time",
			expectedError: (&apis.FieldError{
				Message: "Timeout must be greater than zero",
				Paths:   []string{"time"},
			}).ViaField("timeout").ViaField("options"),
		},
		{
			name: "stage_timeout_without_time",
			expectedError: (&apis.FieldError{
				Message: "Timeout must be greater than zero",
				Paths:   []string{"time"},
			}).ViaField("timeout").ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "top_level_timeout_with_invalid_unit",
			expectedError: (&apis.FieldError{
				Message: "years is not a valid time unit. Valid time units are seconds, minutes, hours, days",
				Paths:   []string{"unit"},
			}).ViaField("timeout").ViaField("options"),
		},
		{
			name: "stage_timeout_with_invalid_unit",
			expectedError: (&apis.FieldError{
				Message: "years is not a valid time unit. Valid time units are seconds, minutes, hours, days",
				Paths:   []string{"unit"},
			}).ViaField("timeout").ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "top_level_timeout_with_invalid_time",
			expectedError: (&apis.FieldError{
				Message: "Timeout must be greater than zero",
				Paths:   []string{"time"},
			}).ViaField("timeout").ViaField("options"),
		},
		{
			name: "stage_timeout_with_invalid_time",
			expectedError: (&apis.FieldError{
				Message: "Timeout must be greater than zero",
				Paths:   []string{"time"},
			}).ViaField("timeout").ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "top_level_retry_with_invalid_count",
			expectedError: (&apis.FieldError{
				Message: "Retry count cannot be negative",
				Paths:   []string{"retry"},
			}).ViaField("options"),
		},
		{
			name: "stage_retry_with_invalid_count",
			expectedError: (&apis.FieldError{
				Message: "Retry count cannot be negative",
				Paths:   []string{"retry"},
			}).ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "stash_without_name",
			expectedError: (&apis.FieldError{
				Message: "The stash name must be provided",
				Paths:   []string{"name"},
			}).ViaField("stash").ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "stash_without_files",
			expectedError: (&apis.FieldError{
				Message: "files to stash must be provided",
				Paths:   []string{"files"},
			}).ViaField("stash").ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "unstash_without_name",
			expectedError: (&apis.FieldError{
				Message: "The unstash name must be provided",
				Paths:   []string{"name"},
			}).ViaField("unstash").ViaField("options").ViaFieldIndex("stages", 0),
		},
		{
			name: "blank_stage_name",
			expectedError: (&apis.FieldError{
				Message: "Stage name must contain at least one ASCII letter",
				Paths:   []string{"name"},
			}).ViaFieldIndex("stages", 0),
		},
		{
			name: "stage_name_duplicates",
			expectedError: &apis.FieldError{
				Message: "Stage names must be unique",
				Details: "The following stage names are used more than once: 'A Working Stage'",
			},
		},
		{
			name: "stage_name_duplicates_deeply_nested",
			expectedError: &apis.FieldError{
				Message: "Stage names must be unique",
				Details: "The following stage names are used more than once: 'Stage With Stages'",
			},
		},
		{
			name: "stage_name_duplicates_nested",
			expectedError: &apis.FieldError{
				Message: "Stage names must be unique",
				Details: "The following stage names are used more than once: 'Stage With Stages'",
			},
		},
		{
			name: "stage_name_duplicates_sequential",
			expectedError: &apis.FieldError{
				Message: "Stage names must be unique",
				Details: "The following stage names are used more than once: 'A Working title 2', 'A Working title'",
			},
		},
		{
			name: "stage_name_duplicates_unique_in_scope",
			expectedError: &apis.FieldError{
				Message: "Stage names must be unique",
				Details: "The following stage names are used more than once: 'A Working title 1', 'A Working title 2'",
			},
		},
		{
			name:          "loop_without_variable",
			expectedError: apis.ErrMissingField("variable").ViaField("loop").ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name:          "loop_without_steps",
			expectedError: apis.ErrMissingField("steps").ViaField("loop").ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name:          "loop_without_values",
			expectedError: apis.ErrMissingField("values").ViaField("loop").ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "top_level_container_options_with_command",
			expectedError: (&apis.FieldError{
				Message: "Command cannot be specified in containerOptions",
				Paths:   []string{"command"},
			}).ViaField("containerOptions").ViaField("options"),
		},
		{
			name:          "unknown_field",
			expectedError: errors.New("Validation failures in YAML file test_data/validation_failures/unknown_field/jenkins-x.yml:\npipelineConfig: Additional property banana is not allowed"),
		},
		{
			name: "comment_field",
			expectedError: (&apis.FieldError{
				Message: "the comment field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
				Paths:   []string{"comment"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "groovy_field",
			expectedError: (&apis.FieldError{
				Message: "the groovy field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
				Paths:   []string{"groovy"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "when_field",
			expectedError: (&apis.FieldError{
				Message: "the when field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
				Paths:   []string{"when"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "container_field",
			expectedError: (&apis.FieldError{
				Message: "the container field is deprecated - please use image instead",
				Paths:   []string{"container"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "legacy_steps_field",
			expectedError: (&apis.FieldError{
				Message: "the steps field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it and list the nested stages sequentially instead.",
				Paths:   []string{"steps"},
			}).ViaFieldIndex("steps", 0).ViaFieldIndex("stages", 0),
		},
		{
			name: "agent_dir_field",
			expectedError: (&apis.FieldError{
				Message: "the dir field is only valid in legacy build packs, not in jenkins-x.yml. Please remove it.",
				Paths:   []string{"dir"},
			}).ViaField("agent"),
		},
		{
			name: "agent_container_field",
			expectedError: (&apis.FieldError{
				Message: "the container field is deprecated - please use image instead",
				Paths:   []string{"container"},
			}).ViaField("agent"),
		},
		{
			name: "duplicate_step_names",
			expectedError: (&apis.FieldError{
				Message: "step names within a stage must be unique",
				Details: "The following step names in the stage A Working Stage are used more than once: A Step With Spaces And Such, Another Step Name",
				Paths:   []string{"steps"},
			}).ViaFieldIndex("stages", 0),
		},
		{
			name:          "volume_missing_name",
			expectedError: apis.ErrMissingField("name").ViaFieldIndex("volumes", 0).ViaField("options"),
		},
		{
			name: "top_level_missing_volume",
			expectedError: (&apis.FieldError{
				Message: "Volume mount name not-present not found in volumes for stage or pipeline",
				Paths:   []string{"name"},
			}).ViaFieldIndex("volumeMounts", 0).ViaField("containerOptions").ViaField("options"),
		},
		{
			name: "volume_does_not_exist",
			expectedError: (&apis.FieldError{
				Message: "PVC does-not-exist does not exist, so cannot be used as a volume",
				Paths:   []string{"claimName"},
			}).ViaFieldIndex("volumes", 0).ViaField("options"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectConfig, fn, err := config.LoadProjectConfig(filepath.Join("test_data", "validation_failures", tt.name))
			if err != nil && err.Error() != tt.expectedError.Error() {
				t.Fatalf("Failed to parse YAML for %s: %q", tt.name, err)
			}
			if _, ok := tt.expectedError.(*apis.FieldError); ok {

				if projectConfig.PipelineConfig == nil {
					t.Fatalf("PipelineConfig at %s is nil: %+v", fn, projectConfig)
				}
				if &projectConfig.PipelineConfig.Pipelines == nil {
					t.Fatalf("Pipelines at %s is nil: %+v", fn, projectConfig.PipelineConfig)
				}
				if projectConfig.PipelineConfig.Pipelines.Release == nil {
					t.Fatalf("Release at %s is nil: %+v", fn, projectConfig.PipelineConfig.Pipelines)
				}
				if projectConfig.PipelineConfig.Pipelines.Release.Pipeline == nil {
					t.Fatalf("Pipeline at %s is nil: %+v", fn, projectConfig.PipelineConfig.Pipelines.Release)
				}
				parsed := projectConfig.PipelineConfig.Pipelines.Release.Pipeline
				kubeClient := kubefake.NewSimpleClientset()

				err = parsed.ValidateInCluster(ctx, kubeClient, "jx")

				if err == nil {
					t.Fatalf("Expected a validation failure but none occurred")
				}

				if d := cmp.Diff(tt.expectedError, err, cmp.AllowUnexported(apis.FieldError{})); d != "" {
					t.Fatalf("Validation error did not meet expectation: %s", d)
				}
			}
		})
	}
}

func getOverridesTestPipeline() *syntax.ParsedPipeline {
	return sh.ParsedPipeline(
		sh.PipelineAgent("some-image"),
		sh.PipelineStage("A Working Stage",
			sh.StageStep(
				sh.StepCmd("echo"),
				sh.StepArg("hello"), sh.StepArg("world")),
		),
		sh.PipelineStage("Another stage",
			sh.StageStep(
				sh.StepCmd("echo"),
				sh.StepArg("again"))),
	)
}

func getOverridesTestVolume() *corev1.Volume {
	return &corev1.Volume{
		Name: "stage-volume",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "stage-volume",
				ReadOnly:  true,
			},
		},
	}
}

func getOverridesTestContainerOptions() *corev1.Container {
	return &corev1.Container{
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"cpu":    resource.MustParse("100m"),
				"memory": resource.MustParse("128Mi"),
			},
		},
	}
}

func TestApplyNonStepOverridesToPipeline(t *testing.T) {
	tests := []struct {
		name     string
		override *syntax.PipelineOverride
		expected *syntax.ParsedPipeline
	}{
		{
			name: "volume-on-whole-pipeline",
			override: &syntax.PipelineOverride{
				Volumes: []*corev1.Volume{getOverridesTestVolume()},
			},
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(sh.PipelineVolume(getOverridesTestVolume())),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
		},
		{
			name: "volume-on-single-stage",
			override: &syntax.PipelineOverride{
				Stage:   "Another stage",
				Volumes: []*corev1.Volume{getOverridesTestVolume()},
			},
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageOptions(sh.StageVolume(getOverridesTestVolume())),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
		},
		{
			name: "containerOptions-on-whole-pipeline",
			override: &syntax.PipelineOverride{
				ContainerOptions: getOverridesTestContainerOptions(),
			},
			expected: sh.ParsedPipeline(
				sh.PipelineOptions(
					sh.PipelineContainerOptions(
						sh.ContainerResourceLimits("100m", "128Mi"),
					),
				),
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
		},
		{
			name: "containerOptions-on-single-stage",
			override: &syntax.PipelineOverride{
				Stage:            "Another stage",
				ContainerOptions: getOverridesTestContainerOptions(),
			},
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageOptions(
						sh.StageContainerOptions(
							sh.ContainerResourceLimits("100m", "128Mi"),
						),
					),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
		},
		{
			name: "agent-on-whole-pipeline",
			override: &syntax.PipelineOverride{
				Agent: &syntax.Agent{
					Image: "some-other-image",
				},
			},
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-other-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
		},
		{
			name: "agent-on-single-stage",
			override: &syntax.PipelineOverride{
				Stage: "Another stage",
				Agent: &syntax.Agent{
					Image: "some-other-image",
				},
			},
			expected: sh.ParsedPipeline(
				sh.PipelineAgent("some-image"),
				sh.PipelineStage("A Working Stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("hello"), sh.StepArg("world")),
				),
				sh.PipelineStage("Another stage",
					sh.StageAgent("some-other-image"),
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("again"))),
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newPipeline := syntax.ApplyNonStepOverridesToPipeline(getOverridesTestPipeline(), tt.override)

			if d, _ := kmp.SafeDiff(tt.expected, newPipeline); d != "" {
				t.Errorf("Overridden pipeline did not match expected: %s", d)
			}
		})
	}
}

func TestRfc1035LabelMangling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "unmodified",
			input:    "unmodified",
			expected: "unmodified-suffix",
		},
		{
			name:     "spaces",
			input:    "A Simple Test.",
			expected: "a-simple-test-suffix",
		},
		{
			name:     "no leading digits",
			input:    "0123456789no-leading-digits",
			expected: "no-leading-digits-suffix",
		},
		{
			name:     "no leading hyphens",
			input:    "----no-leading-hyphens",
			expected: "no-leading-hyphens-suffix",
		},
		{
			name:     "no consecutive hyphens",
			input:    "no--consecutive- hyphens",
			expected: "no-consecutive-hyphens-suffix",
		},
		{
			name:     "no trailing hyphens",
			input:    "no-trailing-hyphens----",
			expected: "no-trailing-hyphens-suffix",
		},
		{
			name:     "no symbols",
			input:    "&$^#@(*&$^-whoops",
			expected: "whoops-suffix",
		},
		{
			name:     "no unprintable characters",
			input:    "a\n\t\x00b",
			expected: "ab-suffix",
		},
		{
			name:     "no unicode",
			input:    "japan-日本",
			expected: "japan-suffix",
		},
		{
			name:     "no non-bmp characters",
			input:    "happy 😃",
			expected: "happy-suffix",
		},
		{
			name:     "truncated to 63",
			input:    "a0123456789012345678901234567890123456789012345678901234567890123456789",
			expected: "a0123456789012345678901234567890123456789012345678901234-suffix",
		},
		{
			name:     "truncated to 62",
			input:    "a012345678901234567890123456789012345678901234567890123-567890123456789",
			expected: "a012345678901234567890123456789012345678901234567890123-suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mangled := syntax.MangleToRfc1035Label(tt.input, "suffix")
			if d := cmp.Diff(tt.expected, mangled); d != "" {
				t.Fatalf("Mangled output did not match expected output: %s", d)
			}
		})
	}
}

func TestParsedPipelineHelpers(t *testing.T) {
	input := sh.ParsedPipeline(
		sh.PipelineAgent("some-image"),
		sh.PipelineOptions(
			sh.PipelineOptionsRetry(5),
			sh.PipelineOptionsTimeout(30, syntax.TimeoutUnitSeconds),
			sh.PipelineVolume(&corev1.Volume{Name: "banana"}),
		),
		sh.PipelineEnvVar("ANIMAL", "MONKEY"),
		sh.PipelineEnvVar("FRUIT", "BANANA"),
		sh.PipelinePost(syntax.PostConditionSuccess,
			sh.PostAction("mail", map[string]string{
				"to":      "foo@bar.com",
				"subject": "Yay, it passed",
			})),
		sh.PipelinePost(syntax.PostConditionFailure,
			sh.PostAction("slack", map[string]string{
				"whatever": "the",
				"slack":    "config",
				"actually": "is. =)",
			})),
		sh.PipelineStage("A Working Stage",
			sh.StageOptions(
				sh.StageOptionsWorkspace(customWorkspace),
				sh.StageOptionsStash("some-name", "**/*"),
				sh.StageOptionsUnstash("some-name", ""),
				sh.StageOptionsTimeout(15, syntax.TimeoutUnitMinutes),
				sh.StageOptionsRetry(2),
				sh.StageVolume(&corev1.Volume{Name: "apple"}),
				sh.StageVolume(&corev1.Volume{Name: "orange"}),
			),
			sh.StageStep(
				sh.StepCmd("echo"),
				sh.StepArg("hello"),
				sh.StepArg("world"),
			),
		),
		sh.PipelineStage("Parent Stage",
			sh.StageParallel("First Nested Stage",
				sh.StageAgent("some-other-image"),
				sh.StageStep(
					sh.StepCmd("echo"),
					sh.StepArg("hello"),
					sh.StepArg("world"),
					sh.StepAgent("some-other-image"),
				),
				sh.StageEnvVar("STAGE_VAR_ONE", "some value"),
				sh.StageEnvVar("STAGE_VAR_TWO", "some other value"),
				sh.StagePost(syntax.PostConditionAlways,
					sh.PostAction("junit", map[string]string{
						"pattern": "target/surefire-reports/**/*.xml",
					}),
				),
			),
			sh.StageParallel("Nested In Parallel",
				sh.StageSequential("Another stage",
					sh.StageStep(
						sh.StepLoop("SOME_VAR", []string{"a", "b", "c"},
							sh.LoopStep(
								sh.StepCmd("echo"),
								sh.StepArg("SOME_VAR is ${SOME_VAR}"),
							),
						),
					),
				),
				sh.StageSequential("Some other stage",
					sh.StageStep(
						sh.StepCmd("echo"),
						sh.StepArg("otherwise"),
						sh.StepDir(customWorkspace),
					),
					sh.StageStep(
						sh.StepStep("some-step"),
						sh.StepOptions(map[string]string{"first": "arg", "second": "arg"}),
					),
				),
			),
		),
	)

	expected := &syntax.ParsedPipeline{
		Agent: &syntax.Agent{
			Image: "some-image",
		},
		Options: &syntax.RootOptions{
			Retry: 5,
			Timeout: &syntax.Timeout{
				Time: 30,
				Unit: syntax.TimeoutUnitSeconds,
			},
			Volumes: []*corev1.Volume{{
				Name: "banana",
			}},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "ANIMAL",
				Value: "MONKEY",
			},
			{
				Name:  "FRUIT",
				Value: "BANANA",
			},
		},
		Post: []syntax.Post{
			{
				Condition: "success",
				Actions: []syntax.PostAction{{
					Name: "mail",
					Options: map[string]string{
						"to":      "foo@bar.com",
						"subject": "Yay, it passed",
					},
				}},
			},
			{
				Condition: "failure",
				Actions: []syntax.PostAction{{
					Name: "slack",
					Options: map[string]string{
						"whatever": "the",
						"slack":    "config",
						"actually": "is. =)",
					},
				}},
			},
		},
		Stages: []syntax.Stage{
			{
				Name: "A Working Stage",
				Options: &syntax.StageOptions{
					Workspace: &customWorkspace,
					Stash: &syntax.Stash{
						Name:  "some-name",
						Files: "**/*",
					},
					Unstash: &syntax.Unstash{
						Name: "some-name",
					},
					RootOptions: &syntax.RootOptions{
						Timeout: &syntax.Timeout{
							Time: 15,
							Unit: syntax.TimeoutUnitMinutes,
						},
						Retry: 2,
						Volumes: []*corev1.Volume{{
							Name: "apple",
						}, {
							Name: "orange",
						}},
					},
				},
				Steps: []syntax.Step{{
					Command:   "echo",
					Arguments: []string{"hello", "world"},
				}},
			},
			{
				Name: "Parent Stage",
				Parallel: []syntax.Stage{
					{
						Name: "First Nested Stage",
						Agent: &syntax.Agent{
							Image: "some-other-image",
						},
						Steps: []syntax.Step{{
							Command:   "echo",
							Arguments: []string{"hello", "world"},
							Agent: &syntax.Agent{
								Image: "some-other-image",
							},
						}},
						Env: []corev1.EnvVar{
							{
								Name:  "STAGE_VAR_ONE",
								Value: "some value",
							},
							{
								Name:  "STAGE_VAR_TWO",
								Value: "some other value",
							},
						},
						Post: []syntax.Post{{
							Condition: "always",
							Actions: []syntax.PostAction{{
								Name: "junit",
								Options: map[string]string{
									"pattern": "target/surefire-reports/**/*.xml",
								},
							}},
						}},
					},
					{
						Name: "Nested In Parallel",
						Stages: []syntax.Stage{
							{
								Name: "Another stage",
								Steps: []syntax.Step{{
									Loop: &syntax.Loop{
										Variable: "SOME_VAR",
										Values:   []string{"a", "b", "c"},
										Steps: []syntax.Step{{
											Command:   "echo",
											Arguments: []string{"SOME_VAR is ${SOME_VAR}"},
										}},
									},
								}},
							},
							{
								Name: "Some other stage",
								Steps: []syntax.Step{
									{
										Command:   "echo",
										Arguments: []string{"otherwise"},
										Dir:       customWorkspace,
									},
									{
										Step:    "some-step",
										Options: map[string]string{"first": "arg", "second": "arg"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if d := cmp.Diff(expected, input); d != "" {
		t.Fatalf("ParsedPipeline diff -want, +got: %v", d)
	}
}
