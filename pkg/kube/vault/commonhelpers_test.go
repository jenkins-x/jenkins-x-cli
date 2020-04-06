// +build unit

package vault_test

import (
	"testing"

	"github.com/banzaicloud/bank-vaults/operator/pkg/apis/vault/v1alpha1"
	"github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned/fake"
	cmdMocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	kubevault "github.com/jenkins-x/jx/pkg/kube/vault"
	. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/kubernetes"
)

func setupMocks(t *testing.T, term *terminal.Stdio) (*fake.Clientset, *kubevault.VaultClientFactory, kubernetes.Interface, error) {
	commonOpts := opts.NewCommonOptionsWithFactory(cmdMocks.NewMockFactory())
	options := &commonOpts
	if term != nil {
		options.In, options.Out, options.Err = term.In, term.Out, term.Err
	}
	testhelpers.ConfigureTestOptions(options, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	vaultOperatorClient := fake.NewSimpleClientset()
	When(options.VaultOperatorClient()).ThenReturn(vaultOperatorClient, nil)
	f, err := kubevault.NewInteractiveVaultClientFactory(options)
	kubeClient, err := options.KubeClient()
	assert.NoError(t, err)
	return vaultOperatorClient, f, kubeClient, err
}

func createMockedVault(vaultName string, namespace string, vaultUrl string, jwt string,
	vaultOperatorClient *fake.Clientset, kubeClient kubernetes.Interface) v1alpha1.Vault {

	role := map[string]interface{}{"name": vaultName + "-auth-sa"}
	auth := map[string]interface{}{"roles": []interface{}{role}}
	v := v1alpha1.Vault{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vaultName,
			Namespace: namespace,
		},
		Spec: v1alpha1.VaultSpec{
			ExternalConfig: map[string]interface{}{
				"auth": []interface{}{auth},
			},
		},
	}
	secretName := vaultName + "-secret"
	_, _ = vaultOperatorClient.Vault().Vaults(namespace).Create(&v)
	serviceAccountName := vaultName + "-auth-sa"
	_, _ = kubeClient.CoreV1().ServiceAccounts(namespace).Create(&v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName},
		Secrets:    []v1.ObjectReference{{Name: secretName}},
	})
	_, _ = kubeClient.CoreV1().Services(namespace).Create(&v1.Service{ObjectMeta: metav1.ObjectMeta{Name: vaultName}})
	_, _ = kubeClient.ExtensionsV1beta1().Ingresses(namespace).Create(&v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: vaultName},
		Spec:       v1beta1.IngressSpec{Rules: []v1beta1.IngressRule{{Host: vaultUrl}}},
	})
	_, _ = kubeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Annotations: map[string]string{"kubernetes.io/service-account.name": serviceAccountName},
		},
		Data: map[string][]byte{"token": []byte(jwt)},
	})
	return v
}
