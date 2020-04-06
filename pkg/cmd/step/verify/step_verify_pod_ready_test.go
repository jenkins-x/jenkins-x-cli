// +build unit

package verify

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStepVerifyPod_WaitForReadyPods(t *testing.T) {
	t.Parallel()

	options := StepVerifyPodReadyOptions{
		ExcludeBuildPods: true,
	}
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	options.CommonOptions = &commonOpts

	labels := make(map[string]string)
	labels[builds.LabelPipelineRunName] = "some-pipeline"

	podList := &corev1.PodList{
		Items: []corev1.Pod{{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "web",
				Namespace: "jx",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "web",
						Image: "nginx:1.12",
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 80,
							},
						},
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		}, {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-pipeline",
				Labels:    labels,
				Namespace: "jx",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "web",
						Image: "nginx:1.12",
						Ports: []corev1.ContainerPort{
							{
								Name:          "http",
								Protocol:      corev1.ProtocolTCP,
								ContainerPort: 80,
							},
						},
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionFalse,
						Reason: "Some container not ready",
					},
				},
			},
		}},
	}

	testhelpers.ConfigureTestOptionsWithResources(options.CommonOptions, []runtime.Object{podList}, nil, gits_test.NewMockGitter(), nil, helm_test.NewMockHelmer(), nil)

	kubeClient, err := options.KubeClient()
	assert.NoError(t, err)
	table, err := options.waitForReadyPods(kubeClient, "jx")
	assert.NoError(t, err, "Command failed: %#v", options)

	rows := table.Rows
	assert.Equal(t, 2, len(rows))
}

func TestStepVerifyPod(t *testing.T) {
	t.Parallel()

	options := StepVerifyPodReadyOptions{}
	// fake the output stream to be checked later
	r, fakeStdout, _ := os.Pipe()
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = fakeStdout
	commonOpts.Err = os.Stderr
	options.CommonOptions = &commonOpts

	testhelpers.ConfigureTestOptions(options.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	err := options.Run()
	assert.NoError(t, err, "Command failed: %#v", options)

	// check output
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	assert.Contains(t, string(outBytes), "POD STATUS")

}

func TestStepVerifyPodDebug(t *testing.T) {
	t.Parallel()

	options := StepVerifyPodReadyOptions{Debug: true}
	// fake the output stream to be checked later
	r, fakeStdout, _ := os.Pipe()
	commonOpts := opts.NewCommonOptionsWithFactory(nil)
	commonOpts.Out = fakeStdout
	commonOpts.Err = os.Stderr
	options.CommonOptions = &commonOpts

	testhelpers.ConfigureTestOptions(options.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	err := options.Run()
	assert.NoError(t, err, "Command failed: %#v", options)

	// check output
	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	assert.Contains(t, string(outBytes), "POD STATUS")

	//check DEBUG file created
	filename := "verify-pod.log"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Error("Debug log does not exist")
	}

	assert.NoError(t, err, "Command failed: %#v", options)

}
