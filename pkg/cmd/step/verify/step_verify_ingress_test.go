// +build unit

package verify_test

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/verify"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"

	"github.com/jenkins-x/jx/pkg/config"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVerifyIngress(t *testing.T) {
	testData := path.Join("test_data", "verify_ingress")
	assert.DirExists(t, testData)

	outputDir, err := ioutil.TempDir("", "test-step-verify-ingress-")
	require.NoError(t, err)

	err = util.CopyDir(testData, outputDir, true)
	require.NoError(t, err, "failed to copy test data into temp dir")

	o := &verify.StepVerifyIngressOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &opts.CommonOptions{
				In:  os.Stdin,
				Out: os.Stdout,
				Err: os.Stderr,
			},
		},
		Dir:              outputDir,
		Namespace:        "jx",
		IngressNamespace: opts.DefaultIngressNamesapce,
		IngressService:   opts.DefaultIngressServiceName,
	}

	ingressHostName := "1.2.3.4"
	//expectedDomain := ingressHostName + ".nip.io"

	runtimeObjects := []runtime.Object{
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      opts.DefaultIngressServiceName,
				Namespace: opts.DefaultIngressNamesapce,
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{
						{
							Hostname: ingressHostName,
						},
					},
				},
			},
		},
	}
	testhelpers.ConfigureTestOptionsWithResources(o.CommonOptions,
		runtimeObjects,
		nil,
		gits.NewGitCLI(),
		nil,
		helm.NewHelmCLI("helm", helm.V2, "", true),
		resources_test.NewMockInstaller(),
	)

	err = o.Run()
	require.NoError(t, err, "failed to run step")
}

func TestExternalDNSDisabledDomainNotOwned(t *testing.T) {
	t.Parallel()

	commonOpts := opts.CommonOptions{
		BatchMode: false,
	}
	o := verify.StepVerifyIngressOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &commonOpts,
		},
	}

	dir, err := ioutil.TempDir("", "test-requirements-external-")
	assert.NoError(t, err, "should create a temporary config dir")

	o.Dir = dir
	file := filepath.Join(o.Dir, config.RequirementsConfigFileName)
	requirements := getRequirements()

	// using nip.io on gke should disable the use of external dns as we cannot transfer domain ownership to google dns
	requirements.Ingress.Domain = "34.76.24.247.nip.io"
	requirements.Cluster.Provider = "gke"

	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)

	requirements, fileName, err := config.LoadRequirementsConfig(o.Dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", o.Dir)
	assert.FileExists(t, fileName)

	err = o.Run()
	assert.NoError(t, err, "failed to run step in dir %s", o.Dir)

	requirements, fileName, err = config.LoadRequirementsConfig(o.Dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", o.Dir)
	assert.FileExists(t, fileName)

	assert.Equal(t, false, requirements.Ingress.ExternalDNS, "requirements.Ingress.ExternalDNS")

}

func TestExternalDNSDisabledNotGKE(t *testing.T) {
	t.Parallel()

	commonOpts := opts.CommonOptions{
		BatchMode: false,
	}
	o := verify.StepVerifyIngressOptions{
		StepOptions: step.StepOptions{
			CommonOptions: &commonOpts,
		},
	}

	dir, err := ioutil.TempDir("", "test-requirements-external-")
	assert.NoError(t, err, "should create a temporary config dir")

	o.Dir = dir
	file := filepath.Join(o.Dir, config.RequirementsConfigFileName)
	requirements := getRequirements()

	// using nip.io on gke should disable the use of external dns as we cannot transfer domain ownership to google dns
	requirements.Ingress.Domain = "foobar.com"
	requirements.Cluster.Provider = "aws"

	err = requirements.SaveConfig(file)
	assert.NoError(t, err, "failed to save file %s", file)

	requirements, fileName, err := config.LoadRequirementsConfig(o.Dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", o.Dir)
	assert.FileExists(t, fileName)

	err = o.Run()
	assert.NoError(t, err, "failed to run step in dir %s", o.Dir)

	requirements, fileName, err = config.LoadRequirementsConfig(o.Dir)
	assert.NoError(t, err, "failed to load requirements file in dir %s", o.Dir)
	assert.FileExists(t, fileName)

	assert.Equal(t, false, requirements.Ingress.ExternalDNS, "requirements.Ingress.ExternalDNS")

}

func getRequirements() *config.RequirementsConfig {
	requirements := config.NewRequirementsConfig()
	requirements.Cluster.ProjectID = "test-project"
	requirements.Cluster.ClusterName = "test-cluster"
	requirements.Cluster.EnvironmentGitOwner = "test-org"
	requirements.Cluster.Zone = "test-zone"
	return requirements
}

func AssertMapPathValueAsString(t *testing.T, values map[string]interface{}, path string, expected string) {
	actual := util.GetMapValueAsStringViaPath(values, path)
	assert.Equal(t, expected, actual, "invalid helm value for path %s", path)
}
