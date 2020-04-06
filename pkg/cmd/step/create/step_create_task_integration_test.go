// +build integration

package create

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/step/syntax"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStepCreateTask(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Step Git Create Task Integration Test Suite")
}

var _ = Describe("step git create task", func() {
	var (
		createTaskOption StepCreateTaskOptions
		outDir           string
		err              error
	)

	BeforeSuite(func() {
		// comment out to see logging output
		log.SetOutput(ioutil.Discard)
		_ = log.SetLevel("info")
	})

	BeforeEach(func() {
		By("creating test out dir")
		outDir, err = ioutil.TempDir("", "jenkins-x-test-dir-")
		Expect(err).NotTo(HaveOccurred())

		By("preparing test CommonOptions")
		commonOpts := opts.CommonOptions{}
		commonOpts.GetFactory()
		commonOpts.Out = os.Stdout

		By("preparing test StepCreateTaskOptions")
		createTaskOption = StepCreateTaskOptions{}
		createTaskOption.CommonOptions = &commonOpts
		createTaskOption.PipelineKind = jenkinsfile.PipelineKindRelease
		createTaskOption.Trigger = "manual"
		createTaskOption.Branch = "master"
		noApply := true
		createTaskOption.NoApply = &noApply
		createTaskOption.DryRun = true
		createTaskOption.OutDir = outDir
	})

	AfterEach(func() {
		By("deleting temp out dir")
		_ = os.RemoveAll(outDir)
	})

	Context("execution pipeline mode", func() {
		BeforeEach(func() {
			createTaskOption.CloneGitURL = "https://github.com/jenkins-x-quickstarts/golang-http"
			createTaskOption.DeleteTempDir = true
		})

		It("creates Tekton CRDs", func() {
			err = createTaskOption.Run()
			Expect(err).NotTo(HaveOccurred())

			fileInfos, err := ioutil.ReadDir(outDir)
			Expect(err).Should(BeNil())
			Expect(len(fileInfos)).Should(Equal(5))
			expectedFiles := []string{"pipeline.yml", "pipelinerun.yml", "structure.yml", "tasks.yml", "pipelineresources.yml"}
			for _, file := range fileInfos {
				index := util.StringArrayIndex(expectedFiles, file.Name())
				By(fmt.Sprintf("Checking for file %s", file.Name()))
				Expect(index).ShouldNot(Equal(-1))
				expectedFiles = removeElement(expectedFiles, index)
			}
			Expect(len(expectedFiles)).Should(Equal(0))
		})
	})

	Context("meta pipeline mode", func() {
		var (
			cloneDir string
		)

		BeforeEach(func() {
			cloneDir, err = ioutil.TempDir("", "jenkins-x-test-repo-dir-")
			Expect(err).NotTo(HaveOccurred())

			By("cloning test repo")
			err = createTaskOption.Git().Clone("https://github.com/jenkins-x-quickstarts/golang-http", cloneDir)
			Expect(err).NotTo(HaveOccurred())
			createTaskOption.CloneDir = cloneDir

			By("creating effective pipeline")
			createEffectivePipeline := syntax.StepSyntaxEffectiveOptions{}
			createEffectivePipeline.CommonOptions = &opts.CommonOptions{}
			createEffectivePipeline.GetFactory()
			createEffectivePipeline.Out = os.Stdout
			createEffectivePipeline.OutDir = "."

			currentDir, err := os.Getwd()
			defer func() {
				_ = os.Chdir(currentDir)
			}()
			Expect(err).NotTo(HaveOccurred())
			err = os.Chdir(createTaskOption.CloneDir)
			Expect(err).NotTo(HaveOccurred())

			err = createEffectivePipeline.Run()
			Expect(err).NotTo(HaveOccurred())
			Expect(filepath.Join(createEffectivePipeline.OutDir, "jenkins-x-effective.yml")).Should(BeARegularFile())
		})

		AfterEach(func() {
			_ = os.RemoveAll(cloneDir)
		})

		It("creates Tekton CRDs", func() {
			Expect(createTaskOption.effectiveProjectConfigExists()).Should(BeTrue())
			err = createTaskOption.Run()
			Expect(err).NotTo(HaveOccurred())

			fileInfos, err := ioutil.ReadDir(outDir)
			Expect(err).Should(BeNil())
			Expect(len(fileInfos)).Should(Equal(5))
			expectedFiles := []string{"pipeline.yml", "pipelinerun.yml", "structure.yml", "tasks.yml", "pipelineresources.yml"}
			for _, file := range fileInfos {
				index := util.StringArrayIndex(expectedFiles, file.Name())
				By(fmt.Sprintf("Checking for file %s", file.Name()))
				Expect(index).ShouldNot(Equal(-1))
				expectedFiles = removeElement(expectedFiles, index)
			}
			Expect(len(expectedFiles)).Should(Equal(0))
		})
	})
})

func removeElement(s []string, i int) []string {
	return append(s[:i], s[i+1:]...)
}
