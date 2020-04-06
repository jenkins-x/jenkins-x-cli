// +build unit

package eks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	eksctltest "github.com/jenkins-x/jx/pkg/cloud/amazon/eksctl/mocks"

	"github.com/jenkins-x/jx/pkg/cluster"

	"github.com/petergtz/pegomock"

	"github.com/jenkins-x/jx/pkg/cloud/amazon/testutils"
)

func TestAWSClusterClient_List(t *testing.T) {
	p := testutils.NewMockProvider("", "")
	pegomock.When(p.EKS().ListClusters()).ThenReturn([]*cluster.Cluster{
		{
			Name:   "cluster-1",
			Status: "ACTIVE",
			Labels: map[string]string{
				"tag1": "value1",
			},
			Location: "this.is.an/endpoint",
		},
	}, nil)
	c := awsClusterClient{
		Provider: p,
	}

	clusters, err := c.List()
	assert.NoError(t, err)

	assert.Len(t, clusters, 1, "there must be at least one returned cluster from List")
	if len(clusters) > 0 {
		cluster := clusters[0]
		assert.Equal(t, "cluster-1", cluster.Name)
		assert.Equal(t, "ACTIVE", cluster.Status)
		assert.Equal(t, "this.is.an/endpoint", cluster.Location)
		assert.Equal(t, map[string]string{
			"tag1": "value1",
		}, cluster.Labels)
	}
}

func TestAWSClusterClient_Get(t *testing.T) {
	p := testutils.NewMockProvider("", "")
	pegomock.When(p.EKS().DescribeCluster(pegomock.EqString("cluster-1"))).ThenReturn(
		&cluster.Cluster{
			Name: "cluster-1",
			Labels: map[string]string{
				"tag1": "value1",
			},
			Status:   "ACTIVE",
			Location: "this.is.an/endpoint",
		}, "", nil,
	)

	c := awsClusterClient{
		Provider: p,
	}

	obtainedCluster, err := c.Get("cluster-1")
	assert.NoError(t, err)

	assert.Equal(t, "cluster-1", obtainedCluster.Name)
	assert.Equal(t, "ACTIVE", obtainedCluster.Status)
	assert.Equal(t, "this.is.an/endpoint", obtainedCluster.Location)
	assert.Equal(t, map[string]string{
		"tag1": "value1",
	}, obtainedCluster.Labels)
}

func TestAWSClusterClient_ListFilter(t *testing.T) {
	p := testutils.NewMockProvider("", "")
	pegomock.When(p.EKS().ListClusters()).ThenReturn([]*cluster.Cluster{
		{
			Name: "cluster-1",
			Labels: map[string]string{
				"tag1": "value1",
			},
			Status:   "ACTIVE",
			Location: "this.is.an/endpoint",
		},
		{
			Name: "cluster-2",
			Labels: map[string]string{
				"IWANT": "THIS_CLUSTER",
			},
			Status:   "ACTIVE",
			Location: "this.is.an/endpoint",
		},
		{
			Name:     "cluster-3",
			Labels:   map[string]string{},
			Status:   "ACTIVE",
			Location: "this.is.an/endpoint",
		},
	}, nil)

	c := awsClusterClient{
		Provider: p,
	}

	clusters, err := c.ListFilter(map[string]string{
		"IWANT": "THIS_CLUSTER",
	})
	assert.NoError(t, err)

	assert.Len(t, clusters, 1, "it must return only one cluster matching tags")
	if len(clusters) > 0 {
		obtainedCluster := clusters[0]
		assert.Equal(t, "cluster-2", obtainedCluster.Name)
	}
}

func TestAWSClusterClient_Delete(t *testing.T) {
	pegomock.RegisterMockTestingT(t)
	p := testutils.NewMockProvider("", "")

	c := awsClusterClient{
		Provider: p,
	}
	cl := &cluster.Cluster{
		Name: "cluster1",
	}
	err := c.Delete(cl)
	assert.NoError(t, err)
	p.EKSCtl().(*eksctltest.MockEKSCtl).VerifyWasCalledOnce().DeleteCluster(cl)
}
