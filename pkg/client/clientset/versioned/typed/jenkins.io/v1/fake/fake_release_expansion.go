package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a release and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the release, and an error, if there is any.
func (c *FakeReleases) PatchUpdate(app *v1.Release) (*v1.Release, error) {
	return c.Update(app)
}
