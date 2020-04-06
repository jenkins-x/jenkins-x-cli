// +build unit

package credentials

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/MakeNowJust/heredoc"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/auth"

	"github.com/jenkins-x/jx/pkg/gits"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStepGitCredentials(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Step Git Credentials Suite")
}

var _ = Describe("step git merge", func() {
	var (
		authSvc    auth.ConfigService
		testWriter *os.File
	)

	BeforeEach(func() {
		By("capturing log output")
		_, testWriter, _ = os.Pipe()
		log.SetOutput(testWriter)
		_ = log.SetLevel("info")

		By("creating in memory auth service")
		authSvc = auth.NewMemoryAuthConfigService()
		cfg := auth.AuthConfig{
			Servers: []*auth.AuthServer{
				{
					URL: "https://" + "github.com",
					Users: []*auth.UserAuth{
						{
							GithubAppOwner: "jstrachan-gh-app",
							Username:       "jstrachan",
							ApiToken:       "lovelyLager",
						},
					},
					Name:        "gh",
					Kind:        gits.KindGitHub,
					CurrentUser: "jstrachan",
				},
				{
					URL: "http://" + "github.beescloud.com",
					Users: []*auth.UserAuth{
						{
							Username: "rawlingsj",
							ApiToken: "glassOfNice",
						},
					},
					Name:        "bee",
					Kind:        gits.KindGitHub,
					CurrentUser: "rawlingsj",
				},
			},
		}
		authSvc.SetConfig(&cfg)
	})

	Context("#createGitCredentialsFile", func() {
		var (
			tmpDir  string
			outFile string
			err     error
		)

		BeforeEach(func() {
			tmpDir, err = ioutil.TempDir("", "gitcredentials")
			Expect(err).Should(BeNil())

			outFile = filepath.Join(tmpDir, "credentials")

		})

		AfterEach(func() {
			_ = os.RemoveAll(tmpDir)
		})

		It("successfully creates git credential file for known users", func() {

			expected := heredoc.Doc(`https://jstrachan:lovelyLager@github.com
                                          http://rawlingsj:glassOfNice@github.beescloud.com
                                          https://rawlingsj:glassOfNice@github.beescloud.com
			`)

			options := &StepGitCredentialsOptions{
				OutputFile: outFile,
			}

			credentials, err := options.CreateGitCredentialsFromAuthService(authSvc, false)
			Expect(err).Should(BeNil())
			err = options.createGitCredentialsFile(outFile, credentials)
			Expect(err).Should(BeNil())

			data, err := ioutil.ReadFile(outFile)
			Expect(err).Should(BeNil())
			actual := string(data)
			Expect(actual).Should(Equal(expected))
		})

		It("successfully creates git credential file for GitHub App", func() {

			expected := heredoc.Doc(`https://jstrachan:lovelyLager@github.com
			`)

			options := &StepGitCredentialsOptions{
				OutputFile: outFile,
				RepoOwner:  "jstrachan-gh-app",
				GitKind:    gits.KindGitHub,
			}

			credentials, err := options.CreateGitCredentialsFromAuthService(authSvc, true)
			Expect(err).Should(BeNil())
			err = options.createGitCredentialsFile(outFile, credentials)
			Expect(err).Should(BeNil())

			data, err := ioutil.ReadFile(outFile)
			Expect(err).Should(BeNil())
			actual := string(data)
			Expect(actual).Should(Equal(expected))
		})
	})
})
