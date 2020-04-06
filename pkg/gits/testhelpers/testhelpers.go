package testhelpers

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// WriteFile creates a file with the specified name underneath the gitRepo directory adding the specified content.
// The file name can be path as well and intermediate directories are created.
func WriteFile(fail func(string, ...int), repoDir string, name string, contents string) {
	path := filepath.Join(repoDir, name)
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		log.Logger().Error(err.Error())
		fail("unable to create directory")
	}

	b := []byte(contents)
	err = ioutil.WriteFile(path, b, 0644)
	if err != nil {
		log.Logger().Error(err.Error())
		fail("unable to write file content")
	}
}

// HeadSha returns the commit SHA of the current HEAD commit within the specified git directory
func HeadSha(fail func(string, ...int), repoDir string) string {
	data, err := ioutil.ReadFile(filepath.Join(repoDir, ".git", "HEAD"))
	if err != nil {
		log.Logger().Error(err.Error())
		fail("unable to read file")
	}

	var sha string
	if strings.HasPrefix(string(data), "ref:") {
		headRef := strings.TrimPrefix(string(data), "ref: ")
		headRef = strings.Trim(headRef, "\n")
		sha = ReadRef(fail, repoDir, headRef)
	} else {
		sha = string(data)
	}

	return sha
}

// ReadRef reads the commit SHA of the specified ref. Needs to be of the form /refs/heads/<name>.
func ReadRef(fail func(string, ...int), repoDir string, name string) string {
	data, err := ioutil.ReadFile(filepath.Join(repoDir, ".git", name))
	if err != nil {
		log.Logger().Error(err.Error())
		fail("unable to read file")
	}
	return strings.Trim(string(data), "\n")
}

// Add adds all unstaged changes to the index.
func Add(fail func(string, ...int), repoDir string) {
	GitCmd(fail, repoDir, "add", ".")
}

// Commit commits all staged changes with the specified commit message.
func Commit(fail func(string, ...int), repoDir string, message string) string {
	GitCmd(fail, repoDir, "commit", "-m", message, "--no-gpg-sign")
	return HeadSha(fail, repoDir)
}

// Tag creates an annotated tag.
func Tag(fail func(string, ...int), repoDir string, tag string, message string) string {
	GitCmd(fail, repoDir, "tag", "-a", "-m", message, tag)
	return ReadRef(fail, repoDir, fmt.Sprintf("/refs/tags/%s", tag))
}

// Checkout switches to the specified branch.
func Checkout(fail func(string, ...int), repoDir string, branch string) {
	GitCmd(fail, repoDir, "checkout", branch)
}

// Branch creates a new branch with the specified name.
func Branch(fail func(string, ...int), repoDir string, branch string) {
	GitCmd(fail, repoDir, "checkout", "-b", branch)
}

// DetachHead puts the repository in a detached head mode.
func DetachHead(fail func(string, ...int), repoDir string) {
	head := HeadSha(fail, repoDir)
	GitCmd(fail, repoDir, "checkout", head)
}

// Merge merges the specified commits into the current branch
func Merge(fail func(string, ...int), repoDir string, commits ...string) {
	args := []string{"merge", "--no-gpg-sign"}
	args = append(args, commits...)
	GitCmd(fail, repoDir, args...)
}

// Revlist lists commits that are reachable by following the parent links from the given commit
func Revlist(fail func(string, ...int), repoDir string, maxCount int, commit string) string {
	args := []string{"rev-list", fmt.Sprintf("--max-count=%d", maxCount), commit}
	return GitCmd(fail, repoDir, args...)
}

// GitCmd runs a git command with arguments in the specified git repository
func GitCmd(fail func(string, ...int), repoDir string, args ...string) string {
	cmd := util.Command{
		Dir:  repoDir,
		Name: "git",
		Args: args,
	}
	out, err := cmd.RunWithoutRetry()
	if err != nil {
		log.Logger().Error(err.Error())
		fail("unable to write file content")
	}
	return out
}
