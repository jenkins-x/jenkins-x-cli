package kube

import (
	"fmt"
	"strings"

	"strconv"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	IngressConfigConfigmap = "ingress-config"
	Domain                 = "domain"
	Email                  = "email"
	TLS                    = "tls"
	Issuer                 = "issuer"
	ClusterIssuer          = "clusterissuer"
	Exposer                = "exposer"
	UrlTemplate            = "urltemplate"
)

type IngressConfig struct {
	Email         string `structs:"email" yaml:"email" json:"email"`
	Domain        string `structs:"domain" yaml:"domain" json:"domain"`
	Issuer        string `structs:"issuer" yaml:"issuer" json:"issuer"`
	ClusterIssuer bool   `structs:"clusterissuer" yaml:"clusterissuer" json:"clusterissuer"`
	Exposer       string `structs:"exposer" yaml:"exposer" json:"exposer"`
	UrlTemplate   string `structs:"urltemplate" yaml:"urltemplate" json:"urltemplate"`
	TLS           bool   `structs:"tls" yaml:"tls" json:"tls"`
}

func GetIngress(client kubernetes.Interface, ns, name string) (string, error) {

	ing, err := client.ExtensionsV1beta1().Ingresses(ns).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get ingress rule %s. error: %v", name, err)
	}
	if ing == nil {
		return "", fmt.Errorf("failed to find ingress rule %s", name)
	}

	// default to the first rule
	if len(ing.Spec.Rules) > 0 {
		return ing.Spec.Rules[0].Host, nil
	}
	return "", fmt.Errorf("no hostname found for ingress rule %s", name)
}

func GetIngressConfig(c kubernetes.Interface, ns string) (IngressConfig, error) {
	var ic IngressConfig
	configMapInterface := c.CoreV1().ConfigMaps(ns)
	cm, err := configMapInterface.Get(IngressConfigConfigmap, meta_v1.GetOptions{})
	data := map[string]string{}
	if err != nil {
		cm2, err2 := configMapInterface.Get("exposecontroller", meta_v1.GetOptions{})
		if err2 != nil {
			return ic, err
		}
		config := cm2.Data["config.yml"]
		lines := strings.Split(config, "\n")
		for _, pair := range lines {
			z := strings.Split(pair, ":")
			data[z[0]] = strings.TrimSpace(z[1])
		}
		return ic, err
	} else {
		data = cm.Data
	}

	ic.Domain = data[Domain]
	ic.Email = data[Email]
	ic.Exposer = data[Exposer]
	ic.UrlTemplate = data[UrlTemplate]
	ic.Issuer = data[Issuer]
	clusterIssuer, exists := data[ClusterIssuer]

	if exists {
		ic.ClusterIssuer, err = strconv.ParseBool(clusterIssuer)
		if err != nil {
			return ic, fmt.Errorf("failed to parse ClusterIssuer string %s to bool from %s: %v", clusterIssuer, IngressConfigConfigmap, err)
		}
	} else {
		ic.ClusterIssuer = false
	}

	tls, exists := data[TLS]

	if exists && tls != "" {
		ic.TLS, err = strconv.ParseBool(tls)
		if err != nil {
			return ic, fmt.Errorf("failed to parse TLS string %s to bool from %s: %v", tls, IngressConfigConfigmap, err)
		}
	} else {
		ic.TLS = false
	}
	return ic, nil
}

// DeleteIngress removes an ingress by name
func DeleteIngress(client kubernetes.Interface, ns, name string) error {
	return client.ExtensionsV1beta1().Ingresses(ns).Delete(name, &meta_v1.DeleteOptions{})
}
