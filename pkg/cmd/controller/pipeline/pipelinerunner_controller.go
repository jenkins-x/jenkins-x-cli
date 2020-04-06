package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/tekton"
	"github.com/jenkins-x/jx/pkg/tekton/metapipeline"

	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jenkins-x/jx/pkg/prow"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/sirupsen/logrus"

	"github.com/jenkins-x/jx/pkg/cmd/step/create"

	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"

	jxclient "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	prowapi "k8s.io/test-infra/prow/apis/prowjobs/v1"
	"k8s.io/test-infra/prow/pod-utils/downwardapi"
)

const (
	// healthPath is the URL path for the HTTP endpoint that returns health status.
	healthPath = "/health"
	// readyPath URL path for the HTTP endpoint that returns ready status.
	readyPath = "/ready"

	// jobLabel is the label name used to identify the Prow job within PipelineRunRequest.Labels
	jobLabel = "prowJobName"

	shutdownTimeout = 5
)

var (
	logger = log.Logger().WithFields(logrus.Fields{"component": "pipelinerunner"})
)

// PipelineRunRequest the request to trigger a pipeline run
type PipelineRunRequest struct {
	Labels      map[string]string   `json:"labels,omitempty"`
	ProwJobSpec prowapi.ProwJobSpec `json:"prowJobSpec,omitempty"`
}

// PipelineRunResponse the results of triggering a pipeline run
type PipelineRunResponse struct {
	Resources []kube.ObjectReference `json:"resources,omitempty"`
}

// ObjectReference represents a reference to a k8s resource
type ObjectReference struct {
	APIVersion string `json:"apiVersion" protobuf:"bytes,5,opt,name=apiVersion"`
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Name of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
}

type controller struct {
	bindAddress        string
	path               string
	port               int
	useMetaPipeline    bool
	metaPipelineImage  string
	semanticRelease    bool
	serviceAccount     string
	ns                 string
	jxClient           jxclient.Interface
	metaPipelineClient metapipeline.Client
}

func (c *controller) Start() {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.startWorkers(ctx, &wg, cancel)
	c.setupSignalChannel(cancel)
	wg.Wait()
}

func (c *controller) startWorkers(ctx context.Context, wg *sync.WaitGroup, cancel context.CancelFunc) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		mux := http.NewServeMux()
		mux.Handle(c.path, http.HandlerFunc(c.pipeline))
		mux.Handle(healthPath, http.HandlerFunc(c.health))
		mux.Handle(readyPath, http.HandlerFunc(c.ready))
		srv := &http.Server{
			Addr:    fmt.Sprintf("%s:%d", c.bindAddress, c.port),
			Handler: mux,
		}

		go func() {
			logger.Infof("starting HTTP server on %s port %d", c.bindAddress, c.port)
			logger.Infof("using meta pipeline mode: %t", c.useMetaPipeline)
			if c.metaPipelineImage != "" {
				logger.Infof("using custom pipeline image: %s", c.metaPipelineImage)
			}
			if err := srv.ListenAndServe(); err != nil {
				if err == http.ErrServerClosed {
					logger.Debugf("server closed")
				} else {
					logger.Errorf("unexpected error in HTTP server: %s", err.Error())
				}
				cancel()
				return
			}
		}()

		for {
			select {
			case <-ctx.Done():
				logger.Infof("shutting down HTTP server on %s port %d", c.bindAddress, c.port)
				ctx, cancel := context.WithTimeout(ctx, shutdownTimeout*time.Second)
				_ = srv.Shutdown(ctx)
				if c.metaPipelineClient != nil {
					err := c.metaPipelineClient.Close()
					logger.Error(errors.Wrap(err, "Error closing the meta pipeline client"))
				}
				cancel()
				return
			}
		}
	}()
}

// health returns either HTTP 204 if the service is healthy, otherwise nothing ('cos it's dead).
func (c *controller) health(w http.ResponseWriter, r *http.Request) {
	logger.Trace("health check")
	w.WriteHeader(http.StatusNoContent)
}

// ready returns either HTTP 204 if the service is ready to serve requests, otherwise HTTP 503.
func (c *controller) ready(w http.ResponseWriter, r *http.Request) {
	logger.Trace("ready check")
	w.WriteHeader(http.StatusNoContent)
}

// handle request for pipeline runs
func (c *controller) pipeline(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		_, err := fmt.Fprintf(w, "please POST JSON to this endpoint!\n")
		if err != nil {
			logger.Errorf("unable to write response to GET request: %s", err.Error())
		}
	case http.MethodHead:
		logger.Info("HEAD Todo...")
	case http.MethodPost:
		c.handlePostRequest(r, w)
	default:
		logger.Errorf("unsupported method %s for %s", r.Method, c.path)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	}
}

func (c *controller) handlePostRequest(r *http.Request, w http.ResponseWriter) {
	requestParams, err := c.parseStartPipelineRequestParameters(r)
	if err != nil {
		c.returnStatusBadRequest(err, "could not read the JSON request body: "+err.Error(), w)
		return
	}

	pipelineRunResponse, err := c.startPipeline(requestParams)
	if err != nil {
		c.returnStatusBadRequest(err, "could not start pipeline: "+err.Error(), w)
		return
	}

	data, err := c.marshalPayload(pipelineRunResponse)
	if err != nil {
		c.returnStatusBadRequest(err, "failed to marshal payload", w)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		logger.Errorf("error writing PipelineRunResponse: %s", err.Error())
	}
}

func (c *controller) parseStartPipelineRequestParameters(r *http.Request) (PipelineRunRequest, error) {
	request := PipelineRunRequest{}
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return request, errors.Wrapf(err, fmt.Sprintf("could not read the JSON request body: %s", err.Error()))
	}
	err = json.Unmarshal(data, &request)
	if err != nil {
		return request, errors.Wrapf(err, fmt.Sprintf("failed to unmarshal the JSON request body: %s", err.Error()))
	}
	logger.WithField("payload", util.PrettyPrint(request)).Debug("received PipelineRunRequest payload")
	return request, nil
}

// startPipeline handles an incoming request to start a pipeline.
func (c *controller) startPipeline(pipelineRun PipelineRunRequest) (PipelineRunResponse, error) {
	response := PipelineRunResponse{}
	var revision string
	var prNumber string

	prowJobSpec := pipelineRun.ProwJobSpec
	if prowJobSpec.Refs == nil {
		return response, errors.New(fmt.Sprintf("no prowJobSpec.refs passed: %s", util.PrettyPrint(pipelineRun)))
	}

	// Only if there is one Pull in Refs, it's a PR build so we are going to pass it
	if len(prowJobSpec.Refs.Pulls) == 1 {
		revision = prowJobSpec.Refs.Pulls[0].SHA
		prNumber = strconv.Itoa(prowJobSpec.Refs.Pulls[0].Number)
	} else {
		//Otherwise it's a Master / Batch build, and we handle it later
		revision = prowJobSpec.Refs.BaseSHA
	}

	envs, err := downwardapi.EnvForSpec(downwardapi.NewJobSpec(prowJobSpec, "", ""))
	if err != nil {
		return response, errors.Wrap(err, "failed to get env vars from prowjob")
	}

	sourceURL := c.getSourceURL(prowJobSpec.Refs.Org, prowJobSpec.Refs.Repo)
	if sourceURL == "" {
		// fallback to GutHub provider
		sourceURL = fmt.Sprintf("https://github.com/%s/%s.git", prowJobSpec.Refs.Org, prowJobSpec.Refs.Repo)
	}

	if revision == "" {
		revision = "master"
	}

	branch := c.getBranch(prowJobSpec)
	if branch == "" {
		branch = "master"
	}

	logger.WithFields(logrus.Fields{"sourceURL": sourceURL, "branch": branch, "revision": revision, "context": prowJobSpec.Context, "meta": c.useMetaPipeline}).Info("triggering pipeline")

	results := PipelineRunResponse{}
	if c.useMetaPipeline {
		crds, err := c.triggerMetaPipeline(pipelineRun, prNumber, sourceURL, revision, branch, envs)
		if err != nil {
			return response, err
		}

		results.Resources = crds.ObjectReferences()
	} else {
		pipelineCreateOption := c.buildStepCreateTaskOption(prowJobSpec, prNumber, sourceURL, revision, branch, pipelineRun, envs)
		err = pipelineCreateOption.Run()
		if err != nil {
			return response, errors.Wrap(err, "error triggering the pipeline run")
		}
		results.Resources = pipelineCreateOption.Results.ObjectReferences()
	}

	return results, nil
}

func (c *controller) buildStepCreateTaskOption(prowJobSpec prowapi.ProwJobSpec, prNumber string, sourceURL string, revision string, branch string, pipelineRun PipelineRunRequest, envs map[string]string) *create.StepCreateTaskOptions {
	createTaskOption := &create.StepCreateTaskOptions{}
	createTaskOption.CommonOptions = opts.NewCommonOptionsWithTerm(clients.NewFactory(), os.Stdin, os.Stdout, os.Stderr)
	if prowJobSpec.Type == prowapi.PostsubmitJob {
		createTaskOption.PipelineKind = jenkinsfile.PipelineKindRelease
	} else {
		createTaskOption.PipelineKind = jenkinsfile.PipelineKindPullRequest
	}

	// defaults
	createTaskOption.SourceName = "source"
	createTaskOption.Duration = time.Second * 20
	createTaskOption.PullRequestNumber = prNumber
	createTaskOption.CloneGitURL = sourceURL
	createTaskOption.DeleteTempDir = true
	createTaskOption.Context = prowJobSpec.Context
	createTaskOption.Branch = branch
	createTaskOption.Revision = revision
	createTaskOption.ServiceAccount = c.serviceAccount
	createTaskOption.SemanticRelease = c.semanticRelease
	// turn map into string array with = separator to match type of custom labels which are CLI flags
	for key, value := range pipelineRun.Labels {
		createTaskOption.CustomLabels = append(createTaskOption.CustomLabels, fmt.Sprintf("%s=%s", key, value))
	}
	// turn map into string array with = separator to match type of custom env vars which are CLI flags
	for key, value := range envs {
		createTaskOption.CustomEnvs = append(createTaskOption.CustomEnvs, fmt.Sprintf("%s=%s", key, value))
	}

	return createTaskOption
}

func (c *controller) triggerMetaPipeline(pipelineRun PipelineRunRequest, prNumber string, sourceURL string, revision string, branch string, envs map[string]string) (*tekton.CRDWrapper, error) {
	prowJobSpec := pipelineRun.ProwJobSpec
	pullRefs := c.getPullRefs(prowJobSpec)

	job := pipelineRun.Labels[jobLabel]
	if job == "" {
		return nil, errors.Errorf("unable to find prow job name in pipeline request: %s", util.PrettyPrint(pipelineRun))
	}

	pullRef := c.prowToMetaPipelinePullRef(sourceURL, &pullRefs)
	pipelineKind := c.determinePipelineKind(pullRefs)

	pipelineCreateParam := metapipeline.PipelineCreateParam{
		PullRef:        pullRef,
		PipelineKind:   pipelineKind,
		Context:        prowJobSpec.Context,
		EnvVariables:   envs,
		Labels:         pipelineRun.Labels,
		ServiceAccount: c.serviceAccount,
		DefaultImage:   c.metaPipelineImage,
	}

	pipelineActivity, tektonCRDs, err := c.metaPipelineClient.Create(pipelineCreateParam)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create Tekton CRDs")
	}

	logger.WithField("crds", tektonCRDs.String()).Tracef("generated crds for %s", pipelineActivity.Name)

	err = c.metaPipelineClient.Apply(pipelineActivity, tektonCRDs)
	if err != nil {
		return nil, errors.Wrap(err, "unable to apply Tekton CRDs")
	}

	return &tektonCRDs, nil
}

func (c *controller) marshalPayload(payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrapf(err, "marshalling the JSON payload %#v", payload)
	}
	return data, nil
}

func (c *controller) returnStatusBadRequest(err error, message string, w http.ResponseWriter) {
	logger.Infof("%v %s", err, message)
	w.WriteHeader(http.StatusBadRequest)
	_, err = w.Write([]byte(message))
	if err != nil {
		logger.Warnf("Error returning HTTP 400: %s", err)
	}
}

func (c *controller) getBranch(spec prowapi.ProwJobSpec) string {
	branch := spec.Refs.BaseRef
	if spec.Type == prowapi.PostsubmitJob {
		return branch
	}
	if spec.Type == prowapi.BatchJob {
		return "batch"
	}
	if len(spec.Refs.Pulls) > 0 {
		branch = fmt.Sprintf("PR-%v", spec.Refs.Pulls[0].Number)
	}
	return branch
}

func (c *controller) getPullRefs(spec prowapi.ProwJobSpec) prow.PullRefs {
	toMerge := make(map[string]string)
	for _, pull := range spec.Refs.Pulls {
		toMerge[strconv.Itoa(pull.Number)] = pull.SHA
	}

	pullRef := prow.PullRefs{
		BaseBranch: spec.Refs.BaseRef,
		BaseSha:    spec.Refs.BaseSHA,
		ToMerge:    toMerge,
	}
	return pullRef
}

// setupSignalChannel registers a listener for Unix signals for a ordered shutdown
func (c *controller) setupSignalChannel(cancel context.CancelFunc) {
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGTERM)

	go func() {
		<-sigchan
		logger.Info("Received SIGTERM signal. Initiating shutdown.")
		cancel()
	}()
}

func (c *controller) getSourceURL(org, repo string) string {
	resourceInterface := c.jxClient.JenkinsV1().SourceRepositories(c.ns)

	sourceRepos, err := resourceInterface.List(meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("owner=%s,repository=%s", org, repo),
	})

	if err != nil || sourceRepos == nil || len(sourceRepos.Items) == 0 {
		return ""
	}

	gitProviderURL := sourceRepos.Items[0].Spec.Provider
	if gitProviderURL == "" {
		return ""
	}
	if !strings.HasSuffix(gitProviderURL, "/") {
		gitProviderURL = gitProviderURL + "/"
	}

	return fmt.Sprintf("%s%s/%s.git", gitProviderURL, org, repo)
}

func (c *controller) prowToMetaPipelinePullRef(sourceURL string, prowPullRef *prow.PullRefs) metapipeline.PullRef {
	var pullRef metapipeline.PullRef
	if len(prowPullRef.ToMerge) > 0 {
		var prs []metapipeline.PullRequestRef
		for prID, SHA := range prowPullRef.ToMerge {
			prs = append(prs, metapipeline.PullRequestRef{ID: prID, MergeSHA: SHA})
		}
		pullRef = metapipeline.NewPullRefWithPullRequest(sourceURL, prowPullRef.BaseBranch, prowPullRef.BaseSha, prs...)
	} else {
		pullRef = metapipeline.NewPullRef(sourceURL, prowPullRef.BaseBranch, prowPullRef.BaseSha)
	}
	return pullRef
}

func (c *controller) determinePipelineKind(pullRef prow.PullRefs) metapipeline.PipelineKind {
	var kind metapipeline.PipelineKind

	prCount := len(pullRef.ToMerge)
	if prCount > 0 {
		kind = metapipeline.PullRequestPipeline
	} else {
		kind = metapipeline.ReleasePipeline
	}
	log.Logger().Debugf("pipeline kind for pull ref '%s' : '%s'", pullRef.String(), kind)
	return kind
}
