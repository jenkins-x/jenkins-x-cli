// +build unit

package surveyutils

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestTemplateSchemaFile(t *testing.T) {
	type testCase struct {
		GitKind, GitServer, ExpectedTokenDescription, ExpectedPattern string
		ExpectedMin, ExpectedMax                                      int
	}

	testCases := []testCase{
		{
			GitKind:                  "github",
			GitServer:                "",
			ExpectedTokenDescription: "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to https://github.com/settings/tokens/new?scopes=repo,read:user,read:org,user:email,write:repo_hook,delete_repo then enter a name, click Generate token, and copy and paste the token into this prompt.",
			ExpectedMin:              40,
			ExpectedMax:              40,
			ExpectedPattern:          "^[0-9a-f]{40}$",
		},
		{
			GitKind:                  "bitbucketserver",
			GitServer:                "https://bitbucket.myorg.com",
			ExpectedTokenDescription: "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to https://bitbucket.myorg.com/plugins/servlet/access-tokens/manage then enter a name, click Generate token, and copy and paste the token into this prompt.",
			ExpectedMin:              8,
			ExpectedMax:              50,
		},
		{
			GitKind:                  "gitlab",
			GitServer:                "https://gitlab.myorg.com",
			ExpectedTokenDescription: "A token for the Git user that will perform git operations inside a pipeline. This includes environment repository creation, and so this token should have full repository permissions. To create a token go to https://gitlab.myorg.com/profile/personal_access_tokens then enter a name, click Generate token, and copy and paste the token into this prompt.",
			ExpectedMin:              8,
			ExpectedMax:              50,
		},
	}

	for _, tc := range testCases {
		sourceData := filepath.Join("test_data", "template")
		assert.DirExists(t, sourceData)

		testData, err := ioutil.TempDir("", "test-jx-step-create-values-")
		assert.NoError(t, err)

		err = util.CopyDir(sourceData, testData, true)
		assert.NoError(t, err)
		assert.DirExists(t, testData)

		requirements, requirementsFile, err := config.LoadRequirementsConfig(testData)
		require.NoError(t, err, "failed to load requirements in dir %s", testData)

		requirements.Cluster.GitKind = tc.GitKind
		requirements.Cluster.GitServer = tc.GitServer
		err = requirements.SaveConfig(requirementsFile)
		require.NoError(t, err, "failed to save requirements as file %s", requirementsFile)

		schemaFile := filepath.Join(testData, "values.schema.json")
		err = TemplateSchemaFile(schemaFile, requirements)
		require.NoError(t, err, "failed to generate template schema file %s", schemaFile)
		assert.FileExists(t, schemaFile)

		data, err := ioutil.ReadFile(schemaFile)
		require.NoError(t, err, "failed to load schema file %s", schemaFile)

		m := map[string]interface{}{}
		err = json.Unmarshal(data, &m)
		require.NoError(t, err, "failed to parse schema file %s", schemaFile)

		actual := util.GetMapValueAsStringViaPath(m, "properties.pipelineUser.properties.token.description")
		t.Logf("git token for git kind %s and server %s has description %s\n", tc.GitKind, tc.GitServer, actual)
		assert.Equal(t, tc.ExpectedTokenDescription, actual, "properties.pipelineUser.properties.token.description in generated schema for git kind %s and server %s", tc.GitKind, tc.GitServer)

		pattern := util.GetMapValueAsStringViaPath(m, "properties.pipelineUser.properties.token.pattern")
		t.Logf("git token for git kind %s and server %s has pattern: %s\n", tc.GitKind, tc.GitServer, pattern)
		assert.Equal(t, tc.ExpectedPattern, pattern, "properties.pipelineUser.properties.token.pattern in generated schema for git kind %s and server %s", tc.GitKind, tc.GitServer)

		min := util.GetMapValueAsIntViaPath(m, "properties.pipelineUser.properties.token.minLength")
		t.Logf("git token for git kind %s and server %s has minLength %d\n", tc.GitKind, tc.GitServer, min)
		assert.Equal(t, tc.ExpectedMin, min, "properties.pipelineUser.properties.token.minLength in generated schema for git kind %s and server %s", tc.GitKind, tc.GitServer)

		max := util.GetMapValueAsIntViaPath(m, "properties.pipelineUser.properties.token.maxLength")
		t.Logf("git token for git kind %s and server %s has maxLength %d\n", tc.GitKind, tc.GitServer, max)
		assert.Equal(t, tc.ExpectedMax, max, "properties.pipelineUser.properties.token.maxLength in generated schema for git kind %s and server %s", tc.GitKind, tc.GitServer)
	}
}
