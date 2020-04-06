package apps

// AppType defines the type of the App
type AppType uint32

// String returns a string representation of the App type
func (a AppType) String() string {
	switch a {
	case Controller:
		return "controller"
	case PipelineExtension:
		return "pipeline-extension"
	default:
		return "unknown"
	}
}

// These are the different App types
const (
	// AppPodTemplateName the name of the pod template to store default settings for pods executing an App extension step
	AppPodTemplateName = "app-extension"

	// AppTypeLabel label used to store the app type in the App CRD.
	AppTypeLabel = "jenkins.io/app-type"

	// Controller is an App type which installs a controller into the cluster.
	Controller AppType = iota

	// PipelineExtension is an App type which wants to modify the build pipeline by being executed as part of the meta pipeline.
	PipelineExtension
)

// AllTypes exposes all App types in a slice
var AllTypes = []AppType{
	Controller,
	PipelineExtension,
}
