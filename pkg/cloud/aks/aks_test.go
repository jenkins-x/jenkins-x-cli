// +build unit

package aks_test

import (
	"reflect"
	"testing"

	"github.com/jenkins-x/jx/pkg/cloud/aks"
	"github.com/stretchr/testify/assert"

	mocks "github.com/jenkins-x/jx/pkg/util/mocks"
	. "github.com/petergtz/pegomock"
)

func aksWithRunner(t *testing.T, expectedError error, expectedOutput string) *aks.AzureRunner {
	RegisterMockTestingT(t)
	runner := mocks.NewMockCommander()
	When(runner.RunWithoutRetry()).ThenReturn(expectedOutput, expectedError)
	return aks.NewAzureRunnerWithCommander(runner)
}

func TestGetClusterClient(t *testing.T) {
	azureCLI := aksWithRunner(t, nil, `[{
			"group": "musekeen",
			"id": "01234567-89ab-cdef-0123-456789abcdef",
			"name": "scalefrost",
			"uri": "scalefrost-musekeen-2e62fb-6d5429ef.hcp.westus2.azmk8s.io"
		},
		{
			"group": "resource_group",
			"id": "abcd",
			"name": "name",
			"uri": "aks.hcp.eatus.azmk8s.io"
		}
	]`)
	rg, name, client, err := azureCLI.GetClusterClient("https://aks.hcp.eatus.azmk8s.io:443")
	assert.Equal(t, client, "abcd")
	assert.Equal(t, rg, "resource_group")
	assert.Equal(t, name, "name")
	assert.Nil(t, err)
}

func TestNotACR(t *testing.T) {
	azureCLI := aks.NewAzureRunner()
	config, registry, id, err := azureCLI.GetRegistry("", "rg", "name", "azure.docker.io")
	assert.Equal(t, "", config)
	assert.Equal(t, "azure.docker.io", registry)
	assert.Equal(t, "", id)
	assert.Nil(t, err)
}

func TestNoRegistrySet(t *testing.T) {
	RegisterMockTestingT(t)
	runner := mocks.NewMockCommander()
	When(runner.RunWithoutRetry()).Then(func(params []Param) ReturnValues {
		return []ReturnValue{showResult(runner), nil}
	})
	azureCLI := aks.NewAzureRunnerWithCommander(runner)

	config, registry, id, err := azureCLI.GetRegistry("", "rg", "azure", "")
	assert.Equal(t, `{"auths":{"azure.azurecr.io":{"auth":"YXp1cmU6cGFzc3dvcmQxMjM="}}}`, config)
	assert.Equal(t, "azure.azurecr.io", registry)
	assert.Equal(t, "fakeid", id)
	assert.Nil(t, err)
}

func TestSubscriptionSet(t *testing.T) {
	RegisterMockTestingT(t)
	runner := mocks.NewMockCommander()
	When(runner.RunWithoutRetry()).Then(func(params []Param) ReturnValues {
		return []ReturnValue{showResult(runner), nil}
	})
	azureCLI := aks.NewAzureRunnerWithCommander(runner)

	config, registry, id, err := azureCLI.GetRegistry("sub", "rg", "azuresub", "")
	assert.Equal(t, `{"auths":{"azure.azurecr.io":{"auth":"YXp1cmU6cGFzc3dvcmQxMjM="}}}`, config)
	assert.Equal(t, "azuresub.azurecr.io", registry)
	assert.Equal(t, "fakeidsub", id)
	assert.Nil(t, err)
}

func TestRegistry404(t *testing.T) {
	RegisterMockTestingT(t)
	runner := mocks.NewMockCommander()
	When(runner.RunWithoutRetry()).Then(func(params []Param) ReturnValues {
		return []ReturnValue{showResult(runner), nil}
	})
	azureCLI := aks.NewAzureRunnerWithCommander(runner)

	config, registry, id, err := azureCLI.GetRegistry("", "newrg", "newacr", "notfound.azurecr.io")
	assert.Equal(t, `{"auths":{"newacr.azurecr.io":{"auth":"YXp1cmU6cGFzc3dvcmQxMjM="}}}`, config)
	assert.Equal(t, "newacr.azurecr.io", registry)
	assert.Equal(t, "fakeidxxx", id)
	assert.Nil(t, err)
}

func TestRegistry404WithSubSet(t *testing.T) {
	RegisterMockTestingT(t)
	runner := mocks.NewMockCommander()
	When(runner.RunWithoutRetry()).Then(func(params []Param) ReturnValues {
		return []ReturnValue{showResult(runner), nil}
	})
	azureCLI := aks.NewAzureRunnerWithCommander(runner)

	config, registry, id, err := azureCLI.GetRegistry("sub", "newrg", "newacr", "notfound.azurecr.io")
	assert.Equal(t, `{"auths":{"newacr.azurecr.io":{"auth":"YXp1cmU6cGFzc3dvcmQxMjM="}}}`, config)
	assert.Equal(t, "newacr.azurecr.io", registry)
	assert.Equal(t, "fakeidsub", id)
	assert.Nil(t, err)
}

func showResult(runner *mocks.MockCommander) string {
	args := runner.VerifyWasCalled(AtLeast(1)).SetArgs(AnyStringSlice()).GetCapturedArguments()
	if reflect.DeepEqual(args, []string{"acr", "list", "--query", "[].{uri:loginServer,id:id,name:name,group:resourceGroup}"}) {
		return `[
			{
				"group": "musekeen",
				"id": "fakeidnotused",
				"name": "jenkinsx",
				"uri": "jenkinsx.azurecr.io"
			},
			{
				"group": "musekeen",
				"id": "fakeid",
				"name": "azure",
				"uri": "azure.azurecr.io"
			}
		]`
	} else if reflect.DeepEqual(args, []string{"acr", "list", "--query", "[].{uri:loginServer,id:id,name:name,group:resourceGroup}", "--subscription", "sub"}) {
		return `[
			{
				"group": "musekeen",
				"id": "fakeidnotused",
				"name": "jenkinsx",
				"uri": "jenkinsx.azurecr.io"
			},
			{
				"group": "musekeen",
				"id": "fakeidsub",
				"name": "azure",
				"uri": "azuresub.azurecr.io"
			}
		]`
	} else if reflect.DeepEqual(args, []string{"acr", "create", "-g", "newrg", "-n", "newacr", "--sku", "Standard", "--admin-enabled", "--query", "id", "-o", "tsv"}) {
		return `fakeidxxx`
	} else if reflect.DeepEqual(args, []string{"acr", "create", "-g", "newrg", "-n", "newacr", "--sku", "Standard", "--admin-enabled", "--query", "id", "-o", "tsv", "--subscription", "sub"}) {
		return `fakeidsub`
	} else {
		return `{
			"passwords": [
				{
					"name": "password",
					"value": "password123"
				},
				{
					"name": "password2",
					"value": "passwordabc"
				}
			],
			"username": "azure"
		}`
	}
}
