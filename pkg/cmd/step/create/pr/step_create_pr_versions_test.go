// +build unit

package pr_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/step/create/pr"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
)

func TestCreatePullRequestUpdateVersionFilesFn(t *testing.T) {
	commonOpts := &opts.CommonOptions{}
	commonOpts.SetFactory(fake.NewFakeFactory())

	gitter := gits.NewGitCLI()
	roadRunnerOrg, err := gits.NewFakeRepository("acme", "roadrunner", func(dir string) error {
		return ioutil.WriteFile(filepath.Join(dir, "README"), []byte("TODO"), 0655)
	}, gitter)
	assert.NoError(t, err)
	wileOrg, err := gits.NewFakeRepository("acme", "wile", func(dir string) error {
		return ioutil.WriteFile(filepath.Join(dir, "README"), []byte("TODO"), 0655)
	}, gitter)
	assert.NoError(t, err)
	gitProvider := gits.NewFakeProvider(roadRunnerOrg, wileOrg)
	helmer := helm_test.NewMockHelmer()

	testhelpers.ConfigureTestOptionsWithResources(commonOpts,
		[]runtime.Object{},
		[]runtime.Object{
			kube.NewPermanentEnvironment("EnvWhereApplicationIsDeployed"),
		},
		gitter,
		gitProvider,
		helmer,
		resources_test.NewMockInstaller(),
	)
	o := pr.StepCreatePullRequestVersionsOptions{
		StepCreatePrOptions: pr.StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}
	t.Run("star", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)

		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/wile"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "wile",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
			{
				Name:         "wile",
				ChartVersion: "1.0.0",
				AppVersion:   "1.0.0",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/roadrunner"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "roadrunner",
				ChartVersion: "2.0.1",
				AppVersion:   "2.0.1",
				Description:  "",
			},
			{
				Name:         "roadrunner",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		helm_test.StubFetchChart("acme/wile", "1.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "wile",
				Version: "1.0.1",
			},
		}, helmer)
		helm_test.StubFetchChart("acme/roadrunner", "2.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "roadrunner",
				Version: "2.0.1",
				Sources: []string{
					"https://fake.git/acme/roadrunner",
				},
			},
		}, helmer)
		pegomock.When(helmer.IsRepoMissing("https://acme.com/charts")).ThenReturn(pegomock.ReturnValue(false), pegomock.ReturnValue("acme"), pegomock.ReturnValue(nil))
		fn := o.CreatePullRequestUpdateVersionFilesFn([]string{"*"}, make([]string, 0), "charts", helmer)
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata/TestCreatePullRequestUpdateVersionFilesFn"), dir, true)
		assert.NoError(t, err)
		err = gitter.Init(dir)
		assert.NoError(t, err)
		err = gitter.Add(dir, "*")
		assert.NoError(t, err)
		err = gitter.CommitDir(dir, "Initial commit")
		assert.NoError(t, err)
		gitInfo, err := gits.ParseGitURL("https://fake.git/acme/e")
		answer, err := fn(dir, gitInfo)
		assert.NoError(t, err)
		assert.Len(t, answer, 0)
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "wile.yml"), "version: 1.0.1")
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "roadrunner.yml"), "version: 2.0.1")
	})
	t.Run("excludes", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)

		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/wile"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "wile",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
			{
				Name:         "wile",
				ChartVersion: "1.0.0",
				AppVersion:   "1.0.0",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/roadrunner"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "roadrunner",
				ChartVersion: "2.0.1",
				AppVersion:   "2.0.1",
				Description:  "",
			},
			{
				Name:         "roadrunner",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		helm_test.StubFetchChart("acme/wile", "1.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "wile",
				Version: "1.0.1",
			},
		}, helmer)
		helm_test.StubFetchChart("acme/roadrunner", "2.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "roadrunner",
				Version: "2.0.1",
				Sources: []string{
					"https://fake.git/acme/roadrunner",
				},
			},
		}, helmer)
		pegomock.When(helmer.IsRepoMissing("https://acme.com/charts")).ThenReturn(pegomock.ReturnValue(false), pegomock.ReturnValue("acme"), pegomock.ReturnValue(nil))
		fn := o.CreatePullRequestUpdateVersionFilesFn([]string{"*"}, []string{"acme/wile"}, "charts", helmer)
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata/TestCreatePullRequestUpdateVersionFilesFn"), dir, true)
		assert.NoError(t, err)
		err = gitter.Init(dir)
		assert.NoError(t, err)
		err = gitter.Add(dir, "*")
		assert.NoError(t, err)
		err = gitter.CommitDir(dir, "Initial commit")
		assert.NoError(t, err)
		gitInfo, err := gits.ParseGitURL("https://fake.git/acme/e")
		answer, err := fn(dir, gitInfo)
		assert.NoError(t, err)
		assert.Len(t, answer, 0)
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "wile.yml"), "version: 1.0.0")
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "roadrunner.yml"), "version: 2.0.1")
	})
	t.Run("specific", func(t *testing.T) {
		pegomock.RegisterMockTestingT(t)

		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/wile"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "wile",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
			{
				Name:         "wile",
				ChartVersion: "1.0.0",
				AppVersion:   "1.0.0",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		pegomock.When(helmer.SearchCharts(pegomock.EqString("acme/roadrunner"), pegomock.EqBool(true))).ThenReturn(pegomock.ReturnValue([]helm.ChartSummary{
			{
				Name:         "roadrunner",
				ChartVersion: "2.0.1",
				AppVersion:   "2.0.1",
				Description:  "",
			},
			{
				Name:         "roadrunner",
				ChartVersion: "1.0.1",
				AppVersion:   "1.0.1",
				Description:  "",
			},
		}), pegomock.ReturnValue(nil))
		helm_test.StubFetchChart("acme/wile", "1.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "wile",
				Version: "1.0.1",
			},
		}, helmer)
		helm_test.StubFetchChart("acme/roadrunner", "2.0.1", "", &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    "roadrunner",
				Version: "2.0.1",
				Sources: []string{
					"https://fake.git/acme/roadrunner",
				},
			},
		}, helmer)
		pegomock.When(helmer.IsRepoMissing("https://acme.com/charts")).ThenReturn(pegomock.ReturnValue(false), pegomock.ReturnValue("acme"), pegomock.ReturnValue(nil))
		fn := o.CreatePullRequestUpdateVersionFilesFn([]string{"acme/roadrunner"}, make([]string, 0), "charts", helmer)
		dir, err := ioutil.TempDir("", "")
		defer func() {
			err := os.RemoveAll(dir)
			assert.NoError(t, err)
		}()
		assert.NoError(t, err)
		err = util.CopyDir(filepath.Join("testdata/TestCreatePullRequestUpdateVersionFilesFn"), dir, true)
		assert.NoError(t, err)
		err = gitter.Init(dir)
		assert.NoError(t, err)
		err = gitter.Add(dir, "*")
		assert.NoError(t, err)
		err = gitter.CommitDir(dir, "Initial commit")
		assert.NoError(t, err)
		gitInfo, err := gits.ParseGitURL("https://fake.git/acme/e")
		answer, err := fn(dir, gitInfo)
		assert.NoError(t, err)
		assert.Len(t, answer, 0)
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "wile.yml"), "version: 1.0.0")
		tests.AssertFileContains(t, filepath.Join(dir, "charts", "acme", "roadrunner.yml"), "version: 2.0.1")
	})
}
