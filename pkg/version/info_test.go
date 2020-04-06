// +build unit

package version_test

import (
	"testing"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/version"
	"github.com/stretchr/testify/assert"
)

// TODO refactor to encapsulate
func TestGetVersion(t *testing.T) {
	version.Map["version"] = "1.2.1"
	result := version.GetVersion()
	assert.Equal(t, "1.2.1", result)
}

// TODO refactor to encapsulate
func TestGetSemverVersisonWithStandardVersion(t *testing.T) {
	version.Map["version"] = "1.2.1"
	result, err := version.GetSemverVersion()
	expectedResult := semver.Version{Major: 1, Minor: 2, Patch: 1}
	assert.NoError(t, err, "GetSemverVersion should exit without failure")
	assert.Exactly(t, expectedResult, result)
}

// TODO refactor to encapsulate
func TestGetSemverVersisonWithNonStandardVersion(t *testing.T) {
	version.Map["version"] = "1.3.153-dev+7a8285f4"
	result, err := version.GetSemverVersion()

	prVersions := []semver.PRVersion{{VersionStr: "dev"}}
	builds := []string{"7a8285f4"}
	expectedResult := semver.Version{Major: 1, Minor: 3, Patch: 153, Pre: prVersions, Build: builds}
	assert.NoError(t, err, "GetSemverVersion should exit without failure")
	assert.Exactly(t, expectedResult, result)
}
