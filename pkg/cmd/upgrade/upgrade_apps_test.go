// +build unit

package upgrade_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/cmd/upgrade"

	"github.com/jenkins-x/jx/pkg/cmd/add"

	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/petergtz/pegomock"

	"github.com/jenkins-x/jx/pkg/tests"

	google_protobuf "github.com/golang/protobuf/ptypes/any"

	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/jenkins-x/jx/pkg/helm"

	"github.com/blang/semver"

	"github.com/stretchr/testify/assert"
)

var timeout = 5 * time.Second

func TestUpgradeAppForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()
	name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", nil, "")
	assert.NoError(t, err)

	// Now let's upgrade
	newVersion, err := semver.Parse(version)
	assert.NoError(t, err)
	newVersion.Patch++
	commonOpts := *testOptions.CommonOptions
	o := &upgrade.UpgradeAppsOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    newVersion.String(),
		Alias:      alias,
		Repo:       helm.FakeChartmusuem,
		GitOps:     true,
		HelmUpdate: true,
		DevEnv:     testOptions.DevEnv,
	}
	o.Args = []string{name}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir

	helm_test.StubFetchChart(name, newVersion.String(), helm.FakeChartmusuem, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: newVersion.String(),
		},
	}, testOptions.MockHelmer)

	err = o.Run()
	require.NoError(t, err)

	// Validate a PR was created
	pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
	require.NoError(t, err)

	// Validate the PR has the right title, message
	assert.Equal(t, fmt.Sprintf("Upgrade %s to %s", name, newVersion.String()), pr.Title)
	assert.Equal(t, fmt.Sprintf("Upgrade %s from %s to %s", name, version, newVersion.String()), pr.Body)

	// Validate the branch name
	branchName, err := o.Git().Branch(devEnvDir)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("upgrade-app-%s-%s", name, newVersion.String()), branchName[:len(branchName)-6])
	// Validate the updated Requirements.yaml
	requirements, err := helm.LoadRequirementsFile(filepath.Join(devEnvDir, helm.RequirementsFileName))
	require.NoError(t, err)

	found := make([]*helm.Dependency, 0)
	for _, d := range requirements.Dependencies {
		if d.Name == name && d.Alias == alias {
			found = append(found, d)
		}
	}
	assert.Len(t, found, 1)
	assert.Equal(t, newVersion.String(), found[0].Version)
}

func TestUpgradeAppWithShortNameForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()
	name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", nil, "")
	shortName := strings.TrimPrefix(name, "jx-app-")
	assert.NoError(t, err)

	// Now let's upgrade

	newVersion, err := semver.Parse(version)
	assert.NoError(t, err)
	newVersion.Patch++
	commonOpts := *testOptions.CommonOptions
	o := &upgrade.UpgradeAppsOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    newVersion.String(),
		Alias:      alias,
		Repo:       kube.DefaultChartMuseumURL,
		GitOps:     true,
		HelmUpdate: true,
		DevEnv:     testOptions.DevEnv,
	}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
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
		},
		nil,
	)
	o.Args = []string{shortName}

	helm_test.StubFetchChart(name, newVersion.String(), helm.FakeChartmusuem, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: newVersion.String(),
		},
	}, testOptions.MockHelmer)

	err = o.Run()
	assert.NoError(t, err)
	// Validate a PR was created
	pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
	assert.NoError(t, err)
	// Validate the PR has the right title, message
	assert.Equal(t, fmt.Sprintf("Upgrade %s to %s", name, newVersion.String()), pr.Title)
	assert.Equal(t, fmt.Sprintf("Upgrade %s from %s to %s", name, version, newVersion.String()), pr.Body)
	// Validate the branch name
	branchName, err := o.Git().Branch(devEnvDir)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("upgrade-app-%s-%s", name, newVersion.String()), branchName[:len(branchName)-6])
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
	assert.Equal(t, newVersion.String(), found[0].Version)
}

func TestUpgradeAppWithExistingAndDefaultAnswersForGitOpsInBatchMode(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", map[string]interface{}{
			"name": "testing",
		}, "")
		assert.NoError(r, err)

		envDir, err := testOptions.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		appDir := filepath.Join(devEnvDir, name)

		// Now let's upgrade

		newVersion, err := semver.Parse(version)
		assert.NoError(r, err)
		newVersion.Patch++
		o := &upgrade.UpgradeAppsOptions{
			AddOptions: add.AddOptions{
				CommonOptions: testOptions.CommonOptions,
			},
			Version:    newVersion.String(),
			Alias:      alias,
			Repo:       helm.FakeChartmusuem,
			GitOps:     true,
			HelmUpdate: true,
			DevEnv:     testOptions.DevEnv,
		}
		o.CloneDir = devEnvDir
		o.Args = []string{name}

		helm_test.StubFetchChart(name, newVersion.String(),
			helm.FakeChartmusuem, &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    name,
					Version: newVersion.String(),
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
      "type": "string"
    },
    "species": {
      "type": "string",
      "default": "human"
    }
  }
}`),
					},
				},
			}, testOptions.MockHelmer)

		err = o.Run()
		assert.NoError(r, err)
		// Validate a PR was created
		_, err = testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.NoError(r, err)
		// Validate the updated values.yaml
		existingValues, err := ioutil.ReadFile(filepath.Join(appDir, helm.ValuesFileName))
		assert.NoError(r, err)
		assert.Equal(r, `name: testing
species: human
`, string(existingValues))
	})
}

func TestUpgradeAppWithExistingAndDefaultAnswersForGitOps(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", map[string]interface{}{
			"name": "testing",
		}, "")
		assert.NoError(r, err)

		envDir, err := testOptions.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		appDir := filepath.Join(devEnvDir, name)
		// Now let's upgrade

		newVersion, err := semver.Parse(version)
		assert.NoError(r, err)
		newVersion.Patch++
		o := &upgrade.UpgradeAppsOptions{
			AddOptions: add.AddOptions{
				CommonOptions: testOptions.CommonOptions,
			},
			Version:    newVersion.String(),
			Alias:      alias,
			Repo:       helm.FakeChartmusuem,
			GitOps:     true,
			HelmUpdate: true,
			DevEnv:     testOptions.DevEnv,
		}
		o.CloneDir = devEnvDir
		o.Args = []string{name}
		o.BatchMode = false

		helm_test.StubFetchChart(name, newVersion.String(),
			helm.FakeChartmusuem, &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    name,
					Version: newVersion.String(),
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
      "type": "string"
    },
    "species": {
      "type": "string",
      "default": "human"
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
			console.ExpectString("Enter a value for name testing [Automatically accepted existing value]\r\n")
			console.ExpectString("Enter a value for species (human)")
			console.SendLine("martian")
			console.ExpectString("martian? Enter a value for species martian")
		}()

		err = o.Run()
		assert.NoError(r, err)
		// Validate a PR was created
		_, err = testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.NoError(r, err)
		// Validate the updated values.yaml
		existingValues, err := ioutil.ReadFile(filepath.Join(appDir, helm.ValuesFileName))
		assert.NoError(r, err)
		assert.Equal(r, `name: testing
species: martian
`, string(existingValues))
	})
}

func TestUpgradeAppWithExistingAndDefaultAnswersAndAskAllForGitOps(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", map[string]interface{}{
			"name": "testing",
		}, "")
		assert.NoError(r, err)

		envDir, err := testOptions.CommonOptions.EnvironmentsDir()
		assert.NoError(r, err)
		devEnvDir := testOptions.GetFullDevEnvDir(envDir)
		appDir := filepath.Join(devEnvDir, name)

		// Now let's upgrade

		newVersion, err := semver.Parse(version)
		assert.NoError(r, err)
		newVersion.Patch++
		o := &upgrade.UpgradeAppsOptions{
			AddOptions: add.AddOptions{
				CommonOptions: testOptions.CommonOptions,
			},
			Version:    newVersion.String(),
			Alias:      alias,
			Repo:       helm.FakeChartmusuem,
			GitOps:     true,
			HelmUpdate: true,
			DevEnv:     testOptions.DevEnv,
			AskAll:     true,
		}
		o.CloneDir = devEnvDir
		o.Args = []string{name}
		o.BatchMode = false

		helm_test.StubFetchChart(name, newVersion.String(),
			helm.FakeChartmusuem, &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    name,
					Version: newVersion.String(),
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
      "type": "string"
    },
    "species": {
      "type": "string",
      "default": "human"
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
			console.ExpectString("Enter a value for name (testing)")
			console.SendLine("mark")
			console.ExpectString("Enter a value for species (human)")
			console.SendLine("martian")
			console.ExpectString("martian? Enter a value for species martian")
		}()

		err = o.Run()
		assert.NoError(r, err)
		// Validate a PR was created
		_, err = testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.NoError(r, err)
		// Validate the updated values.yaml
		existingValues, err := ioutil.ReadFile(filepath.Join(appDir, helm.ValuesFileName))
		assert.NoError(r, err)
		assert.Equal(r, `name: mark
species: martian
`, string(existingValues))
	})
}

func TestUpgradeMissingExistingOrDefaultInBatchMode(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")
	pegomock.RegisterMockTestingT(t)
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		testOptions := testhelpers.CreateAppTestOptions(true, "", r)
		defer func() {
			err := testOptions.Cleanup()
			assert.NoError(r, err)
		}()

		// Needs console
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()
		testOptions.CommonOptions.In = console.In
		testOptions.CommonOptions.Out = console.Out
		testOptions.CommonOptions.Err = console.Err

		name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", map[string]interface{}{}, "")
		assert.NoError(r, err)

		// Now let's upgrade

		newVersion, err := semver.Parse(version)
		assert.NoError(r, err)
		newVersion.Patch++
		o := &upgrade.UpgradeAppsOptions{
			AddOptions: add.AddOptions{
				CommonOptions: testOptions.CommonOptions,
			},
			Version:    newVersion.String(),
			Alias:      alias,
			Repo:       helm.FakeChartmusuem,
			GitOps:     true,
			HelmUpdate: true,
			DevEnv:     testOptions.DevEnv,
		}
		o.Args = []string{name}

		helm_test.StubFetchChart(name, newVersion.String(),
			helm.FakeChartmusuem, &chart.Chart{
				Metadata: &chart.Metadata{
					Name:    name,
					Version: newVersion.String(),
				},
				Files: []*google_protobuf.Any{
					{
						TypeUrl: "values.schema.json",
						Value: []byte(`{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "required": [
	"name",
	"species"
  ],
  "properties": {
    "name": {
      "type": "string"
    },
    "species": {
      "type": "string",
      "default": "human"
    }
  }
}`),
					},
				},
			}, testOptions.MockHelmer)

		err = o.Run()
		assert.Error(r, err)
		// Validate a PR was not created
		_, err = testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
		assert.Error(r, err)
	})
}

func TestUpgradeAppToLatestForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()
	name, alias, version, err := testOptions.DirectlyAddAppToGitOps("", nil, "")
	assert.NoError(t, err)

	// Now let's upgrade

	newVersion, err := semver.Parse(version)
	assert.NoError(t, err)
	newVersion.Patch++
	commonOpts := *testOptions.CommonOptions
	o := &upgrade.UpgradeAppsOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Version:    newVersion.String(),
		Alias:      alias,
		Repo:       helm.FakeChartmusuem,
		GitOps:     true,
		HelmUpdate: true,
		DevEnv:     testOptions.DevEnv,
	}
	o.Args = []string{name}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir

	helm_test.StubFetchChart(name, "", helm.FakeChartmusuem, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: newVersion.String(),
		},
	}, testOptions.MockHelmer)

	err = o.Run()
	assert.NoError(t, err)
	// Validate a PR was created
	pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
	assert.NoError(t, err)
	// Validate the PR has the right title, message
	assert.Equal(t, fmt.Sprintf("Upgrade %s to %s", name, newVersion.String()), pr.Title)
	assert.Equal(t, fmt.Sprintf("Upgrade %s from %s to %s", name, version, newVersion.String()), pr.Body)
	// Validate the branch name
	branchName, err := o.Git().Branch(devEnvDir)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("upgrade-app-%s-%s", name, newVersion.String()), branchName[:len(branchName)-6])
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
	assert.Equal(t, newVersion.String(), found[0].Version)
}

func TestUpgradeAllAppsForGitOps(t *testing.T) {
	testOptions := testhelpers.CreateAppTestOptions(true, "", t)
	defer func() {
		err := testOptions.Cleanup()
		assert.NoError(t, err)
	}()
	name1, alias1, version1, err := testOptions.DirectlyAddAppToGitOps("", nil, "")
	assert.NoError(t, err)
	name2, alias2, version2, err := testOptions.DirectlyAddAppToGitOps("", nil, "")
	assert.NoError(t, err)

	newVersion1, err := semver.Parse(version1)
	assert.NoError(t, err)
	newVersion1.Patch++

	newVersion2, err := semver.Parse(version2)
	assert.NoError(t, err)
	newVersion2.Minor++

	// Now let's upgrade
	commonOpts := *testOptions.CommonOptions
	o := &upgrade.UpgradeAppsOptions{
		AddOptions: add.AddOptions{
			CommonOptions: &commonOpts,
		},
		Repo:       helm.FakeChartmusuem,
		GitOps:     true,
		HelmUpdate: true,
		DevEnv:     testOptions.DevEnv,
	}
	envDir, err := o.CommonOptions.EnvironmentsDir()
	assert.NoError(t, err)
	devEnvDir := testOptions.GetFullDevEnvDir(envDir)
	o.CloneDir = devEnvDir

	helm_test.StubFetchChart(name1, "", helm.FakeChartmusuem, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name1,
			Version: newVersion1.String(),
		},
	}, testOptions.MockHelmer)

	// The "latest" chart - requested with an empty version
	helm_test.StubFetchChart(name2, "",
		helm.FakeChartmusuem, &chart.Chart{
			Metadata: &chart.Metadata{
				Name:    name2,
				Version: newVersion2.String(),
			},
		}, testOptions.MockHelmer)

	err = o.Run()
	assert.NoError(t, err)
	// Validate a PR was created
	pr, err := testOptions.FakeGitProvider.GetPullRequest(testOptions.OrgName, testOptions.DevEnvRepoInfo, 1)
	assert.NoError(t, err)
	// Validate the PR has the right title, message
	assert.Equal(t, fmt.Sprintf("Upgrade all apps"), pr.Title)
	assert.Equal(t, fmt.Sprintf("Upgrade all apps:\n\n* %s from %s to %s\n* %s from %s to %s", name1,
		version1,
		newVersion1.String(), name2, version2, newVersion2.String()), pr.Body)
	// Validate the branch name1
	branchName, err := o.Git().Branch(devEnvDir)
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("upgrade-all-apps"), branchName[:len(branchName)-6])
	// Validate the updated Requirements.yaml
	requirements, err := helm.LoadRequirementsFile(filepath.Join(devEnvDir, helm.RequirementsFileName))
	assert.NoError(t, err)
	found := make([]*helm.Dependency, 0)
	for _, d := range requirements.Dependencies {
		if d.Name == name1 && d.Alias == alias1 {
			found = append(found, d)
			assert.Equal(t, newVersion1.String(), d.Version)
		}
		if d.Name == name2 && d.Alias == alias2 {
			found = append(found, d)
			assert.Equal(t, newVersion2.String(), d.Version)
		}
	}
	assert.Len(t, found, 2)
}
