// +build unit

package uninstall_test

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/cmd/uninstall"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/Netflix/go-expect"
	clients_mocks "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	kuber_mocks "github.com/jenkins-x/jx/pkg/kube/mocks"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"

	. "github.com/petergtz/pegomock"
)

var (
	originalJxHome string
	tempJxHome     string
	kubeMock       *kuber_mocks.MockKuber
)

func TestUninstallOptions_Run_ContextSpecifiedAsOption_FailsWhenContextNamesDoNotMatch(t *testing.T) {
	setup(t, "current-context")
	defer tearDown(t)

	o := &uninstall.UninstallOptions{
		CommonOptions: &opts.CommonOptions{},
		Namespace:     "ns",
		Context:       "target-context",
	}
	o.SetKube(kubeMock)
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())

	err := o.Run()
	assert.EqualError(t, err, "the context 'target-context' must match the current context 'current-context' to uninstall")
}

func TestUninstallOptions_Run_ContextSpecifiedAsOption_PassWhenContextNamesMatch(t *testing.T) {
	setup(t, "correct-context-to-delete")
	defer tearDown(t)

	o := &uninstall.UninstallOptions{
		CommonOptions: &opts.CommonOptions{
			BatchMode: true,
		},
		Namespace: "ns",
		Context:   "correct-context-to-delete",
	}
	o.SetKube(kubeMock)
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())

	// Create fake namespace (that we will uninstall from)
	err := createNamespace(o, "ns")

	// Run the uninstall
	err = o.Run()
	assert.NoError(t, err)

	// Assert that the namespace has been deleted
	client, err := o.KubeClient()
	assert.NoError(t, err)
	_, err = client.CoreV1().Namespaces().Get("ns", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestUninstallOptions_Run_ContextSpecifiedAsOption_PassWhenForced(t *testing.T) {
	setup(t, "correct-context-to-delete")
	defer tearDown(t)

	o := &uninstall.UninstallOptions{
		CommonOptions: &opts.CommonOptions{},
		Namespace:     "ns",
		Force:         true,
	}
	o.SetKube(kubeMock)
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())

	// Create fake namespace (that we will uninstall from)
	err := createNamespace(o, "ns")

	// Run the uninstall
	err = o.Run()
	assert.NoError(t, err)

	// Assert that the namespace has been deleted
	client, err := o.KubeClient()
	assert.NoError(t, err)
	_, err = client.CoreV1().Namespaces().Get("ns", metav1.GetOptions{})
	assert.Error(t, err)
}

func TestUninstallOptions_Run_ContextSpecifiedViaCli_FailsWhenContextNamesDoNotMatch(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	setup(t, "current-context")
	defer tearDown(t)

	// mock terminal
	console := tests.NewTerminal(t, nil)
	defer console.Cleanup()

	// Test interactive IO
	donec := make(chan struct{})
	go func() {
		defer close(donec)
		console.ExpectString("Uninstall JX - this command will remove all JX components and delete all namespaces created by Jenkins X. Do you wish to continue?")
		console.SendLine("Y")
		console.ExpectString("This action will permanently delete Jenkins X from the Kubernetes context current-context. Please type in the name of the context to confirm:")
		console.SendLine("target-context")
		console.ExpectEOF()
	}()

	commonOpts := opts.NewCommonOptionsWithFactory(clients_mocks.NewMockFactory())
	commonOpts.In = console.In
	commonOpts.Out = console.Out
	commonOpts.Err = console.Err
	o := &uninstall.UninstallOptions{
		CommonOptions: &commonOpts,
		Namespace:     "ns",
	}

	o.SetKube(kubeMock)

	err := o.Run()
	if !assert.EqualError(t, err, "the context 'target-context' must match the current context 'current-context' to uninstall") {
		// Dump the terminal's screen.
		t.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))
	}

	console.Close()
	<-donec
}

func TestUninstallOptions_Run_ContextSpecifiedViaCli_PassWhenContextNamesMatch(t *testing.T) {
	tests.SkipForWindows(t, "go-expect does not work on windows")

	setup(t, "correct-context-to-delete")
	defer tearDown(t)

	// mock terminal
	console := tests.NewTerminal(t, nil)
	defer console.Cleanup()

	// Test interactive IO
	donec := make(chan struct{})
	//noinspection GoUnhandledErrorResult
	go func() {
		defer close(donec)
		console.ExpectString("Uninstall JX - this command will remove all JX components and delete all namespaces created by Jenkins X. Do you wish to continue?")
		console.SendLine("Y")
		console.ExpectString("This action will permanently delete Jenkins X from the Kubernetes context correct-context-to-delete. Please type in the name of the context to confirm:")
		console.SendLine("correct-context-to-delete")
		console.ExpectEOF()
	}()

	commonOpts := opts.NewCommonOptionsWithFactory(clients_mocks.NewMockFactory())
	commonOpts.In = console.In
	commonOpts.Out = console.Out
	commonOpts.Err = console.Err
	o := &uninstall.UninstallOptions{
		CommonOptions: &commonOpts,
		Namespace:     "ns",
	}

	o.SetKube(kubeMock)
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), helm_test.NewMockHelmer())
	o.BatchMode = false // The above line sets batch mode to true. Set it back here :-(

	// Create fake namespace (that we will uninstall from)
	err := createNamespace(o, "ns")

	// Run the uninstall
	err = o.Run()
	assert.NoError(t, err)

	// Assert that the namespace has been deleted
	client, err := o.KubeClient()
	assert.NoError(t, err)
	_, err = client.CoreV1().Namespaces().Get("ns", metav1.GetOptions{})
	if !assert.Error(t, err) {
		// Dump the terminal's screen.
		t.Logf(expect.StripTrailingEmptyLines(console.CurrentState()))
	}

	console.Close()
	<-donec
}

func TestDeleteReleaseIfPresent(t *testing.T) {
	RegisterMockTestingT(t)
	commonOpts := opts.NewCommonOptionsWithFactory(clients_mocks.NewMockFactory())
	o := &uninstall.UninstallOptions{
		CommonOptions: &commonOpts,
		Namespace:     "ns",
	}

	mHelm := helm_test.NewMockHelmer()
	When(mHelm.StatusRelease(EqString("ns"), EqString("chart"))).ThenReturn(nil)
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), mHelm)

	errs := o.DeleteReleaseIfPresent("ns", "chart", []error{}, false)
	assert.Len(t, errs, 0)

	mHelm.VerifyWasCalledOnce().DeleteRelease(EqString("ns"), EqString("chart"), EqBool(true))
}

func TestForceDeleteReleaseIfPresentNotFound(t *testing.T) {
	RegisterMockTestingT(t)
	commonOpts := opts.NewCommonOptionsWithFactory(clients_mocks.NewMockFactory())
	o := &uninstall.UninstallOptions{
		CommonOptions: &commonOpts,
		Namespace:     "ns",
	}

	mHelm := helm_test.NewMockHelmer()
	When(mHelm.StatusRelease(EqString("ns"), EqString("chart2"))).ThenReturn(errors.New("chart not found"))
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), mHelm)

	errs := o.DeleteReleaseIfPresent("ns", "chart2", []error{}, true)
	assert.Len(t, errs, 0)

	mHelm.VerifyWasCalledOnce().DeleteRelease(EqString("ns"), EqString("chart2"), EqBool(true))
}

func TestDeleteReleaseNotFound(t *testing.T) {
	RegisterMockTestingT(t)
	commonOpts := opts.NewCommonOptionsWithFactory(clients_mocks.NewMockFactory())
	o := &uninstall.UninstallOptions{
		CommonOptions: &commonOpts,
		Namespace:     "ns",
	}

	mHelm := helm_test.NewMockHelmer()
	When(mHelm.StatusRelease(EqString("ns"), EqString("chart3"))).ThenReturn(errors.New("chart not found"))
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), mHelm)

	output := log.CaptureOutput(func() {
		errs := o.DeleteReleaseIfPresent("ns", "chart3", []error{}, false)
		assert.Len(t, errs, 0)
	})

	assert.Equal(t, "WARNING: Not deleting chart3 because the release is not installed\n", output)
	mHelm.VerifyWasCalled(Never()).DeleteRelease(EqString("ns"), EqString("chart3"), EqBool(true))
}

func TestForceDeleteReleaseNotFound(t *testing.T) {
	RegisterMockTestingT(t)
	commonOpts := opts.NewCommonOptionsWithFactory(clients_mocks.NewMockFactory())
	o := &uninstall.UninstallOptions{
		CommonOptions: &commonOpts,
		Namespace:     "ns",
	}

	mHelm := helm_test.NewMockHelmer()
	When(mHelm.StatusRelease(EqString("ns"), EqString("chart4"))).ThenReturn(errors.New("chart not found"))
	When(mHelm.DeleteRelease(EqString("ns"), EqString("chart4"), EqBool(true))).ThenReturn(errors.New("chart not found"))
	testhelpers.ConfigureTestOptions(o.CommonOptions, gits_test.NewMockGitter(), mHelm)

	errs := o.DeleteReleaseIfPresent("ns", "chart4", []error{}, true)
	assert.Len(t, errs, 1)

	assert.Equal(t, "failed to uninstall the chart4 helm chart in namespace ns: chart not found", errs[0].Error())
	mHelm.VerifyWasCalledOnce().DeleteRelease(EqString("ns"), EqString("chart4"), EqBool(true))
}

func setup(t *testing.T, context string) {
	log.SetOutput(ioutil.Discard)

	var err error
	originalJxHome, tempJxHome, err = testhelpers.CreateTestJxHomeDir()
	assert.NoError(t, err)
	kubeMock = setupUninstall(context)
}

func tearDown(t *testing.T) {
	err := testhelpers.CleanupTestJxHomeDir(originalJxHome, tempJxHome)
	assert.NoError(t, err)
}

func setupUninstall(contextName string) *kuber_mocks.MockKuber {
	kubeMock := kuber_mocks.NewMockKuber()
	fakeKubeConfig := api.NewConfig()
	fakeKubeConfig.CurrentContext = contextName
	When(kubeMock.LoadConfig()).ThenReturn(fakeKubeConfig, nil, nil)
	return kubeMock
}

func createNamespace(o *uninstall.UninstallOptions, ns string) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	_, err = client.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		},
	})
	return err
}
