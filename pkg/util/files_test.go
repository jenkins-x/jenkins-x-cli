// +build unit

package util_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestGlobFiles(t *testing.T) {
	t.Parallel()

	files := []string{}
	fn := func(name string) error {
		if util.StringArrayIndex(files, name) < 0 {
			files = append(files, name)
		}
		return nil
	}

	/*	pwd, err := os.Getwd()
		require.NoError(t, err)
		t.Logf("Current dir is %s\n", pwd)
	*/
	err := util.GlobAllFiles("", "test_data/glob_test/*", fn)
	require.NoError(t, err)

	sort.Strings(files)
	expected := []string{
		filepath.Join("test_data", "glob_test", "artifacts", "goodbye.txt"),
		filepath.Join("test_data", "glob_test", "hello.txt"),
	}

	assert.Equal(t, expected, files, "globbed files")
}

func TestDeleteDirContents(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "TestDeleteDirContents")
	require.NoError(t, err, "Failed to create temporary directory.")
	defer func() {
		err = os.RemoveAll(tmpDir)
	}()

	// Various types
	var testFileNames = []string{
		"file1",
		"file2.out",
		"file.3.ex-tension",
	}
	for _, filename := range testFileNames {
		//Validate filename deletion as a file, directory, and non-empty directory.
		filePath := filepath.Join(tmpDir, filename)
		ioutil.WriteFile(filePath, []byte(filename), os.ModePerm)
		dirPath := filepath.Join(tmpDir, filename+"-dir")
		os.Mkdir(dirPath, os.ModeDir)
		fileInDirPath := filepath.Join(dirPath, filename)
		ioutil.WriteFile(fileInDirPath, []byte(filename), os.ModePerm)
	}

	//delete contents
	util.DeleteDirContents(tmpDir)

	//check dir still exists.
	_, err = os.Stat(tmpDir)
	require.NoError(t, err, "Directory has been deleted.")

	//check empty
	remainingFiles, err := filepath.Glob(filepath.Join(tmpDir, "*"))
	assert.Equal(t, 0, len(remainingFiles),
		fmt.Sprintf("Expected tmp dir %s to be empty, but contains %v.", tmpDir, remainingFiles))

}

func TestDeleteDirContentsExcept(t *testing.T) {
	t.Parallel()

	tmpDir, err := ioutil.TempDir("", "TestDeleteDirContents")
	require.NoError(t, err, "Failed to create temporary directory.")
	defer func() {
		err = os.RemoveAll(tmpDir)
	}()

	// Various types
	var testFileNames = []string{
		"file1",
		"file2.out",
		"file.3.ex-tension",
	}
	for _, filename := range testFileNames {
		//Validate filename deletion as a file, directory, and non-empty directory.
		filePath := filepath.Join(tmpDir, filename)
		ioutil.WriteFile(filePath, []byte(filename), os.ModePerm)
		dirPath := filepath.Join(tmpDir, filename+"-dir")
		os.Mkdir(dirPath, os.ModeDir)
		fileInDirPath := filepath.Join(dirPath, filename)
		ioutil.WriteFile(fileInDirPath, []byte(filename), os.ModePerm)
	}

	//delete contents
	util.DeleteDirContentsExcept(tmpDir, "file1")

	//check dir still exists.
	_, err = os.Stat(tmpDir)
	require.NoError(t, err, "Directory has been deleted.")

	//check empty
	remainingFiles, err := filepath.Glob(filepath.Join(tmpDir, "*"))
	assert.Equal(t, 1, len(remainingFiles),
		fmt.Sprintf("Expected tmp dir %s to be empty, but contains %v.", tmpDir, remainingFiles))

}

func TestToValidFileSystemName(t *testing.T) {
	assert.Equal(t, util.ToValidFileSystemName("x.y/z"), "x_y_z")
}

func Test_FileExists_for_non_existing_file_returns_false(t *testing.T) {
	exists, err := util.FileExists("/foo/bar")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func Test_FileExists_for_existing_file_returns_true(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "Test_FileExists_for_existing_file_returns_true")
	require.NoError(t, err, "failed to create temporary directory")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	data := []byte("hello\nworld\n")
	testFile := filepath.Join(tmpDir, "hello.txt")
	err = ioutil.WriteFile(testFile, data, 0644)
	require.NoError(t, err, "failed to create test file %s", testFile)

	exists, err := util.FileExists(testFile)
	assert.NoError(t, err)
	assert.True(t, exists)
}

func Test_FileExists_for_existing_directory_returns_false(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "Test_FileExists_for_existing_file_returns_true")
	require.NoError(t, err, "failed to create temporary directory")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	exists, err := util.FileExists(tmpDir)
	assert.NoError(t, err)
	assert.False(t, exists)
}
