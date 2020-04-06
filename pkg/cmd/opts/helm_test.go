// +build integration

package opts

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/util"
)

// Test_HelmInitRecursiveDependencyBuild_extraction tests that chart achives
// are properly extracted
func Test_HelmInitRecursiveDependencyBuild_extraction(t *testing.T) {
	o := NewCommonOptionsWithFactory(clients.NewFactory())

	dir, err := ioutil.TempDir("", "dowload_test")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	dirRecursive := filepath.Join(dir, "recursive")
	err = util.CopyDir("test_data/recursive", dirRecursive, true)
	require.NoError(t, err)

	dirChart1 := filepath.Join(dirRecursive, "chart1")
	err = o.HelmInitRecursiveDependencyBuild(dirChart1, []string{}, []string{})
	require.NoError(t, err, "calling HelmInitRecursiveDependencyBuild")

	expected := []string{
		"Chart.yaml",
		"charts/chart2/Chart.yaml",
		"charts/chart2/charts/chart3/Chart.yaml",
		"charts/chart2/charts/chart3/templates/template.yaml",
		"charts/chart2/charts/chart3/values.yaml",
		"charts/chart2/requirements.lock",
		"charts/chart2/requirements.yaml",
		"charts/chart2/templates/template.yaml",
		"charts/chart2/values.yaml",
		"requirements.lock",
		"requirements.yaml",
		"templates/template.yaml",
		"values.yaml",
	}
	found := []string{}

	require.NoError(t, filepath.Walk(dirChart1, func(path string, info os.FileInfo, err error) error {
		if !assert.NoError(t, err) {
			return err
		}
		if info.IsDir() {
			return nil
		}
		name := strings.TrimPrefix(path, dirChart1+"/")
		assert.True(t, info.Mode().IsRegular(), "Non regular file %s", name)
		found = append(found, name)
		return nil
	}))
	require.Equal(t, expected, found, "wrong files extracted")
}

// Test_HelmInitDependencyBuild_lint tests that the lint phase
// passes the required variables
func Test_HelmInitDependencyBuild_lint(t *testing.T) {
	o := NewCommonOptionsWithFactory(clients.NewFactory())

	dir, err := ioutil.TempDir("", "lint_test")
	require.NoError(t, err)
	defer os.RemoveAll(dir)

	dirChart := filepath.Join(dir, "lint")
	err = util.CopyDir("test_data/lint", dirChart, true)
	require.NoError(t, err)

	_, err = o.HelmInitDependencyBuild(dirChart, []string{}, []string{})
	require.NoError(t, err, "calling HelmInitDependencyBuild")
}
