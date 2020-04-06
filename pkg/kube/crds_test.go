// +build unit

package kube_test

import (
	"testing"

	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	apiextentions_mocks "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
)

func TestRegisterEnvironmentCRD(t *testing.T) {
	// mock factory
	factory := cmd_mocks.NewMockFactory()

	// mock apiExtensions interface
	apiextensionsInterface := apiextentions_mocks.NewSimpleClientset()
	// Override CreateApiExtensionsClient to return mock apiextensions interface
	When(factory.CreateApiExtensionsClient()).ThenReturn(apiextensionsInterface, nil)

	err := kube.RegisterEnvironmentCRD(apiextensionsInterface)

	assert.NoError(t, err, "Should not error")
}

func TestRegisterPipelineStructureCRD(t *testing.T) {
	// mock factory
	factory := cmd_mocks.NewMockFactory()

	// mock apiExtensions interface
	apiextensionsInterface := apiextentions_mocks.NewSimpleClientset()
	// Override CreateApiExtensionsClient to return mock apiextensions interface
	When(factory.CreateApiExtensionsClient()).ThenReturn(apiextensionsInterface, nil)

	err := kube.RegisterPipelineStructureCRD(apiextensionsInterface)

	assert.NoError(t, err, "Should not error")
}
