package gcp

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/jenkins-x/jx/pkg/jxfactory/connector"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// CreateGCPConfig creates a kubernetes config for the given connector
func CreateGCPConfig(connector *connector.GKEConnector, dir string) (*rest.Config, error) {
	project := connector.Project
	if project == "" {
		return nil, fmt.Errorf("missing project")
	}
	cluster := connector.Cluster
	if cluster == "" {
		return nil, fmt.Errorf("missing cluster")
	}
	args := []string{"container", "clusters", "get-credentials", cluster, "--project", project}
	if connector.Zone != "" {
		args = append(args, "--zone", connector.Zone)
	} else if connector.Region != "" {
		args = append(args, "--region", connector.Region)
	} else {
		return nil, fmt.Errorf("missing zone or region")
	}

	file, err := ioutil.TempFile(dir, "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create temp file in %s", dir)
	}
	fileName := file.Name()

	cmd := util.Command{
		Dir:  dir,
		Name: "gcloud",
		Args: args,
		Env: map[string]string{
			"KUBECONFIG": fileName,
		},
		Out: os.Stdout,
		Err: os.Stderr,
	}
	text, err := cmd.RunWithoutRetry()
	fmt.Printf(text)
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{Precedence: []string{fileName}},
		&clientcmd.ConfigOverrides{ClusterInfo: clientcmdapi.Cluster{Server: ""}}).ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create client-go config for file %s", fileName)
	}
	return config, nil
}
