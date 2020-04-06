// +build unit

package builds_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetBuildNumberFromLabelsFileData(t *testing.T) {
	t.Parallel()

	assertBuildNumberFromLabelsData(t, `build.knative.dev/buildName="jstrachan-mynodething-master-24-build"`, "24")
	assertBuildNumberFromLabelsData(t, `build.knative.dev/buildName="jstrachan-mynodething-master-12"`, "12")
	assertBuildNumberFromLabelsData(t, `build-number="45"`, "45")
}

func TestGetBranchNameFromLabelsFileData(t *testing.T) {
	t.Parallel()

	assertBranchFromLabelsData(t, `branch="PR-1234"`, "PR-1234")
}

func assertBuildNumberFromLabelsData(t *testing.T, text string, expected string) {
	m := builds.LoadDownwardAPILabels(text)
	require.NotNil(t, "could not load map from downward API text: %s", text)
	actual := builds.GetBuildNumberFromLabels(m)
	assert.Equal(t, expected, actual, "GetBuildNumberFromLabels() with map %#v and text %s", m, text)
}

func assertBranchFromLabelsData(t *testing.T, text string, expected string) {
	m := builds.LoadDownwardAPILabels(text)
	require.NotNil(t, "could not load map from downward API text: %s", text)
	actual := builds.GetBranchNameFromLabels(m)
	assert.Equal(t, expected, actual, "GetBranchNameFromLabels() with map %#v and text %s", m, text)
}
