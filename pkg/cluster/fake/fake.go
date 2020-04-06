package fake

import (
	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// Client a fake implementation of the cluster client
type Client struct {
	Clusters []*cluster.Cluster
}

// verify we implement the interface
var _ cluster.Client = &Client{}

// NewClient create a new fake client for testing
func NewClient(clusters []*cluster.Cluster) *Client {
	return &Client{
		Clusters: clusters,
	}
}

// List lists the clusters
func (c *Client) List() ([]*cluster.Cluster, error) {
	return c.Clusters, nil
}

// ListFilter lists the clusters with a filter
func (c *Client) ListFilter(labels map[string]string) ([]*cluster.Cluster, error) {
	return cluster.ListFilter(c, labels)
}

// Connect connects to a cluster
func (c *Client) Connect(cluster *cluster.Cluster) error {
	log.Logger().Infof("fake cluster connecting to cluster: %s", util.ColorInfo(cluster.Name))
	return nil
}

// String return the string representation
func (c *Client) String() string {
	return "Client"
}

// Get looks up a cluster by name
func (c *Client) Get(name string) (*cluster.Cluster, error) {
	return cluster.GetCluster(c, name)
}

// Delete should delete the cluster from the clusters list
func (c *Client) Delete(cluster *cluster.Cluster) error {
	for i, v := range c.Clusters {
		if v.Name == cluster.Name {
			c.Clusters = append(c.Clusters[:i], c.Clusters[i+1:]...)
		}
	}
	return nil
}

// SetClusterLabels labels the given cluster
func (c *Client) SetClusterLabels(cluster *cluster.Cluster, labels map[string]string) error {
	cluster.Labels = labels
	return nil
}
