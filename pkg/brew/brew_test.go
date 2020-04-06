// +build unit

package brew_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/brew"

	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestUpdateVersion(t *testing.T) {
	tmpDirs := make([]string, 0)
	defer func() {
		for _, dir := range tmpDirs {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}
	}()
	for _, src := range []string{"standard"} {
		// Create the dir structure
		dir, err := ioutil.TempDir("", src)
		tmpDirs = append(tmpDirs, dir)
		assert.NoError(t, err)
		srcDir := filepath.Join("test_data", src)
		err = util.CopyDir(srcDir, dir, true)
		assert.NoError(t, err)
		oldVersions, oldShas, err := brew.UpdateVersionAndSha(dir, "1.0.2", "ef7a95c23bc5858cff6fd2825836af7e8342a9f6821d91ddb0b5b5f87f0f4e85")
		assert.NoError(t, err)
		tests.AssertDirContentsEqual(t, fmt.Sprintf("%s.golden", srcDir), dir)
		assert.Equal(t, []string{"1.0.1"}, oldVersions)
		assert.Equal(t, []string{"7d7d380c5f0760027ae73f1663a1e1b340548fd93f68956e6b0e2a0d984774fa"}, oldShas)
	}
}
