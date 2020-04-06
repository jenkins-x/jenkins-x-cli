// +build unit

package kube_test

import (
	"errors"
	"testing"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	mock "k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

func buildConfigMap(jxInstallConfig *kube.JXInstallConfig, ns string) *v1.ConfigMap {
	config := util.ToStringMapStringFromStruct(jxInstallConfig)
	cm := &v1.ConfigMap{
		Data: config,
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      kube.ConfigMapNameJXInstallConfig,
			Namespace: ns,
		},
	}
	return cm
}

func TestToStringMapStringFromStructJXInstallConfig(t *testing.T) {
	t.Parallel()
	jxInstallConfig := &kube.JXInstallConfig{
		Server: "derek.zoolander.reallygoodlooking.com",
		CA:     []byte{0x49, 0x43, 0x41, 0x54},
	}

	m := util.ToStringMapStringFromStruct(jxInstallConfig)

	assert.Equal(t, 3, len(m))
	assert.Equal(t, "derek.zoolander.reallygoodlooking.com", m["server"])
	assert.Equal(t, "ICAT", m["ca.crt"])
}

func TestSaveAsConfigMapExistingCM(t *testing.T) {
	t.Parallel()
	ns := "my-namespace"
	jxInstallConfig := &kube.JXInstallConfig{
		Server: "derek.zoolander.reallygoodlooking.com",
		CA:     []byte{0x49, 0x43, 0x41, 0x54},
	}

	// Build a config map from our JXInstallConfig struct and a ns string
	cm := buildConfigMap(jxInstallConfig, ns)

	// Setup mock Kubernetes client api (pass objects to set as existing resources)
	kubernetesInterface := mock.NewSimpleClientset(cm)

	// Run our method
	_, err := kube.SaveAsConfigMap(kubernetesInterface, kube.ConfigMapNameJXInstallConfig, ns, jxInstallConfig)

	// Get Kubernetes client api actions
	actions := kubernetesInterface.Actions()

	assert.NoError(t, err)
	assert.Equal(t, 2, len(actions))
	assert.Equal(t, "get", actions[0].GetVerb())
	assert.Equal(t, ns, actions[0].GetNamespace())
	assert.Equal(t, "update", actions[1].GetVerb())
	assert.Equal(t, ns, actions[1].GetNamespace())
}

func TestSaveAsConfigMapNoCM(t *testing.T) {
	t.Parallel()
	ns := "my-namespace"
	jxInstallConfig := &kube.JXInstallConfig{
		Server: "derek.zoolander.reallygoodlooking.com",
		CA:     []byte{0x49, 0x43, 0x41, 0x54},
	}

	kubernetesInterface := mock.NewSimpleClientset()

	_, err := kube.SaveAsConfigMap(kubernetesInterface, kube.ConfigMapNameJXInstallConfig, ns, jxInstallConfig)

	actions := kubernetesInterface.Actions()

	assert.NoError(t, err)
	assert.Equal(t, 2, len(actions))
	assert.Equal(t, "get", actions[0].GetVerb())
	assert.Equal(t, ns, actions[0].GetNamespace())
	assert.Equal(t, "create", actions[1].GetVerb())
	assert.Equal(t, ns, actions[1].GetNamespace())
}

func TestSaveAsConfigMapCreateError(t *testing.T) {
	t.Parallel()
	ns := "my-namespace"

	jxInstallConfig := &kube.JXInstallConfig{
		Server: "derek.zoolander.reallygoodlooking.com",
		CA:     []byte{0x49, 0x43, 0x41, 0x54},
	}

	kubernetesInterface := mock.NewSimpleClientset()

	// Note Using AddReactor will not work here as reactors are already registered through NewSimpleClientset()
	// Prepend reactor to catch action and perform custom logic. In this case return an error on configmap create.
	kubernetesInterface.PrependReactor("create", "configmaps", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("KABOOM")
	})

	_, err := kube.SaveAsConfigMap(kubernetesInterface, kube.ConfigMapNameJXInstallConfig, ns, jxInstallConfig)

	actions := kubernetesInterface.Actions()

	assert.Error(t, err)
	assert.Equal(t, 2, len(actions))
	assert.Equal(t, "get", actions[0].GetVerb())
	assert.Equal(t, ns, actions[0].GetNamespace())
	assert.Equal(t, "create", actions[1].GetVerb())
	assert.Equal(t, ns, actions[1].GetNamespace())
}

func TestSaveAsConfigMapUpdateError(t *testing.T) {
	t.Parallel()
	ns := "my-namespace"

	jxInstallConfig := &kube.JXInstallConfig{
		Server: "derek.zoolander.reallygoodlooking.com",
		CA:     []byte{0x49, 0x43, 0x41, 0x54},
	}

	cm := buildConfigMap(jxInstallConfig, ns)

	kubernetesInterface := mock.NewSimpleClientset(cm)

	kubernetesInterface.PrependReactor("update", "configmaps", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("KABOOM")
	})

	_, err := kube.SaveAsConfigMap(kubernetesInterface, kube.ConfigMapNameJXInstallConfig, ns, jxInstallConfig)

	actions := kubernetesInterface.Actions()

	assert.Error(t, err)
	assert.Equal(t, 2, len(actions))
	assert.Equal(t, "get", actions[0].GetVerb())
	assert.Equal(t, ns, actions[0].GetNamespace())
	assert.Equal(t, "update", actions[1].GetVerb())
	assert.Equal(t, ns, actions[1].GetNamespace())
}
