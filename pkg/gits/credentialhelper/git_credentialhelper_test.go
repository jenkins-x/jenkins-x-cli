// +build unit

package credentialhelper

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

func TestGitCredentialHelper(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GitCredentials Suite")
}

var _ = Describe("GitCredential", func() {
	Context("#CreateGitCredential", func() {
		It("successfully creates GitCredential", func() {
			data := []string{
				"Protocol=http",
				"Host=github.com",
				"Path=jenkins-x/jx",
			}
			credentials, err := CreateGitCredential(data)
			Expect(err).Should(BeNil())
			Expect(credentials).To(MatchAllFields(Fields{
				"Protocol": Equal("http"),
				"Host":     Equal("github.com"),
				"Path":     Equal("jenkins-x/jx"),
				"Username": BeEmpty(),
				"Password": BeEmpty(),
			}))
		})

		It("passing nil fails GitCredential creation", func() {
			credentials, err := CreateGitCredential(nil)
			Expect(err).ShouldNot(BeNil())
			Expect(credentials).To(MatchAllFields(Fields{
				"Protocol": BeEmpty(),
				"Host":     BeEmpty(),
				"Path":     BeEmpty(),
				"Username": BeEmpty(),
				"Password": BeEmpty(),
			}))
		})

		It("missing key value pairs format fails GitCredential creation", func() {
			data := []string{"foo"}
			credentials, err := CreateGitCredential(data)
			Expect(err).ShouldNot(BeNil())
			Expect(credentials).To(MatchAllFields(Fields{
				"Protocol": BeEmpty(),
				"Host":     BeEmpty(),
				"Path":     BeEmpty(),
				"Username": BeEmpty(),
				"Password": BeEmpty(),
			}))
		})
	})

	Context("#CreateGitCredentialFromURL", func() {
		It("successfully creates GitCredential", func() {
			credentials, err := CreateGitCredentialFromURL("http://github.com", "johndoe", "1234")
			Expect(err).Should(BeNil())
			Expect(credentials).To(MatchAllFields(Fields{
				"Protocol": Equal("http"),
				"Host":     Equal("github.com"),
				"Path":     Equal(""),
				"Username": Equal("johndoe"),
				"Password": Equal("1234"),
			}))
		})
	})

	Context("#Clone", func() {
		var (
			testCredential GitCredential
			err            error
		)

		BeforeEach(func() {
			data := []string{
				"Protocol=http",
				"Host=github.com",
				"Path=jenkins-x/jx",
				"Username=johndoe",
				"Password=1234",
			}
			testCredential, err = CreateGitCredential(data)
			Expect(err).Should(BeNil())
		})

		It("successful clone", func() {
			clone := testCredential.Clone()
			Expect(&clone).ShouldNot(BeIdenticalTo(&testCredential))
			Expect(clone).To(MatchAllFields(Fields{
				"Protocol": Equal("http"),
				"Host":     Equal("github.com"),
				"Path":     Equal("jenkins-x/jx"),
				"Username": Equal("johndoe"),
				"Password": Equal("1234"),
			}))
		})

		Context("#String", func() {
			It("string representation of GitCredential has additional newline (needed by git credential protocol", func() {
				credential, err := CreateGitCredentialFromURL("http://github.com/jenkins-x", "johndoe", "1234")
				Expect(err).Should(BeNil())
				expected := heredoc.Doc(`protocol=http
                                              host=github.com
                                              path=/jenkins-x
                                              username=johndoe
                                              password=1234
			    `)
				expected = expected + "\n"
				Expect(credential.String()).Should(Equal(expected))
			})
		})
	})
})

var _ = Describe("GitCredentialsHelper", func() {
	Context("#CreateGitCredentialsHelper", func() {
		var (
			testIn          io.Reader
			testOut         io.Writer
			testCredentials []GitCredential
		)

		BeforeEach(func() {
			testIn = strings.NewReader("")
			testOut = bytes.NewBufferString("")
			testCredentials = []GitCredential{}
		})

		It("fails with no input writer", func() {
			helper, err := CreateGitCredentialsHelper(nil, testOut, testCredentials)
			Expect(err).ShouldNot(BeNil())
			Expect(helper).Should(BeNil())
		})

		It("fails with no output writer", func() {
			helper, err := CreateGitCredentialsHelper(testIn, nil, testCredentials)
			Expect(err).ShouldNot(BeNil())
			Expect(helper).Should(BeNil())
		})

		It("fails with no credentials", func() {
			helper, err := CreateGitCredentialsHelper(testIn, testOut, nil)
			Expect(err).ShouldNot(BeNil())
			Expect(helper).Should(BeNil())
		})

		It("succeeds when all parameters are specified", func() {
			helper, err := CreateGitCredentialsHelper(testIn, testOut, testCredentials)
			Expect(err).Should(BeNil())
			Expect(helper).ShouldNot(BeNil())
		})
	})

	Context("#Run", func() {
		var (
			testIn          io.Reader
			testOut         *bytes.Buffer
			testCredentials []GitCredential
			helper          *GitCredentialsHelper
			err             error
		)

		BeforeEach(func() {
			in := heredoc.Doc(`protocol=https
                                    host=github.com
                                    username=jx-bot
            `)
			testIn = strings.NewReader(in)
			testOut = bytes.NewBufferString("")
			testCredentials = []GitCredential{
				{Protocol: "https", Host: "github.com", Username: "jx-bot", Password: "1234"},
			}
			helper, err = CreateGitCredentialsHelper(testIn, testOut, testCredentials)
			Expect(err).Should(BeNil())
		})

		It("succeeds filling credentials", func() {
			expected := heredoc.Doc(`protocol=https
                                    host=github.com
                                    path=
                                    username=jx-bot
                                    password=1234
            `)
			expected = expected + "\n"
			err = helper.Run("get")
			Expect(err).Should(BeNil())
			Expect(testOut.String()).Should(Equal(expected))
		})
	})
})
