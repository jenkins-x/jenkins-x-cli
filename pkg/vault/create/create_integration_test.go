// +build integration

package create

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/jenkins-x/jx/pkg/kube/cluster"

	"github.com/Pallinder/go-randomdata"
	"k8s.io/apimachinery/pkg/util/errors"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"k8s.io/client-go/kubernetes"

	vaultoperatorclient "github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	originalJxHome      string
	tempJxHome          string
	originalKubeCfg     string
	tempKubeCfg         string
	devNamespace        string
	factory             clients.Factory
	kubeClient          kubernetes.Interface
	vaultOperatorClient vaultoperatorclient.Interface
	gcloud              gke.GCloud
	err                 error

	vaultCreatorUnderTest defaultVaultCreator

	randomSuffix   string
	testNamespace  string
	testBucketName string
)

func TestVaultCreation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Vault Test Suite")
}

var _ = BeforeSuite(func() {
	By("Silencing logger")
	log.SetOutput(ioutil.Discard)

	By("Setting up a temporary JX_HOME")
	originalJxHome, tempJxHome, err = testhelpers.CreateTestJxHomeDir()
	Expect(err).Should(BeNil())

	By("Creating a temporary KUBECONFIG")
	originalKubeCfg, tempKubeCfg, err = testhelpers.CreateTestKubeConfigDir()
	Expect(err).Should(BeNil())

	By("Creating the clients factory")
	factory = clients.NewFactory()

	By("Creating a Kube client")
	var ns string
	kubeClient, ns, err = factory.CreateKubeClient()
	Expect(err).Should(BeNil())

	By("Retrieving the dev namespace")
	devNamespace, _, err = kube.GetDevNamespace(kubeClient, ns)
	Expect(err).Should(BeNil())

	By("Creating a Vault operator client")
	vaultOperatorClient, err = factory.CreateVaultOperatorClient()
	Expect(err).Should(BeNil())

	By("Creating a gcloud client")
	gcloud = gke.GCloud{}

	By("Creating VaultCreator under test")
	vaultCreatorUnderTest = defaultVaultCreator{}
})

var _ = AfterSuite(func() {
	By("Deleting temporary JX_HOME")
	err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
	Expect(err).Should(BeNil())

	By("Deleting a temporary KUBECONFIG")
	err = testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
	Expect(err).Should(BeNil())

})

var _ = Describe("Vault create/update", func() {
	BeforeEach(func() {
		randomSuffix = strings.ToLower(randomdata.SillyName())
		testNamespace = "vault-creation-test-" + randomSuffix
		testBucketName = "create-integration-test-vault-bucket-" + randomSuffix
	})

	AfterEach(func() {
		By("Deleting a test namespace")
		_ = kubeClient.CoreV1().Namespaces().Delete(testNamespace, &metav1.DeleteOptions{})
	})

	Describe("#createAuthServiceAccount", func() {
		It("should successfully create service account", func() {
			vaultName := "my-vault"
			sa, err := vaultCreatorUnderTest.createAuthServiceAccount(kubeClient, vaultName, "test-sa", testNamespace)
			Expect(err).Should(BeNil())
			Expect(sa).Should(Equal("test-sa"))
		})
	})

	Describe("#dockerImages", func() {
		It("should return docker images with versions", func() {
			versionsDir := path.Join("test_data", "jenkins-x-versions")
			Expect(versionsDir).Should(BeADirectory())

			resolver := versionstream.VersionResolver{
				VersionsDir: versionsDir,
			}
			imageMap, err := vaultCreatorUnderTest.dockerImages(resolver)
			Expect(err).Should(BeNil())

			Expect(imageMap).Should(HaveKeyWithValue("banzaicloud/bank-vaults", "banzaicloud/bank-vaults:0.5.3"))
			Expect(imageMap).Should(HaveKeyWithValue("vault", "vault:1.2.3"))
		})

		It("should return images unresolved if versions are missing", func() {
			emptyDir, err := ioutil.TempDir("", "jx-create-integration-test")
			Expect(err).Should(BeNil())
			defer func() {
				_ = os.RemoveAll(emptyDir)
			}()

			resolver := versionstream.VersionResolver{
				VersionsDir: emptyDir,
			}
			imageMap, err := vaultCreatorUnderTest.dockerImages(resolver)
			Expect(imageMap).Should(HaveKeyWithValue("banzaicloud/bank-vaults", "banzaicloud/bank-vaults"))
			Expect(imageMap).Should(HaveKeyWithValue("vault", "vault"))
		})
	})

	Describe("#CreateOrUpdateVault", func() {
		var (
			projectID   string
			clusterName string
			zone        string
			resolver    versionstream.VersionResolver
		)

		BeforeEach(func() {
			data, err := kube.ReadInstallValues(kubeClient, devNamespace)
			Expect(err).Should(BeNil())

			projectID = data[kube.ProjectID]
			Expect(projectID).ShouldNot(BeEmpty())

			clusterName = data[kube.ClusterName]
			Expect(clusterName).ShouldNot(BeEmpty())

			zone = data[kube.Zone]
			Expect(zone).ShouldNot(BeEmpty())

			versionsDir := path.Join("test_data", "jenkins-x-versions")
			Expect(versionsDir).Should(BeADirectory())
			resolver = versionstream.VersionResolver{
				VersionsDir: versionsDir,
			}
		})

		AfterEach(func() {
			err = gcloud.DeleteBucket(testBucketName)
			if err != nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "%s", err.Error())
			}
		})

		It("fails to install vault with insufficient parameters", func() {
			testParam := VaultCreationParam{}
			err = vaultCreatorUnderTest.CreateOrUpdateVault(testParam)
			Expect(err).ShouldNot(BeNil())

			aggregate, ok := err.(errors.Aggregate)
			Expect(ok).Should(BeTrue())
			Expect(len(aggregate.Errors())).Should(Equal(5))

			var errorMessages []string
			for _, validationError := range aggregate.Errors() {
				errorMessages = append(errorMessages, validationError.Error())
			}
			Expect(errorMessages).Should(ContainElement("the Vault name needs to be provided"))
			Expect(errorMessages).Should(ContainElement("the namespace to create the Vault instance into needs to be provided"))
			Expect(errorMessages).Should(ContainElement("a kube client needs to be provided"))
			Expect(errorMessages).Should(ContainElement("a vault operator client needs to be provided"))
			Expect(errorMessages).Should(ContainElement("a kube/cloud provider needs be provided"))
		})

		It("successfully installs vault", func() {
			if cluster.IsInCluster() {
				Skip("only runs locally since it needs to create cloud resources")
			}

			fileHandles := util.IOFileHandles{}

			gkeParam := &GKEParam{
				ProjectID:  projectID,
				Zone:       zone,
				BucketName: testBucketName,
			}

			testParam := VaultCreationParam{
				VaultName:            "acme-vault",
				Namespace:            testNamespace,
				ClusterName:          clusterName,
				ServiceAccountName:   "test-sa",
				KubeProvider:         cloud.GKE,
				KubeClient:           kubeClient,
				VaultOperatorClient:  vaultOperatorClient,
				CreateCloudResources: true,
				VersionResolver:      resolver,
				FileHandles:          fileHandles,
				GKE:                  gkeParam,
			}

			err = vaultCreatorUnderTest.CreateOrUpdateVault(testParam)
			Expect(err).Should(BeNil())

			pod, err := kubeClient.CoreV1().Pods(testNamespace).Get("acme-vault-0", metav1.GetOptions{})
			Expect(pod).ShouldNot(BeNil())

			By("re-running CreateOrUpdateVault is successful as well")
			err = vaultCreatorUnderTest.CreateOrUpdateVault(testParam)
			Expect(err).Should(BeNil())
		})
	})
})
