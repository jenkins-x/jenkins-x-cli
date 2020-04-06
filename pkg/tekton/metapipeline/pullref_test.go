// +build unit

package metapipeline

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func Test_pull_ref_to_string(t *testing.T) {
	pullRef := NewPullRef("https://github.com/jenkins-x/jx", "master", "1234567")
	toString := pullRef.String()

	assert.Equal(t, "master:1234567", toString)
}
