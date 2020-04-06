package v1

import (
	"bytes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	util "github.com/jenkins-x/jx/pkg/util/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// CommitStatusExpansion expands the default CRUD interface for CommitStatus.
type CommitStatusExpansion interface {
	PatchUpdate(commitStatus *v1.CommitStatus) (result *v1.CommitStatus, err error)
}

// PatchUpdate takes the representation of a commitStatus and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the commitStatus, and an error, if there is any.
func (c *commitStatuses) PatchUpdate(commitStatus *v1.CommitStatus) (*v1.CommitStatus, error) {
	resourceName := commitStatus.ObjectMeta.Name

	// force retrieval from cache
	options := metav1.GetOptions{ResourceVersion: "0"}
	orig, err := c.Get(resourceName, options)
	if err != nil {
		return nil, err
	}

	patch, err := util.CreatePatch(orig, commitStatus)
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
