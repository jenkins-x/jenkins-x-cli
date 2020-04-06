// +build unit

package kube_test

import (
	"testing"

	"github.com/Jeffail/gabs"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
)

func TestInsecureRegistry(t *testing.T) {
	t.Parallel()
	registry := "foo.bar.com"

	nodeJSON := "[\\n      {\\n        \\\"Effect\\\": \\\"Allow\\\",\\n        \\\"Action\\\": [\\\"ecr:InitiateLayerUpload\\\", \\\"ecr:UploadLayerPart\\\",\\\"ecr:CompleteLayerUpload\\\",\\\"ecr:PutImage\\\"],\\n        \\\"Resource\\\": [\\\"*\\\"]\\n      }\\n    ]"

	input := `{"kind":"InstanceGroup","apiVersion":"kops/v1alpha2","metadata":{"name":"nodes","creationTimestamp":"2018-03-14T19:30:51Z","labels":{"kops.k8s.io/cluster":"aws1.cluster.k8s.local"}},"spec":{"role":"Node","image":"kope.io/k8s-1.8-debian-jessie-amd64-hvm-ebs-2018-02-08","minSize":2,"maxSize":2,"machineType":"t2.medium","subnets":["eu-west-1a","eu-west-1b","eu-west-1c"],"nodeLabels":{"kops.k8s.io/instancegroup":"nodes"}}}`

	output := `{"kind":"InstanceGroup","apiVersion":"kops/v1alpha2","metadata":{"name":"nodes","creationTimestamp":"2018-03-14T19:30:51Z","labels":{"kops.k8s.io/cluster":"aws1.cluster.k8s.local"}},"spec":{"additionalPolicies":{"node":"` + nodeJSON + `"},"role":"Node","image":"kope.io/k8s-1.8-debian-jessie-amd64-hvm-ebs-2018-02-08","minSize":2,"maxSize":2,"machineType":"t2.medium","subnets":["eu-west-1a","eu-west-1b","eu-west-1c"],"nodeLabels":{"kops.k8s.io/instancegroup":"nodes"},"docker":{"insecureRegistry":"` + registry + `"}}}`

	// lets parse and output the JSON to ensure the same ordering when testing the results
	outputModel, err := gabs.ParseJSON([]byte(output))
	assert.Nil(t, err)
	expected := outputModel.String()

	result, err := kube.EnableInsecureRegistry(input, registry)
	assert.Nil(t, err)

	assert.Equal(t, expected, result, "adding insecure registry for %s", registry)
}
