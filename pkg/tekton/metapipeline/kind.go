package metapipeline

import "strings"

const (
	// ReleasePipeline indicates a release pipeline build.
	ReleasePipeline PipelineKind = iota

	// PullRequestPipeline indicates a pull request pipeline build.
	PullRequestPipeline

	// FeaturePipeline indicates a feature pipeline build.
	FeaturePipeline
)

// PipelineKind defines the type of the pipeline
type PipelineKind uint32

// String returns a string representation of the pipeline type
func (p PipelineKind) String() string {
	switch p {
	case ReleasePipeline:
		return "release"
	case PullRequestPipeline:
		return "pullrequest"
	case FeaturePipeline:
		return "feature"
	default:
		return "unknown"
	}
}

// StringToPipelineKind converts text to a PipelineKind
func StringToPipelineKind(text string) PipelineKind {
	switch strings.ToLower(text) {
	case "release":
		return ReleasePipeline
	case "pullrequest":
		return PullRequestPipeline
	case "feature":
		return FeaturePipeline
	default:
		return ReleasePipeline
	}
}
