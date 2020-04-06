package gke

import (
	"fmt"
	"os"

	gcp "github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

type gcloud struct {
	region  string
	project string
	gcloud  gcp.GCloud
}

// NewGKE create a new client for working with GKE clusters using the given region and project
func NewGKE(project string, region string) (cluster.Client, error) {
	return &gcloud{
		region:  region,
		project: project,
	}, nil
}

// NewGKEFromEnv create a new client for working with GKE clusters using environment variables to define the region/project
func NewGKEFromEnv() (cluster.Client, error) {
	project := os.Getenv(cluster.EnvGKEProject)
	if project == "" {
		return nil, util.MissingEnv(cluster.EnvGKEProject)
	}
	region := os.Getenv(cluster.EnvGKERegion)
	if region == "" {
		return nil, util.MissingEnv(cluster.EnvGKERegion)
	}
	return NewGKE(project, region)
}

// List lists the clusters
func (c *gcloud) List() ([]*cluster.Cluster, error) {
	items, err := c.gcloud.ListClusters(c.region, c.project)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list clusters in region %s project %s", c.region, c.project)
	}
	var answer []*cluster.Cluster

	for _, item := range items {
		answer = append(answer, &cluster.Cluster{
			Name:     item.Name,
			Labels:   item.ResourceLabels,
			Status:   item.Status,
			Location: item.Location,
		})
	}
	return answer, nil
}

// ListFilter lists the clusters with a filter
func (c *gcloud) ListFilter(labels map[string]string) ([]*cluster.Cluster, error) {
	return cluster.ListFilter(c, labels)
}

// Connect connects to a cluster
func (c *gcloud) Connect(cluster *cluster.Cluster) error {
	return c.gcloud.ConnectToRegionCluster(c.project, cluster.Location, cluster.Name)
}

// String return the string representation
func (c *gcloud) String() string {
	return fmt.Sprintf("GKE project: %s region: %s", c.project, c.region)
}

// Get looks up a cluster by name
func (c *gcloud) Get(name string) (*cluster.Cluster, error) {
	return cluster.GetCluster(c, name)
}

// Delete should delete the cluster from GKE
func (c *gcloud) Delete(cluster *cluster.Cluster) error {
	return fmt.Errorf("not implemented")
}

// SetClusterLabels labels the given cluster
func (c *gcloud) SetClusterLabels(cluster *cluster.Cluster, labelMap map[string]string) error {
	labels := util.MapToKeyValues(labelMap)
	return c.gcloud.UpdateGkeClusterLabels(cluster.Location, c.project, cluster.Name, labels)
}
