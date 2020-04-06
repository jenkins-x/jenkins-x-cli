package amazon

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jenkins-x/jx/pkg/cloud/amazon/session"
)

// CreateS3Bucket creates a new S3 bucket in the default region with the given bucket name
// returning the location string
func CreateS3Bucket(bucketName string, profile string, region string) (string, error) {
	location := ""
	sess, err := session.NewAwsSession(profile, region)
	if err != nil {
		return location, err
	}

	input := &s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: sess.Config.Region,
		},
	}
	svc := s3.New(sess)
	result, err := svc.CreateBucket(input)
	if result != nil && result.Location != nil {
		location = *result.Location
	}
	return location, err
}
