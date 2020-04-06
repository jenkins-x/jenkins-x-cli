package metapipeline

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	jxclient "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "k8s.io/client-go/kubernetes"
)

const (
	retryDuration      = time.Second * 30
	defaultCheckoutDir = "source"
)

var (
	logger = log.Logger().WithFields(logrus.Fields{"component": "meta-pipeline-client"})
)

// clientFactory implements the interface methods to create and apply the meta pipeline.
type clientFactory struct {
	jxClient     versioned.Interface
	tektonClient tektonclient.Interface
	kubeClient   kubernetes.Interface
	ns           string

	versionDir       string
	versionStreamURL string
	versionStreamRef string
}

// NewMetaPipelineClient creates a new client for the creation and application of meta pipelines.
// The responsibility of the meta pipeline is to prepare the execution pipeline and to allow Apps to contribute
// the this execution pipeline.
func NewMetaPipelineClient() (Client, error) {
	tektonClient, jxClient, kubeClient, ns, err := getClientsAndNamespace()
	if err != nil {
		return nil, err
	}

	return NewMetaPipelineClientWithClientsAndNamespace(jxClient, tektonClient, kubeClient, ns)
}

// NewMetaPipelineClientWithClientsAndNamespace creates a new client for the creation and application of meta pipelines using the specified parameters.
func NewMetaPipelineClientWithClientsAndNamespace(jxClient versioned.Interface, tektonClient tektonclient.Interface, kubeClient kubernetes.Interface, ns string) (Client, error) {
	url, ref, err := versionStreamURLAndRef(jxClient, ns)
	if err != nil {
		return nil, errors.Wrap(err, "unable to determine versions stream URL and ref")
	}

	versionDir, err := cloneVersionStream(url, ref)
	if err != nil {
		return nil, errors.Wrap(err, "unable to clone version dir")
	}

	client := clientFactory{
		jxClient:         jxClient,
		tektonClient:     tektonClient,
		kubeClient:       kubeClient,
		ns:               ns,
		versionDir:       versionDir,
		versionStreamURL: url,
		versionStreamRef: ref,
	}

	return &client, nil
}

// Create creates the Tekton CRDs needed for executing the pipeline as defined by the input parameters.
func (c *clientFactory) Create(param PipelineCreateParam) (kube.PromoteStepActivityKey, tekton.CRDWrapper, error) {
	err := c.cloneVersionStreamIfNeeded()
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, errors.Wrap(err, "unable to clone version stream")
	}

	gitInfo, err := gits.ParseGitURL(param.PullRef.SourceURL())
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, errors.Wrap(err, fmt.Sprintf("unable to determine needed git info from the specified git url '%s'", param.PullRef.SourceURL()))
	}

	podTemplates, err := c.getPodTemplates(apps.AppPodTemplateName)
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, errors.Wrap(err, "unable to retrieve pod templates")
	}

	branchIdentifier, err := c.determineBranchIdentifier(param.PipelineKind, param.PullRef)
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, errors.Wrap(err, "unable to create branch identifier")
	}

	pipelineName := tekton.PipelineResourceNameFromGitInfo(gitInfo, branchIdentifier, param.Context, tekton.MetaPipeline.String())
	buildNumber, err := tekton.GenerateNextBuildNumber(c.tektonClient, c.jxClient, c.ns, gitInfo, branchIdentifier, retryDuration, param.Context, param.UseActivityForNextBuildNumber)
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, errors.Wrap(err, "unable to determine next build number")
	}

	logger.WithField("repo", gitInfo.URL).WithField("buildNumber", buildNumber).Debug("creating meta pipeline CRDs")

	extendingApps, err := getExtendingApps(c.jxClient, c.ns)
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, err
	}

	crdCreationParams := CRDCreationParameters{
		Namespace:           c.ns,
		Context:             param.Context,
		PipelineName:        pipelineName,
		PipelineKind:        param.PipelineKind,
		BuildNumber:         buildNumber,
		BranchIdentifier:    branchIdentifier,
		PullRef:             param.PullRef,
		SourceDir:           defaultCheckoutDir,
		PodTemplates:        podTemplates,
		ServiceAccount:      param.ServiceAccount,
		Labels:              param.Labels,
		EnvVars:             param.EnvVariables,
		DefaultImage:        param.DefaultImage,
		Apps:                extendingApps,
		VersionsDir:         c.versionDir,
		GitInfo:             *gitInfo,
		UseBranchAsRevision: param.UseBranchAsRevision,
		NoReleasePrepare:    param.NoReleasePrepare,
	}

	return c.createActualCRDs(buildNumber, branchIdentifier, param.Context, param.PullRef, crdCreationParams)
}

func (c *clientFactory) createActualCRDs(buildNumber string, branchIdentifier string, context string, pullRef PullRef, parameters CRDCreationParameters) (kube.PromoteStepActivityKey, tekton.CRDWrapper, error) {
	tektonCRDs, err := createMetaPipelineCRDs(parameters)
	if err != nil {
		return kube.PromoteStepActivityKey{}, tekton.CRDWrapper{}, errors.Wrap(err, "failed to generate Tekton CRDs for meta pipeline")
	}

	pr, _ := tekton.ParsePullRefs(pullRef.String())
	pipelineActivity := tekton.GeneratePipelineActivity(buildNumber, branchIdentifier, &parameters.GitInfo, context, pr)

	return *pipelineActivity, *tektonCRDs, nil
}

// Apply takes the given CRDs to process them, usually applying them to the cluster.
func (c *clientFactory) Apply(pipelineActivity kube.PromoteStepActivityKey, crds tekton.CRDWrapper) error {
	err := tekton.ApplyPipeline(c.jxClient, c.tektonClient, &crds, c.ns, &pipelineActivity)
	if err != nil {
		return errors.Wrapf(err, "failed to apply Tekton CRDs")
	}
	logger.WithField("pipeline", crds.PipelineRun().Name).Debug("applied tekton CRDs")
	return nil
}

// Close cleans up the resources use by this client.
func (c *clientFactory) Close() error {
	return os.RemoveAll(c.versionDir)
}

func (c *clientFactory) getPodTemplates(containerName string) (map[string]*corev1.Pod, error) {
	podTemplates, err := kube.LoadPodTemplates(c.kubeClient, c.ns)
	if err != nil {
		return nil, err
	}

	return podTemplates, nil
}

func (c *clientFactory) determineBranchIdentifier(pipelineType PipelineKind, pullRef PullRef) (string, error) {
	var branch string
	switch pipelineType {
	case ReleasePipeline:
		// no pull requests to merge, taking base branch name as identifier
		branch = pullRef.baseBranch
	case PullRequestPipeline:
		if len(pullRef.pullRequests) == 0 {
			return "", errors.New("pullrequest pipeline requested, but no pull requests specified")
		}
		branch = fmt.Sprintf("PR-%s", pullRef.PullRequests()[0].ID)
	default:
		branch = "unknown"
	}
	return branch, nil
}

func versionStreamURLAndRef(jxClient versioned.Interface, ns string) (string, string, error) {
	devEnv, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil {
		return "", "", errors.Wrap(err, "unable to retrieve team environment")
	}

	if devEnv == nil {
		return config.DefaultVersionsURL, config.DefaultVersionsRef, nil
	}

	teamSettings := devEnv.Spec.TeamSettings
	url := teamSettings.VersionStreamURL
	ref := teamSettings.VersionStreamRef
	if url == "" {
		url = config.DefaultVersionsURL
	}
	if ref == "" {
		ref = config.DefaultVersionsRef
	}

	return url, ref, nil
}

func (c *clientFactory) cloneVersionStreamIfNeeded() error {
	url, ref, err := versionStreamURLAndRef(c.jxClient, c.ns)
	if err != nil {
		return err
	}

	if c.versionStreamURL != url || c.versionStreamRef != ref {
		oldVersionStreamDir := c.versionDir
		c.versionDir, err = cloneVersionStream(url, ref)
		if err != nil {
			return err
		}
		_ = os.RemoveAll(oldVersionStreamDir)
	}

	return nil
}

func cloneVersionStream(url string, ref string) (string, error) {
	dir, err := ioutil.TempDir("", "jx-version-repo-")
	if err != nil {
		return "", errors.Wrap(err, "unable to create temp dir for version stream")
	}

	logger.Debugf("cloning version stream url: %s ref: %s into %s", url, ref, dir)

	// Not using GitCLi Clone/ShallowClone atm, since it does not work with tags.
	// Once https://github.com/jenkins-x/jx/issues/5087 is resolved we should switch to that.
	// As a quick hack is assumes that any ref with a '.' won't be a SHA.
	if ref == "master" || strings.Contains(ref, ".") {
		args := []string{"clone", "--depth", "1", "--branch", ref, url, "."}
		cmd := util.Command{
			Dir:  dir,
			Name: "git",
			Args: args,
		}
		output, err := cmd.RunWithoutRetry()
		if err != nil {
			return "", errors.Wrapf(err, "unable to clone version stream and checking out branch/tag: %s", output)
		}
	} else {
		// assuming we deal with a SHA
		args := []string{"clone", url, "."}
		cmd := util.Command{
			Dir:  dir,
			Name: "git",
			Args: args,
		}
		output, err := cmd.RunWithoutRetry()
		if err != nil {
			return "", errors.Wrapf(err, "unable to clone version stream: %s", output)
		}

		// Fetch PR refs before checking out the ref
		args = []string{"fetch", "origin", ref}
		cmd = util.Command{
			Dir:  dir,
			Name: "git",
			Args: args,
		}
		output, err = cmd.RunWithoutRetry()
		if err != nil {
			return "", errors.Wrapf(err, "unable to fetch pull request refs for version stream: %s", output)
		}

		args = []string{"checkout", ref}
		cmd = util.Command{
			Dir:  dir,
			Name: "git",
			Args: args,
		}
		output, err = cmd.RunWithoutRetry()
		if err != nil {
			return "", errors.Wrapf(err, "unable checkout sha %s for version stream %s: %s", ref, url, output)
		}
	}

	return dir, err
}

func getClientsAndNamespace() (tektonclient.Interface, jxclient.Interface, kubeclient.Interface, string, error) {
	factory := jxfactory.NewFactory()

	tektonClient, _, err := factory.CreateTektonClient()
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to create Tekton client")
	}

	jxClient, _, err := factory.CreateJXClient()
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to create JX client")
	}

	kubeClient, ns, err := factory.CreateKubeClient()
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to create Kube client")
	}
	ns, _, err = kube.GetDevNamespace(kubeClient, ns)
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to find the dev namespace")
	}
	return tektonClient, jxClient, kubeClient, ns, nil
}
