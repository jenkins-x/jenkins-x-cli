package kube

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

type HeapterConfig struct {
	KubeClient        kubernetes.Interface
	HeapsterNamespace string
	HeapsterScheme    string
	HeapsterPort      string
	HeapsterService   string
	checkedForService bool
}

func (q *HeapterConfig) GetPodMetrics(ns string, pod string, selector string, metric string, start string, end string) ([]byte, error) {
	kubeClient := q.KubeClient
	heapsterNamespace := util.FirstNotEmptyString(q.HeapsterNamespace, "kube-system")
	heapsterScheme := util.FirstNotEmptyString(q.HeapsterScheme, "http")
	heapsterService := util.FirstNotEmptyString(q.HeapsterService, "heapster")
	heapsterPort := util.FirstNotEmptyString(q.HeapsterPort, "80")

	if !q.checkedForService {
		svc, err := kubeClient.CoreV1().Services(heapsterNamespace).Get(heapsterService, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("Could not find heapster service %s in namespace %s: %s", heapsterService, heapsterNamespace, err)
		}
		q.checkedForService = true
		// lets check the port is OK?
		found := false
		ports := []string{}
		if svc.Spec.Ports != nil {
			for _, p := range svc.Spec.Ports {
				i := p.Port
				if i > 0 {
					t := util.Int32ToA(i)
					ports = append(ports, t)
					if !found {
						if q.HeapsterPort == "" {
							q.HeapsterPort = t
							found = true
						}
						if q.HeapsterPort == t {
							found = true
						}
					}
				}
			}
		}
		if !found {
			if q.HeapsterPort == "" {
				return nil, fmt.Errorf("Heapster service %s in namespace %s is not listing on a port", heapsterService, heapsterNamespace)
			} else {
				return nil, util.InvalidOption("heapster-port", q.HeapsterPort, ports)
			}
		}
	}

	path := util.UrlJoin("/apis/metrics/v1alpha1/namespaces/", ns, "/pods")
	if pod != "" {
		path = util.UrlJoin(path, pod)
		if metric != "" {
			path = util.UrlJoin(path, "metrics", metric)
		}
	}
	params := map[string]string{}
	if selector != "" {
		params["labelSelector"] = selector
	}
	if start != "" {
		params["start"] = start
	}
	if end != "" {
		params["end"] = end
	}
	log.Logger().Infof("Querying %s using query parameters: %#v", path, params)
	resp := kubeClient.CoreV1().Services(heapsterNamespace).ProxyGet(heapsterScheme, heapsterService, heapsterPort, path, params)
	return resp.DoRaw()
}

func GetPodMetrics(client *metricsclient.Clientset, ns string) (*metricsv1beta1.PodMetricsList, error) {
	return client.MetricsV1beta1().PodMetricses(ns).List(metav1.ListOptions{})
}
