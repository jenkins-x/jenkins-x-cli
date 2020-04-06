// +build unit

package jenkins_test

import (
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateCenter(t *testing.T) {
	AssertLoadUpdateCenterFile(t, filepath.Join("test_data", "update_center.json"))
}

func TestUpdateCenterJSONP(t *testing.T) {
	AssertLoadUpdateCenterFile(t, filepath.Join("test_data", "update_center.jsonp"))
}

func AssertLoadUpdateCenterFile(t *testing.T, fileName string) {
	data, err := jenkins.LoadUpdateCenterFile(fileName)
	require.NoError(t, err, "failed to load file %s", fileName)
	assert.Equal(t, "default", data.ID, "id")
	assert.True(t, len(data.Plugins) > 0, "no plugins found!")
	AssertPluginVersion(t, data, "jx-resources", "1.0.33")
}

func AssertPluginVersion(t *testing.T, data *jenkins.UpdateCenter, name string, version string) {
	plugin := data.Plugins[name]
	require.NotNil(t, plugin, "no plugin found for name %s", name)

	t.Logf("plugin %s has version %s\n", name, plugin.Version)
	assert.Equal(t, version, plugin.Version, "plugin version")
}
