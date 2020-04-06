// +build unit

package get

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/logs"
	v1 "k8s.io/api/core/v1"

	jxfake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/tekton_helpers_test"
	"github.com/stretchr/testify/assert"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	"k8s.io/apimachinery/pkg/runtime"
	kubeMocks "k8s.io/client-go/kubernetes/fake"
)

type BuildLogsTestWriter struct {
	StreamLinesLogged []string
	SingleLinesLogged []string
}

func TestGetTektonLogsForRunningBuild(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	commonOpts.BatchMode = true
	testCaseDir := path.Join("test_data", "get_build_logs", "tekton_build_logs")

	activities := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	structure := tekton_helpers_test.AssertLoadSinglePipelineStructure(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(activities, structure)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)}
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	pod := tekton_helpers_test.AssertLoadSinglePod(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(pod)

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &commonOpts,
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	_, err := o.getTektonLogs(kubeClient, tektonClient, jxClient, ns)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))

	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)
	assert.Contains(t, output, "Build logs for fakeowner/fakerepo/fakebranch #1")
	for _, c := range containers {
		assert.Contains(t, output, fmt.Sprintf("Showing logs for build fakeowner/fakerepo/fakebranch #1 stage %s and container %s", pod.Labels["jenkins.io/task-stage-name"], c.Name))
	}
}

func TestGetTektonLogsForRunningBuildWithPendingPod(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	commonOpts.BatchMode = true
	testCaseDir := path.Join("test_data", "get_build_logs", "tekton_build_logs_pending")

	activities := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(activities)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)}
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	kubeClient := kubeMocks.NewSimpleClientset()

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &commonOpts,
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	_, err := o.getTektonLogs(kubeClient, tektonClient, jxClient, ns)
	assert.NotNil(t, err)
	assert.Equal(t, "there are no build logs for the supplied filters", err.Error())
}

func TestGetTektonLogsForRunningBuildWithLegacyRepoLabel(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	commonOpts.BatchMode = true
	testCaseDir := path.Join("test_data", "get_build_logs", "tekton_build_logs_legacy_label")

	activities := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	structure := tekton_helpers_test.AssertLoadSinglePipelineStructure(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(activities, structure)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)}
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	pod := tekton_helpers_test.AssertLoadSinglePod(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(pod)

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &commonOpts,
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	_, err := o.getTektonLogs(kubeClient, tektonClient, jxClient, ns)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))

	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)
	assert.Contains(t, output, "Build logs for fakeowner/fakerepo/fakebranch #1")
	for _, c := range containers {
		assert.Contains(t, output, fmt.Sprintf("Showing logs for build fakeowner/fakerepo/fakebranch #1 stage %s and container %s", pod.Labels["jenkins.io/task-stage-name"], c.Name))
	}
}

func TestGetTektonLogsForRunningBuildWithWaitTime(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	commonOpts.BatchMode = true
	testCaseDir := path.Join("test_data", "get_build_logs", "tekton_build_logs")

	activities := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	structure := tekton_helpers_test.AssertLoadSinglePipelineStructure(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(activities, structure)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)}
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	pod := tekton_helpers_test.AssertLoadSinglePod(t, testCaseDir)
	pod2 := pod.DeepCopy()
	pod.Namespace = ""
	kubeClient := kubeMocks.NewSimpleClientset(pod2, pod)

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &commonOpts,
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	_, err := o.getTektonLogs(kubeClient, tektonClient, jxClient, ns)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))

	containers, _, _ := kube.GetContainersWithStatusAndIsInit(pod)
	assert.Contains(t, output, "Build logs for fakeowner/fakerepo/fakebranch #1")
	for _, c := range containers {
		assert.Contains(t, output, fmt.Sprintf("Showing logs for build fakeowner/fakerepo/fakebranch #1 stage %s and container %s", pod.Labels["jenkins.io/task-stage-name"], c.Name))
	}
}

func TestGetTektonLogsForStoredLogs(t *testing.T) {
	t.Skip("Skipping until we find a way to mock the gsutil calls")
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	commonOpts.BatchMode = true
	testCaseDir := path.Join("test_data", "get_build_logs", "tekton_build_logs")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, `Logs stored in a bucket`)
	}))

	pipelineActivity := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	pipelineActivity.Spec.BuildLogsURL = server.URL
	jxClient := jxfake.NewSimpleClientset(pipelineActivity)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)}
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	pod := tekton_helpers_test.AssertLoadSinglePod(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(pod)

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &commonOpts,
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	_, err := o.getTektonLogs(kubeClient, tektonClient, jxClient, ns)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))

	assert.Contains(t, output, "Logs stored in a bucket")
}

func TestWithMetapipeline(t *testing.T) {
	testCaseDir := path.Join("test_data", "get_build_logs", "with-metapipeline")

	activities := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures, activities)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir), tekton_helpers_test.AssertLoadPipelines(t, testCaseDir)}
	tektonObjects = append(tektonObjects, tekton_helpers_test.AssertLoadTasks(t, testCaseDir))
	tektonObjects = append(tektonObjects, tekton_helpers_test.AssertLoadTaskRuns(t, testCaseDir))
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	podList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podList)

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &opts.CommonOptions{
				BatchMode: true,
			},
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout
	o.Args = []string{"fakeowner/fakerepo/fakebranch #1"}

	err := o.getProwBuildLog(kubeClient, tektonClient, jxClient, ns, true)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))
	assert.NotContains(t, output, "stage app-extension")
	assert.Contains(t, output, "stage from-build-pack")
}

func TestGetTektonLogsForRunningBuildWithMultipleStages(t *testing.T) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	commonOpts.BatchMode = true
	testCaseDir := path.Join("test_data", "get_build_logs", "multiple_stages")

	activities := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	structure := tekton_helpers_test.AssertLoadSinglePipelineStructure(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(activities, structure)

	tektonObjects := []runtime.Object{tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)}
	tektonClient := tektonfake.NewSimpleClientset(tektonObjects...)

	pods := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(pods)

	ns := "jx"

	writer := &BuildLogsTestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}

	o := &GetBuildLogsOptions{
		GetOptions: GetOptions{
			CommonOptions: &commonOpts,
		},
		TektonLogger: &logs.TektonLogger{
			KubeClient:        kubeClient,
			JXClient:          jxClient,
			TektonClient:      tektonClient,
			Namespace:         ns,
			LogWriter:         writer,
			LogsRetrieverFunc: LogsProvider,
		},
	}

	r, fakeStdout, _ := os.Pipe()
	log.SetOutput(fakeStdout)
	o.CommonOptions.Out = fakeStdout

	_, err := o.getTektonLogs(kubeClient, tektonClient, jxClient, ns)
	assert.NoError(t, err)

	fakeStdout.Close()
	outBytes, _ := ioutil.ReadAll(r)
	r.Close()
	output := stripansi.Strip(string(outBytes))

	assert.Contains(t, output, "Build logs for abayer/js-test-repo/master #1")
	for _, pod := range pods.Items {
		containers, _, _ := kube.GetContainersWithStatusAndIsInit(&pod)
		for _, c := range containers {
			assert.Contains(t, output, fmt.Sprintf("Showing logs for build abayer/js-test-repo/master #1 stage %s and container %s", pod.Labels["jenkins.io/task-stage-name"], c.Name))
		}
	}
}

func LogsProvider(pod *v1.Pod, container *v1.Container) (io.Reader, func(), error) {
	return bytes.NewReader([]byte("Pod logs...")), func() {
		//nothing to clean
	}, nil
}

func (w *BuildLogsTestWriter) WriteLog(logLine logs.LogLine, lch chan<- logs.LogLine) error {
	w.SingleLinesLogged = append(w.SingleLinesLogged, logLine.Line)
	lch <- logLine
	return nil
}

func (w *BuildLogsTestWriter) StreamLog(lch <-chan logs.LogLine, ech <-chan error) error {
	for {
		select {
		case l, ok := <-lch:
			if !ok {
				return nil
			}
			w.StreamLinesLogged = append(w.StreamLinesLogged, l.Line)
			log.Logger().Info(l.Line)
		case e := <-ech:
			fmt.Println(e)
			continue
		}
	}
}

func (w BuildLogsTestWriter) BytesLimit() int {
	return 0
}
