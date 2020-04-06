package buckets

import (
	"bufio"
	"io"
)

// Provider represents a bucket provider
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/cloud/buckets Provider -o mocks/buckets_interface.go
type Provider interface {
	// CreateNewBucketForCluster creates a new dynamically named bucket
	CreateNewBucketForCluster(clusterName string, bucketKind string) (string, error)
	EnsureBucketIsCreated(bucketURL string) error
	UploadFileToBucket(r io.Reader, outputName string, bucketURL string) (string, error)
	DownloadFileFromBucket(bucketURL string) (*bufio.Scanner, error)
}
