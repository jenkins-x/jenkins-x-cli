package syntax_helpers_test

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/test/builder"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineStructureOp is an operation used in generating a PipelineStructure
type PipelineStructureOp func(structure *v1.PipelineStructure)

// PipelineStructureStageOp is an operation used in generating a PipelineStructureStage
type PipelineStructureStageOp func(stage *v1.PipelineStructureStage)

// PipelineStructure creates a PipelineStructure
func PipelineStructure(name string, ops ...PipelineStructureOp) *v1.PipelineStructure {
	s := &v1.PipelineStructure{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, op := range ops {
		op(s)
	}

	return s
}

// StructurePipelineRunRef adds a run reference to the structure
func StructurePipelineRunRef(name string) PipelineStructureOp {
	return func(structure *v1.PipelineStructure) {
		structure.PipelineRunRef = &name
	}
}

// StructureStage adds a stage to the structure
func StructureStage(name string, ops ...PipelineStructureStageOp) PipelineStructureOp {
	return func(structure *v1.PipelineStructure) {
		stage := v1.PipelineStructureStage{Name: name}

		for _, op := range ops {
			op(&stage)
		}

		structure.Stages = append(structure.Stages, stage)
	}
}

// StructureStageTaskRef adds a task ref to the stage
func StructureStageTaskRef(name string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.TaskRef = &name
	}
}

// StructureStageTaskRunRef adds a task run ref to the stage
func StructureStageTaskRunRef(name string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.TaskRunRef = &name
	}
}

// StructureStageDepth sets the depth on the stage
func StructureStageDepth(depth int8) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.Depth = depth
	}
}

// StructureStageParent sets the parent stage for the stage
func StructureStageParent(parent string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.Parent = &parent
	}
}

// StructureStagePrevious sets the previous stage for the stage
func StructureStagePrevious(previous string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.Previous = &previous
	}
}

// StructureStageNext sets the next stage for the stage
func StructureStageNext(Next string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.Next = &Next
	}
}

// StructureStageStages sets the nested sequential stages for the stage
func StructureStageStages(stages ...string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.Stages = append(stage.Stages, stages...)
	}
}

// StructureStageParallel sets the nested parallel stages for the stage
func StructureStageParallel(stages ...string) PipelineStructureStageOp {
	return func(stage *v1.PipelineStructureStage) {
		stage.Parallel = append(stage.Parallel, stages...)
	}
}

// PipelineOp is an operation on a ParsedPipeline
type PipelineOp func(*syntax.ParsedPipeline)

// PipelineOptionsOp is an operation on RootOptions
type PipelineOptionsOp func(*syntax.RootOptions)

// PipelinePostOp is an operation on Post
type PipelinePostOp func(*syntax.Post)

// StageOp is an operation on a Stage
type StageOp func(*syntax.Stage)

// StageOptionsOp is an operation on StageOptions
type StageOptionsOp func(*syntax.StageOptions)

// StepOp is an operation on a step
type StepOp func(*syntax.Step)

// LoopOp is an operation on a Loop
type LoopOp func(*syntax.Loop)

// ParsedPipeline creates a ParsedPipeline from the provided operations
func ParsedPipeline(ops ...PipelineOp) *syntax.ParsedPipeline {
	s := &syntax.ParsedPipeline{}

	for _, op := range ops {
		op(s)
	}

	return s
}

// PipelineAgent sets the agent for the pipeline
func PipelineAgent(image string) PipelineOp {
	return func(parsed *syntax.ParsedPipeline) {
		parsed.Agent = &syntax.Agent{
			Image: image,
		}
	}
}

// PipelineOptions sets the RootOptions for the pipeline
func PipelineOptions(ops ...PipelineOptionsOp) PipelineOp {
	return func(parsed *syntax.ParsedPipeline) {
		parsed.Options = &syntax.RootOptions{}

		for _, op := range ops {
			op(parsed.Options)
		}
	}
}

// PipelineVolume adds a volume to the RootOptions for the pipeline
func PipelineVolume(volume *corev1.Volume) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		if options.Volumes == nil {
			options.Volumes = []*corev1.Volume{}
		}
		options.Volumes = append(options.Volumes, volume)
	}
}

// StageVolume adds a volume to the StageOptions for the stage
func StageVolume(volume *corev1.Volume) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		if options.RootOptions == nil {
			options.RootOptions = &syntax.RootOptions{}
		}
		if options.Volumes == nil {
			options.Volumes = []*corev1.Volume{}
		}
		options.Volumes = append(options.Volumes, volume)
	}
}

// PipelineContainerOptions sets the containerOptions for the pipeline
func PipelineContainerOptions(ops ...builder.ContainerOp) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		options.ContainerOptions = &corev1.Container{}

		for _, op := range ops {
			op(options.ContainerOptions)
		}
	}
}

// PipelineTolerations sets the tolerations for the pipeline
func PipelineTolerations(tolerations []corev1.Toleration) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		options.Tolerations = append(options.Tolerations, tolerations...)
	}
}

// PipelinePodLabels sets the optional pod labels for the pipeline
func PipelinePodLabels(labels map[string]string) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		options.PodLabels = util.MergeMaps(options.PodLabels, labels)
	}
}

// StageContainerOptions sets the containerOptions for a stage
func StageContainerOptions(ops ...builder.ContainerOp) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		if options.RootOptions == nil {
			options.RootOptions = &syntax.RootOptions{}
		}
		options.ContainerOptions = &corev1.Container{}

		for _, op := range ops {
			op(options.ContainerOptions)
		}
	}
}

// PipelineDir sets the default working directory for the pipeline
func PipelineDir(dir string) PipelineOp {
	return func(pipeline *syntax.ParsedPipeline) {
		pipeline.WorkingDir = &dir
	}
}

// StageDir sets the default working directory for the stage
func StageDir(dir string) StageOp {
	return func(stage *syntax.Stage) {
		stage.WorkingDir = &dir
	}
}

// ContainerResourceLimits sets the resource limits for container options
func ContainerResourceLimits(cpus, memory string) builder.ContainerOp {
	return func(container *corev1.Container) {
		cpuQuantity, _ := resource.ParseQuantity(cpus)
		memoryQuantity, _ := resource.ParseQuantity(memory)
		container.Resources.Limits = corev1.ResourceList{
			"cpu":    cpuQuantity,
			"memory": memoryQuantity,
		}
	}
}

// ContainerResourceRequests sets the resource requests for container options
func ContainerResourceRequests(cpus, memory string) builder.ContainerOp {
	return func(container *corev1.Container) {
		cpuQuantity, _ := resource.ParseQuantity(cpus)
		memoryQuantity, _ := resource.ParseQuantity(memory)
		container.Resources.Requests = corev1.ResourceList{
			"cpu":    cpuQuantity,
			"memory": memoryQuantity,
		}
	}
}

// ContainerSecurityContext sets the security context for container options
func ContainerSecurityContext(privileged bool) builder.ContainerOp {
	return func(container *corev1.Container) {
		container.SecurityContext = &corev1.SecurityContext{
			Privileged: &privileged,
		}
	}
}

// ContainerVolumeMount adds a VolumeMount to the container options
func ContainerVolumeMount(name string, mountPath string) builder.ContainerOp {
	return func(container *corev1.Container) {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      name,
			MountPath: mountPath,
		})
	}
}

// StepVolumeMount adds a VolumeMount to the container options on a step
func StepVolumeMount(name string, mountPath string) builder.StepOp {
	return func(step *v1alpha1.Step) {
		step.VolumeMounts = append(step.VolumeMounts, corev1.VolumeMount{
			Name:      name,
			MountPath: mountPath,
		})
	}
}

// StepResourceLimits sets the resource limits for container options on a step
func StepResourceLimits(cpus, memory string) builder.StepOp {
	return func(step *v1alpha1.Step) {
		cpuQuantity, _ := resource.ParseQuantity(cpus)
		memoryQuantity, _ := resource.ParseQuantity(memory)
		step.Resources.Limits = corev1.ResourceList{
			"cpu":    cpuQuantity,
			"memory": memoryQuantity,
		}
	}
}

// StepResourceRequests sets the resource requests for container options on a step
func StepResourceRequests(cpus, memory string) builder.StepOp {
	return func(step *v1alpha1.Step) {
		cpuQuantity, _ := resource.ParseQuantity(cpus)
		memoryQuantity, _ := resource.ParseQuantity(memory)
		step.Resources.Requests = corev1.ResourceList{
			"cpu":    cpuQuantity,
			"memory": memoryQuantity,
		}
	}
}

// StepSecurityContext sets the security context for container options on a step
func StepSecurityContext(privileged bool) builder.StepOp {
	return func(step *v1alpha1.Step) {
		step.SecurityContext = &corev1.SecurityContext{
			Privileged: &privileged,
		}
	}
}

// EnvVarFrom adds an environment variable using EnvVarSource to the container options
func EnvVarFrom(name string, source *corev1.EnvVarSource) builder.ContainerOp {
	return func(container *corev1.Container) {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:      name,
			ValueFrom: source,
		})
	}
}

// EnvVar adds an environment variable with a value to the container options
func EnvVar(name string, value string) builder.ContainerOp {
	return func(container *corev1.Container) {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
}

// PipelineOptionsTimeout sets the timeout for the pipeline
func PipelineOptionsTimeout(time int64, unit syntax.TimeoutUnit) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		options.Timeout = &syntax.Timeout{
			Time: time,
			Unit: unit,
		}
	}
}

// PipelineOptionsRetry sets the retry count for the pipeline
func PipelineOptionsRetry(count int8) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		options.Retry = count
	}
}

// PipelineOptionsDistributeParallelAcrossNodes sets the value for distributeParallelAcrossNodes
func PipelineOptionsDistributeParallelAcrossNodes(b bool) PipelineOptionsOp {
	return func(options *syntax.RootOptions) {
		options.DistributeParallelAcrossNodes = b
	}
}

// PipelineEnvVar add an environment variable, with specified name and value, to the pipeline.
func PipelineEnvVar(name, value string) PipelineOp {
	return func(parsed *syntax.ParsedPipeline) {
		parsed.Env = append(parsed.GetEnv(), corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
}

// PipelinePost adds a post condition to the pipeline
func PipelinePost(condition syntax.PostCondition, ops ...PipelinePostOp) PipelineOp {
	return func(parsed *syntax.ParsedPipeline) {
		post := syntax.Post{
			Condition: condition,
		}

		for _, op := range ops {
			op(&post)
		}

		parsed.Post = append(parsed.Post, post)
	}
}

// PipelineStage adds a stage to the pipeline
func PipelineStage(name string, ops ...StageOp) PipelineOp {
	return func(parsed *syntax.ParsedPipeline) {
		s := syntax.Stage{
			Name: name,
		}

		for _, op := range ops {
			op(&s)
		}
		parsed.Stages = append(parsed.Stages, s)
	}
}

// PostAction adds a post action to a post condition
func PostAction(name string, options map[string]string) PipelinePostOp {
	return func(post *syntax.Post) {
		post.Actions = append(post.Actions, syntax.PostAction{
			Name:    name,
			Options: options,
		})
	}
}

// StageAgent sets the image/agent for a stage
func StageAgent(image string) StageOp {
	return func(stage *syntax.Stage) {
		stage.Agent = &syntax.Agent{
			Image: image,
		}
	}
}

// StageOptions sets the StageOptions for a stage
func StageOptions(ops ...StageOptionsOp) StageOp {
	return func(stage *syntax.Stage) {
		stage.Options = &syntax.StageOptions{}

		for _, op := range ops {
			op(stage.Options)
		}
	}
}

// StageOptionsTimeout sets the timeout for a stage
func StageOptionsTimeout(time int64, unit syntax.TimeoutUnit) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		if options.RootOptions == nil {
			options.RootOptions = &syntax.RootOptions{}
		}
		options.Timeout = &syntax.Timeout{
			Time: time,
			Unit: unit,
		}
	}
}

// StageOptionsRetry sets the retry count for a stage
func StageOptionsRetry(count int8) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		if options.RootOptions == nil {
			options.RootOptions = &syntax.RootOptions{}
		}
		options.Retry = count
	}
}

// StageOptionsWorkspace sets the workspace for a stage
func StageOptionsWorkspace(ws string) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		options.Workspace = &ws
	}
}

// StageOptionsStash adds a stash to the stage
func StageOptionsStash(name, files string) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		options.Stash = &syntax.Stash{
			Name:  name,
			Files: files,
		}
	}
}

// StageOptionsUnstash adds an unstash to the stage
func StageOptionsUnstash(name, dir string) StageOptionsOp {
	return func(options *syntax.StageOptions) {
		options.Unstash = &syntax.Unstash{
			Name: name,
		}
		if dir != "" {
			options.Unstash.Dir = dir
		}
	}
}

// StageEnvVar add an environment variable, with specified name and value, to the stage.
func StageEnvVar(name, value string) StageOp {
	return func(stage *syntax.Stage) {
		stage.Env = append(stage.GetEnv(), corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
}

// StagePost adds a post condition to the stage
func StagePost(condition syntax.PostCondition, ops ...PipelinePostOp) StageOp {
	return func(stage *syntax.Stage) {
		post := syntax.Post{
			Condition: condition,
		}

		for _, op := range ops {
			op(&post)
		}

		stage.Post = append(stage.Post, post)
	}
}

// StepAgent sets the agent for a step
func StepAgent(image string) StepOp {
	return func(step *syntax.Step) {
		step.Agent = &syntax.Agent{
			Image: image,
		}
	}
}

// StepImage sets the image for a step
func StepImage(image string) StepOp {
	return func(step *syntax.Step) {
		step.Image = image
	}
}

// StepCmd sets the command for a step
func StepCmd(cmd string) StepOp {
	return func(step *syntax.Step) {
		step.Command = cmd
	}
}

// StepName sets the name for a step
func StepName(name string) StepOp {
	return func(step *syntax.Step) {
		step.Name = name
	}
}

// StepArg sets the arguments for a step
func StepArg(arg string) StepOp {
	return func(step *syntax.Step) {
		step.Arguments = append(step.Arguments, arg)
	}
}

// StepStep sets the alias step for a step
func StepStep(s string) StepOp {
	return func(step *syntax.Step) {
		step.Step = s
	}
}

// StepOptions sets the alias step options for a step
func StepOptions(options map[string]string) StepOp {
	return func(step *syntax.Step) {
		step.Options = options
	}
}

// StepDir sets the working dir for a step
func StepDir(dir string) StepOp {
	return func(step *syntax.Step) {
		step.Dir = dir
	}
}

// StepLoop adds a loop to the step
func StepLoop(variable string, values []string, ops ...LoopOp) StepOp {
	return func(step *syntax.Step) {
		loop := &syntax.Loop{
			Variable: variable,
			Values:   values,
		}

		for _, op := range ops {
			op(loop)
		}

		step.Loop = loop
	}
}

// StepEnvVar add an environment variable, with specified name and value, to the step.
func StepEnvVar(name, value string) StepOp {
	return func(step *syntax.Step) {
		step.Env = append(step.Env, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}
}

// LoopStep adds a step to the loop
func LoopStep(ops ...StepOp) LoopOp {
	return func(loop *syntax.Loop) {
		step := syntax.Step{}

		for _, op := range ops {
			op(&step)
		}

		loop.Steps = append(loop.Steps, step)
	}
}

// StageStep adds a step to the stage
func StageStep(ops ...StepOp) StageOp {
	return func(stage *syntax.Stage) {
		step := syntax.Step{}

		for _, op := range ops {
			op(&step)
		}

		stage.Steps = append(stage.Steps, step)
	}
}

// StageParallel adds a nested parallel stage to the stage
func StageParallel(name string, ops ...StageOp) StageOp {
	return func(stage *syntax.Stage) {
		n := syntax.Stage{Name: name}

		for _, op := range ops {
			op(&n)
		}

		stage.Parallel = append(stage.Parallel, n)
	}
}

// StageSequential adds a nested sequential stage to the stage
func StageSequential(name string, ops ...StageOp) StageOp {
	return func(stage *syntax.Stage) {
		n := syntax.Stage{Name: name}

		for _, op := range ops {
			op(&n)
		}

		stage.Stages = append(stage.Stages, n)
	}
}

// TaskStageLabel sets the stage label on the task
func TaskStageLabel(value string) builder.TaskOp {
	return func(t *v1alpha1.Task) {
		if t.ObjectMeta.Labels == nil {
			t.ObjectMeta.Labels = map[string]string{}
		}
		t.ObjectMeta.Labels[syntax.LabelStageName] = syntax.MangleToRfc1035Label(value, "")
	}
}

// OutputsResource adds a resource, with specified name and type, to the Outputs.
// Any number of TaskResource modifier can be passed to transform it.
func OutputsResource(name string, resourceType v1alpha1.PipelineResourceType, ops ...builder.TaskResourceOp) builder.OutputsOp {
	return func(i *v1alpha1.Outputs) {
		r := &v1alpha1.TaskResource{
			ResourceDeclaration: v1alpha1.ResourceDeclaration{
				Name: name,
				Type: resourceType,
			}}
		for _, op := range ops {
			op(r)
		}
		i.Resources = append(i.Resources, *r)
	}
}
