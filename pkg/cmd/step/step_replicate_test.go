// +build unit

package step

import (
	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/tekton/tekton_helpers_test"
	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kube_mocks "k8s.io/client-go/kubernetes/fake"

	"path"
	"testing"
)

func TestSetReplicatorAnnotationsNoExistingAnnotations(t *testing.T) {

	existingAnnotations := make(map[string]string)
	namespacesToReplicate := []string{"staging", "production"}
	updatedAnnotations := setReplicatorAnnotations(existingAnnotations, namespacesToReplicate)

	assert.Equal(t, updatedAnnotations[annotationReplicationAllowed], "true")
	assert.Equal(t, updatedAnnotations[annotationEeplicationAllowedNamespaces], "staging,production")
}

func TestSetReplicatorAnnotationsWithExistingAnnotations(t *testing.T) {

	existingAnnotations := make(map[string]string)
	existingAnnotations[annotationEeplicationAllowedNamespaces] = "foo"

	namespacesToReplicate := []string{"staging", "production"}
	updatedAnnotations := setReplicatorAnnotations(existingAnnotations, namespacesToReplicate)

	assert.Equal(t, updatedAnnotations[annotationReplicationAllowed], "true")
	assert.Equal(t, updatedAnnotations[annotationEeplicationAllowedNamespaces], "foo,staging,production")
}

func TestSetReplicatorAnnotationsWithWildcard(t *testing.T) {

	existingAnnotations := make(map[string]string)

	namespacesToReplicate := []string{"staging", "production", "preview*"}
	updatedAnnotations := setReplicatorAnnotations(existingAnnotations, namespacesToReplicate)

	assert.Equal(t, updatedAnnotations[annotationReplicationAllowed], "true")
	assert.Equal(t, updatedAnnotations[annotationEeplicationAllowedNamespaces], "staging,production,preview*")
}

func TestSetReplicatorAnnotationsNoDuplicates(t *testing.T) {

	existingAnnotations := make(map[string]string)
	existingAnnotations[annotationReplicationAllowed] = "true"
	existingAnnotations[annotationEeplicationAllowedNamespaces] = "staging"

	namespacesToReplicate := []string{"staging", "production", "preview*"}
	updatedAnnotations := setReplicatorAnnotations(existingAnnotations, namespacesToReplicate)

	assert.Equal(t, updatedAnnotations[annotationReplicationAllowed], "true")
	assert.Equal(t, updatedAnnotations[annotationEeplicationAllowedNamespaces], "staging,production,preview*")
}

func TestRun(t *testing.T) {
	o := ReplicateOptions{
		ReplicateToNamepace: []string{"bar"},
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}

	currentNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "foo",
		},
	}

	// the namespace we want to replicate secrets into, we have validation to make sure a namespace exists
	stagingNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
		},
	}

	// load a secret that we will annotate
	testCaseDir := path.Join("test_data", "step_replicate")
	secret := tekton_helpers_test.AssertLoadSecret(t, testCaseDir)

	// setup mocks
	factory := cmd_mocks.NewMockFactory()
	kubernetesInterface := kube_mocks.NewSimpleClientset(currentNamespace)

	// create resources used in the test
	_, err := kubernetesInterface.CoreV1().Secrets("foo").Create(secret)
	assert.NoError(t, err)
	_, err = kubernetesInterface.CoreV1().Namespaces().Create(stagingNamespace)
	assert.NoError(t, err)

	// return our fake kubernetes client in the test
	When(factory.CreateKubeClient()).ThenReturn(kubernetesInterface, "foo", nil)

	// represents the command `jx step replicate secret tls-foo`
	o.Args = []string{"secret", "tls-foo"}
	o.SetFactory(factory)

	// run the command
	err = o.Run()
	assert.NoError(t, err)

	secret, err = kubernetesInterface.CoreV1().Secrets("foo").Get("tls-foo", metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, secret.Annotations[annotationReplicationAllowed], "true")
	assert.Equal(t, secret.Annotations[annotationEeplicationAllowedNamespaces], "bar")
}

func TestRunWithWildcard(t *testing.T) {
	o := ReplicateOptions{
		ReplicateToNamepace: []string{"wine*"},
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}

	currentNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "foo",
		},
	}

	// load a secret that we will annotate
	testCaseDir := path.Join("test_data", "step_replicate")
	secret := tekton_helpers_test.AssertLoadSecret(t, testCaseDir)

	// setup mocks
	factory := cmd_mocks.NewMockFactory()
	kubernetesInterface := kube_mocks.NewSimpleClientset(currentNamespace)

	// create resources used in the test
	_, err := kubernetesInterface.CoreV1().Secrets("foo").Create(secret)
	assert.NoError(t, err)

	// return our fake kubernetes client in the test
	When(factory.CreateKubeClient()).ThenReturn(kubernetesInterface, "foo", nil)

	// represents the command `jx step replicate secret tls-foo`
	o.Args = []string{"secret", "tls-foo"}
	o.SetFactory(factory)

	// run the command
	err = o.Run()
	assert.NoError(t, err)

	secret, err = kubernetesInterface.CoreV1().Secrets("foo").Get("tls-foo", metav1.GetOptions{})
	assert.NoError(t, err)

	assert.Equal(t, secret.Annotations[annotationReplicationAllowed], "true")
	assert.Equal(t, secret.Annotations[annotationEeplicationAllowedNamespaces], "wine*")
}
