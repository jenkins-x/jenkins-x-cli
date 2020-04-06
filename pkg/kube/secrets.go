package kube

import (
	"fmt"
	"sort"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetSecrets returns a map of the Secrets along with a sorted list of names
func GetSecrets(kubeClient kubernetes.Interface, ns string) (map[string]*v1.Secret, []string, error) {
	m := map[string]*v1.Secret{}

	names := []string{}
	resourceList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return m, names, err
	}
	for _, resource := range resourceList.Items {
		n := resource.Name
		copy := resource
		m[n] = &copy
		if n != "" {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return m, names, nil
}

// DefaultModifySecret default implementation of a function to modify
func DefaultModifySecret(kubeClient kubernetes.Interface, ns string, name string, fn func(env *v1.Secret) error, defaultSecret *v1.Secret) (*v1.Secret, error) {
	secretInterface := kubeClient.CoreV1().Secrets(ns)

	create := false
	secret, err := secretInterface.Get(name, metav1.GetOptions{})
	if err != nil {
		create = true
		initialSecret := v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Labels:      map[string]string{},
				Annotations: map[string]string{},
			},
			Data: map[string][]byte{},
		}
		if defaultSecret != nil {
			initialSecret = *defaultSecret
		}
		secret = &initialSecret
	}
	err = fn(secret)
	if err != nil {
		return secret, err
	}
	if create {
		log.Logger().Debugf("Creating Secret %s in namespace %s", util.ColorInfo(name), util.ColorInfo(ns))
		_, err = secretInterface.Create(secret)
		if err != nil {
			return secret, errors.Wrapf(err, "Failed to create Secret %s in namespace %s", name, ns)
		}
		return secret, err
	}
	log.Logger().Infof("Updating Secret %s in namespace %s", util.ColorInfo(name), util.ColorInfo(ns))
	_, err = secretInterface.Update(secret)
	if err != nil {
		return secret, errors.Wrapf(err, "Failed to update Secret %s in namespace %s", name, ns)
	}
	return secret, nil
}

// ValidateSecret checks a given secret and key exists in the provided namespace
func ValidateSecret(kubeClient kubernetes.Interface, secretName, key, ns string) error {
	secret, err := kubeClient.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "could not find the Secret %s in the namespace: %s", secretName, ns)
	}
	if secret.Data == nil || len(secret.Data[key]) == 0 {
		return fmt.Errorf("the Secret %s in the namespace: %s does not have a key: %s", secretName, ns, key)
	}
	log.Logger().Debugf("valid: there is a Secret: %s in namespace: %s\n", util.ColorInfo(secretName), util.ColorInfo(ns))
	return nil
}
