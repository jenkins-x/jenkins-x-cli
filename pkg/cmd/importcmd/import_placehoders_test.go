// +build unit

package importcmd_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/importcmd"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestReplacePlaceholders(t *testing.T) {
	f, err := ioutil.TempDir("", "test-replace-placeholders")
	assert.NoError(t, err)

	testData := path.Join("test_data", "replace_placeholders")
	_, err = os.Stat(testData)
	assert.NoError(t, err)

	util.CopyDir(testData, f, true)

	assert.NoError(t, err)
	o := importcmd.ImportOptions{}
	//o.Out = tests.Output()
	o.Dir = f
	o.AppName = "bar"
	o.Organisation = "foo"

	o.ReplacePlaceholders("github.com", "registry-org")

	// root file
	testFile, err := util.LoadBytes(f, "file.txt")
	assert.NoError(t, err)
	assert.Equal(t, "/home/jenkins/go/src/github.com/foo/bar/registry-org", string(testFile), "replaced placeholder")

	// dir1
	testDir1 := path.Join(f, "dir1")
	testFile, err = util.LoadBytes(testDir1, "file.txt")
	assert.NoError(t, err)
	assert.Equal(t, "/home/jenkins/go/src/github.com/foo/bar/registry-org", string(testFile), "replaced placeholder")

	// dir2
	testDir2 := path.Join(f, "dir2")
	testFile, err = util.LoadBytes(testDir2, "file.txt")
	assert.NoError(t, err)
	assert.Equal(t, "/home/jenkins/go/src/github.com/foo/bar/registry-org", string(testFile), "replaced placeholder")

	// REPLACE_ME_APP_NAME/REPLACE_ME_APP_NAME.txt
	testDirBar := path.Join(f, "bar")
	testFile, err = util.LoadBytes(testDirBar, "bar.txt")
	assert.NoError(t, err)
	assert.Equal(t, "/home/jenkins/go/src/github.com/foo/bar/registry-org", string(testFile), "replaced placeholder")

}
