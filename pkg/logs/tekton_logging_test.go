// +build unit

package logs

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/acarl005/stripansi"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	jxfake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/tekton_helpers_test"
	"github.com/stretchr/testify/assert"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonMocks "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeMocks "k8s.io/client-go/kubernetes/fake"
)

type TestWriter struct {
	StreamLinesLogged []string
	SingleLinesLogged []string
}

const (
	LogsHeadersMultiplier = 2
	FailureLineAddition   = 1
)

func TestGetTektonPipelinesWithActivePipelineActivityNoData(t *testing.T) {
	jxClient, tektonClient, _, _, ns := getFakeClientsAndNs(t)
	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}
	names, paNames, err := tl.GetTektonPipelinesWithActivePipelineActivity([]string{})

	assert.NoError(t, err, "There shouldn't be any error obtaining PipelineActivities and PipelineRuns")
	assert.Empty(t, names, "There shouldn't be any returned build names")
	assert.Empty(t, paNames, "There shouldn't be any returned PipelineActivities")
}

func TestGetTektonPipelinesWithActivePipelineActivitySingleBuild(t *testing.T) {
	testCaseDir := path.Join("test_data", "active_single_run")
	jxClient, _, _, _, ns := getFakeClientsAndNs(t)

	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	_ = assertAndCreatePA1(t, jxClient, ns)

	names, paNames, err := tl.GetTektonPipelinesWithActivePipelineActivity([]string{"context=fakecontext"})

	assert.NoError(t, err, "There shouldn't be any error obtaining PipelineActivities and PipelineRuns")
	assert.Equal(t, "fakeowner/fakerepo/fakebranch #1 fakecontext", names[0], "There should be a match build in the returned names")
	_, exists := paNames[names[0]]
	assert.True(t, exists, "There should be a matching PipelineActivity in the paMap")
	assert.Equal(t, len(names), len(paNames))
}

func TestGetTektonPipelinesWithActivePipelineActivityOnlyWaitingStep(t *testing.T) {
	testCaseDir := path.Join("test_data", "only_waiting_step")
	jxClient, _, _, _, ns := getFakeClientsAndNs(t)

	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	_ = assertAndCreatePA1(t, jxClient, ns)

	names, paNames, err := tl.GetTektonPipelinesWithActivePipelineActivity([]string{"context=fakecontext"})
	assert.NoError(t, err)

	assert.Equal(t, 0, len(names))
	assert.Equal(t, 1, len(paNames))
}

// Based on a real case
func TestGetTektonPipelinesWithActivePipelineActivityInvalidName(t *testing.T) {
	testCaseDir := path.Join("test_data", "invalid_name")
	jxClient, _, _, _, ns := getFakeClientsAndNs(t)

	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	activity := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	_, err := jxClient.JenkinsV1().PipelineActivities(ns).Create(activity)
	assert.NoError(t, err)

	names, paNames, err := tl.GetTektonPipelinesWithActivePipelineActivity([]string{"context=fakecontext"})

	assert.NoError(t, err, "There shouldn't be any error obtaining PipelineActivities and PipelineRuns")
	if assert.Equal(t, len(names), 1, "There should be one found pipeline") {
		assert.Equal(t, "myself/my-awesome-project-org/pr-2 #1 fakecontext", names[0], "There should be a match build in the returned names")
		_, exists := paNames[names[0]]
		assert.True(t, exists, "There should be a matching PipelineActivity in the paMap")
	}
	assert.Equal(t, len(names), len(paNames))
}

func TestGetRunningBuildLogsNoBuildPods(t *testing.T) {
	jxClient, tektonClient, kubeClient, _, ns := getFakeClientsAndNs(t)
	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)
	assert.Error(t, err)
	assert.Equal(t, "the build pods for this build have been garbage collected and the log was not found in the long term storage bucket", err.Error())
}

func TestGetRunningBuildLogsWithPipelineRunButNoBuildPods(t *testing.T) {
	testCaseDir := path.Join("test_data")
	_, _, kubeClient, _, ns := getFakeClientsAndNs(t)

	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		KubeClient:   kubeClient,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)
	assert.Error(t, err)
	assert.Equal(t, "the build pods for this build have been garbage collected and the log was not found in the long term storage bucket", err.Error())
}

func TestGetRunningBuildLogsNoMatchingBuildPods(t *testing.T) {
	testCaseDir := path.Join("test_data")
	jxClient, tektonClient, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)
	assert.Error(t, err)
	assert.Equal(t, "the build pods for this build have been garbage collected and the log was not found in the long term storage bucket", err.Error())
}

func TestGetRunningBuildLogsWithMatchingBuildPods(t *testing.T) {
	// https://github.com/jenkins-x/jx/issues/5171
	testCaseDir := path.Join("test_data")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)

	buildContainers, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[1])
	containersNumber := len(buildContainers) * LogsHeadersMultiplier

	assert.NoError(t, err)
	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged))
}

func TestGetRunningBuildLogsWithMatchingBuildPodsWithFailedContainerInTheMiddle(t *testing.T) {
	// https://github.com/jenkins-x/jx/issues/5171
	testCaseDir := path.Join("test_data", "pod_with_failure")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)

	stepsExecutedBeforeFailure := 4
	containersNumber := stepsExecutedBeforeFailure*LogsHeadersMultiplier + FailureLineAddition

	assert.NoError(t, err)
	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged), "should stop logging after a step has failed")
}

func TestGetRunningBuildLogsWithMatchingBuildPodsWithFailedMetapipeline(t *testing.T) {
	// https://github.com/jenkins-x/jx/issues/5171
	testCaseDir := path.Join("test_data", "metapipeline_failure")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)

	stepsExecutedBeforeFailure := 6
	containersNumber := stepsExecutedBeforeFailure*LogsHeadersMultiplier + FailureLineAddition

	assert.NoError(t, err)
	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged), "should stop logging after a step has failed")
	firstLine := tl.LogWriter.(*TestWriter).StreamLinesLogged[0]
	assert.Regexp(t, "Showing logs for build (?s).* stage app-extension and container (?s).*$",
		stripansi.Strip(firstLine), "Metapipeline failed so 'app-extension' should be the first stage logged")
}

func TestGetRunningBuildLogsForLegacyPipelineRunWithMatchingBuildPods(t *testing.T) {
	// https://github.com/jenkins-x/jx/issues/5171
	testCaseDir := path.Join("test_data", "legacy_pipeline_run")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", false)

	buildContainers, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[1])
	containersNumber := len(buildContainers) * LogsHeadersMultiplier

	assert.NoError(t, err)
	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged))
	firstLine := tl.LogWriter.(*TestWriter).StreamLinesLogged[0]
	assert.Regexp(t, "Showing logs for build (?s).* stage from-fakebranch and container (?s).*$",
		stripansi.Strip(firstLine), "the metapipeline completed successfully so 'from-fakebranch' should be the first stage logged")
}

func TestStreamPipelinePersistentLogsNotInBucket(t *testing.T) {
	_, _, _, commonOptions, _ := getFakeClientsAndNs(t)
	commonOptions.SkipAuthSecretsMerge = true

	lch := make(chan LogLine)
	writer := &TestWriter{
		StreamLinesLogged: make([]string, 0),
		SingleLinesLogged: make([]string, 0),
	}
	tl := TektonLogger{
		LogWriter:   writer,
		logsChannel: lch,
	}

	exampleLogLine := "This is an example log line"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, err := fmt.Fprintf(w, exampleLogLine)
		assert.NoError(t, err)
	}))

	jxClient, ns, err := commonOptions.JXClient()
	assert.NoError(t, err)
	authSvc, err := commonOptions.GitAuthConfigService()
	assert.NoError(t, err)
	err = tl.StreamPipelinePersistentLogs(server.URL, jxClient, ns, authSvc)
	assert.NoError(t, err)

	assert.Contains(t, tl.LogWriter.(*TestWriter).StreamLinesLogged[0], "This is an example log line")
}

func TestStreamPipelinePersistentLogsInUnsupportedBucketProvider(t *testing.T) {
	_, _, _, commonOptions, _ := getFakeClientsAndNs(t)
	commonOptions.SkipAuthSecretsMerge = true
	lch := make(chan LogLine)
	tl := TektonLogger{
		logsChannel: lch,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
	}

	jxClient, ns, err := commonOptions.JXClient()
	assert.NoError(t, err)
	authSvc, err := commonOptions.GitAuthConfigService()
	assert.NoError(t, err)
	err = tl.StreamPipelinePersistentLogs("azblob://nonSupportedBucket", jxClient, ns, authSvc)
	assert.NoError(t, err)
	assert.Contains(t, tl.LogWriter.(*TestWriter).StreamLinesLogged[0], "The provided logsURL scheme is not supported: azblob")
}

func TestGetRunningBuildLogsWithMultipleStages(t *testing.T) {
	// https://github.com/jenkins-x/jx/issues/5171
	testCaseDir := path.Join("test_data", "multiple_stages")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadSinglePipelineRun(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadSinglePipelineStructure(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		KubeClient:   kubeClient,
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := &v1.PipelineActivity{
		ObjectMeta: v12.ObjectMeta{
			Name:      "abayer-js-test-repo-master-1",
			Namespace: ns,
			Labels: map[string]string{
				v1.LabelRepository: "js-test-repo",
				v1.LabelBranch:     "master",
				v1.LabelBuild:      "1",
				v1.LabelOwner:      "abayer",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Build:         "1",
			GitBranch:     "master",
			GitRepository: "js-test-repo",
			GitOwner:      "abayer",
		},
	}

	err := tl.GetRunningBuildLogs(pa, "abayer/js-test-repo/master/1", false)
	assert.NoError(t, err)

	containers1, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[0])
	containers2, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[1])
	containersNumber := (len(containers1) + len(containers2)) * LogsHeadersMultiplier

	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged))
	firstLine := tl.LogWriter.(*TestWriter).StreamLinesLogged[0]
	assert.Regexp(t, "Showing logs for build (?s).* stage build and container (?s).*$",
		stripansi.Strip(firstLine), "'build' should be the first stage logged")
}

func TestGetRunningBuildLogsWithMultipleStagesWithFailureInFirstStage(t *testing.T) {
	testCaseDir := path.Join("test_data", "multiple_stages_with_failure_in_first_stage")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	pa := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures, pa)

	tl := TektonLogger{
		KubeClient:   kubeClient,
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	err := tl.GetRunningBuildLogs(pa, "cb-kubecd/bdd-spring-1568135191/master/1", false)
	assert.NoError(t, err)

	containers1, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[1])
	containersNumber := len(containers1)*LogsHeadersMultiplier + 1 // One additional line for the failure

	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged))
	firstLine := tl.LogWriter.(*TestWriter).StreamLinesLogged[0]
	assert.Regexp(t, "Showing logs for build (?s).* stage from-build-pack and container (?s).*$",
		stripansi.Strip(firstLine), "'from-build-pack' should be the first stage logged")
}

func TestGetRunningBuildLogsWithMultipleStagesFailureActivityDoneRunNotDone(t *testing.T) {
	testCaseDir := path.Join("test_data", "multiple_stages_failure_activity_done_run_not_done")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	pa := tekton_helpers_test.AssertLoadSinglePipelineActivity(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures, pa)

	tl := TektonLogger{
		KubeClient:   kubeClient,
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	err := tl.GetRunningBuildLogs(pa, "cb-kubecd/bdd-spring-1568135191/master/1", false)
	assert.NoError(t, err)

	containers1, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[1])
	containersNumber := len(containers1)*LogsHeadersMultiplier + 1 // One additional line for the failure

	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged))
	firstLine := tl.LogWriter.(*TestWriter).StreamLinesLogged[0]
	assert.Regexp(t, "Showing logs for build (?s).* stage from-build-pack and container (?s).*$",
		stripansi.Strip(firstLine), "'from-build-pack' should be the first stage logged")
}

func TestGetRunningBuildLogsMetapipelineAndPendingGenerated(t *testing.T) {
	testCaseDir := path.Join("test_data", "metapipeline_and_pending_generated")
	_, _, _, _, ns := getFakeClientsAndNs(t)

	podsList := tekton_helpers_test.AssertLoadPods(t, testCaseDir)
	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	kubeClient := kubeMocks.NewSimpleClientset(podsList)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)
	structures := tekton_helpers_test.AssertLoadPipelineStructures(t, testCaseDir)
	jxClient := jxfake.NewSimpleClientset(structures)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		KubeClient:   kubeClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	pa := assertAndCreatePA1(t, jxClient, ns)

	err := tl.GetRunningBuildLogs(pa, "fakeowner/fakerepo/fakebranch/1", true)
	assert.NoError(t, err)

	metapipelineContainers, _, _ := kube.GetContainersWithStatusAndIsInit(&podsList.Items[0])
	containersNumber := len(metapipelineContainers) * LogsHeadersMultiplier

	assert.Equal(t, containersNumber, len(tl.LogWriter.(*TestWriter).StreamLinesLogged))
	firstLine := tl.LogWriter.(*TestWriter).StreamLinesLogged[0]
	assert.Regexp(t, "Showing logs for build (?s).* stage app-extension and container (?s).*$",
		stripansi.Strip(firstLine), "'app-extension' should be the first stage logged")
}

func TestGetTektonPipelinesWithFailedAndRetriedPipeline(t *testing.T) {
	testCaseDir := path.Join("test_data", "failed_and_rerun")
	jxClient, _, _, _, ns := getFakeClientsAndNs(t)

	pipelineRuns := tekton_helpers_test.AssertLoadPipelineRuns(t, testCaseDir)
	tektonClient := tektonMocks.NewSimpleClientset(pipelineRuns)

	tl := TektonLogger{
		JXClient:     jxClient,
		TektonClient: tektonClient,
		Namespace:    ns,
		LogWriter: &TestWriter{
			StreamLinesLogged: make([]string, 0),
			SingleLinesLogged: make([]string, 0),
		},
		LogsRetrieverFunc: LogsProvider,
	}

	_ = assertAndCreatePA1(t, jxClient, ns)

	_, err := jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: v12.ObjectMeta{
			Name:      "PA2",
			Namespace: ns,
			Labels: map[string]string{
				v1.LabelRepository: "fakerepo",
				v1.LabelBranch:     "fakebranch",
				v1.LabelOwner:      "fakeowner",
				v1.LabelContext:    "fakecontext",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Build:         "2",
			GitBranch:     "fakebranch",
			GitRepository: "fakerepo",
			GitOwner:      "fakeowner",
		},
	})
	assert.NoError(t, err)

	names, paNames, err := tl.GetTektonPipelinesWithActivePipelineActivity([]string{"context=fakecontext"})

	assert.NoError(t, err, "There shouldn't be any error obtaining PipelineActivities and PipelineRuns")
	assert.Equal(t, "fakeowner/fakerepo/fakebranch #2 fakecontext", names[0], "There should be a match build in the returned names")
	_, exists := paNames[names[0]]
	assert.True(t, exists, "There should be a matching PipelineActivity in the paMap")
	// The PipelineActivity corresponding to the failed-and-retried PipelineRun is still in the map
	assert.Equal(t, len(names)+1, len(paNames))
}

// Helper method, not supposed to be a test by itself
func getFakeClientsAndNs(t *testing.T) (versioned.Interface, tektonclient.Interface, kubernetes.Interface, opts.CommonOptions, string) {
	commonOpts := opts.NewCommonOptionsWithFactory(fake.NewFakeFactory())
	options := &commonOpts
	testhelpers.ConfigureTestOptions(options, options.Git(), options.Helm())

	jxClient, ns, err := options.JXClientAndDevNamespace()
	assert.NoError(t, err, "There shouldn't be any error getting the fake JXClient and DevEnv")

	tektonClient, _, err := options.TektonClient()
	assert.NoError(t, err, "There shouldn't be any error getting the fake Tekton Client")

	kubeClient, err := options.KubeClient()
	assert.NoError(t, err, "There shouldn't be any error getting the fake Kube Client")

	return jxClient, tektonClient, kubeClient, commonOpts, ns
}

func (w *TestWriter) WriteLog(logLine LogLine, lch chan<- LogLine) error {
	w.SingleLinesLogged = append(w.SingleLinesLogged, logLine.Line)
	lch <- logLine
	return nil
}

func (w *TestWriter) StreamLog(lch <-chan LogLine, ech <-chan error) error {
	for {
		select {
		case l, ok := <-lch:
			if !ok {
				return nil
			}
			w.StreamLinesLogged = append(w.StreamLinesLogged, l.Line)
			log.Logger().Info(l.Line)
		}
	}
}

func (w TestWriter) BytesLimit() int {
	return 0
}

func LogsProvider(pod *corev1.Pod, container *corev1.Container) (io.Reader, func(), error) {
	return bytes.NewReader([]byte(fmt.Sprintf("Writing pod log for pod %s and container %s", pod.Name, container.Name))), func() {
		//nothing to clean
	}, nil
}

func assertAndCreatePA1(t *testing.T, jxClient versioned.Interface, ns string) *v1.PipelineActivity {
	pa, err := jxClient.JenkinsV1().PipelineActivities(ns).Create(&v1.PipelineActivity{
		ObjectMeta: v12.ObjectMeta{
			Name:      "PA1",
			Namespace: ns,
			Labels: map[string]string{
				v1.LabelRepository: "fakerepo",
				v1.LabelBranch:     "fakebranch",
				v1.LabelOwner:      "fakeowner",
				v1.LabelContext:    "fakecontext",
			},
		},
		Spec: v1.PipelineActivitySpec{
			Build:         "1",
			Context:       "fakecontext",
			GitBranch:     "fakebranch",
			GitRepository: "fakerepo",
			GitOwner:      "fakeowner",
		},
	})
	assert.NoError(t, err)

	return pa
}
