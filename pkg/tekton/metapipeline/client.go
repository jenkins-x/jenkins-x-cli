package metapipeline

import (
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/tekton"
)

// PipelineCreateParam wraps all parameters needed for creating the meta pipeline CRDs.
type PipelineCreateParam struct {
	// PullRef contains the information about the source repo as well as which state (commit) needs to be checked out.
	// This parameter is required.
	PullRef PullRef

	// PipelineKind defines the type of the pipeline - release, feature, pull request. This parameter is required.
	PipelineKind PipelineKind

	// The build context, aka which jenkins-x.yml should be executed. The default is the empty string.
	Context string

	// EnvVariables defines a set of environment variables to be set on each container/step of the meta as well as the
	// build pipeline.
	EnvVariables map[string]string

	// Labels defines a set of labels to be applied to the generated CRDs.
	Labels map[string]string

	// ServiceAccount defines the service account under which to execute the pipeline.
	ServiceAccount string

	// DefaultImage defines the default image used for pipeline tasks if no other image is specified.
	// This parameter is optional and mainly used for development.
	DefaultImage string

	// UseActivityForNextBuildNumber overrides the default behavior of getting the next build number via SourceRepository,
	// and instead determines the next build number based on existing PipelineActivitys.
	UseActivityForNextBuildNumber bool

	// UseBranchAsRevision forces step_create_task to use the branch it's passed as the revision to checkout for release
	// pipelines, rather than use the version tag
	UseBranchAsRevision bool

	// NoReleasePrepare do not prepare the release, this passes the --no-release-prepare flag to `jx step create task`
	NoReleasePrepare bool
}

// Client defines the interface for meta pipeline creation and application.
type Client interface {
	// Create creates the Tekton CRDs needed for executing the pipeline as defined by the input parameters.
	Create(param PipelineCreateParam) (kube.PromoteStepActivityKey, tekton.CRDWrapper, error)

	// Apply takes the given CRDs for processing, usually applying them to the cluster.
	Apply(pipelineActivity kube.PromoteStepActivityKey, crds tekton.CRDWrapper) error

	// Close cleans up the resources use by this client.
	Close() error
}
