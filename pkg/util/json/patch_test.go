// +build unit

package json

import (
	"testing"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreatePatch(t *testing.T) {
	t.Parallel()
	orig, clone := setUp(t)

	clone.Spec.Name = "foo"
	patch, err := CreatePatch(orig, clone)

	assert.NoError(t, err, "patch creation should be successful ")
	assert.Equal(t, `[{"op":"add","path":"/spec/name","value":"foo"}]`, string(patch),
		"the patch should have been empty")
}

func TestCreatePatchNil(t *testing.T) {
	t.Parallel()
	orig, clone := setUp(t)

	_, err := CreatePatch(nil, clone)
	assert.Error(t, err, "nil should not be allowed")
	assert.Equal(t, "'before' cannot be nil when creating a JSON patch", err.Error(), "wrong error message")

	_, err = CreatePatch(orig, nil)
	assert.Error(t, err, "nil should not be allowed")
	assert.Equal(t, "'after' cannot be nil when creating a JSON patch", err.Error(), "wrong error message")
}

func TestCreatePatchNoDiff(t *testing.T) {
	t.Parallel()
	orig, clone := setUp(t)

	patch, err := CreatePatch(orig, clone)

	assert.NoError(t, err, "patch creation should be successful ")
	assert.Equal(t, "[]", string(patch), "the patch should have been empty")
}

func setUp(t *testing.T) (*jenkinsv1.Plugin, *jenkinsv1.Plugin) {
	orig := &jenkinsv1.Plugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-plugin",
		},
		Spec: jenkinsv1.PluginSpec{},
	}

	return orig, orig.DeepCopy()
}
