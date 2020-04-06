// +build unit

package auth_test

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/secreturl/fakevault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

func TestConfigMapVaultConfigSaver(t *testing.T) {
	ns := "test"
	secretName := "gitAuth.yaml" // #nosec
	authFile := path.Join("test_data", "configmap_vault_auth.yaml")
	data, err := ioutil.ReadFile(authFile)
	require.NoError(t, err)

	var expectedAuthConfig auth.AuthConfig
	err = yaml.Unmarshal(data, &expectedAuthConfig)
	require.NoError(t, err)

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-auth",
			Namespace: ns,
			Labels: map[string]string{
				"jenkins.io/config-type": "auth",
			},
		},
		Data: map[string]string{
			secretName: string(data),
		},
	}
	k8sClient := fake.NewSimpleClientset(namespace, config)
	configMapInterface := k8sClient.CoreV1().ConfigMaps(ns)

	vaultClient := fakevault.NewFakeClient()
	_, err = vaultClient.Write("test-cluster/pipelineUser", map[string]interface{}{"token": "test"})
	require.NoError(t, err)
	expectedAuthConfig.Servers[0].Users[0].ApiToken = "test"

	handler := auth.NewConfigMapVaultConfigHandler(secretName, configMapInterface, vaultClient)
	authConfig, err := handler.LoadConfig()

	assert.NoError(t, err, "auth config should be loaded")
	assert.NotNil(t, authConfig, "auth config should be set")
	assert.EqualValues(t, expectedAuthConfig, *authConfig)
}

func TestConfigMapVaultConfigSaverWithoutVaultURIs(t *testing.T) {
	ns := "test"
	secretName := "gitAuth.yaml" // #nosec
	authFile := path.Join("test_data", "configmap_withoutvault_auth.yaml")
	data, err := ioutil.ReadFile(authFile)
	require.NoError(t, err)

	var expectedAuthConfig auth.AuthConfig
	err = yaml.Unmarshal(data, &expectedAuthConfig)
	require.NoError(t, err)

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-auth",
			Namespace: ns,
			Labels: map[string]string{
				"jenkins.io/config-type": "auth",
			},
		},
		Data: map[string]string{
			secretName: string(data),
		},
	}
	k8sClient := fake.NewSimpleClientset(namespace, config)
	configMapInterface := k8sClient.CoreV1().ConfigMaps(ns)
	vaultClient := fakevault.NewFakeClient()

	handler := auth.NewConfigMapVaultConfigHandler(secretName, configMapInterface, vaultClient)
	authConfig, err := handler.LoadConfig()

	assert.NoError(t, err, "auth config should be loaded")
	assert.NotNil(t, authConfig, "auth config should be set")
	assert.EqualValues(t, expectedAuthConfig, *authConfig)
}

func TestConfigMapVaultConfigSaverWithoutConfigMapLabel(t *testing.T) {
	ns := "test"
	secretName := "gitAuth.yaml" // #nosec
	authFile := path.Join("test_data", "configmap_vault_auth.yaml")
	data, err := ioutil.ReadFile(authFile)
	require.NoError(t, err)

	var expectedAuthConfig auth.AuthConfig
	err = yaml.Unmarshal(data, &expectedAuthConfig)
	require.NoError(t, err)

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-auth",
			Namespace: ns,
		},
		Data: map[string]string{
			secretName: string(data),
		},
	}
	k8sClient := fake.NewSimpleClientset(namespace, config)
	configMapInterface := k8sClient.CoreV1().ConfigMaps(ns)

	vaultClient := fakevault.NewFakeClient()
	_, err = vaultClient.Write("test-cluster/pipelineUser", map[string]interface{}{"token": "test"})
	require.NoError(t, err)
	expectedAuthConfig.Servers[0].Users[0].ApiToken = "test"

	handler := auth.NewConfigMapVaultConfigHandler(secretName, configMapInterface, vaultClient)
	authConfig, err := handler.LoadConfig()

	assert.Error(t, err, "auth config should not be found")
	assert.Nil(t, authConfig)
}

func TestConfigMapVaultConfigSaverWithoutConfigMapData(t *testing.T) {
	ns := "test"
	secretName := "gitAuth.yaml" // #nosec
	authFile := path.Join("test_data", "configmap_vault_auth.yaml")
	data, err := ioutil.ReadFile(authFile)
	require.NoError(t, err)

	var expectedAuthConfig auth.AuthConfig
	err = yaml.Unmarshal(data, &expectedAuthConfig)
	require.NoError(t, err)

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-auth",
			Namespace: ns,
			Labels: map[string]string{
				"jenkins.io/config-type": "auth",
			},
		},
		Data: map[string]string{},
	}
	k8sClient := fake.NewSimpleClientset(namespace, config)
	configMapInterface := k8sClient.CoreV1().ConfigMaps(ns)

	vaultClient := fakevault.NewFakeClient()
	_, err = vaultClient.Write("test-cluster/pipelineUser", map[string]interface{}{"token": "test"})
	require.NoError(t, err)
	expectedAuthConfig.Servers[0].Users[0].ApiToken = "test"

	handler := auth.NewConfigMapVaultConfigHandler(secretName, configMapInterface, vaultClient)
	authConfig, err := handler.LoadConfig()

	assert.Error(t, err, "auth config should not be found")
	assert.Nil(t, authConfig)
}

func TestConfigMapVaultConfigSaverWithCorrupted(t *testing.T) {
	ns := "test"
	secretName := "gitAuth.yaml" // #nosec
	authFile := path.Join("test_data", "configmap_vault_corrupted_auth.yaml")
	data, err := ioutil.ReadFile(authFile)
	require.NoError(t, err)

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	}
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "jx-auth",
			Namespace: ns,
			Labels: map[string]string{
				"jenkins.io/config-type": "auth",
			},
		},
		Data: map[string]string{
			secretName: string(data),
		},
	}
	k8sClient := fake.NewSimpleClientset(namespace, config)
	configMapInterface := k8sClient.CoreV1().ConfigMaps(ns)

	vaultClient := fakevault.NewFakeClient()
	_, err = vaultClient.Write("test-cluster/pipelineUser", map[string]interface{}{"token": "test"})
	require.NoError(t, err)

	handler := auth.NewConfigMapVaultConfigHandler(secretName, configMapInterface, vaultClient)
	authConfig, err := handler.LoadConfig()

	assert.Error(t, err, "auth config should not be found")
	assert.Nil(t, authConfig)
}
