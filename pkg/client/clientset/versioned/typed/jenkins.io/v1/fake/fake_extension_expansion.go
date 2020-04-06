package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of an extension and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the extension, and an error, if there is any.
func (c *FakeExtensions) PatchUpdate(app *v1.Extension) (*v1.Extension, error) {
	return c.Update(app)
}
