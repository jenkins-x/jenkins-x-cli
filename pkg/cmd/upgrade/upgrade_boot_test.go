// +build unit

package upgrade

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	clientfake "github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	defaultBootRequirements     = "test_data/upgrade_boot"
	alternativeBootRequirements = "test_data/upgrade_boot_alternative"
)

type TestUpgradeBootOptions struct {
	UpgradeBootOptions
	Dir string
}

func (o *TestUpgradeBootOptions) setup(bootRequirements, previewEnvironmentName, sourceURL, namespace string) {
	dir := bootRequirements
	// Not setting it to "dev" will overwrite this preview environment with a new one
	devEnv := kube.NewPreviewEnvironment(previewEnvironmentName)
	devEnv.Spec.Source.URL = sourceURL
	commonOpts := &opts.CommonOptions{}
	commonOpts.SetDevNamespace(namespace)

	factory := clientfake.NewFakeFactory()
	user := jenkinsv1.User{
		ObjectMeta: v1.ObjectMeta{
			Namespace: "jx",
			Name:      "test-user",
		},
		Spec: jenkinsv1.UserDetails{
			Name:     "Test",
			Email:    "test@test.com",
			Accounts: make([]jenkinsv1.AccountReference, 0),
		},
	}

	testhelpers.ConfigureTestOptionsWithResources(commonOpts,
		[]runtime.Object{},
		[]runtime.Object{&user, devEnv},
		&gits.GitFake{CurrentBranch: "job"},
		&gits.FakeProvider{},
		helm_test.NewMockHelmer(),
		resources_test.NewMockInstaller(),
	)

	commonOpts.SetFactory(factory)

	o.UpgradeBootOptions = UpgradeBootOptions{
		CommonOptions: commonOpts,
		Dir:           dir,
	}
}

var clonetests = []struct {
	desc     string
	pEnvName string
	sURL     string
	ns       string
	success  bool
}{
	{"Test should pass - success", "dev", "https://gitlab.com/ankitm123/environment-mikros-cluster-dev.git", "jx", true},
	{"non-dev preview environment - fail", "testing", "https://gitlab.com/ankitm123/environment-mikros-cluster-dev.git", "jx", false},
	{"non-dev ns - fail", "dev", "https://gitlab.com/ankitm123/environment-mikros-cluster-dev.git", "no-jx", false},
	{"missing application url - fail", "dev", "", "jx", false},
}

func TestCloneDevEnvironment(t *testing.T) {
	t.Parallel()
	for _, tt := range clonetests {
		t.Run(tt.desc, func(t *testing.T) {
			o := TestUpgradeBootOptions{}
			o.setup(defaultBootRequirements, tt.pEnvName, tt.sURL, tt.ns)
			err := o.cloneDevEnv()
			if tt.success {
				require.NoError(t, err, "cloned dev")
			} else {
				require.Error(t, err, "could not clone dev")
			}

		})
	}
}

func TestDetermineBootConfigURL(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(defaultBootRequirements, "", "", "")

	requirements, _, err := config.LoadRequirementsConfig(o.UpgradeBootOptions.Dir)
	require.NoError(t, err, "could not get requirements file")
	vs := &requirements.VersionStream

	URL, err := o.determineBootConfigURL(vs.URL)
	require.NoError(t, err, "could not determine boot config URL")
	assert.Equal(t, config.DefaultBootRepository, URL, "DetermineBootConfigURL")
}

func TestRequirementsVersionStream(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(defaultBootRequirements, "", "", "")

	requirements, _, err := config.LoadRequirementsConfig(o.UpgradeBootOptions.Dir)
	require.NoError(t, err, "could not get requirements file")
	vs := &requirements.VersionStream

	assert.Equal(t, "2367726d02b8c", vs.Ref, "RequirementsVersionStream Ref")
	assert.Equal(t, "https://github.com/jenkins-x/jenkins-x-versions.git", vs.URL, "RequirementsVersionStream URL")
}

func TestUpdateVersionStreamRef(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(defaultBootRequirements, "", "", "")

	tmpDir := o.createTmpRequirements(t)
	defer func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err, "could not clean up temp jx-requirements")
	}()

	o.UpgradeBootOptions.Dir = tmpDir
	o.SetGit(gits.NewGitFake())
	err := o.updateVersionStreamRef("22222222")
	require.NoError(t, err, "could not update version stream ref")

	requirements, _, err := config.LoadRequirementsConfig(o.UpgradeBootOptions.Dir)
	require.NoError(t, err, "could not get requirements file")
	vs := &requirements.VersionStream
	assert.Equal(t, "22222222", vs.Ref, "UpdateVersionStreamRef Ref")
}

func TestUpdatePipelineBuilderImage(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(defaultBootRequirements, "", "", "")

	tmpDir, err := ioutil.TempDir("", "")
	defer func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err, "could not clean up temp")
	}()

	o.UpgradeBootOptions.Dir = tmpDir
	from, err := os.Open(filepath.Join("test_data", "upgrade_boot_builders", "jenkins-x-boot-config", "jenkins-x.yml"))
	err = os.MkdirAll(tmpDir, util.DefaultWritePermissions)
	to, err := os.Create(filepath.Join(tmpDir, "jenkins-x.yml"))
	require.NoError(t, err, "unable to create tmp jenkins-x.yml")

	_, err = io.Copy(to, from)
	o.SetGit(gits.NewGitFake())
	resolver := &versionstream.VersionResolver{
		VersionsDir: filepath.Join("test_data", "upgrade_boot_builders", "jenkins-x-versions"),
	}
	err = o.updatePipelineBuilderImage(resolver)
	require.NoError(t, err, "could not update builder image in pipeline")
	data, err := ioutil.ReadFile(to.Name())
	require.Contains(t, string(data), "gcr.io/jenkinsxio/builder-go:1.0.10", "builder version was not correctly updated")
}

func TestUpdateTemplateBuilderImage(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(defaultBootRequirements, "", "", "")

	tmpDir, err := ioutil.TempDir("", "")
	defer func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err, "could not clean up temp")
	}()

	o.UpgradeBootOptions.Dir = tmpDir
	from, err := os.Open(filepath.Join("test_data", "upgrade_boot_builders", "jenkins-x-boot-config", helm.ValuesTemplateFileName))
	err = os.MkdirAll(fmt.Sprintf("%s/env", tmpDir), util.DefaultWritePermissions)
	to, err := os.Create(filepath.Join(tmpDir, fmt.Sprintf("env/%s", helm.ValuesTemplateFileName)))
	require.NoError(t, err, "unable to create tmp template file")

	_, err = io.Copy(to, from)
	o.SetGit(gits.NewGitFake())
	resolver := &versionstream.VersionResolver{
		VersionsDir: filepath.Join("test_data", "upgrade_boot_builders", "jenkins-x-versions"),
	}
	err = o.updateTemplateBuilderImage(resolver)
	require.NoError(t, err, "could not update builder image in template")
	data, err := ioutil.ReadFile(to.Name())
	log.Logger().Infof("**** %s", string(data))
	require.Contains(t, string(data), "gcr.io/jenkinsxio/builder-go:1.0.10", "builder version was not correctly updated")
}

func (o *TestUpgradeBootOptions) createTmpRequirements(t *testing.T) string {
	from, err := os.Open(filepath.Join(o.UpgradeBootOptions.Dir, "jx-requirements.yml"))
	require.NoError(t, err, "unable to open test jx-requirements")

	tmpDir, err := ioutil.TempDir("", "")
	err = os.MkdirAll(tmpDir, util.DefaultWritePermissions)
	to, err := os.Create(filepath.Join(tmpDir, "jx-requirements.yml"))
	require.NoError(t, err, "unable to create tmp jx-requirements")

	_, err = io.Copy(to, from)
	require.NoError(t, err, "unable to copy test jx-requirements to tmp")
	return tmpDir
}

func TestDetermineBootConfigURLAlternative(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(alternativeBootRequirements, "", "", "")

	requirements, _, err := config.LoadRequirementsConfig(o.UpgradeBootOptions.Dir)
	require.NoError(t, err, "could not get requirements file")
	vs := &requirements.VersionStream

	URL, err := o.determineBootConfigURL(vs.URL)
	require.NoError(t, err, "could not determine boot config URL")
	assert.Equal(t, "https://github.com/some-org/some-org-jenkins-x-boot-config.git", URL, "DetermineBootConfigURL")
}

func TestRequirementsVersionStreamAlternative(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(alternativeBootRequirements, "", "", "")

	requirements, _, err := config.LoadRequirementsConfig(o.UpgradeBootOptions.Dir)
	require.NoError(t, err, "could not get requirements file")
	vs := &requirements.VersionStream

	assert.Equal(t, "2367726d02b9c", vs.Ref, "RequirementsVersionStream Ref")
	assert.Equal(t, "https://github.com/some-org/some-org-jenkins-x-versions.git", vs.URL, "RequirementsVersionStream URL")
}

func TestUpdateVersionStreamRefAlternative(t *testing.T) {
	t.Parallel()

	o := TestUpgradeBootOptions{}
	o.setup(alternativeBootRequirements, "", "", "")

	tmpDir := o.createTmpRequirements(t)
	defer func() {
		err := os.RemoveAll(tmpDir)
		require.NoError(t, err, "could not clean up temp jx-requirements")
	}()

	o.UpgradeBootOptions.Dir = tmpDir
	o.SetGit(gits.NewGitFake())
	err := o.updateVersionStreamRef("22222222")
	require.NoError(t, err, "could not update version stream ref")

	requirements, _, err := config.LoadRequirementsConfig(o.UpgradeBootOptions.Dir)
	require.NoError(t, err, "could not get requirements file")
	vs := &requirements.VersionStream

	assert.Equal(t, "22222222", vs.Ref, "UpdateVersionStreamRef Ref")
}
