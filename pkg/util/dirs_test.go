// +build unit

package util

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
)

func TestJXBinaryLocationSuccess(t *testing.T) {
	t.Parallel()
	tempDir, err := ioutil.TempDir("", "")
	require.NoError(t, err, "[TEST SETUP] failed to create temp directory for test")
	defer os.RemoveAll(tempDir)
	// on OS-X tmp is /tmp but a link to /private/tmp which causes the test to fail!
	tempDir, err = filepath.EvalSymlinks(tempDir)
	require.NoError(t, err, "[TEST SETUP] could not resolve symlinks")

	jxpath := filepath.Join(tempDir, "jx")
	// resolving symlinks requires that the file exists (at least on windows...
	_, err = os.Create(jxpath)
	require.NoError(t, err, "[TEST SETUP] failed to create temp directory for test")

	res, err := jXBinaryLocation(stubFunction(jxpath, nil))
	assert.Equal(t, tempDir, res)
	assert.NoError(t, err, "Should not error")
}

func TestJXBinaryLocationFailure(t *testing.T) {
	t.Parallel()

	res, err := jXBinaryLocation(stubFunction("", errors.New("Kaboom")))
	assert.Equal(t, "", res)
	assert.Error(t, err, "Should error")
}

func stubFunction(str string, err error) func() (string, error) {
	return func() (string, error) {
		return str, err
	}
}
