package v1

import (
	"bytes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	util "github.com/jenkins-x/jx/pkg/util/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// TeamExpansion expands the default CRUD interface for Team.
type TeamExpansion interface {
	PatchUpdate(team *v1.Team) (result *v1.Team, err error)
}

// PatchUpdate takes the representation of a team and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the team, and an error, if there is any.
func (c *teams) PatchUpdate(team *v1.Team) (*v1.Team, error) {
	resourceName := team.ObjectMeta.Name

	// force retrieval from cache
	options := metav1.GetOptions{ResourceVersion: "0"}
	orig, err := c.Get(resourceName, options)
	if err != nil {
		return nil, err
	}

	patch, err := util.CreatePatch(orig, team)
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
