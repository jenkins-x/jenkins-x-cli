// +build unit

package cluster_test

import (
	"testing"

	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/jenkins-x/jx/pkg/kube/cluster"
	kube_test "github.com/jenkins-x/jx/pkg/kube/mocks"
	"github.com/jenkins-x/jx/pkg/kube/vault"
)

func TestGetSimplifiedClusterName(t *testing.T) {
	t.Parallel()
	simpleName := cluster.SimplifiedClusterName("gke_jenkinsx-dev_europe-west1-b_my-cluster-name")

	assert.Equal(t, "my-cluster-name", simpleName)
}

func TestShortClusterName(t *testing.T) {
	kuber := kube_test.NewMockKuber()

	config := api.Config{
		CurrentContext: "myContext",
		Contexts: map[string]*api.Context{
			"myContext": {Cluster: "short-cluster-name"},
		},
	}
	When(kuber.LoadConfig()).ThenReturn(&config, nil, nil)

	clusterName, err := cluster.ShortName(kuber)

	assert.NoError(t, err)
	assert.Equal(t, "short-cluster-na", clusterName)

	config = api.Config{
		CurrentContext: "myContext",
		Contexts: map[string]*api.Context{
			"myContext": {Cluster: "short-cluster-na"},
		},
	}
	When(kuber.LoadConfig()).ThenReturn(&config, nil, nil)

	clusterName, err = cluster.ShortName(kuber)

	assert.NoError(t, err)
	assert.Equal(t, "short-cluster-na", clusterName)

	config = api.Config{
		CurrentContext: "myContext",
		Contexts: map[string]*api.Context{
			"myContext": {Cluster: "short-cluster-na-test"},
		},
	}
	When(kuber.LoadConfig()).ThenReturn(&config, nil, nil)

	clusterName, err = cluster.ShortName(kuber)

	assert.NoError(t, err)
	assert.Equal(t, "short-cluster-na", clusterName)
}

func TestClusterName(t *testing.T) {
	kuber := kube_test.NewMockKuber()

	config := api.Config{
		CurrentContext: "myContext",
		Contexts: map[string]*api.Context{
			"myContext": {Cluster: "my-cluster-name"},
		},
	}
	When(kuber.LoadConfig()).ThenReturn(&config, nil, nil)

	clusterName, err := cluster.Name(kuber)

	assert.NoError(t, err)
	assert.Equal(t, "my-cluster-name", clusterName)
}

func TestSystemVaultNameForCluster(t *testing.T) {
	actual := vault.SystemVaultNameForCluster("jstrachan-kp38")
	assert.Equal(t, "jx-vault-jstrachan-kp3", actual, "system vault name")
}
