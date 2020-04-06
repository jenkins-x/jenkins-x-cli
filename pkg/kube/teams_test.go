// +build unit

package kube

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	kube_mocks "k8s.io/client-go/kubernetes/fake"
)

func TestGetAdminNamespace(t *testing.T) {
	t.Parallel()
	namespace := `apiVersion: v1
kind: Namespace
metadata:
  annotations:
    jenkins-x.io/created-by: Jenkins X
    jenkins-x.io/admin-namespace: admin-namespace
  labels:
    env: dev
    team: myteam
  name: myteam
status:
  phase: Active`

	client, err := createMockClient(namespace)
	require.NoError(t, err, "failed to create mock client")
	adminNamespace, err := GetAdminNamespace(client, "myteam")
	assert.NoError(t, err, "GetAdminNamespace should not error")
	assert.Equal(t, "admin-namespace", adminNamespace)
}

func TestGetAdminNamespaceDefaulted(t *testing.T) {
	t.Parallel()
	namespace := `apiVersion: v1
kind: Namespace
metadata:
  annotations:
    jenkins-x.io/created-by: Jenkins X
  labels:
    env: dev
    team: myteam
  name: myteam
status:
  phase: Active`

	client, err := createMockClient(namespace)
	require.NoError(t, err, "failed to create mock client")
	adminNamespace, err := GetAdminNamespace(client, "myteam")
	assert.NoError(t, err, "error occurred calling GetAdminNamespace")
	assert.Equal(t, "myteam", adminNamespace)
}

// TestGetAdminNamespaceOnNonTeam tests that when there are no annotations the code does not blow up.
// there should always be one annotation "jenkins-x.io/created-by: Jenkins X" for a team but better to not blow up
func TestGetAdminNamespaceOnNonTeam(t *testing.T) {

	t.Parallel()
	namespace := `apiVersion: v1
kind: Namespace
metadata:
  name: myteam
status:
  phase: Active`

	client, err := createMockClient(namespace)
	require.NoError(t, err, "failed to create mock client")
	adminNamespace, err := GetAdminNamespace(client, "myteam")
	assert.NoError(t, err, "error occurred calling GetAdminNamespace")
	assert.Equal(t, "myteam", adminNamespace)
}

func TestSetAdminNamespace(t *testing.T) {
	t.Parallel()
	namespace := `apiVersion: v1
kind: Namespace
metadata:
  annotations:
    jenkins-x.io/created-by: Jenkins X
  labels:
    env: dev
    team: myteam
  name: myteam
status:
  phase: Active`
	client, err := createMockClient(namespace)
	require.NoError(t, err, "failed to create mock client")
	err = SetAdminNamespace(client, "myteam", "my-admin-namespace")
	assert.NoError(t, err, "error occurred calling SetAdminNamespace")
	adminNs, err := GetAdminNamespace(client, "myteam")
	assert.NoError(t, err, "error occurred calling GetAdminNamespace")
	assert.Equal(t, "my-admin-namespace", adminNs)
}

func createMockClient(objDef string) (kubernetes.Interface, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode([]byte(objDef), nil, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "could not decode data")
	}

	mockKubeClient := kube_mocks.NewSimpleClientset(obj)
	return mockKubeClient, nil
}
