// +build unit

package storage

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/stretchr/testify/assert"
)

type mockedS3 struct {
	s3iface.S3API
}

type mockedUploader struct {
	s3manageriface.UploaderAPI
}
type mockedDownloader struct {
	s3manager.Downloader
}

var (
	expectedBucketContents = `this is a test
string to download
from the bucket
`
)

type FakeRequestFailure struct {
	awserr.RequestFailure
}

func (r FakeRequestFailure) HostID() string {
	return ""
}

func (m mockedUploader) Upload(u *s3manager.UploadInput, f ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	return &s3manager.UploadOutput{
		Location: fmt.Sprintf("%s%s", *u.Bucket, *u.Key),
	}, nil
}

func (m mockedDownloader) Download(w io.WriterAt, i *s3.GetObjectInput, f ...func(*s3manager.Downloader)) (int64, error) {
	written, err := w.WriteAt([]byte(expectedBucketContents), 0)
	return int64(written), err
}

func (m mockedS3) HeadBucket(input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error) {
	if *input.Bucket == "bucket_that_exists" {
		return &s3.HeadBucketOutput{}, nil
	}
	return nil, FakeRequestFailure{
		awserr.NewRequestFailure(awserr.New(s3.ErrCodeNoSuchBucket, "", nil), 404, ""),
	}
}

func (m mockedS3) CreateBucket(input *s3.CreateBucketInput) (*s3.CreateBucketOutput, error) {
	return nil, nil
}

func TestAmazonBucketProvider_EnsureBucketIsCreated(t *testing.T) {
	p := AmazonBucketProvider{
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				Region: "us-east-1",
			},
		},
		api: &mockedS3{},
	}

	tests := []struct {
		bucket  string
		message string
	}{
		{
			bucket:  "new_bucket",
			message: "The bucket s3://new_bucket does not exist so lets create it\n",
		},
		{
			bucket:  "bucket_that_exists",
			message: "",
		},
	}
	for _, test := range tests {
		t.Run(test.bucket, func(t *testing.T) {
			message := log.CaptureOutput(func() {
				err := p.EnsureBucketIsCreated("s3://" + test.bucket)
				assert.NoError(t, err)
			})

			assert.Equal(t, test.message, stripansi.Strip(message))
		})
	}
}

func TestAmazonBucketProvider_CreateNewBucketForCluster(t *testing.T) {
	p := AmazonBucketProvider{
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				Region: "us-east-1",
			},
		},
		api: &mockedS3{},
	}

	message := log.CaptureOutput(func() {
		url, err := p.CreateNewBucketForCluster("test-cluster", "test-kind")
		assert.NoError(t, err)
		assert.True(t, strings.HasPrefix(url, "s3://test-cluster-test-kind-"))
	})
	assert.NotEmpty(t, message)

	// Test very long name and trimming of hyphens
	message = log.CaptureOutput(func() {
		longName := strings.Repeat("A", 62)
		url, err := p.CreateNewBucketForCluster(longName+"-cluster", "test-kind")
		assert.NoError(t, err)
		assert.Equal(t, "s3://"+longName, url)
	})
	assert.NotEmpty(t, message)
}

func TestAmazonBucketProvider_s3(t *testing.T) {
	p := AmazonBucketProvider{
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				Region: "us-east-1",
			},
		},
	}

	svc, err := p.s3()
	assert.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestAmazonBucketProvider_s3WithNoRegion(t *testing.T) {
	p := AmazonBucketProvider{
		Requirements: &config.RequirementsConfig{},
	}

	svc, err := p.s3()
	assert.Nil(t, svc)
	assert.Error(t, err)
}

func TestAmazonBucketProvider_UploadFileToBucket(t *testing.T) {
	p := AmazonBucketProvider{
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				Region: "us-east-1",
			},
		},
		uploader: mockedUploader{},
	}
	b := []byte("This is uploaded")
	location, err := p.UploadFileToBucket(bytes.NewReader(b), "output", "bucket")
	assert.NoError(t, err)

	assert.Equal(t, "s3://bucket/output", location, "the returned url should be valid")
}

func TestAmazonBucketProvider_DownloadFileFromBucket(t *testing.T) {
	p := AmazonBucketProvider{
		Requirements: &config.RequirementsConfig{
			Cluster: config.ClusterConfig{
				Region: "us-east-1",
			},
		},
		downloader: mockedDownloader{},
	}
	scanner, err := p.DownloadFileFromBucket("s3://bucket/key")
	assert.NoError(t, err)

	var bucketContent string
	for scanner.Scan() {
		bucketContent += fmt.Sprintln(scanner.Text())
	}

	assert.Equal(t, expectedBucketContents, bucketContent, "the returned contents should be match")
}
