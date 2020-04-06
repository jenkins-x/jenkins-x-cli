// +build unit

package builds

import (
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/tekton/tekton_helpers_test"
	"github.com/stretchr/testify/assert"
	kubeMocks "k8s.io/client-go/kubernetes/fake"
)

func TestGetPipelineRunPods(t *testing.T) {
	testCaseDir := path.Join("test_data", "get_pipelinerun_pods")

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)

	prPods, err := GetPipelineRunPods(kubeClient, "jx", "abayer-js-test-repo-master-1")
	assert.NoError(t, err)
	assert.Len(t, prPods, 2)
}
