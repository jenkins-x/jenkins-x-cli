// +build unit

package gc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func getClusters() ([]cluster, error) {
	testDataFile := path.Join("test_data", "clusters.json")
	data, _ := ioutil.ReadFile(testDataFile)
	text := string(data)
	text = strings.TrimSpace(text)

	var clusters []cluster
	err := json.Unmarshal([]byte(text), &clusters)
	if err != nil {
		log.Fatal("Error unmarshalling data", err)
		return nil, err
	}

	return clusters, nil
}

func getServiceAccounts() ([]serviceAccount, error) {
	testDataFile := path.Join("test_data", "service_accounts.json")
	data, _ := ioutil.ReadFile(testDataFile)
	text := string(data)
	text = strings.TrimSpace(text)

	var serviceAccounts []serviceAccount
	err := json.Unmarshal([]byte(text), &serviceAccounts)
	if err != nil {
		log.Fatal("Error unmarshalling data", err)
		return nil, err
	}

	return serviceAccounts, nil
}

func TestDeleteServiceAccount(t *testing.T) {
	t.Parallel()

	gcgkeOptions := &GCGKEOptions{}

	serviceAccounts, _ := getServiceAccounts()
	clusters, _ := getClusters()
	serviceAccounts, _ = gcgkeOptions.getFilteredServiceAccounts(serviceAccounts, clusters)

	fmt.Println(fmt.Sprintf("Filtered Service accounts size %d", len(serviceAccounts)))

	var saNames []string
	for _, v := range serviceAccounts {
		saNames = append(saNames, v.DisplayName)
	}
	assert.Equal(t, 17, len(serviceAccounts), "17 service accounts should be deleted")

	assert.Contains(t, saNames, "test-ko", "Should contain this service account")
	assert.Contains(t, saNames, "pr-4768-8-tekton-gkebdd-ko", "Should contain this service account")
	assert.Contains(t, saNames, "pr-331-167-ng-gkebdd-ko", "Should contain this service account")
	assert.Contains(t, saNames, "pr-331-165-gitop-vt", "Should contain this service account")

	assert.NotContains(t, saNames, "test-dev-ko", "Should not contain this service account")
	assert.NotContains(t, saNames, "pr-331-170-gitop-vt", "Should not contain this service account")
	assert.NotContains(t, saNames, "pr-331-171-tekton-gkebdd-ko", "Should not contain this service account")
	assert.NotContains(t, saNames, "pr-331-172-ng-gkebdd-ko", "Should not contain this service account")
	assert.NotContains(t, saNames, "pr-331-172-ng-gk-vt", "Should not contain this service account")
}
