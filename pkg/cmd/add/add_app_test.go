// +build unit

// +build integration

// TODO these are not supposed to be integration tests but there was a mistaken mis-usage of go-git which means they
//  randomly fail atm

package add_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/jenkins-x/jx/pkg/cmd/add"

	expect "github.com/Netflix/go-expect"
	"github.com/jenkins-x/jx/pkg/apps"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	uuid "github.com/satori/go.uuid"

	"k8s.io/helm/pkg/chartutil"

	"github.com/jenkins-x/jx/pkg/tests"

	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/petergtz/pegomock"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ghodss/yaml"

	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/jenkins-x/jx/pkg/helm"

	"github.com/stretchr/testify/assert"

	google_protobuf "github.com/golang/protobuf/ptypes/any"
)

const (
	namespace = "jx"
)

var timeout = 5 * time.Second

func TestAddAppForGitOps(t *testing.T) {
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		alias := fmt.Sprintf("%s-alias", name)
		repo := "https://storage.googleapis.com/chartmuseum.jenkins-x.io"
		description := "My test chart description"
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Alias:      alias,
			Repo:       repo,
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
		}
		envDir, err := o.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		o.CloneDir = devEnvDir

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:        name,
				Version:     version,
				Description: description,
			},
		}, testOptions.MockHelmer)
		o.Args = []string{name}
		err = o.Run()
		assert.NoError(r, err)
		pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.NoError(r, err)
		// Validate the PR has the right title, message
		assert.Equal(r, fmt.Sprintf("Add %s %s", name, version), pr.Title)
		assert.Equal(r, fmt.Sprintf("Add app %s %s", name, version), pr.Body)
		// Validate the branch name
		branchName, err := o.Git().Branch(devEnvDir)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf("add-app-%s-%s", name, version), branchName)
		// Validate the updated Requirements.yaml
		requirements, err := helm.LoadRequirementsFile(filepath.Join(devEnvDir, helm.RequirementsFileName))
		assert.NoError(r, err)
		found := make([]*helm.Dependency, 0)
		for _, d := range requirements.Dependencies {
			if d.Name == name && d.Alias == alias {
				found = append(found, d)
			}
		}
		assert.Len(r, found, 1)
		if len(found) == 1 {
			assert.Equal(r, version, found[0].Version)
		}
		app := &jenkinsv1.App{}
		appBytes, err := ioutil.ReadFile(filepath.Join(devEnvDir, name, "templates", "app.yaml"))
		_ = yaml.Unmarshal(appBytes, app)
		assert.Equal(r, name, app.Labels[helm.LabelAppName])
		assert.Equal(r, version, app.Labels[helm.LabelAppVersion])
		assert.Equal(r, repo, app.Annotations[helm.AnnotationAppRepository])
		assert.Equal(r, description, app.Annotations[helm.AnnotationAppDescription])
	})
}

func TestAddAppForGitOpsWithShortName(t *testing.T) {
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		shortName := nameUUID.String()
		name := fmt.Sprintf("jx-app-%s", shortName)
		version := "0.0.1"
		alias := fmt.Sprintf("%s-alias", name)
		repo := kube.DefaultChartMuseumURL
		description := "My test chart description"
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Alias:      alias,
			Repo:       repo,
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
		}
		envDir, err := o.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		o.CloneDir = devEnvDir

		pegomock.When(testOptions.MockHelmer.ListRepos()).ThenReturn(
			map[string]string{
				"repo1": kube.DefaultChartMuseumURL,
			}, nil)
		pegomock.When(testOptions.MockHelmer.SearchCharts(pegomock.AnyString(), pegomock.EqBool(false))).ThenReturn(
			[]helm.ChartSummary{
				{
					Name:         fmt.Sprintf("repo1/%s", name),
					ChartVersion: version,
					AppVersion:   version,
				},
			}, nil)
		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:        name,
				Version:     version,
				Description: description,
			},
		}, testOptions.MockHelmer)
		o.Args = []string{shortName}
		err = o.Run()
		assert.NoError(r, err)
		pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.NoError(r, err)
		// Validate the PR has the right title, message
		assert.Equal(r, fmt.Sprintf("Add %s %s", name, version), pr.Title)
		assert.Equal(r, fmt.Sprintf("Add app %s %s", name, version), pr.Body)
		// Validate the branch name
		branchName, err := o.Git().Branch(devEnvDir)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf("add-app-%s-%s", name, version), branchName)
		// Validate the updated Requirements.yaml
		requirements, err := helm.LoadRequirementsFile(filepath.Join(devEnvDir, helm.RequirementsFileName))
		assert.NoError(r, err)
		found := make([]*helm.Dependency, 0)
		for _, d := range requirements.Dependencies {
			if d.Name == name && d.Alias == alias {
				found = append(found, d)
			}
		}
		assert.Len(r, found, 1)
		assert.Equal(r, version, found[0].Version)
		app := &jenkinsv1.App{}
		appBytes, err := ioutil.ReadFile(filepath.Join(devEnvDir, name, "templates", "app.yaml"))
		_ = yaml.Unmarshal(appBytes, app)
		assert.Equal(r, name, app.Labels[helm.LabelAppName])
		assert.Equal(r, version, app.Labels[helm.LabelAppVersion])
		assert.Equal(r, repo, app.Annotations[helm.AnnotationAppRepository])
		assert.Equal(r, description, app.Annotations[helm.AnnotationAppDescription])
		assert.Equal(r, description, app.Annotations[helm.AnnotationAppDescription])
	})
}

func TestAddAppWithSecrets(t *testing.T) {
	pegomock.RegisterMockTestingT(t)
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(false, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console input to create secrets
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
		}
		o.Args = []string{name}
		o.BatchMode = false

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name,
				Version: version,
			},
			Files: []*google_protobuf.Any{
				{
					TypeUrl: "values.schema.json",
					Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "tokenValue": {
      "type": "string",
      "format": "token"
    }
  }
}`),
				},
			},
		}, testOptions.MockHelmer)

		// Test interactive IO
		donec := make(chan struct{})
		// TODO Answer questions
		go func() {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for tokenValue")
			console.SendLine("abc")
			console.ExpectString(" ***")
			console.ExpectEOF()
		}()

		pegomock.When(testOptions.MockHelmer.UpgradeChart(
			pegomock.AnyString(),
			pegomock.EqString(name),
			pegomock.AnyString(),
			pegomock.EqString(version),
			pegomock.AnyBool(),
			pegomock.AnyInt(),
			pegomock.AnyBool(),
			pegomock.AnyBool(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.EqString(kube.DefaultChartMuseumURL),
			pegomock.AnyString(),
			pegomock.AnyString())).
			Then(func(params []pegomock.Param) pegomock.ReturnValues {
				// These assertion must happen inside the UpgradeChart function otherwise the chart dir will have been
				// deleted
				assert.IsType(r, "", params[0])
				assert.IsType(r, make([]string, 0), params[9])
				chart := params[0].(string)
				valuesFiles := params[9].([]string)
				isChartDir, err := chartutil.IsChartDir(chart)
				assert.NoError(r, err)
				assert.True(r, isChartDir)
				assert.Len(r, valuesFiles, 2)
				_, valuesFileName := filepath.Split(valuesFiles[0])
				assert.Contains(r, valuesFileName, "values.yaml")
				bytes, err := ioutil.ReadFile(valuesFiles[0])
				assert.NoError(r, err)
				assert.Equal(r, `tokenValue:
  kind: Secret
  name: tokenvalue
`, string(bytes))
				_, secretsFileName := filepath.Split(valuesFiles[1])
				assert.Contains(r, secretsFileName, "generatedSecrets.yaml")
				bytes, err = ioutil.ReadFile(valuesFiles[1])
				assert.NoError(r, err)
				assert.Equal(r, `appsGeneratedSecrets:
- Name: tokenvalue
  key: token
  value: abc
`, string(bytes))
				// Check the template is in place
				_, err = os.Stat(filepath.Join(chart, "templates", "app-generated-secret-template.yaml"))
				assert.NoError(r, err)
				return []pegomock.ReturnValue{
					nil,
				}
			})

		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		// Validate that the secret reference is generated and the secret is in the chart
		// chart, _, _, _, _, _, _, _, _, valueFiles, _, _, _ :=
		testOptions.MockHelmer.VerifyWasCalledOnce().
			UpgradeChart(
				pegomock.AnyString(),
				pegomock.EqString(fmt.Sprintf("jx-%s", name)),
				pegomock.AnyString(),
				pegomock.EqString(version),
				pegomock.AnyBool(),
				pegomock.AnyInt(),
				pegomock.AnyBool(),
				pegomock.AnyBool(),
				pegomock.AnyStringSlice(),
				pegomock.AnyStringSlice(),
				pegomock.AnyStringSlice(),
				pegomock.EqString(kube.DefaultChartMuseumURL),
				pegomock.AnyString(),
				pegomock.AnyString())
	})
}

func TestAddAppWithDefaults(t *testing.T) {

	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(false, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console input to create secrets
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
		}
		o.Args = []string{name}

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name,
				Version: version,
			},
			Files: []*google_protobuf.Any{
				{
					TypeUrl: "values.schema.json",
					Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "default": "testing"
    }
  }
}`),
				},
			},
		}, testOptions.MockHelmer)

		// Test interactive IO
		donec := make(chan struct{})
		// TODO Answer questions
		go func() {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for name testing [Automatically accepted default value]")
			console.ExpectEOF()
		}()

		pegomock.When(testOptions.MockHelmer.UpgradeChart(
			pegomock.AnyString(),
			pegomock.EqString(name),
			pegomock.AnyString(),
			pegomock.EqString(version),
			pegomock.AnyBool(),
			pegomock.AnyInt(),
			pegomock.AnyBool(),
			pegomock.AnyBool(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.EqString(kube.DefaultChartMuseumURL),
			pegomock.AnyString(),
			pegomock.AnyString())).
			Then(func(params []pegomock.Param) pegomock.ReturnValues {
				// These assertion must happen inside the UpgradeChart function otherwise the chart dir will have been
				// deleted
				assert.IsType(r, "", params[0])
				assert.IsType(r, make([]string, 0), params[9])
				chart := params[0].(string)
				valuesFiles := params[9].([]string)
				isChartDir, err := chartutil.IsChartDir(chart)
				assert.NoError(r, err)
				assert.True(r, isChartDir)
				assert.Len(r, valuesFiles, 1)
				_, valuesFileName := filepath.Split(valuesFiles[0])
				assert.Contains(r, valuesFileName, "values.yaml")
				bytes, err := ioutil.ReadFile(valuesFiles[0])
				assert.NoError(r, err)
				assert.Equal(r, `name: testing
`, string(bytes))

				return []pegomock.ReturnValue{
					nil,
				}
			})

		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		testOptions.MockHelmer.VerifyWasCalledOnce().
			UpgradeChart(
				pegomock.AnyString(),
				pegomock.EqString(fmt.Sprintf("%s-%s", namespace, name)),
				pegomock.AnyString(),
				pegomock.EqString(version),
				pegomock.AnyBool(),
				pegomock.AnyInt(),
				pegomock.AnyBool(),
				pegomock.AnyBool(),
				pegomock.AnyStringSlice(),
				pegomock.AnyStringSlice(),
				pegomock.AnyStringSlice(),
				pegomock.EqString(kube.DefaultChartMuseumURL),
				pegomock.AnyString(),
				pegomock.AnyString())
	})

}

func TestStashValues(t *testing.T) {
	namespace := "jx"

	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(false, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console input to create secrets
		console := tests.NewTerminal(r, &timeout)
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err
		defer console.Cleanup()

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
			Namespace:  namespace,
		}
		o.Args = []string{name}

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name,
				Version: version,
			},
			Files: []*google_protobuf.Any{
				{
					TypeUrl: "values.schema.json",
					Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "default": "testing"
    }
  }
}`),
				},
			},
		}, testOptions.MockHelmer)

		err = o.Run()
		assert.NoError(r, err)
		appCRDName := fmt.Sprintf("%s-%s", name, name)
		jxClient, ns, err := testOptions.CommonOptions.JXClientAndDevNamespace()
		assert.NoError(r, err)
		appList, err := jxClient.JenkinsV1().Apps(ns).List(metav1.ListOptions{})
		assert.Equal(r, namespace, ns)
		assert.NoError(r, err)
		assert.Len(r, appList.Items, 1)
		app, err := jxClient.JenkinsV1().Apps(ns).Get(fmt.Sprintf("%s-%s", namespace, appCRDName), metav1.GetOptions{})
		assert.NoError(r, err)
		val, ok := app.Annotations[apps.ValuesAnnotation]
		assert.True(r, ok)
		dst, err := base64.StdEncoding.DecodeString(val)
		assert.NoError(r, err)
		assert.Equal(r, `name: testing
`, string(dst))
	})
}

func TestAddAppForGitOpsWithSecrets(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console input to create secrets
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		alias := fmt.Sprintf("%s-alias", name)
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Alias:      alias,
			Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
		}
		o.Args = []string{name}
		o.BatchMode = false
		envDir, err := o.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		o.CloneDir = devEnvDir

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name,
				Version: version,
			},
			Files: []*google_protobuf.Any{
				{
					TypeUrl: "values.schema.json",
					Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "tokenValue": {
      "type": "string",
      "format": "token"
    }
  }
}`),
				},
			},
		}, testOptions.MockHelmer)

		// Test interactive IO
		donec := make(chan struct{})
		// TODO Answer questions
		go func() {
			defer close(donec)
			// Test boolean type
			console.ExpectString("Enter a value for tokenValue")
			console.SendLine("abc")
			console.ExpectString(" ***")
			console.ExpectEOF()
		}()
		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		// Validate that the secret reference is generated
		valuesFromPrPath := filepath.Join(devEnvDir, name, helm.ValuesFileName)
		_, err = os.Stat(valuesFromPrPath)
		assert.NoError(r, err)
		data, err := ioutil.ReadFile(valuesFromPrPath)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf(`tokenValue: vault:gitOps/%s/%s:tokenValue
`, testOptions.DevEnvRepo.Owner, testOptions.DevEnvRepo.GitRepo.Name), string(data))
		// Validate that vault has had the secret added
		path := strings.Join([]string{"gitOps", testOptions.OrgName, testOptions.DevEnvRepoInfo.Name},
			"/")
		value := map[string]interface{}{
			"tokenValue": "abc",
		}
		testOptions.MockVaultClient.VerifyWasCalledOnce().Write(path, value)
	})
}

func TestAddApp(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(false, "", t)
	// Can't run in parallel
	pegomock.RegisterMockTestingT(t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.0.1"
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    version,
		Repo:       kube.DefaultChartMuseumURL,
		GitOps:     false,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}
	o.Args = []string{name}
	err = o.Run()
	assert.NoError(t, err)

	// Check chart was installed
	_, _, _, fetchDir, _, _, _ := testOptions.MockHelmer.VerifyWasCalledOnce().FetchChart(
		pegomock.EqString(name),
		pegomock.EqString(version),
		pegomock.AnyBool(),
		pegomock.AnyString(),
		pegomock.EqString(kube.DefaultChartMuseumURL),
		pegomock.AnyString(),
		pegomock.AnyString()).GetCapturedArguments()
	testOptions.MockHelmer.VerifyWasCalledOnce().
		UpgradeChart(
			pegomock.EqString(filepath.Join(fetchDir, name)),
			pegomock.EqString(fmt.Sprintf("%s-%s", namespace, name)),
			pegomock.AnyString(),
			pegomock.EqString(version),
			pegomock.AnyBool(),
			pegomock.AnyInt(),
			pegomock.AnyBool(),
			pegomock.AnyBool(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.EqString(kube.DefaultChartMuseumURL),
			pegomock.AnyString(),
			pegomock.AnyString())

	// Verify the annotation
}

func TestAddAppWithShortName(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(false, "", t)
	// Can't run in parallel
	pegomock.RegisterMockTestingT(t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	shortName := nameUUID.String()
	name := fmt.Sprintf("jx-app-%s", shortName)
	version := "0.0.1"
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    version,
		Repo:       kube.DefaultChartMuseumURL,
		GitOps:     false,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}

	pegomock.When(testOptions.MockHelmer.ListRepos()).ThenReturn(
		map[string]string{
			"repo1": kube.DefaultChartMuseumURL,
		}, nil)
	pegomock.When(testOptions.MockHelmer.SearchCharts(pegomock.AnyString(), pegomock.EqBool(false))).ThenReturn(
		[]helm.ChartSummary{
			{
				Name:         fmt.Sprintf("repo1/%s", name),
				ChartVersion: version,
				AppVersion:   version,
			},
		}, nil)

	o.Args = []string{shortName}
	err = o.Run()
	assert.NoError(t, err)

	// Check chart was installed
	_, _, _, fetchDir, _, _, _ := testOptions.MockHelmer.VerifyWasCalledOnce().FetchChart(
		pegomock.EqString(name),
		pegomock.EqString(version),
		pegomock.AnyBool(),
		pegomock.AnyString(),
		pegomock.EqString(kube.DefaultChartMuseumURL),
		pegomock.AnyString(),
		pegomock.AnyString()).GetCapturedArguments()
	testOptions.MockHelmer.VerifyWasCalledOnce().
		UpgradeChart(
			pegomock.EqString(filepath.Join(fetchDir, name)),
			pegomock.EqString(fmt.Sprintf("%s-%s", namespace, name)),
			pegomock.AnyString(),
			pegomock.EqString(version),
			pegomock.AnyBool(),
			pegomock.AnyInt(),
			pegomock.AnyBool(),
			pegomock.AnyBool(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.EqString(kube.DefaultChartMuseumURL),
			pegomock.AnyString(),
			pegomock.AnyString())

	// Verify the annotation
}

func TestAddAppFromPath(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(false, "", t)
	// Can't run in parallel
	pegomock.RegisterMockTestingT(t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.0.1"
	commonOpts := *testOptions.CommonOptions

	// Make the local chart
	chartDir, err := ioutil.TempDir("", "local-chart")
	assert.NoError(t, err)
	chart := chart.Metadata{
		Version: version,
		Name:    name,
	}
	chartBytes, err := json.Marshal(chart)
	assert.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(chartDir, helm.ChartFileName), chartBytes, 0600)
	assert.NoError(t, err)

	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		GitOps:     false,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}

	o.Args = []string{chartDir}
	err = o.Run()
	assert.NoError(t, err)

	testOptions.MockHelmer.VerifyWasCalledOnce().
		UpgradeChart(
			pegomock.AnyString(),
			pegomock.EqString(fmt.Sprintf("%s-%s", namespace, name)),
			pegomock.AnyString(),
			pegomock.EqString(version),
			pegomock.AnyBool(),
			pegomock.AnyInt(),
			pegomock.AnyBool(),
			pegomock.AnyBool(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyString(),
			pegomock.AnyString(),
			pegomock.AnyString())

	// Verify the annotation
}

func TestAddLatestApp(t *testing.T) {

	testOptions := testhelpers.CreateAppTestOptions(false, "", t)
	// Can't run in parallel
	pegomock.RegisterMockTestingT(t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.1.1"
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Repo:       kube.DefaultChartMuseumURL,
		GitOps:     false,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}
	o.Args = []string{name}
	helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: version,
		},
	}, testOptions.MockHelmer)
	err = o.Run()
	assert.NoError(t, err)

	_, _, _, fetchDir, _, _, _ := testOptions.MockHelmer.VerifyWasCalledOnce().FetchChart(
		pegomock.EqString(name),
		pegomock.AnyString(),
		pegomock.AnyBool(),
		pegomock.AnyString(),
		pegomock.EqString(kube.DefaultChartMuseumURL),
		pegomock.AnyString(),
		pegomock.AnyString()).GetCapturedArguments()
	testOptions.MockHelmer.VerifyWasCalledOnce().
		UpgradeChart(
			pegomock.EqString(filepath.Join(fetchDir, name)),
			pegomock.EqString(fmt.Sprintf("%s-%s", namespace, name)),
			pegomock.AnyString(),
			pegomock.EqString(version),
			pegomock.AnyBool(),
			pegomock.AnyInt(),
			pegomock.AnyBool(),
			pegomock.AnyBool(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.AnyStringSlice(),
			pegomock.EqString(kube.DefaultChartMuseumURL),
			pegomock.AnyString(),
			pegomock.AnyString())
}

func TestAddAppWithValuesFileForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	values := map[string]interface{}{
		"cheese": "cheddar",
	}
	file, err := ioutil.TempFile("", "values.yaml")
	assert.NoError(t, err)
	data, err := yaml.Marshal(values)
	assert.NoError(t, err)
	_, err = file.Write(data)
	assert.NoError(t, err)

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.0.1"
	alias := fmt.Sprintf("%s-alias", name)
	helm_test.StubFetchChart(name, version, kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: version,
		},
	}, testOptions.MockHelmer)
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:     version,
		Alias:       alias,
		Repo:        "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
		GitOps:      true,
		DevEnv:      testOptions.DevEnv,
		HelmUpdate:  true, // Flag default when run on CLI
		ValuesFiles: []string{file.Name()},
	}
	o.Args = []string{name}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir
	err = o.Run()
	assert.NoError(t, err)
	// Validate that the values.yaml file is in the right place
	valuesFromPrPath := filepath.Join(devEnvDir, name, helm.ValuesFileName)
	_, err = os.Stat(valuesFromPrPath)
	assert.NoError(t, err)
	valuesFromPr := make(map[string]interface{})
	data, err = ioutil.ReadFile(valuesFromPrPath)
	assert.NoError(t, err)
	err = yaml.Unmarshal(data, &valuesFromPr)
	assert.NoError(t, err)
	assert.Equal(t, values, valuesFromPr)
}

func TestAddAppWithReadmeForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.0.1"
	alias := fmt.Sprintf("%s-alias", name)
	description := "Example description"
	gitRepository := "https://git.fake/myorg/myrepo"
	releaseNotes := "https://issues.fake/myorg/myrepo/releasenotes/v0.0.1"
	release := jenkinsv1.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, version),
		},
		Spec: jenkinsv1.ReleaseSpec{
			ReleaseNotesURL: releaseNotes,
			GitHTTPURL:      gitRepository,
		},
	}
	data, err := yaml.Marshal(release)
	assert.NoError(t, err)
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    version,
		Alias:      alias,
		Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
		GitOps:     true,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}
	o.Args = []string{name}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir
	helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:        name,
			Version:     version,
			Description: description,
		},
		Templates: []*chart.Template{
			{
				Name: "release.yaml",
				Data: data,
			},
		},
	}, testOptions.MockHelmer)
	err = o.Run()
	assert.NoError(t, err)
	// Validate that the README.md file is in the right place
	readmeFromPrPath := filepath.Join(devEnvDir, name, "README.MD")
	_, err = os.Stat(readmeFromPrPath)
	assert.NoError(t, err)
	data, err = ioutil.ReadFile(readmeFromPrPath)
	assert.NoError(t, err)
	readmeFromPr := string(data)
	assert.Equal(t, fmt.Sprintf(`# %s

|App Metadata||
|---|---|
| **Version** | %s |
| **Description** | %s |
| **Chart Repository** | %s |
| **Git Repository** | %s |
| **Release Notes** | %s |
`, name, version, description, kube.DefaultChartMuseumURL, gitRepository, releaseNotes), readmeFromPr)
	// Validate that the README.md file is in the right place
	releaseyamlFromPrPath := filepath.Join(devEnvDir, name, "release.yaml")
	_, err = os.Stat(releaseyamlFromPrPath)
	assert.NoError(t, err)
	data, err = ioutil.ReadFile(releaseyamlFromPrPath)
	assert.NoError(t, err)
	releaseFromPr := jenkinsv1.Release{}
	err = yaml.Unmarshal(data, &releaseFromPr)
	assert.NoError(t, err)
	assert.Equal(t, release, releaseFromPr)
}

func TestAddAppWithCustomReadmeForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.0.1"
	alias := fmt.Sprintf("%s-alias", name)
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    version,
		Alias:      alias,
		Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
		GitOps:     true,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}
	o.Verbose = true
	o.Args = []string{name}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir
	readmeFileName := "README.MD"
	readme := "Tasty Cheese!\n"
	helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: version,
		},
		Files: []*google_protobuf.Any{
			{
				TypeUrl: readmeFileName,
				Value:   []byte(readme),
			},
		},
	}, testOptions.MockHelmer)
	err = o.Run()
	assert.NoError(t, err)
	// Validate that the README.md file is in the right place
	readmeFromPrPath := filepath.Join(devEnvDir, name, readmeFileName)
	_, err = os.Stat(readmeFromPrPath)
	assert.NoError(t, err)
	data, err := ioutil.ReadFile(readmeFromPrPath)
	assert.NoError(t, err)
	readmeFromPr := string(data)
	assert.Equal(t, fmt.Sprintf(`# %s

|App Metadata||
|---|---|
| **Version** | %s |
| **Chart Repository** | %s |

## App README.MD

%s
`, name, version, kube.DefaultChartMuseumURL, readme), readmeFromPr)
}

func TestAddLatestAppForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()

	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)
	name := nameUUID.String()
	version := "0.1.8"
	alias := fmt.Sprintf("%s-alias", name)
	commonOpts := *testOptions.CommonOptions
	o := &add.AddAppOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Alias:      alias,
		Repo:       kube.DefaultChartMuseumURL,
		GitOps:     true,
		DevEnv:     testOptions.DevEnv,
		HelmUpdate: true, // Flag default when run on CLI
	}
	o.Args = []string{name}
	o.Verbose = true
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir

	helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: version,
		},
	}, testOptions.MockHelmer)

	err = o.Run()
	assert.NoError(t, err)
	pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
	assert.NoError(t, err)
	// Validate the PR has the right title, message
	assert.Equal(t, fmt.Sprintf("Add %s %s", name, version), pr.Title)
	assert.Equal(t, fmt.Sprintf("Add app %s %s", name, version), pr.Body)
	// Validate the branch name
	branchName, err := o.Git().Branch(devEnvDir)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("add-app-%s-%s", name, version), branchName)
	// Validate the updated Requirements.yaml
	requirements, err := helm.LoadRequirementsFile(filepath.Join(devEnvDir, helm.RequirementsFileName))
	assert.NoError(t, err)
	found := make([]*helm.Dependency, 0)
	for _, d := range requirements.Dependencies {
		if d.Name == name && d.Alias == alias {
			found = append(found, d)
		}
	}
	assert.Len(t, found, 1)
	assert.Equal(t, version, found[0].Version)
}

func TestAddAppIncludingConditionalQuestionsForGitOps(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		alias := fmt.Sprintf("%s-alias", name)
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Alias:      alias,
			Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true,
		}
		o.Args = []string{name}
		o.BatchMode = false
		envDir, err := o.CommonOptions.EnvironmentsDir()
		assert.NoError(t, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		o.CloneDir = devEnvDir

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name,
				Version: version,
			},
			Files: []*google_protobuf.Any{
				{
					TypeUrl: "values.schema.json",
					Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "enablePersistentStorage": {
      "type": "boolean"
    }
    },
    "if": {
      "properties": { "enablePersistentStorage": { "const": "true", "type": "boolean" } }
    },
    "then": {
      "properties": { "databaseConnectionUrl": { "type": "string" },
                      "databaseUsername": { "type": "string"},
                      "databasePassword": { "type": "string", "format" : "password"} }
    }}`),
				},
			},
		}, testOptions.MockHelmer)

		donec := make(chan struct{})
		go func() {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("Y")
			console.ExpectString("Enter a value for databaseConnectionUrl")
			console.SendLine("abc")
			console.ExpectString("Enter a value for databaseUsername")
			console.SendLine("wensleydale")
			console.ExpectString("Enter a value for databasePassword")
			console.SendLine("cranberries")
			console.ExpectString(" ***********")
			console.ExpectEOF()
		}()
		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		valuesFromPrPath := filepath.Join(devEnvDir, name, helm.ValuesFileName)
		_, err = os.Stat(valuesFromPrPath)
		assert.NoError(r, err)
		data, err := ioutil.ReadFile(valuesFromPrPath)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf(`databaseConnectionUrl: abc
databasePassword: vault:gitOps/%s/%s:databasePassword
databaseUsername: wensleydale
enablePersistentStorage: true
`, testOptions.DevEnvRepo.Owner, testOptions.DevEnvRepo.GitRepo.Name), string(data))

		// Validate that vault has had the secret added
		path := strings.Join([]string{"gitOps", testOptions.OrgName, testOptions.DevEnvRepoInfo.Name},
			"/")
		value := map[string]interface{}{
			"databasePassword": "cranberries",
		}
		testOptions.MockVaultClient.VerifyWasCalledOnce().Write(path, value)
	})
}

func TestAddAppExcludingConditionalQuestionsForGitOps(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		name := nameUUID.String()
		version := "0.0.1"
		alias := fmt.Sprintf("%s-alias", name)
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Alias:      alias,
			Repo:       "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true,
		}
		o.Args = []string{name}
		o.BatchMode = false
		envDir, err := o.CommonOptions.EnvironmentsDir()
		assert.NoError(t, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		o.CloneDir = devEnvDir

		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name,
				Version: version,
			},
			Files: []*google_protobuf.Any{
				{
					TypeUrl: "values.schema.json",
					Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "enablePersistentStorage": {
      "type": "boolean"
    }
    },
    "if": {
      "properties": { "enablePersistentStorage": { "const": "true" } }
    },
    "then": {
      "properties": { "databaseConnectionUrl": { "type": "string" } }
    }}`),
				},
			},
		}, testOptions.MockHelmer)

		donec := make(chan struct{})
		go func() {
			defer close(donec)
			console.ExpectString("Enter a value for enablePersistentStorage")
			console.SendLine("N")
			console.ExpectEOF()
		}()
		err = o.Run()
		assert.NoError(r, err)
		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))

		valuesFromPrPath := filepath.Join(devEnvDir, name, helm.ValuesFileName)
		_, err = os.Stat(valuesFromPrPath)
		assert.NoError(r, err)
		data, err := ioutil.ReadFile(valuesFromPrPath)
		assert.NoError(r, err)
		assert.Equal(r, `enablePersistentStorage: false
`, string(data))
	})
}

func TestAddAppForGitOpsWithSNAPSHOTVersion(t *testing.T) {
	tests.Retry(t, 1, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		nameUUID, err := uuid.NewV4()
		assert.NoError(r, err)
		shortName := nameUUID.String()
		name := fmt.Sprintf("jx-app-%s", shortName)
		version := "0.0.1-SNAPSHOT"
		alias := fmt.Sprintf("%s-alias", name)
		repo := kube.DefaultChartMuseumURL
		description := "My test chart description"
		commonOpts := *testOptions.CommonOptions
		o := &add.AddAppOptions{
			AddOptions: add.AddOptions{
				CommonOptions: &commonOpts,
			},
			Version:    version,
			Alias:      alias,
			Repo:       repo,
			GitOps:     true,
			DevEnv:     testOptions.DevEnv,
			HelmUpdate: true, // Flag default when run on CLI
		}
		envDir, err := o.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		o.CloneDir = devEnvDir
		pegomock.When(testOptions.MockHelmer.ListRepos()).ThenReturn(
			map[string]string{
				"repo1": kube.DefaultChartMuseumURL,
			}, nil)
		pegomock.When(testOptions.MockHelmer.SearchCharts(pegomock.AnyString(), pegomock.EqBool(false))).ThenReturn(
			[]helm.ChartSummary{
				{
					Name:         fmt.Sprintf("repo1/%s", name),
					ChartVersion: version,
					AppVersion:   version,
				},
			}, nil)
		helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:        name,
				Version:     version,
				Description: description,
			},
		}, testOptions.MockHelmer)
		o.Args = []string{shortName}
		err = o.Run()
		assert.NoError(r, err)
		pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.NoError(r, err)
		// Validate the PR has the right title, message
		assert.Equal(r, fmt.Sprintf("Add %s %s", name, version), pr.Title)
		assert.Equal(r, fmt.Sprintf("Add app %s %s", name, version), pr.Body)
		// Validate the branch name
		branchName, err := o.Git().Branch(devEnvDir)
		assert.NoError(r, err)
		assert.Equal(r, fmt.Sprintf("add-app-%s-%s", name, version), branchName)
		// Validate the updated Requirements.yaml
		requirements, err := helm.LoadRequirementsFile(filepath.Join(devEnvDir, helm.RequirementsFileName))
		assert.NoError(r, err)
		found := make([]*helm.Dependency, 0)
		for _, d := range requirements.Dependencies {
			if d.Name == name && d.Alias == alias {
				found = append(found, d)
			}
		}
		assert.Len(r, found, 1)
		assert.Equal(r, version, found[0].Version)
		app := &jenkinsv1.App{}
		appBytes, err := ioutil.ReadFile(filepath.Join(devEnvDir, name, "templates", "app.yaml"))
		_ = yaml.Unmarshal(appBytes, app)
		assert.Equal(r, name, app.Labels[helm.LabelAppName])
		assert.Equal(r, version, app.Labels[helm.LabelAppVersion])
		assert.Equal(r, repo, app.Annotations[helm.AnnotationAppRepository])
		assert.Equal(r, description, app.Annotations[helm.AnnotationAppDescription])
		assert.Equal(r, description, app.Annotations[helm.AnnotationAppDescription])
	})
}
