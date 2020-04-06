package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a pipelineActivity and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the pipelineActivity, and an error, if there is any.
func (c *FakePipelineActivities) PatchUpdate(activity *v1.PipelineActivity) (*v1.PipelineActivity, error) {
	return c.Update(activity)
}
