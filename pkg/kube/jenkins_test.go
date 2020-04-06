// +build unit

package kube_test

import (
	"testing"

	"github.com/beevik/etree"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestAddGiteaServers(t *testing.T) {
	t.Parallel()
	cm := &corev1.ConfigMap{
		Data: map[string]string{},
	}

	expectedGitURL := "https://my.gitea.com"
	expectedGitName := "mygitea"
	expectedCredentials := "my-credential-name"
	server := &auth.AuthServer{
		Kind: gits.KindGitea,
		Name: expectedGitName,
		URL:  expectedGitURL,
	}
	userAuth := &auth.UserAuth{
		Username: "dummy",
	}

	updated, err := kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.True(t, updated, "Should have updated the ConfigMap for server %s", expectedGitURL)

	for k, v := range cm.Data {
		tests.Debugf("Updated the ConfigMap: %s = %s\n", k, v)
	}

	doc, _, err := kube.ParseXml(cm.Data[kube.GiteaConfigMapKey])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//serverUrl", expectedGitURL)

	updated, err = kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.False(t, updated, "Should not have updated the ConfigMap for server %s", expectedGitURL)

	doc, _, err = kube.ParseXml(cm.Data[kube.GiteaConfigMapKey])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//serverUrl", expectedGitURL)

	// lets add an extra service to an existing one
	cm.Data[kube.GiteaConfigMapKey] = `<?xml version='1.1' encoding='UTF-8'?>
<org.jenkinsci.plugin.gitea.servers.GiteaServers plugin="gitea@1.0.5">
  <servers>
    <org.jenkinsci.plugin.gitea.servers.GiteaServer>
      <displayName>gitea</displayName>
      <serverUrl>http://gitea.changeme.com</serverUrl>
      <manageHooks>true</manageHooks>
      <credentialsId>jenkins-x-gitea</credentialsId>
    </org.jenkinsci.plugin.gitea.servers.GiteaServer>
  </servers>
</org.jenkinsci.plugin.gitea.servers.GiteaServers>
`
	updated, err = kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.True(t, updated, "Should have updated the ConfigMap for server %s", expectedGitURL)

	doc, _, err = kube.ParseXml(cm.Data[kube.GiteaConfigMapKey])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//serverUrl", "http://gitea.changeme.com", expectedGitURL)

	// lets modify an existing credentials value
	// lets add an extra service to an existing one
	cm.Data[kube.GiteaConfigMapKey] = `<?xml version='1.1' encoding='UTF-8'?>
<org.jenkinsci.plugin.gitea.servers.GiteaServers plugin="gitea@1.0.5">
  <servers>
    <org.jenkinsci.plugin.gitea.servers.GiteaServer>
      <displayName>gitea</displayName>
      <serverUrl>` + expectedGitURL + `</serverUrl>
      <manageHooks>true</manageHooks>
      <credentialsId>jenkins-x-gitea</credentialsId>
    </org.jenkinsci.plugin.gitea.servers.GiteaServer>
  </servers>
</org.jenkinsci.plugin.gitea.servers.GiteaServers>
`

	updated, err = kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.True(t, updated, "Should have updated the ConfigMap for server %s", expectedGitURL)

	doc, _, err = kube.ParseXml(cm.Data[kube.GiteaConfigMapKey])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//serverUrl", expectedGitURL)
	assertElementValues(t, doc, "//credentialsId", expectedCredentials)
}

func TestAddGitHuvServers(t *testing.T) {
	t.Parallel()
	key := kube.GithubConfigMapKey
	kind := gits.KindGitHub

	cm := &corev1.ConfigMap{
		Data: map[string]string{},
	}

	expectedGitHostURL := "https://github.bees.com"
	expectedGitURL := expectedGitHostURL + "/api/v3/"
	expectedGitName := "GHE"
	expectedCredentials := "my-credential-name"
	server := &auth.AuthServer{
		Kind: kind,
		Name: expectedGitName,
		URL:  expectedGitHostURL,
	}
	userAuth := &auth.UserAuth{
		Username: "dummy",
	}

	updated, err := kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.True(t, updated, "Should have updated the ConfigMap for server %s", expectedGitURL)

	for k, v := range cm.Data {
		tests.Debugf("Updated the ConfigMap: %s = %s\n", k, v)
	}

	doc, _, err := kube.ParseXml(cm.Data[key])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//apiUri", expectedGitURL)

	updated, err = kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.False(t, updated, "Should not have updated the ConfigMap for server %s", expectedGitURL)

	doc, _, err = kube.ParseXml(cm.Data[key])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//apiUri", expectedGitURL)
}

func TestAddBitBucketServerServers(t *testing.T) {
	t.Parallel()
	kind := gits.KindBitBucketServer
	key := kube.BitbucketConfigMapKey

	cm := &corev1.ConfigMap{
		Data: map[string]string{},
	}

	expectedGitURL := "https://my.bitbucket.com"
	expectedGitName := "mybitbucket"
	expectedCredentials := "my-credential-name"
	server := &auth.AuthServer{
		Kind: kind,
		Name: expectedGitName,
		URL:  expectedGitURL,
	}
	userAuth := &auth.UserAuth{
		Username: "dummy",
	}

	updated, err := kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.True(t, updated, "Should have updated the ConfigMap for server %s", expectedGitURL)

	for k, v := range cm.Data {
		tests.Debugf("Updated the ConfigMap: %s = %s\n", k, v)
	}

	doc, _, err := kube.ParseXml(cm.Data[key])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//serverUrl", expectedGitURL)

	updated, err = kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.False(t, updated, "Should not have updated the ConfigMap for server %s", expectedGitURL)

	doc, _, err = kube.ParseXml(cm.Data[key])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//serverUrl", expectedGitURL)
}

func TestAddBitBucketCloudServers(t *testing.T) {
	t.Parallel()
	kind := gits.KindBitBucketCloud
	key := kube.BitbucketConfigMapKey

	cm := &corev1.ConfigMap{
		Data: map[string]string{},
	}

	expectedGitURL := gits.BitbucketCloudURL
	expectedGitName := "mybitbucket"
	expectedCredentials := "my-credential-name"
	server := &auth.AuthServer{
		Kind: kind,
		Name: expectedGitName,
		URL:  expectedGitURL,
	}
	userAuth := &auth.UserAuth{
		Username: "dummy",
	}

	updated, err := kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.True(t, updated, "Should have updated the ConfigMap for server %s", expectedGitURL)

	for k, v := range cm.Data {
		tests.Debugf("Updated the ConfigMap: %s = %s\n", k, v)
	}

	doc, _, err := kube.ParseXml(cm.Data[key])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//credentialsId", expectedCredentials)

	updated, err = kube.UpdateJenkinsGitServers(cm, server, userAuth, expectedCredentials)
	assert.Nil(t, err, "Failed to update the ConfigMap for server %s", expectedGitURL)
	assert.False(t, updated, "Should not have updated the ConfigMap for server %s", expectedGitURL)

	doc, _, err = kube.ParseXml(cm.Data[key])
	assert.Nil(t, err, "Failed to parse resulting xml for server %s", expectedGitURL)
	assertElementValues(t, doc, "//credentialsId", expectedCredentials)
}

func assertElementValues(t *testing.T, doc *etree.Document, path string, expectedValues ...string) {
	elements := doc.FindElements(path)
	actuals := []string{}
	for _, e := range elements {
		actuals = append(actuals, e.Text())
	}
	assert.EqualValues(t, expectedValues, actuals, "Invalid values for path %s", path)
}
