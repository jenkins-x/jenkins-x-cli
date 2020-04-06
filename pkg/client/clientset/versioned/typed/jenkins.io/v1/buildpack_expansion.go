package v1

import (
	"bytes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	util "github.com/jenkins-x/jx/pkg/util/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// BuildPackExpansion expands the default CRUD interface for BuildPack.
type BuildPackExpansion interface {
	PatchUpdate(buildPack *v1.BuildPack) (result *v1.BuildPack, err error)
}

// PatchUpdate takes the representation of a buildPack and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the buildPack, and an error, if there is any.
func (c *buildPacks) PatchUpdate(buildPack *v1.BuildPack) (*v1.BuildPack, error) {
	resourceName := buildPack.ObjectMeta.Name

	// force retrieval from cache
	options := metav1.GetOptions{ResourceVersion: "0"}
	orig, err := c.Get(resourceName, options)
	if err != nil {
		return nil, err
	}

	patch, err := util.CreatePatch(orig, buildPack)
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
