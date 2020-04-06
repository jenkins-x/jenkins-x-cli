package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/url"
	"strings"

	session2 "github.com/jenkins-x/jx/pkg/cloud/amazon/session"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/jenkins-x/jx/pkg/cloud/buckets"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// AmazonBucketProvider the bucket provider for AWS
type AmazonBucketProvider struct {
	Requirements *config.RequirementsConfig
	api          s3iface.S3API
	uploader     s3manageriface.UploaderAPI
	downloader   s3manageriface.DownloaderAPI
}

func (b AmazonBucketProvider) createAWSSession() (*session.Session, error) {
	region := b.Requirements.Cluster.Region
	if region == "" {
		return nil, errors.New("requirements do not specify a cluster region")
	}
	sess, err := session2.NewAwsSession("", region)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create AWS session")
	}
	return sess, nil
}

func (b *AmazonBucketProvider) s3() (s3iface.S3API, error) {
	if b.api != nil {
		return b.api, nil
	}
	sess, err := b.createAWSSession()
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem creating the s3 API interface")
	}
	b.api = s3.New(sess)

	return b.api, nil
}

func (b *AmazonBucketProvider) s3ManagerDownloader() (s3manageriface.DownloaderAPI, error) {
	if b.downloader != nil {
		return b.downloader, nil
	}
	sess, err := b.createAWSSession()
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem creating the s3ManagerDownloader")
	}
	b.downloader = s3manager.NewDownloader(sess)
	return b.downloader, nil
}

func (b *AmazonBucketProvider) s3ManagerUploader() (s3manageriface.UploaderAPI, error) {
	if b.uploader != nil {
		return b.uploader, nil
	}
	sess, err := b.createAWSSession()
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem creating the s3ManagerUploader")
	}
	b.uploader = s3manager.NewUploader(sess)
	return b.uploader, nil
}

// CreateNewBucketForCluster creates a new dynamic bucket
func (b *AmazonBucketProvider) CreateNewBucketForCluster(clusterName string, bucketKind string) (string, error) {
	uuid4, _ := uuid.NewV4()
	bucketName := fmt.Sprintf("%s-%s-%s", clusterName, bucketKind, uuid4.String())

	// Max length is 63, https://docs.aws.amazon.com/AmazonS3/latest/dev/BucketRestrictions.html
	if len(bucketName) > 63 {
		bucketName = bucketName[:63]
	}
	bucketName = strings.TrimRight(bucketName, "-")
	bucketURL := "s3://" + bucketName
	err := b.EnsureBucketIsCreated(bucketURL)
	if err != nil {
		return bucketURL, errors.Wrapf(err, "failed to create bucket %s", bucketURL)
	}

	return bucketURL, nil
}

// EnsureBucketIsCreated ensures the bucket URL is created
func (b *AmazonBucketProvider) EnsureBucketIsCreated(bucketURL string) error {
	svc, err := b.s3()
	if err != nil {
		return err
	}

	u, err := url.Parse(bucketURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse bucket name from %s", bucketURL)
	}
	bucketName := u.Host

	// Check if bucket exists already
	_, err = svc.HeadBucket(&s3.HeadBucketInput{Bucket: aws.String(bucketName)})
	if err == nil {
		return nil // bucket already exists
	}
	reqFailure, ok := err.(s3.RequestFailure)
	if !ok || reqFailure.StatusCode() != 404 {
		return errors.Wrapf(err, "failed to check if %s bucket exists already", bucketName)
	}

	infoBucketURL := util.ColorInfo(bucketURL)
	log.Logger().Infof("The bucket %s does not exist so lets create it", infoBucketURL)

	cbInput := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	}
	// There's a known problem with the S3 API that will make the request fail if you provide a CreateBucketConfiguration
	// with a LocationConstraint pointing to the S3 default us-east-1 region. If not provided, it will be created in that region.
	if b.Requirements.Cluster.Region != "us-east-1" {
		cbInput.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(b.Requirements.Cluster.Region),
		}
	}
	_, err = svc.CreateBucket(cbInput)
	if err != nil {
		return errors.Wrapf(err, "there was a problem creating the bucket %s in the AWS", bucketName)
	}
	return nil
}

// UploadFileToBucket uploads a file to an S3 bucket to the provided bucket with the provided outputName
func (b *AmazonBucketProvider) UploadFileToBucket(reader io.Reader, outputName string, bucketURL string) (string, error) {
	uploader, err := b.s3ManagerUploader()
	if err != nil {
		return "", nil
	}
	bucketURL = strings.TrimPrefix(bucketURL, "s3://")
	output, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketURL),
		Key:    aws.String("/" + outputName),
		Body:   reader,
	})
	if err != nil {
		return "", err
	}
	log.Logger().Debugf("The file was uploaded successfully, location: %s", output.Location)
	return fmt.Sprintf("%s://%s/%s", "s3", bucketURL, outputName), nil
}

// DownloadFileFromBucket downloads a file from an S3 bucket and converts the contents to a bufio.Scanner
func (b *AmazonBucketProvider) DownloadFileFromBucket(bucketURL string) (*bufio.Scanner, error) {
	downloader, err := b.s3ManagerDownloader()
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem downloading from the bucket")
	}

	u, err := url.Parse(bucketURL)
	if err != nil {
		return nil, errors.Wrapf(err, "the provided bucket location is not a valid URL: %s", bucketURL)
	}
	requestInput := s3.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
	}

	buf := aws.NewWriteAtBuffer([]byte{})
	_, err = downloader.Download(buf, &requestInput)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(buf.Bytes())
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	return scanner, nil
}

// NewAmazonBucketProvider create a new provider for AWS
func NewAmazonBucketProvider(requirements *config.RequirementsConfig) buckets.Provider {
	return &AmazonBucketProvider{
		Requirements: requirements,
	}
}
