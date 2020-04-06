package provider

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jenkins-x/jx/pkg/jxfactory/connector"
	"github.com/jenkins-x/jx/pkg/jxfactory/connector/gcp"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"k8s.io/client-go/rest"
)

type defaultClient struct {
	configs sync.Map
	workDir string
}

// NewClient default implementation using a work directory
func NewClient(workDir string) connector.Client {
	return &defaultClient{workDir: workDir}
}

func (c *defaultClient) Connect(connector *connector.RemoteConnector) (*rest.Config, error) {
	path := filepath.Join(c.workDir, connector.Path())
	var config *rest.Config
	value, ok := c.configs.Load(path)
	if ok {
		config = value.(*rest.Config)
	}
	if config == nil {
		var err error

		err = os.MkdirAll(path, util.DefaultWritePermissions)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create parent dir %s", path)
		}
		config, err = c.createConfig(connector, path)
		if err != nil {
			return nil, err
		}
		if config == nil {
			return nil, fmt.Errorf("connector is not supported")
		}
		c.configs.Store(path, config)
	}
	return config, nil
}

func (c *defaultClient) createConfig(connector *connector.RemoteConnector, dir string) (*rest.Config, error) {
	if connector.GKE != nil {
		return gcp.CreateGCPConfig(connector.GKE, dir)
	}
	return nil, nil
}
