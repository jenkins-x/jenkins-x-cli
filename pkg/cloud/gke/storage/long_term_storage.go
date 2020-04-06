package storage

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// EnableLongTermStorage will take the cluster install values and a provided bucket name and use it / create a new one for gs
func EnableLongTermStorage(gcloud gke.GClouder, installValues map[string]string, providedBucketName string) (string, error) {
	if providedBucketName != "" {
		log.Logger().Infof(util.QuestionAnswer("Configured to use long term storage bucket", providedBucketName))
		return ensureProvidedBucketExists(gcloud, installValues, providedBucketName)
	} else {
		log.Logger().Info("No bucket name provided for long term storage, creating a new one")
		bucketName, installValues := createUniqueBucketName(installValues)
		return createBucket(gcloud, bucketName, installValues)
	}
}

func ensureProvidedBucketExists(gcloud gke.GClouder, installValues map[string]string, providedBucketName string) (string, error) {
	exists, err := gcloud.BucketExists(installValues[kube.ProjectID], providedBucketName)
	if err != nil {
		return "", errors.Wrap(err, "checking if the provided bucket exists")
	}
	if exists {
		return fmt.Sprintf("gs://%s", providedBucketName), nil
	}

	bucketURL, err := createBucket(gcloud, providedBucketName, installValues)
	if err == nil {
		return bucketURL, nil
	}
	log.Logger().Warnf("Attempted to create the bucket %s in the project %s but failed, will now create a "+
		"random bucket", providedBucketName, installValues[kube.ProjectID])

	bucketName, installValues := createUniqueBucketName(installValues)
	return createBucket(gcloud, bucketName, installValues)
}

func createUniqueBucketName(installValues map[string]string) (string, map[string]string) {
	clusterName := installValues[kube.ClusterName]
	bucketName := createUniqueBucketNameForCluster(clusterName)
	return bucketName, installValues
}

func createUniqueBucketNameForCluster(clusterName string) string {
	uuid4, _ := uuid.NewV4()
	bucketName := fmt.Sprintf("%s-lts-%s", clusterName, uuid4.String())
	if len(bucketName) > 60 {
		bucketName = bucketName[:60]
	}
	if strings.HasSuffix(bucketName, "-") {
		bucketName = bucketName[:59]
	}
	return bucketName
}

func createBucket(gcloud gke.GClouder, bucketName string, installValues map[string]string) (string, error) {
	bucketURL := fmt.Sprintf("gs://%s", bucketName)
	infoBucketURL := util.ColorInfo(bucketURL)
	log.Logger().Infof("The bucket %s does not exist so lets create it", infoBucketURL)
	region := gke.GetRegionFromZone(installValues[kube.Zone])
	err := gcloud.CreateBucket(installValues[kube.ProjectID], bucketName, region)
	gcloud.AddBucketLabel(bucketName, gcloud.UserLabel())
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem creating the bucket %s in the GKE Project %s",
			bucketName, installValues[kube.ProjectID])
	}
	return bucketURL, err
}
