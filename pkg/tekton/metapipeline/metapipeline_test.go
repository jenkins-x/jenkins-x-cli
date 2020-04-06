// +build unit

package metapipeline

import (
	"path/filepath"
	"testing"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
)

func TestMetaPipeline(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Meta pipeline Suite")
}

var _ = Describe("Meta pipeline", func() {

	Describe("#createMetaPipelineCRDs", func() {
		var (
			testParams   CRDCreationParameters
			actualCRDs   *tekton.CRDWrapper
			actualError  error
			actualStdout string
		)

		BeforeEach(func() {
			gitInfo, _ := gits.NewGitFake().Info("/acme")
			pullRef := NewPullRefWithPullRequest("https://github.com/jenkins-x/jx", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815", PullRequestRef{ID: "42", MergeSHA: "db8e2d275df53477b1c6871f7d7f4281dacf3169"})

			testParams = CRDCreationParameters{
				PipelineName:     "test-pipeline",
				PipelineKind:     PullRequestPipeline,
				BranchIdentifier: "master",
				PullRef:          pullRef,
				GitInfo:          *gitInfo,
				Labels:           map[string]string{"someLabel": "someValue"},
				EnvVars:          map[string]string{"SOME_VAR": "SOME_VAL", "OTHER_VAR": "OTHER_VAL"},
				BuildNumber:      "1",
				SourceDir:        "source",
				ServiceAccount:   "tekton-bot",
				VersionsDir:      filepath.Join("test_data", "stable_versions"),
			}
		})

		JustBeforeEach(func() {
			actualCRDs, actualStdout, actualError = createMetaPipeline(testParams)
		})

		Context("with no extending Apps", func() {
			It("should not error", func() {
				Expect(actualError).Should(BeNil())
			})

			It("should not write to stdout", func() {
				Expect(actualStdout).Should(BeEmpty())
			})

			It("contain a single task", func() {
				tasks := actualCRDs.Tasks()
				Expect(tasks).Should(HaveLen(1))
			})

			It("contain four task steps", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				Expect(steps).Should(HaveLen(4))
				Expect(steps[0].Name).Should(Equal("git-merge"))
				Expect(steps[1].Name).Should(Equal(mergePullRefsStepName))
				Expect(steps[2].Name).Should(Equal(createEffectivePipelineStepName))
				Expect(steps[3].Name).Should(Equal(createTektonCRDsStepName))
			})

			It("merge pull refs step passes correct pull ref", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				mergePullRefStep := steps[1]
				Expect(mergePullRefStep.Args).Should(Equal([]string{"jx step git merge --verbose --baseBranch master --baseSHA 0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815 --sha db8e2d275df53477b1c6871f7d7f4281dacf3169"}))
			})

			It("should pass custom env variables to step syntax effective to be applied to effective project config", func() {
				step := actualCRDs.Tasks()[0].Spec.Steps[2]
				Expect(step.Args[0]).Should(ContainSubstring("--env SOME_VAR=SOME_VAL"))
				Expect(step.Args[0]).Should(ContainSubstring("--env OTHER_VAR=OTHER_VAL"))
			})

			It("should have correct step create task args", func() {
				step := actualCRDs.Tasks()[0].Spec.Steps[3]
				Expect(step.Args).Should(Equal([]string{"jx step create task --clone-dir /workspace/source --kind pullrequest --pr-number 42 --service-account tekton-bot --source source --branch master --build-number 1 --label someLabel=someValue"}))
			})

			It("should pass labels to step create task to be applied to generated CRDs", func() {
				step := actualCRDs.Tasks()[0].Spec.Steps[3]
				Expect(step.Args[0]).Should(ContainSubstring("--label someLabel=someValue"))
			})
		})

		Context("with extending App missing required metadata", func() {
			JustBeforeEach(func() {
				testApp := jenkinsv1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "acme-app",
						Labels: map[string]string{apps.AppTypeLabel: apps.PipelineExtension.String()},
					},

					Spec: jenkinsv1.AppSpec{},
				}
				testParams.Apps = []jenkinsv1.App{testApp}
				actualCRDs, actualStdout, actualError = createMetaPipeline(testParams)
			})

			It("should not error", func() {
				Expect(actualError).Should(BeNil())
			})

			It("should write warning to stdout", func() {
				Expect(actualStdout).Should(ContainSubstring("WARNING: Skipping app acme-app in meta pipeline"))
			})

			It("contain a single task", func() {
				tasks := actualCRDs.Tasks()
				Expect(tasks).Should(HaveLen(1))
			})

			It("contain three task steps", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				Expect(steps).Should(HaveLen(4))
				Expect(steps[0].Name).Should(Equal("git-merge"))
				Expect(steps[1].Name).Should(Equal(mergePullRefsStepName))
				Expect(steps[2].Name).Should(Equal(createEffectivePipelineStepName))
				Expect(steps[3].Name).Should(Equal(createTektonCRDsStepName))
			})
		})

		Context("with extending App", func() {
			JustBeforeEach(func() {
				testApp := jenkinsv1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "acme-app",
						Labels: map[string]string{apps.AppTypeLabel: apps.PipelineExtension.String()},
					},

					Spec: jenkinsv1.AppSpec{
						PipelineExtension: &jenkinsv1.PipelineExtension{
							Name:    "acme-ext",
							Image:   "acme:1.0.0",
							Command: "run",
							Args:    []string{},
						},
					},
				}
				testParams.Apps = []jenkinsv1.App{testApp}
				actualCRDs, actualStdout, actualError = createMetaPipeline(testParams)
			})

			It("should not error", func() {
				Expect(actualError).Should(BeNil())
			})

			It("should not write to stdout", func() {
				Expect(actualStdout).Should(BeEmpty())
			})

			It("contain a single task", func() {
				tasks := actualCRDs.Tasks()
				Expect(tasks).Should(HaveLen(1))
			})

			It("contain three task steps", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				Expect(steps).Should(HaveLen(5))
				Expect(steps[0].Name).Should(Equal("git-merge"))
				Expect(steps[1].Name).Should(Equal(mergePullRefsStepName))
				Expect(steps[2].Name).Should(Equal(createEffectivePipelineStepName))
				Expect(steps[2].Image).Should(Equal("gcr.io/jenkinsxio/builder-maven:0.1.527"))
				Expect(steps[3].Name).Should(Equal("acme-ext"))
				Expect(steps[4].Name).Should(Equal(createTektonCRDsStepName))
			})
			It("uses version stream for default image", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				Expect(steps[2].Image).Should(Equal("gcr.io/jenkinsxio/builder-maven:0.1.527"))
			})
		})

		Context("with no SHAs to merge (only baseBranch)", func() {
			JustBeforeEach(func() {
				pullRef := NewPullRef("https://github.com/jenkins-x/jx", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815")
				testParams.PullRef = pullRef
				actualCRDs, actualStdout, actualError = createMetaPipeline(testParams)
			})

			It("merge pull refs step passes correct pull ref", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				mergePullRefStep := steps[1]
				Expect(mergePullRefStep.Args[0]).Should(ContainSubstring("SKIP merge-pull-refs: Nothing to merge"))
			})
		})

		Context("with no SHAs to merge (baseBranch & baseSHA)", func() {
			JustBeforeEach(func() {
				pullRef := NewPullRef("https://github.com/jenkins-x/jx", "master", "")

				testParams.PullRef = pullRef
				actualCRDs, actualStdout, actualError = createMetaPipeline(testParams)
			})

			It("merge pull refs step passes correct pull ref", func() {
				steps := actualCRDs.Tasks()[0].Spec.Steps
				mergePullRefStep := steps[1]
				Expect(mergePullRefStep.Args[0]).Should(ContainSubstring("SKIP merge-pull-refs: Nothing to merge"))
			})
		})
	})

	Describe("#getExtendingApps", func() {
		var (
			jxClient versioned.Interface
		)

		BeforeEach(func() {
			jxClient = fake.NewSimpleClientset()
		})

		Context("with no extending App", func() {
			It("should return empty app list", func() {
				apps, err := getExtendingApps(jxClient, "jx")
				Expect(err).Should(BeNil())
				Expect(apps).Should(HaveLen(0))
			})
		})

		Context("with extending App", func() {
			BeforeEach(func() {
				testApp := jenkinsv1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "acme-app",
						Labels: map[string]string{apps.AppTypeLabel: apps.PipelineExtension.String()},
					},

					Spec: jenkinsv1.AppSpec{
						PipelineExtension: &jenkinsv1.PipelineExtension{
							Name:    "acme-ext",
							Image:   "acme:1.0.0",
							Command: "run",
							Args:    []string{},
						},
					},
				}
				_, err := jxClient.JenkinsV1().Apps("jx").Create(&testApp)
				Expect(err).Should(BeNil())
			})

			It("should return the registered App", func() {
				apps, err := getExtendingApps(jxClient, "jx")
				Expect(err).Should(BeNil())
				Expect(apps).Should(HaveLen(1))
				Expect(apps[0].Name).Should(Equal("acme-app"))
			})
		})
	})

	Describe("#buildEnvParams", func() {
		var (
			testParams CRDCreationParameters
		)

		BeforeEach(func() {
			gitInfo, _ := gits.ParseGitURL("https://github.com/jenkins-x/jx")
			pullRef := NewPullRefWithPullRequest("https://github.com/jenkins-x/jx", "master", "0967f9ecd7dd2d0acf883c7656c9dc2ad2bf9815", PullRequestRef{ID: "42", MergeSHA: "db8e2d275df53477b1c6871f7d7f4281dacf3169"})
			testParams = CRDCreationParameters{
				PipelineName:     "test-pipeline",
				PipelineKind:     PullRequestPipeline,
				BranchIdentifier: "master",
				PullRef:          pullRef,
				GitInfo:          *gitInfo,
				Labels:           map[string]string{"someLabel": "someValue"},
				EnvVars:          map[string]string{"SOME_VAR": "OTHER_VALUE", "SOURCE_URL": "http://foo.git"},
				BuildNumber:      "1",
				SourceDir:        "source",
				ServiceAccount:   "tekton-bot",
				VersionsDir:      filepath.Join("test_data", "stable_versions"),
			}
		})

		It("env vars don't contain duplicates", func() {
			vars := buildEnvParams(testParams)
			var seen []string

			for _, envVar := range vars {
				Expect(seen).ShouldNot(ContainElement(envVar.Name))
				seen = append(seen, envVar.Name)
			}

			Expect(len(seen)).Should(Equal(len(vars)))
		})

		It("explicitly set env variables win over custom env variables", func() {
			vars := buildEnvParams(testParams)
			expected := corev1.EnvVar{
				Name:  "SOURCE_URL",
				Value: "https://github.com/jenkins-x/jx",
			}
			Expect(vars).Should(ContainElement(expected))
		})
	})
})

func createMetaPipeline(params CRDCreationParameters) (*tekton.CRDWrapper, string, error) {
	var crds *tekton.CRDWrapper
	var err error
	out := log.CaptureOutput(func() {
		crds, err = createMetaPipelineCRDs(params)
	})
	return crds, out, err
}
