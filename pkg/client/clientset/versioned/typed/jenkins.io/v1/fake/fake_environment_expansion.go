package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of an environment and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the environment, and an error, if there is any.
func (c *FakeEnvironments) PatchUpdate(app *v1.Environment) (*v1.Environment, error) {
	return c.Update(app)
}
