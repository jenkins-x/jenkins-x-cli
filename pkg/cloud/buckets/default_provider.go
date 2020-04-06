package buckets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"gocloud.dev/blob"

	// let's immport every provider's blobs so we don't fail when calling the generic library
	_ "gocloud.dev/blob/azureblob"
	_ "gocloud.dev/blob/fileblob"
	_ "gocloud.dev/blob/gcsblob"
	_ "gocloud.dev/blob/s3blob"
)

// LegacyBucketProvider is the default provider for non boot clusters
type LegacyBucketProvider struct {
	gcloud     gke.GClouder
	bucket     *blob.Bucket
	classifier string
}

// CreateNewBucketForCluster is not supported for LegacyBucketProvider
func (LegacyBucketProvider) CreateNewBucketForCluster(clusterName string, bucketKind string) (string, error) {
	return "", fmt.Errorf("CreateNewBucketForCluster not implemented for LegacyBucketProvider")
}

// EnsureBucketIsCreated is not supported for LegacyBucketProvider
func (LegacyBucketProvider) EnsureBucketIsCreated(bucketURL string) error {
	return fmt.Errorf("EnsureBucketIsCreated not implemented for LegacyBucketProvider")
}

// DownloadFileFromBucket is not supported for LegacyBucketProvider
func (LegacyBucketProvider) DownloadFileFromBucket(bucketURL string) (*bufio.Scanner, error) {
	return nil, fmt.Errorf("DownloadFileFromBucket not implemented for LegacyBucketProvider")
}

// UploadFileToBucket uploads a file to the provider specific bucket with the given outputName using the gocloud library
func (p LegacyBucketProvider) UploadFileToBucket(reader io.Reader, outputName string, bucketURL string) (string, error) {
	opts := &blob.WriterOptions{
		ContentType: util.ContentTypeForFileName(outputName),
		Metadata: map[string]string{
			"classification": p.classifier,
		},
	}
	u := ""
	ctx := p.createContext()
	bytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	err = p.bucket.WriteAll(ctx, outputName, bytes, opts)
	if err != nil {
		return u, errors.Wrapf(err, "failed to write to bucket %s", outputName)
	}
	u = util.UrlJoin(bucketURL, outputName)
	return u, nil
}

func (LegacyBucketProvider) createContext() context.Context {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*20)
	return ctx
}

// Initialize initializes and opens a bucket object for the given bucketURL and classifier
func (p *LegacyBucketProvider) Initialize(bucketURL string, classifier string) error {
	ctx, _ := context.WithTimeout(context.Background(), time.Second*20)
	if bucketURL == "" {
		return fmt.Errorf("no BucketURL is configured for the storage location in the TeamSettings")
	}
	bucket, err := blob.Open(ctx, bucketURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open bucket %s", bucketURL)
	}
	p.bucket = bucket
	p.classifier = classifier
	return nil
}

// NewLegacyBucketProvider create a new provider for non supported providers or non boot clusters
func NewLegacyBucketProvider() Provider {
	return &LegacyBucketProvider{
		gcloud: &gke.GCloud{},
	}
}
