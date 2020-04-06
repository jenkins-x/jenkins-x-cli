package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a fact and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the fact, and an error, if there is any.
func (c *FakeFacts) PatchUpdate(app *v1.Fact) (*v1.Fact, error) {
	return c.Update(app)
}
