// +build unit

package step_test

import (
	"testing"

	step2 "github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	fromNs                   = "from-namespace-powdered-water"
	toNs                     = "to-namespace-journal-entry"
	serviceNameInFromNs      = "service-a-is-for-angry"
	serviceNameDummyInFromNs = "service-p-is-polluted"
	serviceNameInToNs        = "service-b-is-for-berserk"
)

func TestServiceLinking(t *testing.T) {
	t.Parallel()
	o := step.StepLinkServicesOptions{
		StepOptions: step2.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		FromNamespace: fromNs,
		Includes:      []string{serviceNameInFromNs},
		Excludes:      []string{serviceNameDummyInFromNs},
		ToNamespace:   toNs}
	fromNspc := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fromNs,
		},
	}
	svcInFromNs := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceNameInFromNs,
			Namespace: fromNs,
		},
	}
	svcDummyInFromNs := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceNameDummyInFromNs,
			Namespace: fromNs,
		},
	}
	toNspc := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: toNs,
		},
	}
	svcInToNs := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceNameInToNs,
			Namespace: toNs,
		},
	}

	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		[]runtime.Object{fromNspc, toNspc, svcInFromNs, svcInToNs, svcDummyInFromNs},
		nil,
		gits.NewGitCLI(),
		nil,
		helm.NewHelmCLI("helm", helm.V2, "", true),
		resources_test.NewMockInstaller())
	client, err := o.KubeClient()
	assert.NoError(t, err)
	serviceListFromNsBeforeStepLink, _ := client.CoreV1().Services(fromNs).List(metav1.ListOptions{})
	assert.EqualValues(t, len(serviceListFromNsBeforeStepLink.Items), 2)
	serviceListToNsBeforeStepLink, _ := client.CoreV1().Services(toNs).List(metav1.ListOptions{})
	assert.EqualValues(t, len(serviceListToNsBeforeStepLink.Items), 1)
	err = o.Run()
	serviceList, _ := client.CoreV1().Services(toNs).List(metav1.ListOptions{})
	serviceNames := []string{""}
	for _, service := range serviceList.Items {
		serviceNames = append(serviceNames, service.Name)
	}
	serviceListToNsAfterStepLink, _ := client.CoreV1().Services(toNs).List(metav1.ListOptions{})
	assert.EqualValues(t, len(serviceListToNsAfterStepLink.Items), 2)

	assert.Contains(t, serviceNames, serviceNameInFromNs) //Check if service that was in include list got added
	assert.EqualValues(t, len(serviceNames), 3)           //Check if service that was in exclude list didn't get added
	assert.NoError(t, err)
}
