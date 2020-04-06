package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a commitStatus and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the commitStatus, and an error, if there is any
func (c *FakeCommitStatuses) PatchUpdate(app *v1.CommitStatus) (*v1.CommitStatus, error) {
	return c.Update(app)
}
