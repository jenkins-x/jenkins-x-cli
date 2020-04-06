package buckets

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"
	"time"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"gocloud.dev/blob"
)

// CreateBucketURL creates a go-cloud URL to a bucket
func CreateBucketURL(name string, kind string, settings *jenkinsv1.TeamSettings) (string, error) {
	if kind == "" {
		provider := settings.KubeProvider
		if provider == "" {
			return "", fmt.Errorf("No bucket kind provided nor is a kubernetes provider configured for this team so it could not be defaulted")
		}
		kind = KubeProviderToBucketScheme(provider)
		if kind == "" {
			return "", fmt.Errorf("No bucket kind is associated with kubernetes provider %s", provider)
		}
	}
	return kind + "://" + name, nil
}

// KubeProviderToBucketScheme returns the bucket scheme for the cloud provider
func KubeProviderToBucketScheme(provider string) string {
	switch provider {
	case cloud.AKS:
		return "azblob"
	case cloud.AWS, cloud.EKS:
		return "s3"
	case cloud.GKE:
		return "gs"
	default:
		return ""
	}
}

// ReadURL reads the given URL from either a http/https endpoint or a bucket URL path.
// if specified the httpFn is a function which can append the user/password or token if using a git provider
func ReadURL(urlText string, timeout time.Duration, httpFn func(urlString string) (string, error)) ([]byte, error) {
	u, err := url.Parse(urlText)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse URL %s", urlText)
	}
	switch u.Scheme {
	case "http", "https":
		if httpFn != nil {
			urlText, err = httpFn(urlText)
			if err != nil {
				return nil, err
			}
		}
		return ReadHTTPURL(urlText, timeout)
	default:
		return ReadBucketURL(u, timeout)
	}
}

// ReadHTTPURL reads the HTTP based URL and returns the data or returning an error if a 2xx status is not returned
func ReadHTTPURL(u string, timeout time.Duration) ([]byte, error) {
	httpClient := util.GetClientWithTimeout(timeout)
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to invoke GET on %s", u)
	}
	stream := resp.Body
	defer stream.Close()

	data, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to GET data from %s", u)
	}
	if resp.StatusCode >= 400 {
		return data, fmt.Errorf("status %s when performing GET on %s", resp.Status, u)
	}
	return data, err
}

// ReadBucketURL reads the content of a bucket URL of the for 's3://bucketName/foo/bar/whatnot.txt?param=123'
// where any of the query arguments are applied to the underlying Bucket URL and the path is extracted and resolved
// within the bucket
func ReadBucketURL(u *url.URL, timeout time.Duration) ([]byte, error) {
	bucketURL, key := SplitBucketURL(u)

	ctx, _ := context.WithTimeout(context.Background(), timeout)
	bucket, err := blob.Open(ctx, bucketURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open bucket %s", bucketURL)
	}
	data, err := bucket.ReadAll(ctx, key)
	if err != nil {
		return data, errors.Wrapf(err, "failed to read key %s in bucket %s", key, bucketURL)
	}
	return data, nil
}

// WriteBucketURL writes the data to a bucket URL of the for 's3://bucketName/foo/bar/whatnot.txt?param=123'
// with the given timeout
func WriteBucketURL(u *url.URL, data []byte, timeout time.Duration) error {
	bucketURL, key := SplitBucketURL(u)
	return WriteBucket(bucketURL, key, data, timeout)
}

// WriteBucket writes the data to a bucket URL and key of the for 's3://bucketName' and key 'foo/bar/whatnot.txt'
// with the given timeout
func WriteBucket(bucketURL string, key string, data []byte, timeout time.Duration) error {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	bucket, err := blob.Open(ctx, bucketURL)
	if err != nil {
		return errors.Wrapf(err, "failed to open bucket %s", bucketURL)
	}
	err = bucket.WriteAll(ctx, key, data, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to write key %s in bucket %s", key, bucketURL)
	}
	return nil
}

// SplitBucketURL splits the full bucket URL into the URL to open the bucket and the file name to refer to
// within the bucket
func SplitBucketURL(u *url.URL) (string, string) {
	u2 := *u
	u2.Path = ""
	return u2.String(), strings.TrimPrefix(u.Path, "/")
}
