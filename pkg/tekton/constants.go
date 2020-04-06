package tekton

import (
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
)

const (
	// LastBuildNumberAnnotationPrefix used to annotate SourceRepository with the latest build number for a branch
	LastBuildNumberAnnotationPrefix = "jenkins.io/last-build-number-for-"

	// LabelOwner is the label added to Tekton CRDs for the owner of the repository being built.
	LabelOwner = v1.LabelOwner

	// LabelRepo is the label added to Tekton CRDs for the repository being built.
	LabelRepo = v1.LabelRepository

	// LabelBranch is the label added to Tekton CRDs for the branch being built.
	LabelBranch = v1.LabelBranch

	// LabelBuild is the label added to Tekton CRDs for the build number.
	LabelBuild = v1.LabelBuild

	// LabelContext is the label added to Tekton CRDs for the context being built.
	LabelContext = "context"

	// LabelType is the label added to Tekton CRDs for the type of pipeline.
	LabelType = "jenkins.io/pipelineType"
)
