package kserving

import (
	"github.com/knative/serving/pkg/apis/serving/v1alpha1"
	kserve "github.com/knative/serving/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// FindServiceURL finds the service URL for the given knative service name
func FindServiceURL(client kserve.Interface, kubeClient kubernetes.Interface, namespace string, name string) (string, *v1alpha1.Service, error) {
	if client == nil {
		return "", nil, nil
	}
	svc, err := client.ServingV1alpha1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", svc, err
	}
	answer := GetServiceURL(svc, kubeClient, namespace)
	return answer, svc, nil
}

// GetServiceURL returns the URL for the given knative service
func GetServiceURL(service *v1alpha1.Service, kubeClient kubernetes.Interface, namespace string) string {
	if service == nil {
		return ""
	}
	domain := service.Status.DeprecatedDomain
	if domain == "" {
		if service.Status.Address != nil {
			domain = service.Status.Address.Hostname
		}
	}
	if domain == "" {
		return ""
	}

	name := service.Status.LatestReadyRevisionName
	if name == "" {
		name = service.Status.LatestCreatedRevisionName
	}
	scheme := "http://"
	if name != "" {
		name = name + "-service"
		// lets find the service to determine if its https or http
		svc, err := kubeClient.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
		if err == nil && svc != nil {
			for _, port := range svc.Spec.Ports {
				if port.Port == 443 {
					scheme = "https://"
				}
			}
		}
	}
	return scheme + domain
}
