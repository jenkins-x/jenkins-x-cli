package cmd_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/jx/cmd"
	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

const testUsername = "derek_zoolander"

func TestCreateProwOwnersFileExistsDoNothing(t *testing.T) {
	t.Parallel()
	path, err := ioutil.TempDir("", "prow")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(path)
	ownerFilePath := filepath.Join(path, "OWNERS")
	_, err = os.Create(ownerFilePath)
	if err != nil {
		panic(err)
	}

	cmd := cmd.ImportOptions{
		Dir: path,
	}

	err = cmd.CreateProwOwnersFile()
	assert.NoError(t, err, "There should be no error")
}

func TestCreateProwOwnersFileCreateWhenDoesNotExist(t *testing.T) {
	t.Parallel()
	path, err := ioutil.TempDir("", "prow")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(path)

	cmd := cmd.ImportOptions{
		Dir: path,
		GitUserAuth: &auth.UserAuth{
			Username: testUsername,
		},
	}

	err = cmd.CreateProwOwnersFile()
	assert.NoError(t, err, "There should be no error")

	wantFile := filepath.Join(path, "OWNERS")
	exists, err := util.FileExists(wantFile)
	assert.NoError(t, err, "It should find the OWNERS file without error")
	assert.True(t, exists, "It should create an OWNERS file")

	wantOwners := prow.Owners{
		Approvers: []string{testUsername},
		Reviewers: []string{testUsername},
	}
	data, err := ioutil.ReadFile(wantFile)
	assert.NoError(t, err, "It should read the OWNERS file without error")
	owners := prow.Owners{}
	err = yaml.Unmarshal(data, &owners)
	assert.NoError(t, err, "It should unmarshal the OWNERS file without error")
	assert.Equal(t, wantOwners, owners)
}

func TestCreateProwOwnersFileCreateWhenDoesNotExistAndNoGitUserSet(t *testing.T) {
	t.Parallel()
	path, err := ioutil.TempDir("", "prow")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(path)

	cmd := cmd.ImportOptions{
		Dir: path,
	}

	err = cmd.CreateProwOwnersFile()
	assert.Error(t, err, "There should an error")
}

func TestCreateProwOwnersAliasesFileExistsDoNothing(t *testing.T) {
	t.Parallel()
	path, err := ioutil.TempDir("", "prow")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(path)
	ownerFilePath := filepath.Join(path, "OWNERS_ALIASES")
	_, err = os.Create(ownerFilePath)
	if err != nil {
		panic(err)
	}

	cmd := cmd.ImportOptions{
		Dir: path,
	}

	err = cmd.CreateProwOwnersAliasesFile()
	assert.NoError(t, err, "There should be no error")
}

func TestCreateProwOwnersAliasesFileCreateWhenDoesNotExist(t *testing.T) {
	t.Parallel()
	path, err := ioutil.TempDir("", "prow")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(path)
	cmd := cmd.ImportOptions{
		Dir: path,
		GitUserAuth: &auth.UserAuth{
			Username: testUsername,
		},
	}

	err = cmd.CreateProwOwnersAliasesFile()
	assert.NoError(t, err, "There should be no error")

	wantFile := filepath.Join(path, "OWNERS_ALIASES")
	exists, err := util.FileExists(wantFile)
	assert.NoError(t, err, "It should find the OWNERS_ALIASES file without error")
	assert.True(t, exists, "It should create an OWNERS_ALIASES file")

	wantOwnersAliases := prow.OwnersAliases{
		Aliases:       []string{testUsername},
		BestApprovers: []string{testUsername},
		BestReviewers: []string{testUsername},
	}
	data, err := ioutil.ReadFile(wantFile)
	assert.NoError(t, err, "It should read the OWNERS_ALIASES file without error")
	ownersAliases := prow.OwnersAliases{}
	err = yaml.Unmarshal(data, &ownersAliases)
	assert.NoError(t, err, "It should unmarshal the OWNERS_ALIASES file without error")
	assert.Equal(t, wantOwnersAliases, ownersAliases)
}

func TestCreateProwOwnersAliasesFileCreateWhenDoesNotExistAndNoGitUserSet(t *testing.T) {
	t.Parallel()
	path, err := ioutil.TempDir("", "prow")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(path)

	cmd := cmd.ImportOptions{
		Dir: path,
	}

	err = cmd.CreateProwOwnersAliasesFile()
	assert.Error(t, err, "There should an error")
}
