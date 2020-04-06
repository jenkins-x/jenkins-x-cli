package connector

import (
	"k8s.io/client-go/rest"
)

// Client a client for connecting to remote clusters to help support
// multiple clusters for Environments
type Client interface {
	Connect(connector *RemoteConnector) (*rest.Config, error)
}
