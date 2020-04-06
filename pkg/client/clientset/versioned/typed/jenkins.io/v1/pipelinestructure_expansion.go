package v1

import (
	"bytes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	util "github.com/jenkins-x/jx/pkg/util/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// PipelineStructureExpansion expands the default CRUD interface for PipelineStructure.
type PipelineStructureExpansion interface {
	PatchUpdate(pipelineStructure *v1.PipelineStructure) (result *v1.PipelineStructure, err error)
}

// PatchUpdate takes the representation of a pipelineStructure and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the pipelineStructure, and an error, if there is any.
func (c *pipelineStructures) PatchUpdate(pipelineStructure *v1.PipelineStructure) (*v1.PipelineStructure, error) {
	resourceName := pipelineStructure.ObjectMeta.Name

	// force retrieval from cache
	options := metav1.GetOptions{ResourceVersion: "0"}
	orig, err := c.Get(resourceName, options)
	if err != nil {
		return nil, err
	}

	patch, err := util.CreatePatch(orig, pipelineStructure)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(patch, []byte("[]")) {
		return orig, nil
	}
	patched, err := c.Patch(resourceName, types.JSONPatchType, patch)
	if err != nil {
		return nil, err
	}

	return patched, nil
}
