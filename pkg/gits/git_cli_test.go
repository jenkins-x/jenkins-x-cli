// +build unit

package gits_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/gits/testhelpers"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGitCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Git CLI Test Suite")
}

var _ = Describe("Git CLI", func() {
	var (
		git                *gits.GitCLI
		repoDir            string
		err                error
		origAuthor         string
		origAuthorEmail    string
		origCommitter      string
		origCommitterEmail string
	)

	BeforeSuite(func() {
		// comment out to see logging output
		//log.SetOutput(ioutil.Discard)
		_ = log.SetLevel("info")

		origCommitter = os.Getenv("GIT_COMMITTER_NAME")
		_ = os.Setenv("GIT_COMMITTER_NAME", "test-committer")
		origCommitterEmail = os.Getenv("GIT_COMMITTER_EMAIL")
		_ = os.Setenv("GIT_COMMITTER_EMAIL", "test-committer@acme.com")

		origAuthor = os.Getenv("GIT_AUTHOR_NAME")
		_ = os.Setenv("GIT_AUTHOR_NAME", "test-author")
		origAuthorEmail = os.Getenv("GIT_AUTHOR_EMAIL")
		_ = os.Setenv("GIT_AUTHOR_EMAIL", "test-author@acme.com")

		git = &gits.GitCLI{}
	})

	AfterSuite(func() {
		if origCommitter != "" {
			_ = os.Setenv("GIT_COMMITTER_NAME", origCommitter)
		} else {
			_ = os.Unsetenv("GIT_COMMITTER_NAME")
		}

		if origCommitterEmail != "" {
			_ = os.Setenv("GIT_COMMITTER_EMAIL", origCommitterEmail)
		} else {
			_ = os.Unsetenv("GIT_COMMITTER_EMAIL")
		}

		if origAuthor != "" {
			_ = os.Setenv("GIT_AUTHOR_NAME", origAuthor)
		} else {
			_ = os.Unsetenv("GIT_AUTHOR_NAME")
		}

		if origAuthorEmail != "" {
			_ = os.Setenv("GIT_AUTHOR_EMAIL", origAuthorEmail)
		} else {
			_ = os.Unsetenv("GIT_AUTHOR_EMAIL")
		}
	})

	BeforeEach(func() {
		repoDir, err = ioutil.TempDir("", "jenkins-x-git-test-repo-")
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("creating a test repository in '%s'", repoDir))
		testhelpers.GitCmd(Fail, repoDir, "init")
	})

	AfterEach(func() {
		By("deleting temp repo")
		_ = os.RemoveAll(repoDir)
	})

	Describe("#ConvertToValidBranchName", func() {
		It("converts a string into a valid git branch name", func() {
			testCases := []struct {
				input    string
				expected string
			}{
				{
					"testing-thingy", "testing-thingy",
				},
				{
					"testing-thingy/", "testing-thingy",
				},
				{
					"testing-thingy.lock", "testing-thingy",
				},
				{
					"foo bar", "foo_bar",
				},
				{
					"foo\t ~bar", "foo_bar",
				},
			}
			for _, data := range testCases {
				actual := git.ConvertToValidBranchName(data.input)
				Expect(actual).Should(Equal(data.expected))
			}
		})
	})

	Describe("#Config", func() {
		It("should return an error if no repoDir is specified", func() {
			err := git.Config("")
			Expect(err).ShouldNot(BeNil())
		})

		It("should return error if no parameters are passed", func() {
			err := git.Config(repoDir)
			Expect(err).ShouldNot(BeNil())
		})

		It("should apply the specified config", func() {
			err := git.Config(repoDir, "--local", "credential.helper", "/path/to/jx step git credentials --credential-helper")
			Expect(err).Should(BeNil())

			filename := filepath.Join(repoDir, ".git", "config")
			Expect(util.FileExists(filename)).Should(Equal(true))
			contents, err := ioutil.ReadFile(filename)
			Expect(err).Should(BeNil())
			Expect(string(contents)).Should(ContainSubstring("helper = /path/to/jx step git credentials --credential-helper"))
		})
	})

	Describe("#GetCommits", func() {
		var (
			commitASha string
			commitBSha string
			commitCSha string
			commitDSha string
			commitESha string
		)

		BeforeEach(func() {
			By("adding commit a on master branch")
			testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
			testhelpers.Add(Fail, repoDir)
			commitASha = testhelpers.Commit(Fail, repoDir, "commit a")

			By("creating branch 'b' and adding a commit")
			testhelpers.Branch(Fail, repoDir, "b")
			testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
			testhelpers.Add(Fail, repoDir)
			commitBSha = testhelpers.Commit(Fail, repoDir, "commit b")

			By("creating branch 'c' and adding a commit")
			testhelpers.Checkout(Fail, repoDir, "master")
			testhelpers.Branch(Fail, repoDir, "c")
			testhelpers.WriteFile(Fail, repoDir, "c.txt", "c")
			testhelpers.Add(Fail, repoDir)
			commitCSha = testhelpers.Commit(Fail, repoDir, "commit c")

			testhelpers.Checkout(Fail, repoDir, "master")
			By("adding commit d on master branch")
			testhelpers.WriteFile(Fail, repoDir, "d.txt", "d")
			testhelpers.Add(Fail, repoDir)
			commitDSha = testhelpers.Commit(Fail, repoDir, "commit d")

			By("adding commit e on master branch")
			testhelpers.WriteFile(Fail, repoDir, "e.txt", "e")
			testhelpers.Add(Fail, repoDir)
			commitESha = testhelpers.Commit(Fail, repoDir, "commit e")
		})

		It("returns all commits in range", func() {
			commits, err := git.GetCommits(repoDir, commitASha, commitESha)
			Expect(err).NotTo(HaveOccurred())
			Expect(commits).Should(HaveLen(2))
			Expect(commits[0].SHA).Should(Equal(commitESha))
			Expect(commits[0].Message).Should(Equal("commit e"))
			Expect(commits[1].SHA).Should(Equal(commitDSha))
			Expect(commits[1].Message).Should(Equal("commit d"))
		})

		It("returns author and committer", func() {
			commits, err := git.GetCommits(repoDir, commitASha, commitDSha)
			Expect(err).NotTo(HaveOccurred())
			Expect(commits).Should(HaveLen(1))
			Expect(commits[0].Author.Name).Should(Equal("test-author"))
			Expect(commits[0].Author.Email).Should(Equal("test-author@acme.com"))
			Expect(commits[0].Committer.Name).Should(Equal("test-committer"))
			Expect(commits[0].Committer.Email).Should(Equal("test-committer@acme.com"))
		})

		It("returns merge commits", func() {
			testhelpers.Merge(Fail, repoDir, commitBSha, commitCSha)
			Expect(err).NotTo(HaveOccurred())

			commits, err := git.GetCommits(repoDir, commitESha, "HEAD")
			Expect(err).NotTo(HaveOccurred())
			Expect(commits).Should(HaveLen(3))
			Expect(commits[0].Message).Should(ContainSubstring("Merge commit"))
		})
	})

	Describe("#GetLatestCommitSha", func() {
		Context("when there is no commit", func() {
			Specify("an error is returned", func() {
				_, err := git.GetLatestCommitSha(repoDir)
				Expect(err).ShouldNot(BeNil())
				// TODO Currently the error message is returned which seems odd. Should be empty string imo (HF)
				//Expect(sha).Should(BeEmpty())
			})
		})

		Context("when there are commits", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "foo")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "first commit")
			})

			Specify("the sha of the last commit is returned", func() {
				sha, err := git.GetLatestCommitSha(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(testhelpers.ReadRef(Fail, repoDir, "refs/heads/master")))
			})
		})
	})

	Describe("#GetCommitPointedToByTag", func() {
		Context("when there is no commit", func() {
			Specify("an error is returned", func() {
				sha, err := git.GetCommitPointedToByTag(repoDir, "v0.0.1")
				Expect(err).ShouldNot(BeNil())
				Expect(sha).Should(BeEmpty())
			})
		})

		Context("when there is no tags", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "foo")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "first commit")
			})
			Specify("an error is returned", func() {
				sha, err := git.GetCommitPointedToByTag(repoDir, "v0.0.1")
				Expect(err).ShouldNot(BeNil())
				Expect(sha).Should(BeEmpty())
			})
		})

		Context("when there are commits", func() {
			var (
				tag2CommitSHA string
			)
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "foo")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "first commit")
				testhelpers.Tag(Fail, repoDir, "v0.0.1", "version 0.0.1")

				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "second commit")
				testhelpers.Tag(Fail, repoDir, "v0.0.2", "version 0.0.2")
				tag2CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.2")

				testhelpers.WriteFile(Fail, repoDir, "c.txt", "c")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "third commit")
				testhelpers.Tag(Fail, repoDir, "v0.0.3", "version 0.0.3")
				testhelpers.Revlist(Fail, repoDir, 1, "v0.0.3")
			})

			Specify("the sha of the specified tag is returned", func() {
				sha, err := git.GetCommitPointedToByTag(repoDir, "v0.0.2")
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2CommitSHA))
			})
		})
	})

	Describe("Get version tags", func() {
		var (
			tag2SHA       string
			tag3SHA       string
			tag2CommitSHA string
			tag3CommitSHA string
		)

		Context("when tags are on master", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "first commit")
				testhelpers.Tag(Fail, repoDir, "v0.0.1", "version 0.0.1")

				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "second commit")
				tag2SHA = testhelpers.Tag(Fail, repoDir, "v0.0.2", "version 0.0.2")
				tag2CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.2")

				testhelpers.WriteFile(Fail, repoDir, "c.txt", "c")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "third commit")
				tag3SHA = testhelpers.Tag(Fail, repoDir, "v0.0.3", "version 0.0.3")
				tag3CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.3")
			})

			It("#GetNthTagSHA1", func() {
				sha, tag, err := git.NthTag(repoDir, 1)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag3SHA))
				Expect(tag).Should(Equal("v0.0.3"))
			})

			It("#GetNthTagSHA2", func() {
				sha, tag, err := git.NthTag(repoDir, 2)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2SHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})
			It("#GetCommitPointedToByLatestTag", func() {
				sha, tag, err := git.GetCommitPointedToByLatestTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag3CommitSHA))
				Expect(tag).Should(Equal("v0.0.3"))
			})

			It("#GetCommitPointedToByPreviousTag", func() {
				sha, tag, err := git.GetCommitPointedToByPreviousTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2CommitSHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})
		})

		Context("when there is only one tag", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "second commit")
				tag2SHA = testhelpers.Tag(Fail, repoDir, "v0.0.2", "version 0.0.2")
				tag2CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.2")
			})

			It("#GetNthTagSHA1", func() {
				sha, tag, err := git.NthTag(repoDir, 1)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2SHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})

			It("#GetNthTagSHA2", func() {
				sha, tag, err := git.NthTag(repoDir, 2)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(""))
				Expect(tag).Should(Equal(""))
			})
			It("#GetCommitPointedToByLatestTag", func() {
				sha, tag, err := git.GetCommitPointedToByLatestTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2CommitSHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})

			It("#GetCommitPointedToByPreviousTag", func() {
				sha, tag, err := git.GetCommitPointedToByPreviousTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(""))
				Expect(tag).Should(Equal(""))
			})
		})

		Context("when tags are made on release branches", func() {
			BeforeEach(func() {
				By("creating commit on master")
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "first master commit")

				By("creating first release branch")
				testhelpers.Branch(Fail, repoDir, "release_0_0_1")
				testhelpers.WriteFile(Fail, repoDir, "VERSION", "0.0.1")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "adding version")
				testhelpers.Tag(Fail, repoDir, "v0.0.1", "version 0.0.1")

				By("creating commit on master")
				testhelpers.Checkout(Fail, repoDir, "master")
				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "second master commit")

				By("creating second release branch")
				testhelpers.Branch(Fail, repoDir, "release_0_0_2")
				testhelpers.WriteFile(Fail, repoDir, "VERSION", "0.0.2")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "adding version")
				tag2SHA = testhelpers.Tag(Fail, repoDir, "v0.0.2", "version 0.0.2")
				tag2CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.2")

				By("creating commit on master")
				testhelpers.Checkout(Fail, repoDir, "master")
				testhelpers.WriteFile(Fail, repoDir, "c.txt", "c")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "third master commit")

				By("creating third release branch")
				testhelpers.Branch(Fail, repoDir, "release_0_0_3")
				testhelpers.WriteFile(Fail, repoDir, "VERSION", "0.0.3")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "adding version")
				tag3SHA = testhelpers.Tag(Fail, repoDir, "v0.0.3", "version 0.0.3")
				tag3CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.3")
			})

			It("#GetNthTagSHA1", func() {
				sha, tag, err := git.NthTag(repoDir, 1)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag3SHA))
				Expect(tag).Should(Equal("v0.0.3"))
			})

			It("#GetNthTagSHA2", func() {
				sha, tag, err := git.NthTag(repoDir, 2)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2SHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})
			It("#GetCommitPointedToByLatestTag", func() {
				sha, tag, err := git.GetCommitPointedToByLatestTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag3CommitSHA))
				Expect(tag).Should(Equal("v0.0.3"))
			})

			It("#GetCommitPointedToByPreviousTag", func() {
				sha, tag, err := git.GetCommitPointedToByPreviousTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2CommitSHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})
		})

		Context("when tags are made in detached HEAD mode", func() {
			BeforeEach(func() {
				By("creating commit on master")
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "first master commit")

				By("detaching HEAD and creating first release")
				testhelpers.DetachHead(Fail, repoDir)
				testhelpers.WriteFile(Fail, repoDir, "VERSION", "0.0.1")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "adding version")
				testhelpers.Tag(Fail, repoDir, "v0.0.1", "version 0.0.1")

				By("creating commit on master")
				testhelpers.Checkout(Fail, repoDir, "master")
				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "second master commit")

				By("detaching HEAD and creating second release")
				testhelpers.DetachHead(Fail, repoDir)
				testhelpers.WriteFile(Fail, repoDir, "VERSION", "0.0.2")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "adding version")
				tag2SHA = testhelpers.Tag(Fail, repoDir, "v0.0.2", "version 0.0.2")
				tag2CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.2")

				By("creating commit on master")
				testhelpers.Checkout(Fail, repoDir, "master")
				testhelpers.WriteFile(Fail, repoDir, "c.txt", "c")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "third master commit")

				By("detaching HEAD and creating second release")
				testhelpers.DetachHead(Fail, repoDir)
				testhelpers.WriteFile(Fail, repoDir, "VERSION", "0.0.3")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "adding version")
				tag3SHA = testhelpers.Tag(Fail, repoDir, "v0.0.3", "version 0.0.3")
				tag3CommitSHA = testhelpers.Revlist(Fail, repoDir, 1, "v0.0.3")
			})

			It("#GetNthTagSHA1", func() {
				sha, tag, err := git.NthTag(repoDir, 1)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag3SHA))
				Expect(tag).Should(Equal("v0.0.3"))
			})

			It("#GetNthTagSHA2", func() {
				sha, tag, err := git.NthTag(repoDir, 2)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2SHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})
			It("#GetCommitPointedToByLatestTag", func() {
				sha, tag, err := git.GetCommitPointedToByLatestTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag3CommitSHA))
				Expect(tag).Should(Equal("v0.0.3"))
			})

			It("#GetCommitPointedToByPreviousTag", func() {
				sha, tag, err := git.GetCommitPointedToByPreviousTag(repoDir)
				Expect(err).Should(BeNil())
				Expect(sha).Should(Equal(tag2CommitSHA))
				Expect(tag).Should(Equal("v0.0.2"))
			})
		})
	})

	Describe("#DeleteLocalBranch", func() {
		Context("when there is no branch", func() {
			Specify("no error is returned", func() {
				err := git.DeleteLocalBranch(repoDir, "b")
				Expect(err).ShouldNot(BeNil())
			})
		})

		Context("when there is a branch", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "commit a")

				testhelpers.Branch(Fail, repoDir, "b")
				testhelpers.Checkout(Fail, repoDir, "master")
			})

			Specify("the branch is deleted", func() {
				err := git.DeleteLocalBranch(repoDir, "b")
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("#CheckoutCommitFiles", func() {
		var (
			commitSha string
		)

		Context("when there is no file to checkout", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.Add(Fail, repoDir)
				commitSha = testhelpers.Commit(Fail, repoDir, "commit a")
			})
			Specify("an error is returned", func() {
				err := git.CheckoutCommitFiles(repoDir, commitSha, []string{"b.txt"})
				Expect(err).ShouldNot(BeNil())
			})
		})

		Context("when there is single file to checkout", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.Add(Fail, repoDir)
				commitSha = testhelpers.Commit(Fail, repoDir, "commit a")

				testhelpers.WriteFile(Fail, repoDir, "a.txt", "ab")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "commit b")
			})

			Specify("the file is checked out", func() {
				err := git.CheckoutCommitFiles(repoDir, commitSha, []string{"a.txt"})
				Expect(err).Should(BeNil())
			})
		})

		Context("when there are multiple files to checkout", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
				testhelpers.WriteFile(Fail, repoDir, "c.txt", "c")
				testhelpers.Add(Fail, repoDir)
				commitSha = testhelpers.Commit(Fail, repoDir, "commit a")

				testhelpers.WriteFile(Fail, repoDir, "a.txt", "new a")
				testhelpers.WriteFile(Fail, repoDir, "b.txt", "new b")
				testhelpers.Add(Fail, repoDir)
				testhelpers.Commit(Fail, repoDir, "commit b")
			})

			Specify("the file is checked out", func() {
				err := git.CheckoutCommitFiles(repoDir, commitSha, []string{"a.txt", "b.txt"})
				Expect(err).Should(BeNil())
			})
		})
	})

	Describe("#HasChanges", func() {
		Context("when there are changes in directory", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
			})

			Specify("a changes are shown", func() {
				changed, err := git.HasChanges(repoDir)
				Expect(err).Should(BeNil())
				Expect(changed).Should(BeTrue())
			})
		})
	})

	Describe("#HasFileChanged", func() {
		Context("when there is not a file change", func() {
			Specify("a file does not show as changed", func() {
				changed, err := git.HasFileChanged(repoDir, "a.txt")
				Expect(err).Should(BeNil())
				Expect(changed).Should(BeFalse())
			})
		})

		Context("when there is a file change", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
			})

			Specify("a file shows as changed", func() {
				changed, err := git.HasFileChanged(repoDir, "a.txt")
				Expect(err).Should(BeNil())
				Expect(changed).Should(BeTrue())
			})
		})

		Context("when there is multiple file changes", func() {
			BeforeEach(func() {
				testhelpers.WriteFile(Fail, repoDir, "a.txt", "a")
				testhelpers.WriteFile(Fail, repoDir, "b.txt", "b")
			})

			Specify("a specific file shows as changed", func() {
				changed, err := git.HasFileChanged(repoDir, "a.txt")
				Expect(err).Should(BeNil())
				Expect(changed).Should(BeTrue())
			})
		})
	})
})

func TestTags(t *testing.T) {
	gitter := gits.NewGitCLI()
	dir, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	err = gitter.Init(dir)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "README.md"), []byte("Hello"), 0655)
	assert.NoError(t, err)
	err = gitter.Add(dir, "README.md")
	assert.NoError(t, err)
	err = gitter.CommitDir(dir, "commit 1")
	assert.NoError(t, err)
	err = gitter.CreateTag(dir, "v0.0.1", "version 0.0.1")
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "README.md"), []byte("Hello again"), 0655)
	assert.NoError(t, err)
	err = gitter.Add(dir, "README.md")
	assert.NoError(t, err)
	err = gitter.CommitDir(dir, "commit 3")
	assert.NoError(t, err)
	err = gitter.CreateTag(dir, "v0.0.2", "version 0.0.2")
	assert.NoError(t, err)
	tags, err := gitter.Tags(dir)
	assert.NoError(t, err)
	assert.Len(t, tags, 2)
	assert.Contains(t, tags, "v0.0.1")
	assert.Contains(t, tags, "v0.0.2")
}

func TestGitCLI_Stash(t *testing.T) {
	tests := []struct {
		name        string
		g           *gits.GitCLI
		initFn      func(dir string, gitter gits.Gitter) error
		postPush    func(dir string) error
		postPop     func(dir string) error
		wantPushErr bool
		wantPopErr  bool
	}{
		{
			name: "README",
			initFn: func(dir string, gitter gits.Gitter) error {
				err := ioutil.WriteFile(filepath.Join(dir, "README"), []byte("Hello!"), 0655)
				if err != nil {
					return errors.WithStack(err)
				}
				err = gitter.Add(dir, "*")
				assert.NoError(t, err)
				return nil
			},
			postPush: func(dir string) error {
				_, err := os.Stat(filepath.Join(dir, "README"))
				assert.Error(t, err)
				assert.True(t, os.IsNotExist(err))
				return nil
			},
			postPop: func(dir string) error {
				_, err := os.Stat(filepath.Join(dir, "README"))
				assert.NoError(t, err)
				data, err := ioutil.ReadFile(filepath.Join(dir, "README"))
				assert.NoError(t, err)
				assert.Equal(t, "Hello!", string(data))
				return nil
			},
		}, {
			name: "NothingToPop",
			initFn: func(dir string, gitter gits.Gitter) error {
				return nil
			},
			postPush: func(dir string) error {
				_, err := os.Stat(filepath.Join(dir, "README"))
				assert.Error(t, err)
				assert.True(t, os.IsNotExist(err))
				return nil
			},
			postPop: func(dir string) error {
				_, err := os.Stat(filepath.Join(dir, "README"))
				assert.Error(t, err)
				assert.True(t, os.IsNotExist(err))
				return nil
			},
			wantPopErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gits.NewGitCLI()
			dir, err := ioutil.TempDir("", "")
			defer func() {
				os.RemoveAll(dir)
			}()
			assert.NoError(t, err)
			err = g.Init(dir)
			assert.NoError(t, err)
			err = g.AddCommit(dir, "Initial Commit")
			assert.NoError(t, err)
			err = tt.initFn(dir, g)
			assert.NoError(t, err)
			err = g.StashPush(dir)
			if tt.wantPushErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			err = tt.postPush(dir)
			assert.NoError(t, err)

			err = g.StashPop(dir)
			if tt.wantPopErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			err = tt.postPop(dir)
			assert.NoError(t, err)
		})
	}
}

func TestGitCLI_Remotes(t *testing.T) {
	tests := []struct {
		initFn  func(dir string, gitter gits.Gitter) error
		name    string
		g       *gits.GitCLI
		want    []string
		wantErr bool
	}{
		{
			name: "origin",
			initFn: func(dir string, gitter gits.Gitter) error {
				rDir, err := ioutil.TempDir("", "")
				defer func() {
					os.RemoveAll(rDir)
				}()
				assert.NoError(t, err)
				err = gitter.Init(rDir)
				assert.NoError(t, err)
				err = gitter.AddCommit(rDir, "Initial Commit")
				assert.NoError(t, err)
				err = gitter.AddRemote(dir, "origin", fmt.Sprintf("file://%s", rDir))
				assert.NoError(t, err)
				return nil
			},
			want: []string{"origin"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := gits.NewGitCLI()
			dir, err := ioutil.TempDir("", "")
			defer func() {
				os.RemoveAll(dir)
			}()
			assert.NoError(t, err)
			err = g.Init(dir)
			assert.NoError(t, err)
			err = g.AddCommit(dir, "Initial Commit")
			assert.NoError(t, err)
			err = tt.initFn(dir, g)
			assert.NoError(t, err)
			got, err := g.Remotes(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("GitCLI.Remotes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GitCLI.Remotes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDescribe(t *testing.T) {
	tests := []struct {
		initFn   func(dir string, gitter gits.Gitter) error
		name     string
		want     []string
		wantErr  bool
		contains bool
		abbrev   string // Number of hex digits from object name, defaults to 7
		fallback bool   // If given, will just return the original ref
	}{
		{
			initFn: func(dir string, git gits.Gitter) error {
				err := git.Init(dir)
				assert.NoError(t, err)

				err = git.AddCommit(dir, "Initial Commit")
				assert.NoError(t, err)
				err = git.CreateTag(dir, "v0.0.1", "First Tag")
				assert.NoError(t, err)
				return err
			},
			name:     "Valid commit and tag, !contains, abbrev=default, !fallback",
			wantErr:  false,
			contains: false,
			abbrev:   "",
			fallback: false,
		},
		{
			initFn: func(dir string, git gits.Gitter) error {
				var err error
				assert.NoError(t, err)
				err = git.Init(dir)
				assert.NoError(t, err)

				err = git.AddCommit(dir, "Initial Commit")
				assert.NoError(t, err)
				return err
			},
			name:     "Commit but no tag, !contains, abbrev=default, !fallback",
			wantErr:  true,
			contains: false,
			abbrev:   "",
			fallback: false,
		},
		{
			initFn: func(dir string, git gits.Gitter) error {
				var err error
				assert.NoError(t, err)
				err = git.Init(dir)
				assert.NoError(t, err)

				err = git.AddCommit(dir, "Initial Commit")
				assert.NoError(t, err)
				return err
			},
			name:     "Commit but no tag, contains, abbrev=default, fallback",
			wantErr:  false,
			contains: true,
			abbrev:   "",
			fallback: true,
		},
	}

	for _, test := range tests {
		dir, err := ioutil.TempDir("", "")
		assert.NoError(t, err)

		gitCli := gits.NewGitCLI()

		err = test.initFn(dir, gitCli)
		assert.NoError(t, err)
		parts1, parts2, err := gitCli.Describe(
			dir,
			test.contains,
			"HEAD",
			test.abbrev,
			test.fallback,
		)

		if !test.wantErr {
			assert.NoError(t, err)
			assert.NotNil(t, parts1)
			assert.Equal(t, "", parts2)
		} else {
			assert.Error(t, err)
			assert.Equal(t, "", parts1)
			assert.Equal(t, "", parts2)
		}
	}

}
