// +build unit

package fake

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFakeFactory(t *testing.T) {
	f := NewFakeFactory()
	client, ns, err := f.CreateKubeClient()
	require.NoError(t, err, "CreateKubeClient() failed")
	assert.Equal(t, "jx", ns, "namespace")
	assert.NotNil(t, client, "client")

	namespaces, err := client.CoreV1().Namespaces().List(metav1.ListOptions{})
	require.NoError(t, err, "Namespaces().List() failed")
	require.NotNil(t, namespaces, "namespaces")
	assert.Equal(t, 0, len(namespaces.Items), "namespaces")

}
