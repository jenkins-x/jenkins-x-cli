package fake

import v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

// PatchUpdate takes the representation of a team and updates using Patch generating a JSON patch to do so.
// Returns the server's representation of the team, and an error, if there is any.
func (c *FakeTeams) PatchUpdate(app *v1.Team) (*v1.Team, error) {
	return c.Update(app)
}
