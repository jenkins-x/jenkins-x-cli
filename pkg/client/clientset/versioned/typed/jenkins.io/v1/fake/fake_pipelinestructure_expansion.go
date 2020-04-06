package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a pipelineStructure and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the pipelineStructure, and an error, if there is any.
func (c *FakePipelineStructures) PatchUpdate(app *v1.PipelineStructure) (*v1.PipelineStructure, error) {
	return c.Update(app)
}
