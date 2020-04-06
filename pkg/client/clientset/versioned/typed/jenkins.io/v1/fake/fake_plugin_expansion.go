package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a plugin and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the plugin, and an error, if there is any.
func (c *FakePlugins) PatchUpdate(app *v1.Plugin) (*v1.Plugin, error) {
	return c.Update(app)
}
