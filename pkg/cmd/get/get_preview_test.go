// +build unit

package get_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	"k8s.io/apimachinery/pkg/runtime"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	clientmocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGetPreview(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Get Preview Suite")
}

var _ = Describe("get preview", func() {
	Describe("CurrentPreviewUrl()", func() {
		var (
			originalRepoOwner  string
			originalRepoName   string
			originalJobName    string
			originalBranchName string

			devEnv *jenkinsv1.Environment

			err    error
			stdout []byte
		)

		BeforeEach(func() {
			originalRepoOwner = os.Getenv("REPO_OWNER")
			originalRepoName = os.Getenv("REPO_NAME")
			originalJobName = os.Getenv("JOB_NAME")
			originalBranchName = os.Getenv("BRANCH_NAME")

			os.Setenv("REPO_OWNER", "jx-testing")
			os.Setenv("REPO_NAME", "jx-testing")
			os.Setenv("JOB_NAME", "job")
			os.Setenv("BRANCH_NAME", "job")

			devEnv = kube.NewPreviewEnvironment("jx-testing-jx-testing-job")
			devEnv.Spec.PreviewGitSpec.ApplicationURL = "http://example.com"
		})

		AfterEach(func() {
			os.Setenv("REPO_OWNER", originalRepoOwner)
			os.Setenv("REPO_NAME", originalRepoName)
			os.Setenv("JOB_NAME", originalJobName)
			os.Setenv("BRANCH_NAME", originalBranchName)
		})

		JustBeforeEach(func() {
			commonOpts := &opts.CommonOptions{}
			commonOpts.SetDevNamespace("jx")

			factory := clientmocks.NewMockFactory()
			user := jenkinsv1.User{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "jx",
					Name:      "test-user",
				},
				Spec: jenkinsv1.UserDetails{
					Name:     "Test",
					Email:    "test@test.com",
					Accounts: make([]jenkinsv1.AccountReference, 0),
				},
			}

			testhelpers.ConfigureTestOptionsWithResources(commonOpts,
				[]runtime.Object{},
				[]runtime.Object{&user, devEnv},
				&gits.GitFake{CurrentBranch: "job"},
				&gits.FakeProvider{},
				helm_test.NewMockHelmer(),
				resources_test.NewMockInstaller(),
			)

			commonOpts.SetFactory(factory)

			options := &get.GetPreviewOptions{
				GetEnvOptions: get.GetEnvOptions{
					GetOptions: get.GetOptions{
						CommonOptions: commonOpts,
					},
				},
			}

			r, w, _ := os.Pipe()
			tmp := os.Stdout
			defer func() {
				os.Stdout = tmp
			}()
			os.Stdout = w
			go func() {
				err = options.CurrentPreviewUrl()
				w.Close()
			}()
			stdout, _ = ioutil.ReadAll(r)
		})

		It("prints the preview url to stdout", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(string(stdout)).To(Equal("http://example.com\n"))
		})

		Context("Without the required environment variables", func() {
			BeforeEach(func() {
				os.Unsetenv("REPO_OWNER")
				os.Unsetenv("REPO_NAME")
				os.Unsetenv("JOB_NAME")
			})

			It("errors out", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("No $JOB_NAME defined for the current pipeline job to use"))
				Expect(string(stdout)).To(Equal(""))
			})
		})

		Context("Without any previews", func() {
			BeforeEach(func() {
				devEnv = &jenkinsv1.Environment{}
			})

			It("errors out", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("No Preview for name: jx-testing-jx-testing-job"))
				Expect(string(stdout)).To(Equal(""))
			})
		})
	})
})
