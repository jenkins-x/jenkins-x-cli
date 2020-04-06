package collector

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/jenkins-x/jx/pkg/cloud/buckets"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// BucketCollector stores the state for the git collector
type BucketCollector struct {
	Timeout time.Duration

	bucketURL  string
	classifier string
	provider   buckets.Provider
}

// NewBucketCollector creates a new git based collector
func NewBucketCollector(bucketURL string, classifier string, provider buckets.Provider) (Collector, error) {
	return &BucketCollector{
		Timeout:    time.Second * 20,
		classifier: classifier,
		provider:   provider,
		bucketURL:  bucketURL,
	}, nil
}

// CollectFiles collects files and returns the URLs
func (c *BucketCollector) CollectFiles(patterns []string, outputPath string, basedir string) ([]string, error) {
	urls := []string{}
	for _, p := range patterns {
		fn := func(name string) error {
			var err error
			toName := name
			if basedir != "" {
				toName, err = filepath.Rel(basedir, name)
				if err != nil {
					return errors.Wrapf(err, "failed to remove basedir %s from %s", basedir, name)
				}
			}
			if outputPath != "" {
				toName = filepath.Join(outputPath, toName)
			}
			data, err := ioutil.ReadFile(name)
			if err != nil {
				return errors.Wrapf(err, "failed to read file %s", name)
			}
			url, err := c.provider.UploadFileToBucket(bytes.NewReader(data), toName, c.bucketURL)
			if err != nil {
				return err
			}
			urls = append(urls, url)
			return nil
		}

		err := util.GlobAllFiles("", p, fn)
		if err != nil {
			return urls, err
		}
	}
	return urls, nil
}

// CollectData collects the data storing it at the given output path and returning the URL to access it
func (c *BucketCollector) CollectData(data []byte, outputName string) (string, error) {
	url, err := c.provider.UploadFileToBucket(bytes.NewReader(data), outputName, c.bucketURL)
	if err != nil {
		return "", err
	}
	return url, nil
}
