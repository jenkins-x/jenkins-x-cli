// +build unit

package kustomize_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/kustomize"
)

func TestKustomizeCLI_FindKustomize(t *testing.T) {
	testDir, err := filepath.Abs(filepath.Dir("test_data/kustomize_dummy/base"))
	assert.NoError(t, err, "failed to find test data")
	wantedOutput := []string{
		filepath.Join(testDir, "base", "charts", "kustomization.yaml"),
		filepath.Join(testDir, "base", "kustomization.yaml"),
		filepath.Join(testDir, "staging", "kustomization.yaml"),
	}

	k := kustomize.NewKustomizeCLI()
	output := k.FindKustomizationYamlPaths(testDir)

	assert.ElementsMatch(t, wantedOutput, output, "not able to find all of the kustomize resource")
}

func TestKustomizeCLI_ContainsKustomizeConfig(t *testing.T) {
	testDir, err := filepath.Abs(filepath.Dir("test_data/kustomize_dummy/base"))
	assert.NoError(t, err, "failed to find test data")

	k := kustomize.NewKustomizeCLI()
	assert.True(t, k.ContainsKustomizeConfig(testDir))
	assert.False(t, k.ContainsKustomizeConfig(filepath.Join(testDir, "foo")))
}
