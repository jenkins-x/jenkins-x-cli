package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a workflow and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the workflow, and an error, if there is any.
func (c *FakeWorkflows) PatchUpdate(app *v1.Workflow) (*v1.Workflow, error) {
	return c.Update(app)
}
