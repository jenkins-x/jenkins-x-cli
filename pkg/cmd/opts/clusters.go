package opts

import (
	"sort"

	gcp "github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cluster/fake"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/cluster/gke"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// ClusterOptions used to determine which kind of cluster to query
type ClusterOptions struct {
	GKE  GKEClusterOptions
	Fake bool
}

// GKEClusterOptions GKE specific configurations
type GKEClusterOptions struct {
	Project string
	Region  string
}

// CreateClient creates a new cluster client from the CLI options
func (o *ClusterOptions) CreateClient(requireProject bool) (cluster.Client, error) {
	if o.Fake {
		clusters := []*cluster.Cluster{
			{
				Name: "cluster1",
			},
			{
				Name: "cluster2",
			},
			{
				Name: "cluster1",
			},
		}
		return fake.NewClient(clusters), nil
	}
	if o.GKE.Project != "" || o.GKE.Region != "" {
		if o.GKE.Project == "" {
			return nil, util.MissingOption("gke-project")
		}
		if o.GKE.Region == "" {
			return nil, util.MissingOption("gke-region")
		}
	}

	if o.GKE.Project == "" {
		// lets try detect the GKE project...
		g := &gcp.GCloud{}
		project, err := g.CurrentProject()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to detect if we are using GKE")
		}
		if requireProject {
			if project == "" {
				log.Logger().Warn("could not detect the current GKE project")
				return nil, util.MissingOption("gke-project")
			}
		}
		o.GKE.Project = project
	}
	return gke.NewGKE(o.GKE.Project, o.GKE.Region)

	//return nil, fmt.Errorf("could not detect the kind of cluster via the command line options")
}

// AddClusterFlags adds the cluster CLI arguments
func (o *ClusterOptions) AddClusterFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.GKE.Project, "gke-project", "", "", "The GKE project name")
	cmd.Flags().StringVarP(&o.GKE.Region, "gke-region", "", "", "The GKE project name")
	cmd.Flags().BoolVarP(&o.Fake, "fake", "", false, "Use the fake clusters client")
}

// GetOrPickClusterName returns the selected cluster name or returns an error
func (o *CommonOptions) GetOrPickClusterName(client cluster.Client, clusterName string) (string, error) {
	if clusterName == "" && !o.BatchMode {
		clusters, err := client.List()
		if err != nil {
			return "", errors.Wrapf(err, "failed to list clusters for %s", client.String())
		}
		names := []string{}
		for _, c := range clusters {
			names = append(names, c.Name)
		}
		sort.Strings(names)

		clusterName, err = util.PickName(names, "pick the cluster name", "you need to provide a cluster name to unlock", o.GetIOFileHandles())
		if err != nil {
			return "", errors.Wrap(err, "failed to pick cluster name")
		}
	}
	if clusterName == "" {
		return "", util.MissingOption("name")
	}
	return clusterName, nil
}
