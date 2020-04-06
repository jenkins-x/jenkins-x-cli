// +build unit

package helm_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	kube_test "github.com/jenkins-x/jx/pkg/kube/mocks"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/stretchr/testify/assert"

	mocks "github.com/jenkins-x/jx/pkg/util/mocks"
	. "github.com/petergtz/pegomock"
)

const binary = "helm"
const binaryV3 = "helm3"
const cwd = "test"
const repo = "test-repo"
const repoURL = "http://test-repo"
const serviceAccount = "test-sa"
const namespace = "test-namespace"
const chart = "test-chart"
const releaseName = "test-release"
const listRepoOutput = `
NAME            URL
stable          https://kubernetes-charts.storage.googleapis.com
local           http://127.0.0.1:8879/charts
jenkins-x       https://storage.googleapis.com/chartmuseum.jenkins-x.io
	`
const searchVersionOutput = `
NAME                            		CHART VERSION	APP VERSION		DESCRIPTION
jenkins-x/jenkins-x-platform        	0.0.1481						Jenkins X
jenkins-x/jenkins-x-platform        	0.0.1480						Jenkins X
jenkins-x/jenkins-x-platform        	0.0.1479 						Jenkins X
`
const listReleasesOutput = `
NAME                            REVISION        UPDATED                         STATUS          CHART                           NAMESPACE
jenkins-x                       1               Mon Jul  2 16:16:20 2018        DEPLOYED        jenkins-x-platform-0.0.1655     jx
jx-production                   1               Mon Jul  2 16:22:47 2018        DEPLOYED        env-0.0.1                       jx-production
jx-staging                      1               Mon Jul  2 16:21:06 2018        DEPLOYED        env-0.0.1                       jx-staging
jxing                           1               Wed Jun  6 14:24:42 2018        DEPLOYED        nginx-ingress-0.20.1            kube-system
vault-operator                  1               Mon Jun 25 16:09:28 2018        DEPLOYED        vault-operator-0.1.0            jx
`

const listReleasesOutputHelm3 = `
NAME 	NAMESPACE	REVISION	UPDATED                             	STATUS  	CHART
jxing	jx       	2       	2019-05-17 15:30:07.629472 +0100 BST	deployed	nginx-ingress-1.3.1`

func createHelm(t *testing.T, expectedError error, expectedOutput string) (*helm.HelmCLI, *mocks.MockCommander) {
	return createHelmWithCwdAndHelmVersion(t, helm.V2, cwd, expectedError, expectedOutput)
}

func createHelmWithVersion(t *testing.T, version helm.Version, expectedError error, expectedOutput string) (*helm.HelmCLI, *mocks.MockCommander) {
	return createHelmWithCwdAndHelmVersion(t, version, cwd, expectedError, expectedOutput)
}

func createHelmWithCwdAndHelmVersion(t *testing.T, version helm.Version, dir string, expectedError error, expectedOutput string) (*helm.HelmCLI, *mocks.MockCommander) {
	RegisterMockTestingT(t)
	runner := mocks.NewMockCommander()
	When(runner.RunWithoutRetry()).ThenReturn(expectedOutput, expectedError)
	helmBinary := binary
	if version == helm.V3 {
		helmBinary = binaryV3
	}
	mockKuber := kube_test.NewMockKuber()
	cli := helm.NewHelmCLIWithRunner(runner, helmBinary, version, dir, true, mockKuber)
	return cli, runner
}

func verifyArgs(t *testing.T, cli *helm.HelmCLI, runner *mocks.MockCommander, expectedArgs ...string) {
	runner.VerifyWasCalledOnce().SetArgs(expectedArgs)
}

func TestNewHelmCLI(t *testing.T) {
	cli := helm.NewHelmCLI(binary, helm.V2, cwd, true, "arg1 arg2 arg3")
	assert.Equal(t, binary, cli.Binary)
	assert.Equal(t, cwd, cli.CWD)
	assert.Equal(t, helm.V2, cli.BinVersion)
	assert.Equal(t, true, cli.Debug)
	assert.NotNil(t, cli.Runner)
	assert.Equal(t, []string{"arg1", "arg2", "arg3"}, cli.Runner.CurrentArgs())
	assert.Equal(t, binary, cli.Runner.CurrentName())
	assert.Equal(t, cwd, cli.Runner.CurrentDir())
}

func TestInit(t *testing.T) {
	expectedArgs := []string{"init", "--client-only", "--service-account", serviceAccount,
		"--tiller-namespace", namespace, "--upgrade", "--wait", "--force-upgrade"}
	helm, runner := createHelm(t, nil, "")

	err := helm.Init(true, serviceAccount, namespace, true)

	assert.NoError(t, err, "should init helm without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestAddRepo(t *testing.T) {
	expectedArgs := []string{"repo", "add", repo, repoURL}
	helm, runner := createHelm(t, nil, "")

	err := helm.AddRepo(repo, repoURL, "", "")

	assert.NoError(t, err, "should add helm repo without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestRemoveRepo(t *testing.T) {
	expectedArgs := []string{"repo", "remove", repo}
	helm, runner := createHelm(t, nil, "")

	err := helm.RemoveRepo(repo)

	assert.NoError(t, err, "should remove helm repo without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestListRepos(t *testing.T) {
	expectedArgs := []string{"repo", "list"}
	helm, runner := createHelm(t, nil, listRepoOutput)

	repos, err := helm.ListRepos()
	assert.NoError(t, err, "should list helm repos without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
	expectedRepos := map[string]string{
		"stable":    "https://kubernetes-charts.storage.googleapis.com",
		"local":     "http://127.0.0.1:8879/charts",
		"jenkins-x": "https://storage.googleapis.com/chartmuseum.jenkins-x.io",
	}
	assert.Equal(t, len(expectedRepos), len(repos), "should list the same number of repos")
	for k, v := range repos {
		assert.Equal(t, expectedRepos[k], v, "should parse correctly the repo URL")
	}
}

func TestIsRepoMissing(t *testing.T) {
	expectedArgs := []string{"repo", "list"}
	helm, runner := createHelm(t, nil, listRepoOutput)

	url := "https://storage.googleapis.com/chartmuseum.jenkins-x.io"
	missing, _, err := helm.IsRepoMissing(url)

	assert.NoError(t, err, "should search missing repos without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
	assert.False(t, missing, "should find url '%s'", url)

	url = "https://test"
	missing, _, err = helm.IsRepoMissing(url)

	assert.NoError(t, err, "search missing repos should not return an error")
	assert.True(t, missing, "should not find url '%s'", url)

	url = "http://127.0.0.1:8879/chartsv2"
	missing, _, err = helm.IsRepoMissing(url)

	assert.NoError(t, err, "search missing repos should not return an error")
	assert.True(t, missing, "should not find url '%s'", url)
}

func TestUpdateRepo(t *testing.T) {
	expectedArgs := []string{"repo", "update"}
	helm, runner := createHelm(t, nil, "")

	err := helm.UpdateRepo()

	assert.NoError(t, err, "should update  helm repo without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestRemoveRequirementsLock(t *testing.T) {
	dir, err := ioutil.TempDir("", "reqtest")
	assert.NoError(t, err, "should be able to create a temporary dir")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "requirements.lock")
	ioutil.WriteFile(path, []byte("test"), 0644)

	helm, _ := createHelmWithCwdAndHelmVersion(t, helm.V2, dir, nil, "")

	err = helm.RemoveRequirementsLock()
	assert.NoError(t, err, "should remove requirements.lock file")
}

func TestBuildDependency(t *testing.T) {
	expectedArgs := []string{"dependency", "build"}
	helm, runner := createHelm(t, nil, "")

	err := helm.BuildDependency()
	assert.NoError(t, err, "should build helm repo dependencies without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestInstallChart(t *testing.T) {
	value := []string{"test=true"}
	valueString := []string{"context=test"}
	valueFile := []string{"./myvalues.yaml"}
	expectedArgs := []string{"install", "--wait", "--name", releaseName, "--namespace", namespace,
		chart, "--set", value[0], "--set-string", valueString[0], "--values", valueFile[0]}
	helm, runner := createHelm(t, nil, "")

	err := helm.InstallChart(chart, releaseName, namespace, "", -1, value, valueString, valueFile, "", "", "")
	assert.NoError(t, err, "should install the chart without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestUpgradeChart(t *testing.T) {
	value := []string{"test=true"}
	valueString := []string{"context=test"}
	valueFile := []string{"./myvalues.yaml"}
	version := "0.0.1"
	timeout := 600
	expectedArgs := []string{"upgrade", "--namespace", namespace, "--install", "--wait", "--force",
		"--timeout", fmt.Sprintf("%d", timeout), "--version", version, "--set", value[0], "--set-string", valueString[0], "--values", valueFile[0], releaseName, chart}
	helm, runner := createHelm(t, nil, "")

	err := helm.UpgradeChart(chart, releaseName, namespace, version, true, timeout, true, true, value, valueString, valueFile, "", "", "")

	assert.NoError(t, err, "should upgrade the chart without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestDeleteRelaese(t *testing.T) {
	expectedArgs := []string{"delete", "--purge", releaseName}
	helm, runner := createHelm(t, nil, "")
	ns := "default"

	err := helm.DeleteRelease(ns, releaseName, true)

	assert.NoError(t, err, "should delete helm chart release without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestStatusRelease(t *testing.T) {
	expectedArgs := []string{"status", releaseName}
	helm, runner := createHelm(t, nil, "")
	ns := "default"

	err := helm.StatusRelease(ns, releaseName)

	assert.NoError(t, err, "should get the status of a helm chart release without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestStatusReleaseWithOutputNoFormat(t *testing.T) {
	expectedArgs := []string{"status", releaseName}
	helm, runner := createHelm(t, nil, "")
	ns := "default"

	_, err := helm.StatusReleaseWithOutput(ns, releaseName, "")

	assert.NoErrorf(t, err, "should return the status of the helm chart without format")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestStatusReleaseWithOutputWithFormat(t *testing.T) {
	expectedArgs := []string{"status", releaseName, "--output", "json"}
	helm, runner := createHelm(t, nil, "")
	ns := "default"

	_, err := helm.StatusReleaseWithOutput(ns, releaseName, "json")

	assert.NoErrorf(t, err, "should return the status of the helm chart without in Json format")
	verifyArgs(t, helm, runner, expectedArgs...)
}

func TestStatusReleases(t *testing.T) {
	expectedArgs := []string{"list", "--all", "--namespace", "default"}
	expectedStatusMap := map[string]string{
		"jenkins-x":      "DEPLOYED",
		"jx-production":  "DEPLOYED",
		"jx-staging":     "DEPLOYED",
		"jxing":          "DEPLOYED",
		"vault-operator": "DEPLOYED",
	}
	helm, runner := createHelm(t, nil, listReleasesOutput)
	ns := "default"

	releaseMap, _, err := helm.ListReleases(ns)

	assert.NoError(t, err, "should list the release statuses without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
	for release, details := range releaseMap {
		assert.Equal(t, expectedStatusMap[release], details.Status, "expected details '%s', got '%s'",
			expectedStatusMap[release], details)
	}
}

func TestStatusReleasesForHelm3(t *testing.T) {
	expectedArgs := []string{"list", "--all", "--namespace", "default"}
	expectedStatusMap := map[string]string{
		"jxing": "DEPLOYED",
	}
	helm, runner := createHelmWithVersion(t, helm.V3, nil, listReleasesOutputHelm3)
	ns := "default"

	releaseMap, _, err := helm.ListReleases(ns)

	assert.NoError(t, err, "should list the release statuses without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
	for release, details := range releaseMap {
		assert.Equal(t, expectedStatusMap[release], details.Status, "expected details '%s', got '%s'",
			expectedStatusMap[release], details)
	}
}

func TestLint(t *testing.T) {
	expectedArgs := []string{"lint",
		"--set", "tags.jx-lint=true",
		"--set", "global.jxLint=true",
		"--set-string", "global.jxTypeEnv=lint",
	}
	expectedOutput := "test"
	helm, runner := createHelm(t, nil, expectedOutput)

	output, err := helm.Lint(nil)

	assert.NoError(t, err, "should lint the chart without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
	assert.Equal(t, "test", output)
}

func TestVersion(t *testing.T) {
	var versionTests = []struct {
		versionString           string
		expectedSemanticVersion string
		shouldError             bool
	}{
		{"Client: v2.16.3+g1ee0254", "2.16.3", false},
		{"v3.0.3+gac925eb", "3.0.3", false},
		{"3.0.3+gac925eb", "3.0.3", false},
		{"", "", true},
		{"foo", "", true},
	}

	for _, versionTest := range versionTests {
		t.Run(versionTest.versionString, func(t *testing.T) {
			helm, runner := createHelm(t, nil, versionTest.versionString)
			actualVersion, err := helm.Version(false)

			if versionTest.shouldError {
				assert.Error(t, err, "should not be able to get semantic version from version output")
				assert.Equal(t, versionTest.expectedSemanticVersion, actualVersion)
			} else {
				assert.NoError(t, err, "should get the version without any error")
				verifyArgs(t, helm, runner, "version", "--short", "--client")
				assert.Equal(t, versionTest.expectedSemanticVersion, actualVersion)
			}
		})
	}
}

func TestSearchChartVersions(t *testing.T) {
	expectedOutput := searchVersionOutput
	expectedArgs := []string{"search", chart, "--versions"}
	helm, runner := createHelm(t, nil, expectedOutput)

	versions, err := helm.SearchCharts(chart, true)

	assert.NoError(t, err, "should search chart versions without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
	expectedVersions := []string{"0.0.1481", "0.0.1480", "0.0.1479"}
	for i, version := range versions {
		assert.Equal(t, expectedVersions[i], version.ChartVersion, "should parse the version '%s'", version)
	}
}

func TestFindChart(t *testing.T) {
	dir, err := ioutil.TempDir("", "charttest")
	assert.NoError(t, err, "should be able to create a temporary dir")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, helm.ChartFileName)
	ioutil.WriteFile(path, []byte("test"), 0644)
	helm, _ := createHelmWithCwdAndHelmVersion(t, helm.V2, dir, nil, "")

	chartFile, err := helm.FindChart()

	assert.NoError(t, err, "should find the chart file")
	assert.Equal(t, path, chartFile, "should find chart file '%s'", path)
}

func TestPackage(t *testing.T) {
	expectedArgs := []string{"package", cwd}
	helm, runner := createHelm(t, nil, "")

	err := helm.PackageChart()

	assert.NoError(t, err, "should package chart without any error")
	verifyArgs(t, helm, runner, expectedArgs...)
}
