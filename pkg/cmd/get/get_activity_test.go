// +build unit

package get_test

import (
	"os"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/get"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestGetActivity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Get Activity Suite")
}

var _ = Describe("get activity", func() {
	Describe("Run()", func() {
		var (
			originalRepoOwner  string
			originalRepoName   string
			originalJobName    string
			originalBranchName string

			sort   bool
			err    error
			stdout *testhelpers.FakeOut
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
		})

		AfterEach(func() {
			os.Setenv("REPO_OWNER", originalRepoOwner)
			os.Setenv("REPO_NAME", originalRepoName)
			os.Setenv("JOB_NAME", originalJobName)
			os.Setenv("BRANCH_NAME", originalBranchName)
		})

		JustBeforeEach(func() {
			stdout = &testhelpers.FakeOut{}
			commonOpts := &opts.CommonOptions{
				Out: stdout,
			}
			commonOpts.SetDevNamespace("jx")

			testhelpers.ConfigureTestOptionsWithResources(commonOpts,
				[]runtime.Object{},
				[]runtime.Object{},
				&gits.GitFake{CurrentBranch: "job"},
				&gits.FakeProvider{},
				helm_test.NewMockHelmer(),
				resources_test.NewMockInstaller(),
			)

			c, ns, _ := commonOpts.JXClient()

			testhelpers.CreateTestPipelineActivityWithTime(c, ns, "jx-testing", "jx-testing", "job", "1", "workflow", v1.Date(2019, time.October, 10, 23, 0, 0, 0, time.UTC))
			testhelpers.CreateTestPipelineActivityWithTime(c, ns, "jx-testing", "jx-testing", "job", "2", "workflow", v1.Date(2019, time.January, 10, 23, 0, 0, 0, time.UTC))

			options := &get.GetActivityOptions{
				CommonOptions: commonOpts,
				Sort:          sort,
			}

			err = options.Run()
		})

		Context("Without flags", func() {
			BeforeEach(func() {
				sort = false
			})

			It("Prints a list of activities", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stdout.GetOutput()).To(ContainSubstring(`STARTED AGO DURATION STATUS
jx-testing/jx-testing/job #1`))
			})
		})

		Context("With  the sort flag", func() {
			BeforeEach(func() {
				sort = true
			})

			It("Prints a sorted list of activities", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stdout.GetOutput()).To(ContainSubstring(`STARTED AGO DURATION STATUS
jx-testing/jx-testing/job #2`))
			})
		})
	})
})
