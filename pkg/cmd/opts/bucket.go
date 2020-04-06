package opts

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud/gke"

	"github.com/jenkins-x/jx/pkg/cloud/buckets"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gocloud.dev/blob"
)

// CreateBucketValues contains the values to create a Bucket on cloud storage
type CreateBucketValues struct {
	Bucket     string
	BucketKind string

	// GKE specific values
	GKEProjectID string
	GKEZone      string
}

// AddCreateBucketFlags adds the CLI arguments to be able to specify to create a new bucket along with any cloud specific parameters
func (cb *CreateBucketValues) AddCreateBucketFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&cb.Bucket, "bucket", "", "", "Specify the name of the bucket to use")
	cmd.Flags().StringVarP(&cb.BucketKind, "bucket-kind", "", "", "The kind of bucket to use like 'gs, s3, azure' etc")
	cmd.Flags().StringVarP(&cb.GKEProjectID, "gke-project-id", "", "", "Google Project ID to use for a new bucket")
	cmd.Flags().StringVarP(&cb.GKEZone, "gke-zone", "", "", "The GKE zone (e.g. us-central1-a) where the new bucket will be created")
}

// IsEmpty returns true if there is no bucket name specified
func (cb *CreateBucketValues) IsEmpty() bool {
	return cb.Bucket == ""
}

// CreateBucket creates a new bucket using the create bucket values and team settings returning the newly created bucket URL
func (o *CommonOptions) CreateBucket(cb *CreateBucketValues, settings *v1.TeamSettings) (string, error) {
	bucketURL, err := buckets.CreateBucketURL(cb.Bucket, cb.BucketKind, settings)
	if err != nil {
		return bucketURL, errors.Wrapf(err, "failed to create the bucket URL for %s", cb.Bucket)
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Second*20)
	bucket, err := blob.Open(ctx, bucketURL)
	if err != nil {
		return bucketURL, errors.Wrapf(err, "failed to open the bucket for %s", bucketURL)
	}

	// lets check if the bucket exists
	iter := bucket.List(nil)
	obj, err := iter.Next(ctx)
	if err != nil {
		if err == io.EOF {
			log.Logger().Infof("bucket %s is empty", bucketURL)
		} else {
			log.Logger().Infof("The bucket %s does not exist yet so lets create it...", util.ColorInfo(bucketURL))
			err = o.CreateBucketFromURL(bucketURL, bucket, cb)
			if err != nil {
				return bucketURL, errors.Wrapf(err, "failed to create the bucket for %s", bucketURL)
			}
		}
	} else {
		log.Logger().Infof("Found item in bucket %s for %s", bucketURL, obj.Key)
	}
	return bucketURL, nil
}

// CreateBucketFromURL creates a bucket if it does not already exist
func (o *CommonOptions) CreateBucketFromURL(bucketURL string, bucket *blob.Bucket, cb *CreateBucketValues) error {
	u, err := url.Parse(bucketURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "gs":
		return o.createGcsBucket(u, bucket, cb)
	default:
		return fmt.Errorf("Cannot create a bucket for provider %s", bucketURL)
	}
}

func (o *CommonOptions) createGcsBucket(u *url.URL, bucket *blob.Bucket, cb *CreateBucketValues) error {
	var err error
	if cb.GKEProjectID == "" {
		if kubeClient, ns, err := o.KubeClientAndDevNamespace(); err == nil {
			if data, err := kube.ReadInstallValues(kubeClient, ns); err == nil && data != nil {
				cb.GKEProjectID = data[kube.ProjectID]
				if cb.GKEZone == "" {
					cb.GKEZone = data[kube.Zone]
				}
			}
		}
	}
	if cb.GKEProjectID == "" {
		cb.GKEProjectID, err = o.GetGoogleProjectID("")
		if err != nil {
			return err
		}
	}

	err = o.RunCommandVerbose(
		"gcloud", "config", "set", "project", cb.GKEProjectID)
	if err != nil {
		return err
	}

	if cb.GKEZone == "" {
		defaultZone := ""
		if cluster, err := cluster.Name(o.Kube()); err == nil && cluster != "" {
			if clusterZone, err := o.GCloud().ClusterZone(cluster); err == nil {
				defaultZone = clusterZone
			}
		}

		cb.GKEZone, err = o.GetGoogleZoneWithDefault(cb.GKEProjectID, defaultZone)
		if err != nil {
			return err
		}
	}

	bucketName := u.Host
	region := gke.GetRegionFromZone(cb.GKEZone)
	err = o.GCloud().CreateBucket(cb.GKEProjectID, bucketName, region)
	if err != nil {
		return errors.Wrapf(err, "creating bucket %s in project %s and region %s", bucketName, cb.GKEProjectID, region)
	}
	return nil
}
