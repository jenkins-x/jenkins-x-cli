package bdd

import (
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"io/ioutil"
)

// CreateClusters contains an array of clusters
type CreateClusters struct {
	Clusters []*CreateCluster `json:"clusters,omitempty"`
}

// CreateCluster defines how to create a cluster
type CreateCluster struct {
	Name      string   `json:"name,omitempty"`
	Args      []string `json:"args,omitempty"`
	NoLabels  bool     `json:"noLabels,omitempty"`
	Labels    string   `json:"labels,omitempty"`
	Terraform bool     `json:"terraform,omitempty"`

	Commands []Command `json:"commands,omitempty"`
}

// Command for running post create cluster commands
type Command struct {
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
}

// LoadBddClusters loads the cluster configuration from the given file
func LoadBddClusters(fileName string) (*CreateClusters, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load file %s", fileName)
	}
	answer := &CreateClusters{}
	err = yaml.Unmarshal(data, answer)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal YAML in file %s", fileName)
	}
	return answer, nil
}
