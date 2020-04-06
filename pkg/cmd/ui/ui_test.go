// +build unit

package ui

import (
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/acarl005/stripansi"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func TestShowErrorMessageIfIngressFound(t *testing.T) {
	_, kubeClient, co, ns := getFakeClientsAndNs(t)

	ingress := &v1beta1.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name: "jxui-ingress",
			Labels: map[string]string{
				"jenkins.io/ui-resource": "true",
			},
		},
	}
	_, err := kubeClient.ExtensionsV1beta1().Ingresses(ns).Create(ingress)
	assert.NoError(t, err, "there shouldn't be an error when creating the ingress")

	o := UIOptions{
		CommonOptions: &co,
	}

	logOutput := log.CaptureOutput(func() {
		err = o.Run()
		assert.NoError(t, err)
	})

	assert.Equal(t, "WARNING: Only single-user mode is available for the UI at this time\n", logOutput,
		"we should show a warning message informing that the UI is not ready to be launched in SSO / TLS mode")
}

func TestGetLocalURL(t *testing.T) {
	jxClient, kubeClient, co, ns := getFakeClientsAndNs(t)

	_, err := kubeClient.CoreV1().Services(ns).Create(&corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: "ui",
			Labels: map[string]string{
				"jenkins.io/ui-resource": "true",
			},
		},
	})
	assert.NoError(t, err, "there shouldn't be an error when creating the service")

	_, err = jxClient.JenkinsV1().Apps(ns).Create(&jenkinsv1.App{
		ObjectMeta: v1.ObjectMeta{
			Name: "jx-app-ui",
			Labels: map[string]string{
				"jenkins.io/ui-resource": "true",
			},
		},
	})
	assert.NoError(t, err, "there shouldn't be an error when creating the app")

	o := UIOptions{
		CommonOptions: &co,
		LocalPort:     "9000",
	}

	listOptions := v1.ListOptions{
		LabelSelector: "jenkins.io/ui-resource=true",
	}

	url, serviceName, err := o.getLocalURL(listOptions)
	assert.NoError(t, err)

	assert.Equal(t, "http://localhost:9000", url, "The local URL should use port 9000")
	assert.Equal(t, "ui", serviceName, "The serviceName should be the same as the UI service")
}

func TestGetLocalURLMissingApp(t *testing.T) {
	_, _, co, _ := getFakeClientsAndNs(t)

	o := UIOptions{
		CommonOptions: &co,
	}
	listOptions := v1.ListOptions{
		LabelSelector: "jenkins.io/ui-resource=true",
	}
	logOutput := log.CaptureOutput(func() {
		o.getLocalURL(listOptions)
	})

	assert.Equal(t, "ERROR: Couldn't find the jx-app-ui app installed in the cluster. Did you add it via jx add app jx-app-ui?\n", stripansi.Strip(logOutput))
}

func TestGetLocalURLMissingService(t *testing.T) {
	jxClient, _, co, ns := getFakeClientsAndNs(t)
	_, err := jxClient.JenkinsV1().Apps(ns).Create(&jenkinsv1.App{
		ObjectMeta: v1.ObjectMeta{
			Name: "jx-app-ui",
			Labels: map[string]string{
				"jenkins.io/ui-resource": "true",
			},
		},
	})
	assert.NoError(t, err, "there shouldn't be an error when creating the app")

	o := UIOptions{
		CommonOptions: &co,
	}
	listOptions := v1.ListOptions{
		LabelSelector: "jenkins.io/ui-resource=true",
	}
	logOutput := log.CaptureOutput(func() {
		o.getLocalURL(listOptions)
	})

	assert.Equal(t, "ERROR: Couldn't find the ui service in the cluster\n", stripansi.Strip(logOutput))
}

func TestChooseLocalPort(t *testing.T) {
	t.Parallel()
	_, _, co, _ := getFakeClientsAndNs(t)
	co.BatchMode = false
	o := UIOptions{
		CommonOptions: &co,
	}
	var timeout = 5 * time.Second
	tests.SkipForWindows(t, "go-expect does not work on windows")
	tests.Retry(t, 5, time.Second*10, func(r *tests.R) {
		console := tests.NewTerminal(r, &timeout)
		defer console.Cleanup()

		o.CommonOptions.In = console.In
		o.CommonOptions.Out = console.Out
		o.CommonOptions.Err = console.Err

		// Test interactive IO
		donec := make(chan struct{})
		// TODO Answer questions
		go func() {
			defer close(donec)
			// Test boolean type
			console.ExpectString("? What local port should the UI be forwarded to? [? for help] (9000)")
			console.SendLine("8080")
			console.ExpectEOF()
		}()

		err := o.decideLocalForwardPort()
		assert.NoError(t, err)

		console.Close()
		<-donec
		r.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))
		assert.Equal(t, "8080", o.LocalPort, "the CLI should prompt the user to choose the port if not in batch mode")
	})
}

// Helper method, not supposed to be a test by itself
func getFakeClientsAndNs(t *testing.T) (versioned.Interface, kubernetes.Interface, opts.CommonOptions, string) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	jxClient, ns, err := options.JXClientAndDevNamespace()
	assert.NoError(t, err, "There shouldn't be any error getting the fake JXClient and DevEnv")

	kubeClient, err := options.KubeClient()
	assert.NoError(t, err, "There shouldn't be any error getting the fake Kube Client")

	return jxClient, kubeClient, commonOpts, ns
}
