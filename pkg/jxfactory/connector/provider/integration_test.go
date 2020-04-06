// +build integration

package provider_test

import (
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/jxfactory/connector"
	"github.com/jenkins-x/jx/pkg/jxfactory/connector/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestGCPConnector tests we can connect to a remote cluster/project/region easily
func TestGCPConnector(t *testing.T) {
	project := os.Getenv("TEST_PROJECT")
	cluster := os.Getenv("TEST_CLUSTER")
	region := os.Getenv("TEST_REGION")

	if project == "" || cluster == "" || region == "" {
		t.SkipNow()
		return
	}

	t.Logf("connecting to project %s cluster %s region %s", project, cluster, region)

	workDir, err := ioutil.TempDir("", "jx-test-gcp-connector-")
	require.NoError(t, err, "failed to generate temp dir")

	t.Logf("using workdir: %s", workDir)

	client := provider.NewClient(workDir)

	conn := &connector.RemoteConnector{GKE: &connector.GKEConnector{
		Project: project,
		Cluster: cluster,
		Region:  region,
	}}

	config, err := client.Connect(conn)
	require.NoError(t, err, "failed to connect to %#v", conn.GKE)
	assert.NotNil(t, config, "no config created")

	factory := connector.NewConfigClientFactory("remote", config)
	kubeClient, err := factory.CreateKubeClient()
	require.NoError(t, err, "failed to create KubeClient to %#v", conn.GKE)
	require.NotNil(t, kubeClient, "no KubeClient created for %#v", conn.GKE)

	list, err := kubeClient.CoreV1().Pods("jx").List(metav1.ListOptions{})
	require.NoError(t, err, "Failed to list pods in remote connection namespace jx")

	now := time.Now()

	t.Log("PODS")
	for _, r := range list.Items {
		t.Logf("%s %s", r.Name, now.Sub(r.CreationTimestamp.Time).String())
	}
}
