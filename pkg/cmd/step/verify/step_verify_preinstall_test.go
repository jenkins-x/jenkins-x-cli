// +build unit

package verify

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

var timeout = 1 * time.Second

func Test_verifyPrivateRepos_returns_nil_in_batch_mode(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	testOptions := &StepVerifyPreInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: true,
				},
			},
		},
	}

	testConfig := &config.RequirementsConfig{}

	assert.NoError(t, testOptions.verifyPrivateRepos(testConfig))
}

func Test_confirm_private_repos_with_github_provider(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	console := tests.NewTerminal(t, &timeout)
	defer console.Cleanup()

	testOptions := &StepVerifyPreInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					In:  console.In,
					Out: console.Out,
					Err: console.Err,
				},
			},
		},
	}

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.GitKind = "github"
	testConfig.Cluster.EnvironmentGitOwner = "acme"

	done := make(chan struct{})
	go func() {
		defer close(done)
		console.ExpectString("If 'acme' is an GitHub organisation it needs to have a paid subscription to create private repos. Do you wish to continue?")
		console.SendLine("Y")
		console.ExpectEOF()
	}()
	err := testOptions.verifyPrivateRepos(testConfig)
	console.Close()
	<-done

	assert.NoError(t, err)
}

func Test_doesnt_ask_for_confirmation_when_in_gke(t *testing.T) {
	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)

	testOptions := &StepVerifyPreInstallOptions{
		WorkloadIdentity: true,
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					Out: fakeStdout,
				},
			},
		},
	}

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.GitKind = "github"
	testConfig.Cluster.Provider = "gke"
	testConfig.Cluster.EnvironmentGitOwner = "acme"
	testConfig.Cluster.ProjectID = "test"
	testConfig.Cluster.Zone = "exzone"
	testConfig.Cluster.ClusterName = "acme"
	testConfig.Cluster.DevEnvApprovers = []string{"bob"}

	testOptions.gatherRequirements(testConfig, "")
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.NotContains(t, output, fmt.Sprintf("jx boot has only been validated on GKE"))
}

func Test_doesnt_ask_for_confirmation_when_in_batch_mode_and_with_different_provider(t *testing.T) {
	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)

	testOptions := &StepVerifyPreInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: true,
					Out:       fakeStdout,
				},
			},
		},
	}

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.GitKind = "github"
	testConfig.Cluster.Provider = "iks"
	testConfig.Cluster.EnvironmentGitOwner = "acme"
	testConfig.Cluster.ProjectID = "test"
	testConfig.Cluster.Zone = "exzone"
	testConfig.Cluster.ClusterName = "acme"
	testConfig.Cluster.DevEnvApprovers = []string{"bob"}

	testOptions.gatherRequirements(testConfig, "")
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.Contains(t, output, fmt.Sprintf("jx boot has only been validated on GKE"))
}

func Test_asks_for_confirmation_when_not_in_batch_mode_and_with_different_provider(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	console := tests.NewTerminal(t, &timeout)
	defer console.Cleanup()

	testOptions := &StepVerifyPreInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: false,
					In:        console.In,
					Out:       console.Out,
					Err:       console.Err,
				},
			},
		},
	}

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.GitKind = "github"
	testConfig.Cluster.Provider = "iks"
	testConfig.Cluster.EnvironmentGitOwner = "acme"
	testConfig.Cluster.ProjectID = "test"
	testConfig.Cluster.Zone = "exzone"
	testConfig.Cluster.ClusterName = "acme"
	testConfig.Cluster.DevEnvApprovers = []string{"bob"}

	done := make(chan struct{})
	go func() {
		defer close(done)
		console.ExpectString("Continue execution anyway?")
		console.SendLine("N")
		console.ExpectEOF()
	}()
	testOptions.gatherRequirements(testConfig, "")
	console.Close()
	<-done
}

func Test_abort_private_repos_with_github_provider(t *testing.T) {
	t.Parallel()
	log.SetOutput(ioutil.Discard)

	console := tests.NewTerminal(t, &timeout)
	defer console.Cleanup()

	testOptions := &StepVerifyPreInstallOptions{
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					In:  console.In,
					Out: console.Out,
					Err: console.Err,
				},
			},
		},
	}

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.GitKind = "github"
	testConfig.Cluster.EnvironmentGitOwner = "acme"

	done := make(chan struct{})
	go func() {
		defer close(done)
		console.ExpectString("If 'acme' is an GitHub organisation it needs to have a paid subscription to create private repos. Do you wish to continue?")
		console.SendLine("N")
		console.ExpectEOF()
	}()
	err := testOptions.verifyPrivateRepos(testConfig)
	console.Close()
	<-done

	assert.Error(t, err)
	assert.Equal(t, "cannot continue without completed git requirements", err.Error())
}

func TestGatherRequirements_PreserveEnvironmentGitOwnerCase(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-step-verify-preinstall-")
	require.NoError(t, err)

	requirementsFileName := filepath.Join(tempDir, "jx-requirements.yml")

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.Provider = "gke"
	testConfig.Cluster.EnvironmentGitOwner = "ACME"
	testConfig.Cluster.ProjectID = "test"
	testConfig.Cluster.Zone = "exzone"
	testConfig.Cluster.ClusterName = "acme"

	testOptions := &StepVerifyPreInstallOptions{
		WorkloadIdentity: true,
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: true,
				},
			},
		},
	}

	_, err = testOptions.gatherRequirements(testConfig, requirementsFileName)
	assert.NoError(t, err)

	requirementsWithDefaults, err := config.LoadRequirementsConfigFile(requirementsFileName)
	assert.NoError(t, err)

	assert.Equal(t, "jx", requirementsWithDefaults.Cluster.Namespace)
	assert.Equal(t, "https://github.com", requirementsWithDefaults.Cluster.GitServer)
	assert.Equal(t, "github", requirementsWithDefaults.Cluster.GitKind)
	assert.Equal(t, "github", requirementsWithDefaults.Cluster.GitName)
	assert.Equal(t, "-jx.", requirementsWithDefaults.Ingress.NamespaceSubDomain)
	assert.Equal(t, config.RepositoryTypeNexus, requirementsWithDefaults.Repository)
	assert.Equal(t, "ACME", requirementsWithDefaults.Cluster.EnvironmentGitOwner)
}

func TestGatherRequirements_DevEnvApprovers(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-step-verify-preinstall-")
	require.NoError(t, err)

	requirementsFileName := filepath.Join(tempDir, "jx-requirements.yml")

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.Provider = "gke"
	testConfig.Cluster.EnvironmentGitOwner = "acme"
	testConfig.Cluster.ProjectID = "test"
	testConfig.Cluster.Zone = "exzone"
	testConfig.Cluster.ClusterName = "acme"
	testConfig.Cluster.DevEnvApprovers = []string{
		"bob",
		"steve",
	}

	testOptions := &StepVerifyPreInstallOptions{
		WorkloadIdentity: true,
		Dir:              tempDir,
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: true,
				},
			},
		},
	}

	_, err = testOptions.gatherRequirements(testConfig, requirementsFileName)
	assert.NoError(t, err)

	requirementsWithDefaults, err := config.LoadRequirementsConfigFile(requirementsFileName)
	assert.NoError(t, err)

	assert.Equal(t, "jx", requirementsWithDefaults.Cluster.Namespace)
	assert.Equal(t, "https://github.com", requirementsWithDefaults.Cluster.GitServer)
	assert.Equal(t, "github", requirementsWithDefaults.Cluster.GitKind)
	assert.Equal(t, "github", requirementsWithDefaults.Cluster.GitName)
	assert.Equal(t, "-jx.", requirementsWithDefaults.Ingress.NamespaceSubDomain)
	assert.Equal(t, config.RepositoryTypeNexus, requirementsWithDefaults.Repository)
	assert.Len(t, requirementsWithDefaults.Cluster.DevEnvApprovers, 2)
	assert.Equal(t, []string{"bob", "steve"}, requirementsWithDefaults.Cluster.DevEnvApprovers)

	ownersFileName := filepath.Join(tempDir, "OWNERS")
	ownersExists, err := util.FileExists(ownersFileName)
	assert.NoError(t, err)
	assert.True(t, ownersExists)

	expectedOwners := &prow.Owners{
		Approvers: []string{"bob", "steve"},
		Reviewers: []string{"bob", "steve"},
	}
	loadedOwners := &prow.Owners{}

	ownersContent, err := ioutil.ReadFile(ownersFileName)
	assert.NoError(t, err)

	err = yaml.Unmarshal(ownersContent, loadedOwners)
	assert.NoError(t, err)
	assert.Equal(t, expectedOwners, loadedOwners)
}

func TestGatherRequirements_SetsDefaults(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-step-verify-preinstall-")
	require.NoError(t, err)

	requirementsFileName := filepath.Join(tempDir, "jx-requirements.yml")

	testConfig := &config.RequirementsConfig{}
	testConfig.Cluster.Provider = "gke"
	testConfig.Cluster.EnvironmentGitOwner = "acme"
	testConfig.Cluster.ProjectID = "test"
	testConfig.Cluster.Zone = "exzone"
	testConfig.Cluster.ClusterName = "acme"

	testOptions := &StepVerifyPreInstallOptions{
		WorkloadIdentity: true,
		Dir:              tempDir,
		StepVerifyOptions: StepVerifyOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: true,
				},
			},
		},
	}

	_, err = testOptions.gatherRequirements(testConfig, requirementsFileName)
	assert.NoError(t, err)

	requirementsWithDefaults, err := config.LoadRequirementsConfigFile(requirementsFileName)
	assert.NoError(t, err)

	assert.Equal(t, "jx", requirementsWithDefaults.Cluster.Namespace)
	assert.Equal(t, "https://github.com", requirementsWithDefaults.Cluster.GitServer)
	assert.Equal(t, "github", requirementsWithDefaults.Cluster.GitKind)
	assert.Equal(t, "github", requirementsWithDefaults.Cluster.GitName)
	assert.Equal(t, "-jx.", requirementsWithDefaults.Ingress.NamespaceSubDomain)
	assert.Equal(t, config.RepositoryTypeNexus, requirementsWithDefaults.Repository)
	assert.Len(t, requirementsWithDefaults.Cluster.DevEnvApprovers, 0)

	ownersFileName := filepath.Join(tempDir, "OWNERS")
	ownersExists, err := util.FileExists(ownersFileName)
	assert.NoError(t, err)
	assert.False(t, ownersExists)
}
