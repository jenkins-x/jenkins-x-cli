// +build unit

package e2e_test

import (
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cluster"
	"github.com/jenkins-x/jx/pkg/cmd/step/e2e"
	"github.com/stretchr/testify/assert"
)

func TestGetBuildNumberCluster(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{}
	num, err := o.GetBuildNumberFromCluster(getCluster("168", "jenkins-gkebdd", "159"))
	assert.NoError(t, err)
	assert.Equal(t, 159, num)
	num, err = o.GetBuildNumberFromCluster(getCluster("169", "jenkins-gkebdd", "160"))
	assert.NoError(t, err)
	assert.Equal(t, 160, num)
	num, err = o.GetBuildNumberFromCluster(getCluster("169", "jenkins-gkebdd", "xx"))
	assert.NotNil(t, err)

}

func TestDeleteDueToNewerRun(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{}
	cluster1 := getCluster("168", "jenkins-gkebdd", "159")
	cluster2 := getCluster("168", "jenkins-gkebdd", "160")
	clusters := make([]gke.Cluster, 0)
	clusters = append(clusters, *cluster1, *cluster2)
	assert.Equal(t, true, o.ShouldDeleteDueToNewerRun(cluster1, clusters))
	cluster4 := getCluster("168", "jenkins-gkebdd", "160")
	clusters = append(clusters, *cluster4)
	assert.Equal(t, false, o.ShouldDeleteDueToNewerRun(cluster4, clusters))
}

func TestDeleteDueToNewerRunEKS(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{
		Providers: []string{"eks"},
	}
	cluster1 := getEKSCluster("168", "jenkins-gkebdd", "159")
	cluster2 := getEKSCluster("168", "jenkins-gkebdd", "160")
	clusters := make([]*cluster.Cluster, 0)
	clusters = append(clusters, cluster1, cluster2)
	assert.Equal(t, true, o.ShouldDeleteDueToNewerRunEKS(cluster1, clusters))
	cluster4 := getEKSCluster("168", "jenkins-gkebdd", "160")
	clusters = append(clusters, cluster4)
	assert.Equal(t, false, o.ShouldDeleteDueToNewerRunEKS(cluster4, clusters))
}

func TestShouldDeleteMarkedClusterEKS(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{
		Providers: []string{"eks"},
	}
	cluster := getEKSCluster("168", "jenkins-gkebdd", "159")
	cluster2 := getEKSCluster("170", "jenkins-gkebdd", "159")
	assert.Equal(t, false, o.ShouldDeleteMarkedEKSCluster(cluster))
	cluster2.Labels["delete-me"] = "true"
	assert.Equal(t, true, o.ShouldDeleteMarkedEKSCluster(cluster2))
}

func TestShouldDeleteMarkedCluster(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{}
	cluster := getCluster("168", "jenkins-gkebdd", "159")
	cluster2 := getCluster("170", "jenkins-gkebdd", "159")
	assert.Equal(t, false, o.ShouldDeleteMarkedCluster(cluster))
	cluster2.ResourceLabels["delete-me"] = "true"
	assert.Equal(t, true, o.ShouldDeleteMarkedCluster(cluster2))
}

func TestShouldDeleteOlderThanDurationEKS(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{
		Providers: []string{"eks"},
	}
	o.Duration = 2
	cluster := getEKSCluster("168", "jenkins-gkebdd", "159")
	cluster.Labels["create-time"] = time.Now().UTC().Format("Mon-Jan-2-2006-15-04-05")
	assert.Equal(t, false, o.ShouldDeleteOlderThanDurationEKS(cluster))
	cluster2 := getEKSCluster("170", "jenkins-gkebdd", "159")
	cluster2.Labels["create-time"] = time.Now().UTC().Add(-3 * time.Hour).Format("Mon-Jan-2-2006-15-04-05")
	assert.Equal(t, true, o.ShouldDeleteOlderThanDurationEKS(cluster2))
	cluster2.Labels["keep-me"] = "true"
	assert.Equal(t, false, o.ShouldDeleteOlderThanDurationEKS(cluster2))
}

func TestShouldDeleteOlderThanDuration(t *testing.T) {
	t.Parallel()
	o := e2e.StepE2EGCOptions{}
	o.Duration = 2
	cluster := getCluster("168", "jenkins-gkebdd", "159")
	cluster.ResourceLabels["create-time"] = time.Now().UTC().Format("Mon-Jan-2-2006-15-04-05")
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(cluster))
	cluster2 := getCluster("170", "jenkins-gkebdd", "159")
	cluster2.ResourceLabels["create-time"] = time.Now().UTC().Add(-3 * time.Hour).Format("Mon-Jan-2-2006-15-04-05")
	assert.Equal(t, true, o.ShouldDeleteOlderThanDuration(cluster2))
	cluster2.ResourceLabels["keep-me"] = "true"
	assert.Equal(t, false, o.ShouldDeleteOlderThanDuration(cluster2))
}

func getEKSCluster(prNumber string, clusterType string, buildNumber string) *cluster.Cluster {
	resourceLabels := make(map[string]string)
	resourceLabels["branch"] = "pr-" + prNumber
	resourceLabels["cluster"] = clusterType
	return &cluster.Cluster{
		Name:   resourceLabels["branch"] + "-" + buildNumber + "-" + clusterType,
		Labels: resourceLabels,
		Status: "RUNNING",
	}
}

func getCluster(prNumber string, clusterType string, buildNumber string) *gke.Cluster {
	resourceLabels := make(map[string]string)
	resourceLabels["branch"] = "pr-" + prNumber
	resourceLabels["cluster"] = clusterType
	return &gke.Cluster{
		Name:           resourceLabels["branch"] + "-" + buildNumber + "-" + clusterType,
		ResourceLabels: resourceLabels,
		Status:         "RUNNING",
	}
}
