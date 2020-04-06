package kube

import (
	"sort"
	"time"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/log"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

// EnvironmentNamespaceCache caches the Environments for a single namespace
type EnvironmentNamespaceCache struct {
	items map[string]*v1.Environment
}

// CreateEnvironmentCache creates a cache for the given namespace of Environments
func CreateEnvironmentCache(jxClient versioned.Interface, ns string) *EnvironmentNamespaceCache {
	Environment := &v1.Environment{}
	stop := make(chan struct{})
	environmentListWatch := cache.NewListWatchFromClient(jxClient.JenkinsV1().RESTClient(), "environments", ns, fields.Everything())

	namespaceCache := &EnvironmentNamespaceCache{
		items: map[string]*v1.Environment{},
	}

	_, EnvironmentController := cache.NewInformer(
		environmentListWatch,
		Environment,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				namespaceCache.onEnvironmentObj(obj, jxClient, ns)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				namespaceCache.onEnvironmentObj(newObj, jxClient, ns)
			},
			DeleteFunc: func(obj interface{}) {
				namespaceCache.onEnvironmentDelete(obj, jxClient, ns)
			},
		},
	)

	go EnvironmentController.Run(stop)

	return namespaceCache
}

// Items returns the Environments in this namespace sorted in name order
func (c *EnvironmentNamespaceCache) Items() []*v1.Environment {
	answer := []*v1.Environment{}
	names := []string{}
	for k := range c.items {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		Environment := c.items[name]
		if Environment != nil {
			answer = append(answer, Environment)
		}
	}
	return answer
}

// Item returns the Environment for the tiven name
func (c *EnvironmentNamespaceCache) Item(name string) *v1.Environment {
	return c.items[name]
}

func (c *EnvironmentNamespaceCache) onEnvironmentObj(obj interface{}, jxClient versioned.Interface, ns string) {
	Environment, ok := obj.(*v1.Environment)
	if !ok {
		log.Logger().Warnf("Object is not a Environment %#v", obj)
		return
	}
	if Environment != nil {
		c.items[Environment.Name] = Environment
	}
}

func (c *EnvironmentNamespaceCache) onEnvironmentDelete(obj interface{}, jxClient versioned.Interface, ns string) {
	Environment, ok := obj.(*v1.Environment)
	if !ok {
		log.Logger().Warnf("Object is not a Environment %#v", obj)
		return
	}
	if Environment != nil {
		delete(c.items, Environment.Name)
	}
}
