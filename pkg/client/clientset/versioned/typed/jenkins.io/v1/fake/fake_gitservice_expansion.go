package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a gitService and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the gitService, and an error, if there is any.
func (c *FakeGitServices) PatchUpdate(app *v1.GitService) (*v1.GitService, error) {
	return c.Update(app)
}
