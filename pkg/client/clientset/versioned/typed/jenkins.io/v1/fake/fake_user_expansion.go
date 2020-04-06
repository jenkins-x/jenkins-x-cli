package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a user and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the user, and an error, if there is any.
func (c *FakeUsers) PatchUpdate(app *v1.User) (*v1.User, error) {
	return c.Update(app)
}
