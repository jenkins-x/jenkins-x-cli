// +build unit

package packages

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInstallIfExtractorIsEmpty(t *testing.T) {
	isInstalled, err := IsBinaryWithProperVersionInstalled("cat", "0.0", nil)
	assert.False(t, isInstalled)
	assert.Nil(t, err)
}

type mockedCatVersionExtractor struct {
}

func (mockedCatVersionExtractor) arguments() []string {
	return []string{"--version"}
}
func (mockedCatVersionExtractor) extractVersion(command string, arguments []string) (string, error) {
	return "0.0", nil
}

func TestShouldNotInstall(t *testing.T) {
	isInstalled, err := IsBinaryWithProperVersionInstalled("cat", "0.0", mockedCatVersionExtractor{})
	assert.True(t, isInstalled)
	assert.Nil(t, err)
}

func TestShouldInstall(t *testing.T) {
	isInstalled, err := IsBinaryWithProperVersionInstalled("cat", "0.1", mockedCatVersionExtractor{})
	assert.False(t, isInstalled)
	assert.Nil(t, err)
}
