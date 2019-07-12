package version_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/stretchr/testify/require"

	"github.com/jenkins-x/jx/pkg/version"
	"github.com/stretchr/testify/assert"
)

const (
	dataDir = "test_data/jenkins-x-versions"
)

// TODO refactor to encapsulate
func TestLoadVersionData(t *testing.T) {
	AssertLoadTestData(t, dataDir, version.KindChart, "jenkins-x/knative-build", "0.1.13")
	AssertLoadTestData(t, dataDir, version.KindChart, "doesNotExist", "")
	AssertLoadTestData(t, dataDir, version.KindPackage, "helm", "2.12.2")
}

// AssertLoadTestData asserts that the StableVersion can be loaded/created for the given kind
func AssertLoadTestData(t *testing.T, dataDir string, kind version.VersionKind, name string, expectedValue string) {
	data, err := version.LoadStableVersion(dataDir, kind, name)
	require.NoError(t, err, "failed to load StableVersion for dir %s kind %s name %s", dataDir, string(kind), name)

	assert.Equal(t, expectedValue, data.Version, "wrong version for kind %s name %s", string(kind), name)
}

// TestForEachVersion tests that we can loop through all the charts in the work dir
func TestForEachVersion(t *testing.T) {
	chartMap := map[string]*version.StableVersion{}

	callback := func(kind version.VersionKind, name string, stableVersion *version.StableVersion) (bool, error) {
		t.Logf("invokved callabck with kind %s name %s and version %s\n", string(kind), name, stableVersion.Version)
		if kind == version.KindChart {
			chartMap[name] = stableVersion
		}
		return true, nil
	}

	err := version.ForEachVersion(dataDir, callback)
	require.NoError(t, err, "calling ForEachVersion on dir %s", dataDir)

	stableVersion := chartMap["jenkins-x/knative-build"]
	require.NotNil(t, stableVersion, "should have a StableVersion for jenkins-x/knative-build")
	assert.Equal(t, "0.1.13", stableVersion.Version)
}

// TestExactPackage tests an exact package version
func TestExactPackage(t *testing.T) {
	resolver := &opts.VersionResolver{
		VersionsDir: dataDir,
	}

	AssertPackageVersion(t, resolver, "helm", "2.12.2", true)
	AssertPackageVersion(t, resolver, "helm", "2.12.3", false)
}

// TestExactPackageVersionRange tests ranges of packages
func TestExactPackageVersionRange(t *testing.T) {
	resolver := &opts.VersionResolver{
		VersionsDir: dataDir,
	}

	AssertPackageVersion(t, resolver, "kubectl", "1.12.0", true)
	AssertPackageVersion(t, resolver, "kubectl", "1.12.1", true)
	AssertPackageVersion(t, resolver, "kubectl", "1.13.1", true)

	AssertPackageVersion(t, resolver, "kubectl", "v1.13.1", true)

	AssertPackageVersion(t, resolver, "kubectl", "1.10.1", false)
	AssertPackageVersion(t, resolver, "kubectl", "2.0.0", false)
	AssertPackageVersion(t, resolver, "kubectl", "2.0.1", false)
}

func AssertPackageVersion(t *testing.T, resolver *opts.VersionResolver, name string, version string, expectedValid bool) {
	err := resolver.VerifyPackage(name, version)
	if expectedValid {
		assert.NoError(t, err, "expected a valid version %s for package %s", version, name)
	} else {
		t.Logf("got expected error %s\n", err.Error())
		assert.Error(t, err, "expected an invalid version %s for package %s", version, name)
	}
}
