// +build integration

package kube_test

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLoadPodTemplatest(t *testing.T) {
	originalKubeCfg, tempKubeCfg, err := testhelpers.CreateTestKubeConfigDir()
	assert.NoError(t, err)
	defer func() {
		err := testhelpers.CleanupTestKubeConfigDir(originalKubeCfg, tempKubeCfg)
		assert.NoError(t, err)
	}()
	testData := path.Join("test_data", "load_pod_templates")
	assert.DirExists(t, testData)

	files, err := ioutil.ReadDir(testData)
	assert.NoError(t, err)

	ns := "jx"

	runtimeObjects := []runtime.Object{}

	for _, f := range files {
		if !f.IsDir() {
			name := f.Name()
			srcFile := filepath.Join(testData, name)
			data, err := ioutil.ReadFile(srcFile)
			require.NoError(t, err, "failed to read file %s", srcFile)

			cm := &corev1.ConfigMap{}
			err = yaml.Unmarshal(data, cm)
			require.NoError(t, err, "failed to unmarshal file %s", srcFile)

			require.NotEqual(t, "", cm.Name, "file %s contains a ConfigMap with no name", srcFile)
			cm.Namespace = ns
			runtimeObjects = append(runtimeObjects, cm)
		}
	}

	kubeClient := fake.NewSimpleClientset(runtimeObjects...)

	podTemplates, err := kube.LoadPodTemplates(kubeClient, ns)
	require.NoError(t, err, "failed to load pod templates")

	for _, name := range []string{"gradle", "maven"} {
		assert.NotNil(t, podTemplates[name], "no pod template found for key %s", name)
	}

	assert.Equal(t, 2, len(podTemplates), "size of loaded pod template map")
}
