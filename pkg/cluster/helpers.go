package cluster

import (
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// NewLabelValue returns a cluster safe unique label we can use for locking
func NewLabelValue() (string, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", err
	}
	return naming.ToValidName(id.String()), nil
}

// LockCluster tries to use the given label and value to lock the cluster.
// Return nil if there are no clusters available
func LockCluster(client Client, lockLabels map[string]string, filterLabels map[string]string) (*Cluster, error) {
	clusters, err := client.ListFilter(filterLabels)
	if err != nil {
		return nil, err
	}

	for _, c := range clusters {
		if !HasAnyKey(c.Labels, lockLabels) {
			// lets try to update label
			allLabels := util.MergeMaps(map[string]string{}, c.Labels, lockLabels)
			err = client.SetClusterLabels(c, allLabels)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to label cluster %s with label %#v", c.Name, lockLabels)
			}

			// now lets requery to verify the label got applied
			copy, err := client.Get(c.Name)
			if err != nil {
				return nil, err
			}
			if copy != nil && LabelsMatch(copy.Labels, lockLabels) {
				return copy, nil
			}
			if copy == nil {
				log.Logger().Infof("cluster %s no longer exists", c.Name)
			} else {
				log.Logger().Infof("could not label cluster %s with lock labels %#v it has labels %#v so could be labelled by another process",
					c.Name, lockLabels, copy.Labels)
			}
		}
	}
	return nil, nil
}

// GetCluster gets a cluster by listing the clusters
func GetCluster(client Client, name string) (*Cluster, error) {
	clusters, err := client.List()
	if err != nil {
		return nil, err
	}

	for _, c := range clusters {
		if c.Name == name {
			return c, nil
		}
	}
	return nil, nil
}

// ListFilter lists the clusters with a filter
func ListFilter(client Client, labels map[string]string) ([]*Cluster, error) {
	answer := []*Cluster{}
	clusters, err := client.List()
	if err != nil {
		return answer, err
	}
	for _, c := range clusters {
		if LabelsMatch(c.Labels, labels) {
			answer = append(answer, c)
		}
	}
	return answer, nil
}

// LabelsMatch returns true if the filter labels are contained in the label map
func LabelsMatch(labels map[string]string, filter map[string]string) bool {
	for k, v := range filter {
		if labels == nil || labels[k] != v {
			return false
		}
	}
	return true
}

// HasAnyKey returns true if the labels map has none of the keys in the filter
func HasAnyKey(labels map[string]string, filters map[string]string) bool {
	if labels != nil && filters != nil {
		for k := range filters {
			if labels[k] != "" {
				return true
			}
		}
	}
	return false
}

// RemoveLabels removes the set of labels from the cluster
func RemoveLabels(client Client, cluster *Cluster, removeLabels []string) (map[string]string, error) {
	if cluster.Labels == nil {
		return cluster.Labels, nil
	}

	updated := false
	for _, label := range removeLabels {
		if _, ok := cluster.Labels[label]; ok {
			delete(cluster.Labels, label)
			updated = true
		}
	}

	var err error
	if updated {
		err = client.SetClusterLabels(cluster, cluster.Labels)
	}
	return cluster.Labels, err
}
