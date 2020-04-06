// +build unit

package helmfile

import (
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/core/v1"

	mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	helmfile2 "github.com/jenkins-x/jx/pkg/helmfile"
	kube_test "github.com/jenkins-x/jx/pkg/kube/mocks"
	. "github.com/petergtz/pegomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestDedupeRepositories(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-applications-config")
	assert.NoError(t, err, "should create a temporary config dir")

	o := &CreateHelmfileOptions{
		outputDir:     tempDir,
		dir:           "test_data",
		CreateOptions: *getCreateOptions(),
	}
	err = o.Run()
	assert.NoError(t, err)

	h, err := loadHelmfile(path.Join(tempDir, "apps"))
	assert.NoError(t, err)

	// assert there are 3 repos and not 4 as one of them in the jx-applications.yaml is a duplicate
	assert.Equal(t, 3, len(h.Repositories))

}

func TestExtraAppValues(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-applications-config")
	assert.NoError(t, err, "should create a temporary config dir")

	o := &CreateHelmfileOptions{
		outputDir:     tempDir,
		dir:           path.Join("test_data", "extra-values"),
		CreateOptions: *getCreateOptions(),
	}
	err = o.Run()
	assert.NoError(t, err)

	h, err := loadHelmfile(path.Join(tempDir, "apps"))
	assert.NoError(t, err)

	// assert we added the local values.yaml for the velero app
	assert.Equal(t, "velero/values.yaml", h.Releases[0].Values[0])

}

func TestExtraFlagValues(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-applications-config")
	assert.NoError(t, err, "should create a temporary config dir")

	o := &CreateHelmfileOptions{
		outputDir:     tempDir,
		dir:           path.Join("test_data"),
		valueFiles:    []string{"foo/bar.yaml"},
		CreateOptions: *getCreateOptions(),
	}
	err = o.Run()
	assert.NoError(t, err)

	h, err := loadHelmfile(path.Join(tempDir, "apps"))
	assert.NoError(t, err)

	// assert we added the values file passed in as a CLI flag
	assert.Equal(t, "foo/bar.yaml", h.Releases[0].Values[0])

}

func TestCreateNamespaceChart(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-applications-config")
	assert.NoError(t, err, "should create a temporary config dir")

	o := &CreateHelmfileOptions{
		outputDir:     tempDir,
		dir:           path.Join("test_data", "create-namespace-chart"),
		valueFiles:    []string{"foo/bar.yaml"},
		CreateOptions: *getCreateOptions(),
	}
	err = o.Run()
	assert.NoError(t, err)

	h, err := loadHelmfile(path.Join(tempDir, "apps"))
	assert.NoError(t, err)

	exists, err := util.FileExists(path.Join(tempDir, "apps", "generated", "foo", "values.yaml"))
	assert.True(t, exists, "generated namespace values file not found")

	// assert we added the values file passed in as a CLI flag
	assert.Equal(t, 2, len(h.Releases), "should have two charts, one for the app and a second added to create the missing namespace")

	for _, release := range h.Releases {
		if release.Name == "velero" {
			assert.Equal(t, "foo/bar.yaml", release.Values[0])
		} else {
			assert.Equal(t, "namespace-foo", release.Name)
			assert.Equal(t, path.Join("generated", "foo", "values.yaml"), release.Values[0])
		}
	}

}

func TestSystem(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "test-applications-config")
	assert.NoError(t, err, "should create a temporary config dir")

	o := &CreateHelmfileOptions{
		outputDir:     tempDir,
		dir:           path.Join("test_data", "system"),
		CreateOptions: *getCreateOptions(),
	}
	err = o.Run()
	assert.NoError(t, err)

	appHelmfile, err := loadHelmfile(path.Join(tempDir, "apps"))
	assert.NoError(t, err)

	systemHelmfile, err := loadHelmfile(path.Join(tempDir, "system"))
	assert.NoError(t, err)

	// assert we added the local values.yaml for the velero app
	assert.Equal(t, "velero", appHelmfile.Releases[0].Name)
	assert.Equal(t, "cert-manager", systemHelmfile.Releases[0].Name)
}

func loadHelmfile(dir string) (*helmfile2.HelmState, error) {

	fileName := helmfile
	if dir != "" {
		fileName = filepath.Join(dir, helmfile)
	}

	exists, err := util.FileExists(fileName)
	if err != nil || !exists {
		return nil, errors.Errorf("no %s found in directory %s", fileName, dir)
	}

	config := &helmfile2.HelmState{}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return config, fmt.Errorf("failed to load file %s due to %s", fileName, err)
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal YAML file %s due to %s", fileName, err)
	}

	return config, err
}

func getCreateOptions() *options.CreateOptions {
	// default jx namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "jx",
		},
	}
	// mock factory
	factory := mocks.NewMockFactory()
	// mock Kubernetes interface
	kubeInterface := fake.NewSimpleClientset(ns)

	// Override CreateKubeClient to return mock Kubernetes interface
	When(factory.CreateKubeClient()).ThenReturn(kubeInterface, "jx", nil)
	commonOpts := opts.NewCommonOptionsWithFactory(factory)

	helmer := helm_test.NewMockHelmer()
	kuber := kube_test.NewMockKuber()

	commonOpts.SetHelm(helmer)
	commonOpts.SetKube(kuber)

	return &options.CreateOptions{
		CommonOptions: &commonOpts,
	}
}
