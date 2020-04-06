// +build unit

package docker_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/jenkins-x/jx/pkg/docker"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestUpdateVersions(t *testing.T) {
	tmpDirs := make([]string, 0)
	defer func() {
		for _, dir := range tmpDirs {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}
	}()
	for _, src := range []string{"simple", "nested", "dotname"} {
		// Create the dir structure
		dir, err := ioutil.TempDir("", src)
		tmpDirs = append(tmpDirs, dir)
		assert.NoError(t, err)
		srcDir := filepath.Join("test_data", src)
		err = util.CopyDir(srcDir, dir, true)
		assert.NoError(t, err)
		oldVersions, err := docker.UpdateVersions(dir, "10", "cheese")
		assert.NoError(t, err)
		tests.AssertDirContentsEqual(t, fmt.Sprintf("%s.golden", srcDir), dir)
		assert.Equal(t, []string{"7", "8", "9"}, oldVersions)
	}
}
