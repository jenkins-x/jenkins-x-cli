// +build unit

package get

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	cmd_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextentions_mocks "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dymamic_mocks "k8s.io/client-go/dynamic/fake"
	kube_mocks "k8s.io/client-go/kubernetes/fake"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
)

const (
	group = "wine.io"
)

func TestRun(t *testing.T) {
	o := CRDCountOptions{
		CommonOptions: &opts.CommonOptions{},
	}

	scheme := runtime.NewScheme()

	// setup mocks
	factory := cmd_mocks.NewMockFactory()
	kubernetesInterface := kube_mocks.NewSimpleClientset(getNamespace("cellar"), getNamespace("cellarx"))
	apiextensionsInterface := apiextentions_mocks.NewSimpleClientset(getClusterScopedCRD(), getNamespaceScopedCRD())
	dynamicInterface := dymamic_mocks.NewSimpleDynamicClient(scheme)
	r := schema.GroupVersionResource{Group: group, Version: "v1", Resource: "rioja"}

	_, err := dynamicInterface.Resource(r).Namespace("cellar").Create(getNamespaceResource("test1"), metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = dynamicInterface.Resource(r).Namespace("cellarx").Create(getNamespaceResource("test2"), metav1.CreateOptions{})
	assert.NoError(t, err)

	r = schema.GroupVersionResource{Group: group, Version: "v1", Resource: "shiraz"}

	_, err = dynamicInterface.Resource(r).Create(getClusterResource("test3"), metav1.CreateOptions{})
	assert.NoError(t, err)

	// return our fake kubernetes client in the test
	When(factory.CreateKubeClient()).ThenReturn(kubernetesInterface, "cellar", nil)
	When(factory.CreateApiExtensionsClient()).ThenReturn(apiextensionsInterface, nil)
	When(factory.CreateDynamicClient()).ThenReturn(dynamicInterface, "cellar", nil)

	o.SetFactory(factory)

	// run the command
	rs, err := o.getCustomResourceCounts()
	assert.NoError(t, err)

	// the order is important here, larger counts should appear at the bottom of the list so we can see them sooner
	clusterScopedLine := rs[0]
	namespace1ScopedLine := rs[1]
	namespace2ScopedLine := rs[2]

	assert.Equal(t, 1, clusterScopedLine.count)
	assert.Equal(t, 1, namespace1ScopedLine.count)
	assert.Equal(t, 1, namespace2ScopedLine.count)

}

func getNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: name,
		},
	}
}

func getNamespaceScopedCRD() *v1beta1.CustomResourceDefinition {
	return &v1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rioja",
		},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group: group,
			Versions: []v1beta1.CustomResourceDefinitionVersion{
				{
					Name: "v1",
				},
			},
			Scope: v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: "rioja",
			},
		},
	}
}

func getClusterScopedCRD() *v1beta1.CustomResourceDefinition {
	return &v1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "shiraz",
		},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group: group,
			Versions: []v1beta1.CustomResourceDefinitionVersion{
				{
					Name: "v1",
				},
			},
			Scope: v1beta1.ClusterScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: "shiraz",
			},
		},
	}
}

func getNamespaceResource(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "wine.io/v1",
			"kind":       "rioja",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

func getClusterResource(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "wine.io/v1",
			"kind":       "shiraz",
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}
