// +build unit

package kube_test

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/auth"
	v1fake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/secreturl/fakevault"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/yaml"
)

const serviceURL = "https://github.beescloud.com"
const secretName = kube.SecretJenkinsPipelineGitCredentials + "github-ghe"
const serviceKind = "github"

func createSecret(secretName string, labels map[string]string, annotations map[string]string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:                       secretName,
			DeletionGracePeriodSeconds: nil,
			Labels:                     labels,
			Annotations:                annotations,
		},
	}
}

func TestGitServiceKindFromSecrets(t *testing.T) {
	t.Parallel()
	secret := createSecret(secretName,
		map[string]string{
			"jenkins.io/service-kind": serviceKind,
		},
		map[string]string{
			"jenkins.io/url": serviceURL,
		})
	kubeClient := fake.NewSimpleClientset(secret)

	foundServiceKind, err := kube.GetServiceKindFromSecrets(kubeClient, "", serviceURL)

	assert.NoError(t, err, "should find a service kind without any error")
	assert.Equal(t, serviceKind, foundServiceKind, "should find a service kind equal with '%s'", serviceKind)
}

func TestGitServiceKindFromSecretsWithMissingKind(t *testing.T) {
	t.Parallel()
	secret := createSecret("jx-pipeline-git",
		map[string]string{
			"jenkins.io/kind":         "git",
			"jenkins.io/service-kind": "",
		},
		map[string]string{
			"jenkins.io/url": serviceURL,
		})
	ns := "jx"
	secret.Namespace = ns
	kubeClient := fake.NewSimpleClientset(secret)
	jxClient := v1fake.NewSimpleClientset()

	foundServiceKind, err := kube.GetGitServiceKind(jxClient, kubeClient, ns, nil, serviceURL)

	t.Logf("found service kind %s for URL %s\n\n", foundServiceKind, serviceURL)

	assert.NoError(t, err, "should find a service kind without any error")
	assert.Equal(t, serviceKind, foundServiceKind, "should find a service kind equal with '%s'", serviceKind)
}

func TestGitServiceKindFromClusterAuthConfig(t *testing.T) {
	t.Parallel()

	ns := "jx"
	expectedServiceKind := "bitbucketserver"
	bbsServerURL := "https://bitbucket.example.com"

	authFile := path.Join("test_data", "git_service_kind", "configmap_vault_auth.yaml")
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
	kubeClient := fake.NewSimpleClientset(namespace, config)
	jxClient := v1fake.NewSimpleClientset()
	configMapInterface := kubeClient.CoreV1().ConfigMaps(ns)

	vaultClient := fakevault.NewFakeClient()
	_, err = vaultClient.Write("test-cluster/pipelineUser", map[string]interface{}{"token": "test"})
	require.NoError(t, err)
	expectedAuthConfig.Servers[0].Users[0].ApiToken = "test"

	handler := auth.NewConfigMapVaultConfigHandler(secretName, configMapInterface, vaultClient)
	authConfig, err := handler.LoadConfig()

	assert.NoError(t, err, "auth config should be loaded")
	assert.NotNil(t, authConfig, "auth config should be set")
	assert.EqualValues(t, expectedAuthConfig, *authConfig)

	foundServiceKind, err := kube.GetGitServiceKind(jxClient, kubeClient, ns, authConfig, bbsServerURL)

	assert.NoError(t, err, "should find a service kind without any error")
	assert.Equal(t, expectedServiceKind, foundServiceKind, "should find a service kind equal with '%s'", expectedServiceKind)
}

func TestGitServiceKindFromSecretsWithoutURL(t *testing.T) {
	t.Parallel()
	secret := createSecret(secretName,
		map[string]string{
			"jenkins.io/service-kind": serviceKind,
		},
		nil)

	kubeClient := fake.NewSimpleClientset(secret)
	foundServiceKind, err := kube.GetServiceKindFromSecrets(kubeClient, "", serviceURL)

	assert.Error(t, err, "should not found a service kind")
	assert.Equal(t, "", foundServiceKind, "should return no service kind")
}

func TestGitServiceKindFromSecretsWithoutKindLabel(t *testing.T) {
	t.Parallel()
	secret := createSecret(secretName,
		map[string]string{
			"jenkins.io/service-kind": "test",
		},
		map[string]string{
			"jenkins.io/url": "test",
		})
	kubeClient := fake.NewSimpleClientset(secret)

	foundServiceKind, err := kube.GetServiceKindFromSecrets(kubeClient, "", serviceURL)

	assert.Error(t, err, "should not found a service kind")
	assert.Equal(t, "", foundServiceKind, "should return no service kind")
}
