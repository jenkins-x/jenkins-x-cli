package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a sourceRepository and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the sourceRepository, and an error, if there is any.
func (c *FakeSourceRepositories) PatchUpdate(app *v1.SourceRepository) (*v1.SourceRepository, error) {
	return c.Update(app)
}
