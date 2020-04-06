// +build unit

package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnzip(t *testing.T) {
	t.Parallel()
	dest, err := ioutil.TempDir("", "testUnzip")
	require.NoError(t, err, "[TEST SETUP] failed to create temporary directory for tests")
	defer os.RemoveAll(dest)

	zipFile, err := filepath.Abs("")
	require.NoError(t, err, "[TEST SETUP] could not obtain CWD")
	zipFile = filepath.Join(zipFile, "test_data", "2_files.zip")

	err = Unzip(zipFile, dest)
	require.NoError(t, err, "failed to unzip archive")
	files, err := ioutil.ReadDir(dest)
	require.NoError(t, err, "failed to list files")

	assert.Len(t, files, 2)
	// files are sorted by ReadDir
	assert.Equal(t, "file1.txt", files[0].Name())
	assert.Equal(t, "file2.txt", files[1].Name())
	for _, f := range files {
		assertFileContents(t, filepath.Join(dest, f.Name()), strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())))
	}
}

func TestUnzipSpecificFilesHappyPath(t *testing.T) {
	t.Parallel()
	dest, err := ioutil.TempDir("", "testUnzip")
	require.NoError(t, err, "[TEST SETUP] failed to create temporary directory for tests")
	defer os.RemoveAll(dest)

	zipFile, err := filepath.Abs("")
	require.NoError(t, err, "[TEST SETUP] could not obtain CWD")
	zipFile = filepath.Join(zipFile, "test_data", "2_files.zip")

	err = UnzipSpecificFiles(zipFile, dest, "file2.txt")
	require.NoError(t, err, "failed to unzip archive")
	files, err := ioutil.ReadDir(dest)
	require.NoError(t, err, "failed to list files")

	assert.Len(t, files, 1)
	// files are sorted by ReadDir
	require.Equal(t, "file2.txt", files[0].Name())
	assertFileContents(t, filepath.Join(dest, "file2.txt"), "file2")
}

func TestUnzipSpecificFilesMIssingFile(t *testing.T) {
	t.Parallel()
	dest, err := ioutil.TempDir("", "testUnzip")
	require.NoError(t, err, "[TEST SETUP] failed to create temporary directory for tests")
	defer os.RemoveAll(dest)

	zipFile, err := filepath.Abs("")
	require.NoError(t, err, "[TEST SETUP] could not obtain CWD")
	zipFile = filepath.Join(zipFile, "test_data", "2_files.zip")

	err = UnzipSpecificFiles(zipFile, dest, "file2.txt", "file3.txt")
	assert.Error(t, err, "unzip should fail as file3.txt is not in the zip")
}

func assertFileContents(t *testing.T, file, expected string) {
	bytes, err := ioutil.ReadFile(file)
	if assert.NoError(t, err, "could not read contents of %s", file) {
		assert.Equal(t, expected, string(bytes), "file contents for %s did not match", file)
	}
}
