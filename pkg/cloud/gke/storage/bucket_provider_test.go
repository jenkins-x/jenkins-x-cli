// +build unit

package storage

import (
	"strings"
	"testing"

	gke_test "github.com/jenkins-x/jx/pkg/cloud/gke/mocks"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
)

func TestCreateNewBucketForClusterWithLongClusterNameAndDashAtCharacterSixty(t *testing.T) {
	p := GKEBucketProvider{
		gcloud: gke_test.NewMockGClouder(),
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				ProjectID: "project",
			},
		},
	}

	pegomock.When(p.gcloud.BucketExists(pegomock.AnyString(), pegomock.AnyString())).ThenReturn(true, nil)

	bucketName, err := p.CreateNewBucketForCluster("rrehhhhhhhhhhhhhhhhhhhhhhhhhj3j3k2kwkdkjdbiwabduwabduoawbdb-dbwdbwaoud", "logs")
	assert.NoError(t, err)
	assert.NotNil(t, bucketName, "it should always generate a name")
	assert.False(t, strings.HasSuffix(bucketName, "-"), "the bucket can't end with a dash")
}

func TestCreateNewBucketForClusterWithSmallClusterName(t *testing.T) {
	p := GKEBucketProvider{
		gcloud: gke_test.NewMockGClouder(),
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				ProjectID: "project",
			},
		},
	}

	pegomock.When(p.gcloud.BucketExists(pegomock.AnyString(), pegomock.AnyString())).ThenReturn(true, nil)

	bucketName := createUniqueBucketNameForCluster("cluster")
	assert.NotNil(t, bucketName, "it should always generate a name")
	assert.False(t, strings.HasSuffix(bucketName, "-"), "the bucket can't end with a dash")
}
