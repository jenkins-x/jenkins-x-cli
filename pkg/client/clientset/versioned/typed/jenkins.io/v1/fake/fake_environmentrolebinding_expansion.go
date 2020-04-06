package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a environmentRoleBinding and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the environmentRoleBinding, and an error, if there is any.
func (c *FakeEnvironmentRoleBindings) PatchUpdate(app *v1.EnvironmentRoleBinding) (*v1.EnvironmentRoleBinding, error) {
	return c.Update(app)
}
