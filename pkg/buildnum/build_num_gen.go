// Package buildnum contains stuff to do with generating build numbers.
package buildnum

import (
	"strconv"
	"strings"
	"sync"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	v1 "github.com/jenkins-x/jx/pkg/client/clientset/versioned/typed/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PipelineActivityBuildNumGen generates build numbers backed by PipelineActivity K8S CRDs.
type PipelineActivityBuildNumGen struct {
	//Protect access to pipelineMutexes map.
	mutex *sync.Mutex
	//pipelineID->Mutex to serialise build number generation for a given pipeline ID.
	pipelineMutexes  map[string]*sync.Mutex
	activitiesGetter v1.PipelineActivityInterface
	pipelineCache    *kube.PipelineNamespaceCache
}

// NewCRDBuildNumGen initialises a new PipelineActivityBuildNumGen that will use the supplied
// PipelineActivityInterface to query CRDs.
func NewCRDBuildNumGen(jxClient versioned.Interface, ns string) *PipelineActivityBuildNumGen {
	return &PipelineActivityBuildNumGen{
		mutex:            &sync.Mutex{},
		pipelineMutexes:  make(map[string]*sync.Mutex),
		pipelineCache:    kube.NewPipelineCache(jxClient, ns),
		activitiesGetter: jxClient.JenkinsV1().PipelineActivities(ns),
	}
}

// Ready returns true if the generator's cache has done its initial load.
func (g *PipelineActivityBuildNumGen) Ready() bool {
	return g.pipelineCache.Ready()
}

// NextBuildNumber returns the next build number for the specified pipeline ID, storing the sequence in K8S.
// Returns the build number, or an error if there is a problem with K8S resources.
func (g *PipelineActivityBuildNumGen) NextBuildNumber(pipeline kube.PipelineID) (string, error) {
	//Shouldn't happen in practice, but lock to avoid corruption if we somehow generated >1 build number for the same
	//pipeline concurrently.
	g.mutex.Lock()
	defer g.mutex.Unlock()

	//Scan cached pipelines recording the highest yet build number.
	calc := buildNumCalc{pipeline: pipeline}
	g.pipelineCache.ForEach(calc.processPipelineActivity)

	nextBuild := strconv.Itoa(calc.lastBuildNum + 1)
	name := pipeline.GetActivityName(nextBuild)

	//Save this build number before returning.
	a := &jenkinsv1.PipelineActivity{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: jenkinsv1.PipelineActivitySpec{
			Build:    nextBuild,
			Pipeline: pipeline.ID,
		},
	}

	answer, err := g.activitiesGetter.Create(a)
	if err != nil {
		return "", err
	}

	return answer.Spec.Build, nil
}

// buildNumCalc provides a callback to traverse all cached PipelineActivities, finding & recording the highest build
// number encountered.
type buildNumCalc struct {
	pipeline     kube.PipelineID
	lastBuildNum int
}

// processPipelineActivity records the PipelineActivity with the highest build number in its spec.
func (b *buildNumCalc) processPipelineActivity(activity *jenkinsv1.PipelineActivity) {
	if strings.EqualFold(activity.Spec.Pipeline, b.pipeline.ID) {
		build := activity.Spec.Build
		if build != "" {
			bi, err := strconv.Atoi(build)
			if err == nil {
				if bi > b.lastBuildNum {
					b.lastBuildNum = bi
				}
			}
		}
	}
}
