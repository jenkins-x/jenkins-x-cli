package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of an app and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the app, and an error, if there is any.
func (c *FakeApps) PatchUpdate(app *v1.App) (*v1.App, error) {
	return c.Update(app)
}
