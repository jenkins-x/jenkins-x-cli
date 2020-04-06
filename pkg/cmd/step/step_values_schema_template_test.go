// +build unit

package step

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/pborman/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
)

func TestStepValuesSchemaTemplate(t *testing.T) {
	cmName := uuid.New()
	o := StepValuesSchemaTemplateOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		ConfigMapName: cmName,
		Set: []string{
			"defaultName=def",
		},
	}
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())

	schemaTemplate := `{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "default": "{{ .Values.defaultName }}"
    }
  }
}`

	configMap := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: cmName,
		},
		Data: map[string]string{
			"values.schema.json": schemaTemplate,
		},
	}
	kubeClient, ns, err := o.KubeClientAndNamespace()
	assert.NoError(t, err)
	_, err = kubeClient.CoreV1().ConfigMaps(ns).Create(&configMap)
	assert.NoError(t, err)

	err = o.Run()
	assert.NoError(t, err)
	cm, err := kubeClient.CoreV1().ConfigMaps(ns).Get(cmName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, `{
  "$id": "https:/jenkins-x.io/tests/basicTypes.schema.json",
  "$schema": "http://json-schema.org/draft-07/schema#",
  "description": "test values.yaml",
  "type": "object",
  "properties": {
    "name": {
      "type": "string",
      "default": "def"
    }
  }
}`, cm.Data["values.schema.json"])
}
