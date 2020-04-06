// +build unit

package kube_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestExtractDomain(t *testing.T) {
	t.Parallel()
	values, err := util.LoadBytes("../test_data", "exposecontroller_data.txt")
	assert.NoError(t, err)

	data := make(map[string]string)
	data["config.yml"] = string(values)

	domain, err := kube.ExtractDomainValue(data)
	assert.NoError(t, err)

	assert.Equal(t, domain, "foo.io", "dont match")
}
