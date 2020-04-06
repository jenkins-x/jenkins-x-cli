package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a buildPack and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the buildPack, and an error, if there is any.
func (c *FakeBuildPacks) PatchUpdate(buildPack *v1.BuildPack) (*v1.BuildPack, error) {
	return c.Update(buildPack)
}
