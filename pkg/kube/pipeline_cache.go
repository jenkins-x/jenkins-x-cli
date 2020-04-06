package kube

import (
	"sync"
	"time"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

// PipelineNamespaceCache caches the pipelines for a single namespace
type PipelineNamespaceCache struct {
	pipelines sync.Map
	stop      chan struct{}
	//Flag to indicate whether the cache has done its initial load & is in sync.
	ready bool
}

// NewPipelineCache creates a cache of pipelines for a namespace
func NewPipelineCache(jxClient versioned.Interface, ns string) *PipelineNamespaceCache {
	pipeline := &v1.PipelineActivity{}
	pipelineListWatch := cache.NewListWatchFromClient(jxClient.JenkinsV1().RESTClient(), "pipelineactivities", ns, fields.Everything())

	pipelineCache := &PipelineNamespaceCache{
		stop: make(chan struct{}),
	}

	// lets pre-populate the cache on startup as there's not yet a way to know when the informer has completed its first list operation
	pipelines, _ := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
	if pipelines != nil {
		for _, pipeline := range pipelines.Items {
			copy := pipeline
			pipelineCache.pipelines.Store(pipeline.Name, &copy)
		}
	}
	_, pipelineController := cache.NewInformer(
		pipelineListWatch,
		pipeline,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pipelineCache.onPipelineObj(obj, jxClient, ns)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				pipelineCache.onPipelineObj(newObj, jxClient, ns)
			},
			DeleteFunc: func(obj interface{}) {
				pipelineCache.onPipelineDelete(obj, jxClient, ns)
			},
		},
	)

	go pipelineController.Run(pipelineCache.stop)

	pipelineCache.ready = true

	return pipelineCache
}

// Ready returns true if this cache has done its initial load and is in sync.
func (c *PipelineNamespaceCache) Ready() bool {
	return c.ready
}

// Stop closes the underlying chanel processing events which stops consuming watch events
func (c *PipelineNamespaceCache) Stop() {
	c.ready = false
	close(c.stop)
}

// Pipelines returns the pipelines in this namespace sorted in name order
func (c *PipelineNamespaceCache) Pipelines() []*v1.PipelineActivity {
	answer := []*v1.PipelineActivity{}
	onEntry := func(key interface{}, value interface{}) bool {
		pipeline, ok := value.(*v1.PipelineActivity)
		if ok && pipeline != nil {
			answer = append(answer, pipeline)
		}
		return true
	}
	c.pipelines.Range(onEntry)
	return answer
}

// ForEach runs the supplied function on every element in the Map. In no particular order.
func (c *PipelineNamespaceCache) ForEach(callback func(*v1.PipelineActivity)) {
	onEntry := func(key interface{}, value interface{}) bool {
		pipeline, ok := value.(*v1.PipelineActivity)
		if ok && pipeline != nil {
			callback(pipeline)
		}
		return true
	}
	c.pipelines.Range(onEntry)
}

func (c *PipelineNamespaceCache) onPipelineObj(obj interface{}, jxClient versioned.Interface, ns string) {
	pipeline, ok := obj.(*v1.PipelineActivity)
	if !ok {
		log.Logger().Warnf("Object is not a PipelineActivity %#v", obj)
		return
	}
	if pipeline != nil {
		c.pipelines.Store(pipeline.Name, pipeline)
	}
}

func (c *PipelineNamespaceCache) onPipelineDelete(obj interface{}, jxClient versioned.Interface, ns string) {
	pipeline, ok := obj.(*v1.PipelineActivity)
	if !ok {
		log.Logger().Warnf("Object is not a PipelineActivity %#v", obj)
		return
	}
	if pipeline != nil {
		c.pipelines.Delete(pipeline.Name)
	}
}
