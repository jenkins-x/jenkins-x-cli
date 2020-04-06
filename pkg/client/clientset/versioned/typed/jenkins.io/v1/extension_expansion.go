package v1

import (
	"bytes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	util "github.com/jenkins-x/jx/pkg/util/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ExtensionExpansion expands the default CRUD interface for Extension.
type ExtensionExpansion interface {
	PatchUpdate(extension *v1.Extension) (result *v1.Extension, err error)
}

// PatchUpdate takes the representation of an extension and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the extension, and an error, if there is any.
func (c *extensions) PatchUpdate(extension *v1.Extension) (*v1.Extension, error) {
	resourceName := extension.ObjectMeta.Name

	// force retrieval from cache
	options := metav1.GetOptions{ResourceVersion: "0"}
	orig, err := c.Get(resourceName, options)
	if err != nil {
		return nil, err
	}

	patch, err := util.CreatePatch(orig, extension)
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
