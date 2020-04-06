// +build unit

package upgrade_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/upgrade"

	"github.com/jenkins-x/jx/pkg/kube/services"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

type TestOptions struct {
	upgrade.UpgradeIngressOptions
	Service *v1.Service
}

func (o *TestOptions) Setup() {
	o.UpgradeIngressOptions = upgrade.UpgradeIngressOptions{
		CommonOptions: &opts.CommonOptions{},
		IngressConfig: kube.IngressConfig{
			Issuer: "letsencrypt-prod",
		},
		TargetNamespaces: []string{"test"},
	}
	o.SetKubeClient(testclient.NewSimpleClientset())

	o.Service = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Spec: v1.ServiceSpec{},
	}

	o.Service.Annotations = map[string]string{}
	o.Service.Annotations[services.ExposeAnnotation] = "true"
}

func TestAnnotateNoExisting(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()

	client, err := o.KubeClient()
	assert.NoError(t, err)
	_, err = client.CoreV1().Services("test").Create(o.Service)
	assert.NoError(t, err)

	err = o.CleanServiceAnnotations()
	assert.NoError(t, err)

	_, err = o.AnnotateExposedServicesWithCertManager()
	assert.NoError(t, err)

	rs, err := client.CoreV1().Services("test").Get("foo", metav1.GetOptions{})
	ingressAnnotations := rs.Annotations[services.ExposeIngressAnnotation]

	assert.Equal(t, "certmanager.k8s.io/issuer: letsencrypt-prod", ingressAnnotations)
	assert.NoError(t, err)
}

func TestAnnotateWithExistingAnnotations(t *testing.T) {

	o := TestOptions{}
	o.Setup()

	o.Service.Annotations[services.ExposeIngressAnnotation] = "foo: bar\nkubernetes.io/ingress.class: nginx\nnginx.ingress.kubernetes.io/proxy-body-size: 500m"

	client, err := o.KubeClient()
	assert.NoError(t, err)

	_, err = client.CoreV1().Services("test").Create(o.Service)
	assert.NoError(t, err)

	err = o.CleanServiceAnnotations()
	assert.NoError(t, err)

	_, err = o.AnnotateExposedServicesWithCertManager()
	assert.NoError(t, err)

	rs, err := client.CoreV1().Services("test").Get("foo", metav1.GetOptions{})
	ingressAnnotations := rs.Annotations[services.ExposeIngressAnnotation]

	assert.Equal(t, "foo: bar\nkubernetes.io/ingress.class: nginx\nnginx.ingress.kubernetes.io/proxy-body-size: 500m\ncertmanager.k8s.io/issuer: letsencrypt-prod", ingressAnnotations)
	assert.NoError(t, err)
}

func TestAnnotateWithExistingCertManagerAnnotation(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()

	o.Service.Annotations[services.ExposeIngressAnnotation] = services.CertManagerAnnotation + ": letsencrypt-staging"

	client, err := o.KubeClient()
	assert.NoError(t, err)

	_, err = client.CoreV1().Services("test").Create(o.Service)
	assert.NoError(t, err)

	err = o.CleanServiceAnnotations()
	assert.NoError(t, err)

	_, err = o.AnnotateExposedServicesWithCertManager()
	assert.NoError(t, err)

	rs, err := client.CoreV1().Services("test").Get("foo", metav1.GetOptions{})
	ingressAnnotations := rs.Annotations[services.ExposeIngressAnnotation]

	assert.Equal(t, "certmanager.k8s.io/issuer: letsencrypt-prod", ingressAnnotations)
	assert.NoError(t, err)
}

func TestCleanExistingExposecontrollerReources(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()

	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "exposecontroller",
		},
	}
	client, err := o.KubeClient()
	assert.NoError(t, err)

	_, err = client.CoreV1().ConfigMaps("test").Create(&cm)
	assert.NoError(t, err)
	o.CleanExposecontrollerReources("test")

	_, err = client.CoreV1().ConfigMaps("test").Get("exposecontroller", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestCleanServiceAnnotations(t *testing.T) {
	t.Parallel()
	o := TestOptions{}
	o.Setup()

	o.Service.Annotations[services.ExposeURLAnnotation] = "http://foo.bar"

	client, err := o.KubeClient()
	assert.NoError(t, err)

	_, err = client.CoreV1().Services("test").Create(o.Service)
	assert.NoError(t, err)

	err = o.CleanServiceAnnotations()
	assert.NoError(t, err)

	rs, err := client.CoreV1().Services("test").Get("foo", metav1.GetOptions{})

	assert.Empty(t, rs.Annotations[services.ExposeURLAnnotation])
	assert.NoError(t, err)
}

func TestRealJenkinsService(t *testing.T) {
	t.Parallel()
	filename := filepath.Join(".", "test_data", "upgrade_ingress", "service.yaml")
	serviceYaml, err := ioutil.ReadFile(filename)
	assert.NoError(t, err)

	var svc *v1.Service
	err = yaml.Unmarshal(serviceYaml, &svc)
	assert.NoError(t, err)

	o := TestOptions{}
	o.Setup()

	o.Service = svc

	client, err := o.KubeClient()
	assert.NoError(t, err)

	_, err = client.CoreV1().Services("test").Create(o.Service)
	assert.NoError(t, err)

	err = o.CleanServiceAnnotations()
	assert.NoError(t, err)

	_, err = o.AnnotateExposedServicesWithCertManager()
	assert.NoError(t, err)

	rs, err := client.CoreV1().Services("test").Get("jenkins", metav1.GetOptions{})
	ingressAnnotations := rs.Annotations[services.ExposeIngressAnnotation]

	assert.Equal(t, "kubernetes.io/ingress.class: nginx\nnginx.ingress.kubernetes.io/proxy-body-size: 500m\ncertmanager.k8s.io/issuer: letsencrypt-prod", ingressAnnotations)
	assert.NoError(t, err)
}

func TestReturnUserNameIfPicked_notPicked(t *testing.T) {
	t.Parallel()
	organisation := "MyOrg"
	username := "MyUser"

	actual := upgrade.ReturnUserNameIfPicked(organisation, username)
	expected := ""
	assert.Equal(t, expected, actual, "Username should be empty an organization was picked")
}

func TestReturnUserNameIfPicked_wasPicked(t *testing.T) {
	t.Parallel()
	organisation := ""
	username := "MyUser"

	actual := upgrade.ReturnUserNameIfPicked(organisation, username)
	expected := username
	assert.Equal(t, expected, actual, "Username should be returned as no organization was picked")
}
