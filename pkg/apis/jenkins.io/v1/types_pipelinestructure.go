package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// PipelineStructure contains references to the Pipeline and PipelineRun, and a list of PipelineStructureStages in the
// pipeline. This allows us to map between a running Pod to its TaskRun, to the TaskRun's Task and PipelineRun, and
// finally from there to the stage and potential parent stages that the Pod is actually executing, for use with
// populating PipelineActivity and providing logs.
type PipelineStructure struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	PipelineRef       *string `json:"pipelineRef" protobuf:"bytes,2,opt,name=pipelineref"`
	PipelineRunRef    *string `json:"pipelineRunRef" protobuf:"bytes,3,opt,name=pipelinerunref"`

	Stages []PipelineStructureStage `json:"stages,omitempty" protobuf:"bytes,3,opt,name=stages"`
}

// PipelineStructureStage contains the stage's name, one of either a reference to the Task corresponding
// to the stage if it has steps, a list of sequential stage names nested within this stage, or a list of parallel stage
// names nested within this stage, and information on this stage's depth within the PipelineStructure as a whole, the
// name of its parent stage, if any, the name of the stage before it in execution order, if any, and the name of the
// stage after it in execution order, if any.
type PipelineStructureStage struct {
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Must have one of TaskRef+TaskRunRef, Stages, or Parallel
	// +optional
	TaskRef *string `json:"taskRef,omitempty" protobuf:"bytes,2,opt,name=taskref"`
	// Populated during pod discovery, not at initial creation time.
	// +optional
	TaskRunRef *string `json:"taskRunRef,omitempty" protobuf:"bytes,3,opt,name=taskrunref"`
	// +optional
	Stages []string `json:"stages,omitempty" protobuf:"bytes,4,opt,name=stages"`
	// +optional
	Parallel []string `json:"parallel,omitempty" protobuf:"bytes,5,opt,name=parallel"`

	Depth int8 `json:"depth" protobuf:"bytes,6,opt,name=depth"`
	// +optional
	Parent *string `json:"parent,omitempty" protobuf:"bytes,7,opt,name=parent"`
	// +optional
	Previous *string `json:"previous,omitempty" protobuf:"bytes,8,opt,name=previous"`
	// +optional
	Next *string `json:"next,omitempty" protobuf:"bytes,9,opt,name=next"`
}

// GetStage will get the PipelineStructureStage with the given name, if it exists.
func (ps *PipelineStructure) GetStage(name string) *PipelineStructureStage {
	for _, s := range ps.Stages {
		if s.Name == name {
			return &s
		}
	}

	return nil
}

// PipelineStageAndChildren represents a single stage and its children.
// +k8s:openapi-gen=false
type PipelineStageAndChildren struct {
	Stage    PipelineStructureStage
	Parallel []PipelineStageAndChildren
	Stages   []PipelineStageAndChildren
}

// GetAllStagesAndChildren will get a slice of all top-level stages in this pipeline, with their children
func (ps *PipelineStructure) GetAllStagesAndChildren() []*PipelineStageAndChildren {
	var stages []*PipelineStageAndChildren

	for _, s := range ps.Stages {
		if s.Depth == 0 {
			psc := ps.GetStageAndChildren(s.Name)
			if psc != nil {
				stages = append(stages, psc)
			}
		}
	}

	return stages
}

// GetStageAndChildren will get a stage of a given name and all of its child stages.
func (ps *PipelineStructure) GetStageAndChildren(name string) *PipelineStageAndChildren {
	stage := ps.GetStage(name)
	if stage != nil {
		psc := &PipelineStageAndChildren{
			Stage: *stage.DeepCopy(),
		}
		for _, s := range stage.Parallel {
			childPsc := ps.GetStageAndChildren(s)
			if childPsc != nil {
				psc.Parallel = append(psc.Parallel, *childPsc)
			}
		}
		for _, s := range stage.Stages {
			childPsc := ps.GetStageAndChildren(s)
			if childPsc != nil {
				psc.Stages = append(psc.Stages, *childPsc)
			}
		}

		return psc
	}

	return nil
}

// GetAllStagesWithSteps gets all stages in this pipeline that have steps, and therefore will have a pod.
func (ps *PipelineStructure) GetAllStagesWithSteps() []PipelineStructureStage {
	var stages []PipelineStructureStage

	for _, s := range ps.Stages {
		if len(s.Stages) == 0 && len(s.Parallel) == 0 {
			stages = append(stages, s)
		}
	}
	return stages
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineStructureList is a list of PipelineStructureList resources
type PipelineStructureList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PipelineStructure `json:"items"`
}
