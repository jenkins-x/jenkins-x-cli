package kube

import (
	"encoding/json"

	"github.com/mattbaird/jsonpatch"
	"github.com/pkg/errors"
)

// PatchRow used to generate the patch JSON for patching
type PatchRow struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// CreatePatchBytes creates a kubernetes PATCH block
func CreatePatchBytes(op string, path string, value interface{}) ([]byte, error) {
	payload := &PatchRow{
		Op:    op,
		Path:  path,
		Value: value,
	}
	return json.Marshal(payload)
}

// PatchModifier is a function that is function that mutates an entity during JSON patch generation.
// The function should modify the provided value, and can return an non-nil error if something goes wrong.
type PatchModifier func(obj interface{}) error

// BuildJSONPatch builds JSON patch data for an entity mutation, that can then be applied to K8S as a Patch update.
// The mutation is applied via the supplied callback, which modifies the entity it is given. If the supplied
// mutate method returns an error then the process is aborted and the error returned.
func BuildJSONPatch(obj interface{}, mutate PatchModifier) ([]byte, error) {
	// Get original JSON.
	oJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.WithMessage(err, "marshalling original data")
	}

	err = mutate(obj)
	if err != nil {
		return nil, errors.WithMessage(err, "mutating entity")
	}

	// Get modified JSON & generate a PATCH to apply.
	mJSON, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.WithMessage(err, "marshalling modified data")
	}
	patch, err := jsonpatch.CreatePatch(oJSON, mJSON)
	if err != nil {
		return nil, errors.WithMessage(err, "generating patch data")
	}
	patchJSON, err := json.MarshalIndent(patch, "", "  ")
	if err != nil {
		return nil, errors.WithMessage(err, "marshalling patch data")
	}

	return patchJSON, nil
}
