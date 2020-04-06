// +build unit

package get_test

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	resources_mock "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/test-infra/prow/config"
)

func TestGetPipelinesWithProw(t *testing.T) {
	o := get.GetPipelineOptions{}

	// fake the output stream to be checked later
	r, fakeStdout, _ := os.Pipe()
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = fakeStdout
	commonOpts.Err = os.Stderr
	o.CommonOptions = &commonOpts

	mockProwConfig(&o, t)
	err := o.Run()
	assert.NoError(t, err)

	// check output
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	assert.Contains(t, string(outBytes), "test/repo/master")
}

// fake prow config with a release job (test/repo/master)
func mockProwConfig(o *get.GetPipelineOptions, t *testing.T) {
	devEnv := kube.NewPermanentEnvironment(kube.LabelValueDevEnvironment)
	devEnv.Spec.Kind = v1.EnvironmentKindTypeDevelopment
	devEnv.Spec.PromotionStrategy = v1.PromotionStrategyTypeNever
	devEnv.Spec.Namespace = "jx"

	// this makes o.isProw == true
	devEnv.Spec.TeamSettings.PromotionEngine = v1.PromotionEngineProw

	// prow configmap
	ps := config.Postsubmit{}
	ps.Branches = []string{"master"}
	ps.Name = "release"
	prowConfig := &config.Config{}
	prowConfig.Postsubmits = make(map[string][]config.Postsubmit)
	prowConfig.Postsubmits["test/repo"] = []config.Postsubmit{ps}
	configYAML, err := yaml.Marshal(&prowConfig)
	data := make(map[string]string)
	data[prow.ProwConfigFilename] = string(configYAML)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      prow.ProwConfigMapName,
			Namespace: "jx",
		},
		Data: data,
	}

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{
			cm,
		},
		[]runtime.Object{
			devEnv,
		},
		&gits.GitFake{},
		nil,
		helm_test.NewMockHelmer(),
		resources_mock.NewMockInstaller(),
	)
	_, _, err = o.JXClientAndDevNamespace()
	assert.NoError(t, err)
}
