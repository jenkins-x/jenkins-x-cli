// +build unit

package v1

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/stretchr/testify/assert"
)

var (
	testDataDir = path.Join("test_data", "environment")
)

func TestGitPublic(t *testing.T) {
	var gitPublicTests = []struct {
		jsonFile          string
		expectedGitPublic bool
	}{
		{"git_public_nil_git_private_true.json", false},
		{"git_public_nil_git_private_false.json", true},
		{"git_public_false_git_private_nil.json", false},
		{"git_public_true_git_private_nil.json", true},
	}

	for _, testCase := range gitPublicTests {
		t.Run(testCase.jsonFile, func(t *testing.T) {
			content, err := ioutil.ReadFile(path.Join(testDataDir, testCase.jsonFile))
			assert.NoError(t, err)

			env := Environment{}

			_ = log.CaptureOutput(func() {
				err = json.Unmarshal(content, &env)
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedGitPublic, env.Spec.TeamSettings.GitPublic, "unexpected value for default repository visibility")
			})
		})
	}
}

func Test_GitPublic_and_GitPrivate_specified_throws_error(t *testing.T) {
	content, err := ioutil.ReadFile(path.Join(testDataDir, "git_public_true_git_private_true.json"))
	assert.NoError(t, err)

	env := Environment{}
	err = json.Unmarshal(content, &env)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only GitPublic should be used")
}
