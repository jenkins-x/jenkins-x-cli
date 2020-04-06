package v1

import (
	"bytes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	util "github.com/jenkins-x/jx/pkg/util/json"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// AppExpansion expands the default CRUD interface for App.
type AppExpansion interface {
	PatchUpdate(app *v1.App) (result *v1.App, err error)
}

// PatchUpdate takes the representation of an app and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the app, and an error, if there is any.
func (c *apps) PatchUpdate(app *v1.App) (*v1.App, error) {
	resourceName := app.ObjectMeta.Name

	// force retrieval from cache
	options := metav1.GetOptions{ResourceVersion: "0"}
	orig, err := c.Get(resourceName, options)
	if err != nil {
		return nil, err
	}

	patch, err := util.CreatePatch(orig, app)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(patch, []byte("[]")) {
		return orig, nil
	}
	patchedApp, err := c.Patch(resourceName, types.JSONPatchType, patch)
	if err != nil {
		return nil, err
	}

	return patchedApp, nil
}
