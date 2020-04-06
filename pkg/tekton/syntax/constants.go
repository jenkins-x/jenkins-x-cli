package syntax

const (
	// TektonAPIVersion the APIVersion for using Tekton
	TektonAPIVersion = "tekton.dev/v1alpha1"

	// LabelStageName - the name for the label that will have the stage name on the Task.
	LabelStageName = "jenkins.io/task-stage-name"

	// DefaultStageNameForBuildPack - the name we use for the single stage created from build packs currently.
	DefaultStageNameForBuildPack = "from-build-pack"

	// KanikoDockerImage - the default image used for Kaniko builds
	KanikoDockerImage = "gcr.io/kaniko-project/executor:v0.19.0"

	// DefaultContainerImage - the default image used for pipelines if none is specified.
	DefaultContainerImage = "gcr.io/jenkinsxio/builder-maven"
)
