// +build integration

package test_integration_test

import (
	"fmt"
	"testing"

	jenkinsio_v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	v1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/kube"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPipelineActivity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Pipeline Activity Integration Test Suite")
}

var _ = Describe("PipelineActivity", func() {

	Describe("#ListSelectedPipelineActivities", func() {
		var (
			jxClient               versioned.Interface
			pipelineActivityClient v1.PipelineActivityInterface
			testPipelineActivities []jenkinsio_v1.PipelineActivity
			kubeClient             kubernetes.Interface
			testNamespace          string
			err                    error
		)

		BeforeSuite(func() {
			By("Creating JX and Kube Client")
			factory := clients.NewFactory()
			jxClient, _, err = factory.CreateJXClient()
			Expect(err).Should(BeNil())

			kubeClient, _, err = factory.CreateKubeClient()
			Expect(err).Should(BeNil())

			testNamespace = "activity-integration-test-" + rand.String(5)
			By(fmt.Sprintf("Creating test namespace %s", testNamespace))

			err = createNamespace(kubeClient, testNamespace)
			Expect(err).Should(BeNil())
		})

		BeforeEach(func() {
			pipelineActivityClient = jxClient.JenkinsV1().PipelineActivities(testNamespace)
			Expect(err).Should(BeNil())
		})

		AfterSuite(func() {
			By(fmt.Sprintf("Deleting test namespace %s", testNamespace))
			err = deleteNamespace(kubeClient, testNamespace)
			Expect(err).Should(BeNil())
		})

		Context("w/o PipelineActivity instances", func() {
			It("should return the empty list with nil selectors", func() {
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, nil)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(0))
			})

			It("should return the empty list with any selectors", func() {
				labelSelector := labels.Everything()
				fieldSelector := fields.OneTermEqualSelector("foo", "bar")
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, labelSelector, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(0))
			})
		})

		Context("w/ PipelineActivity instances", func() {
			BeforeEach(func() {
				if testPipelineActivities == nil {
					By("Creating test PipelineActivities")
					testPipelineActivities, err = createTestPipelineActivities(pipelineActivityClient)
					Expect(err).Should(BeNil())
				}
			})

			It("list w/o selectors should return all PipelineActivity instances", func() {
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, nil)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(len(testPipelineActivities)))
			})

			It("list w/ label selectors should return PipelineActivity instances with matching label", func() {
				requirement, err := labels.NewRequirement("gitBranch", selection.Equals, []string{"foo"})
				labelSelector := labels.NewSelector().Add(*requirement)
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, labelSelector, nil)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(0))

				requirement, err = labels.NewRequirement("branch", selection.Equals, []string{"branch-0"})
				labelSelector = labels.NewSelector().Add(*requirement)
				paList, err = kube.ListSelectedPipelineActivities(pipelineActivityClient, labelSelector, nil)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(1))
				Expect(paList.Items[0].Spec).Should(Equal(testPipelineActivities[0].Spec))
			})

			It("list w/ field selectors should return PipelineActivity instances with matching fields", func() {
				fieldSelector := fields.OneTermEqualSelector("gitOwner", "foo")
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(0))

				fieldSelector = fields.OneTermEqualSelector("spec.gitOwner", "johndoe-0")
				paList, err = kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(1))
				Expect(paList.Items[0].Spec).Should(Equal(testPipelineActivities[0].Spec))

				gitOwnerSelector := fields.OneTermEqualSelector("spec.gitOwner", "johndoe-0")
				gitRepoSelector := fields.OneTermEqualSelector("spec.gitRepository", "foo-0")
				fieldSelector = fields.AndSelectors(gitOwnerSelector, gitRepoSelector)
				paList, err = kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(1))
				Expect(paList.Items[0].Spec).Should(Equal(testPipelineActivities[0].Spec))
			})

			It("list w/ multiple field selectors should return PipelineActivity instances with matching fields", func() {
				gitOwnerSelector := fields.OneTermEqualSelector("spec.gitOwner", "johndoe-0")
				gitRepoSelector := fields.OneTermEqualSelector("spec.gitRepository", "bar")
				fieldSelector := fields.AndSelectors(gitOwnerSelector, gitRepoSelector)
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(0))

				gitOwnerSelector = fields.OneTermEqualSelector("spec.gitOwner", "johndoe-0")
				gitRepoSelector = fields.OneTermEqualSelector("spec.gitRepository", "foo-0")
				fieldSelector = fields.AndSelectors(gitOwnerSelector, gitRepoSelector)
				paList, err = kube.ListSelectedPipelineActivities(pipelineActivityClient, nil, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(1))
				Expect(paList.Items[0].Spec).Should(Equal(testPipelineActivities[0].Spec))
			})

			It("list w/ field and label selectors should return matching PipelineActivity instances", func() {
				requirement, err := labels.NewRequirement("branch", selection.Equals, []string{"branch-5"})
				labelSelector := labels.NewSelector().Add(*requirement)

				fieldSelector := fields.OneTermEqualSelector("spec.gitRepository", "foo-5")
				paList, err := kube.ListSelectedPipelineActivities(pipelineActivityClient, labelSelector, fieldSelector)
				Expect(err).Should(BeNil())

				Expect(paList).ShouldNot(BeNil())
				Expect(len(paList.Items)).Should(Equal(2))
			})
		})
	})
})

func createNamespace(client kubernetes.Interface, ns string) error {
	namespace := core_v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: ns,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(&namespace)
	return err
}

func deleteNamespace(client kubernetes.Interface, ns string) error {
	return client.CoreV1().Namespaces().Delete(ns, &meta_v1.DeleteOptions{})
}

func createTestPipelineActivities(client v1.PipelineActivityInterface) ([]jenkinsio_v1.PipelineActivity, error) {
	var pipelineActivities []jenkinsio_v1.PipelineActivity
	var clone jenkinsio_v1.PipelineActivity

	for i := 0; i < 10; i++ {
		pa := jenkinsio_v1.PipelineActivity{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: fmt.Sprintf("pa-%d", i),
				Labels: map[string]string{
					"lastCommitSHA": fmt.Sprintf("%d", i),
					"branch":        fmt.Sprintf("branch-%d", i),
				},
			},
			Spec: jenkinsio_v1.PipelineActivitySpec{
				GitOwner:      fmt.Sprintf("johndoe-%d", i),
				GitRepository: fmt.Sprintf("foo-%d", i),
				LastCommitSHA: fmt.Sprintf("%d", i),
				GitBranch:     fmt.Sprintf("branch-%d", i),
			},
		}

		if i == 5 {
			clone = *pa.DeepCopy()
			clone.Name = clone.Name + "-clone"
			created, err := client.Create(&clone)
			if err != nil {
				return nil, err
			}
			pipelineActivities = append(pipelineActivities, *created)
		}

		created, err := client.Create(&pa)
		if err != nil {
			return nil, err
		}
		pipelineActivities = append(pipelineActivities, *created)
	}

	return pipelineActivities, nil
}
