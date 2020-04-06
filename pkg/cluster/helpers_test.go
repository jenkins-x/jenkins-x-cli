// +build unit

package cluster_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/cluster/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLockLabel   = "mylabel"
	testFilterLabel = "filter"
)

func TestLockCluster(t *testing.T) {
	t.Parallel()

	clusters := []*cluster.Cluster{
		{
			Name: "alreadyLocked",
			Labels: map[string]string{
				testLockLabel: "owned",
			},
		},
		{
			Name: "excluded",
			Labels: map[string]string{
				testFilterLabel: "exclude",
			},
		},
		{
			Name: "expected",
			Labels: map[string]string{
				testFilterLabel: "include",
			},
		},
	}
	client := fake.NewClient(clusters)
	value, err := cluster.NewLabelValue()
	require.NoError(t, err, "failed to call cluster.NewLabelValue()")

	lockLabels := map[string]string{
		testLockLabel: value,
		"owner":       "TestLockCluster",
	}
	cluster, err := cluster.LockCluster(client, lockLabels, map[string]string{
		testFilterLabel: "include",
	})
	require.NoError(t, err, "failed to call cluster.LockCluster()")
	require.NotNil(t, cluster, "no cluster returned")

	assert.Equal(t, "expected", cluster.Name, "locked cluster name")

}
