package builds

const (
	// LabelBuildName the label used on a pod for the build name
	LabelBuildName    = "build.knative.dev/buildName"
	LabelOldBuildName = "build-name"

	// LabelPipelineRunName the label used on a pod created via Build Pipeline for the build name.
	LabelPipelineRunName = "tekton.dev/pipelineRun"

	// LabelTaskName the label used on a pod created via Tekton for the task name
	LabelTaskName = "tekton.dev/task"

	// LabelTaskRunName the label used on a pod created via Tekton for the taskrun name
	LabelTaskRunName = "tekton.dev/taskRun"
)
