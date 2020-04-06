// +build unit

package kube_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
)

func TestValidateName(t *testing.T) {
	t.Parallel()
	err := kube.ValidateName("test")
	assert.NoError(t, err, "Should not error")
}

func TestValidateNameTypeError(t *testing.T) {
	t.Parallel()
	err := kube.ValidateName(1)
	assert.Error(t, err, "Should error")
}

func TestValidateNameEmptyError(t *testing.T) {
	t.Parallel()
	err := kube.ValidateName("")
	assert.Error(t, err, "Should error")
}

func TestValidNameOption(t *testing.T) {
	t.Parallel()
	err := kube.ValidNameOption("test-option", "test-value")
	assert.NoError(t, err, "Should not error")
}

func TestValidNameOptionEmpty(t *testing.T) {
	t.Parallel()
	err := kube.ValidNameOption("", "")
	assert.NoError(t, err, "Should not error")
}

func TestValidNameOptionInvalidSubdomainError(t *testing.T) {
	t.Parallel()
	err := kube.ValidNameOption("", "not a valid subdomain")
	assert.Error(t, err, "Should error")
}
