package kube

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LogMasker replaces words in a log from a set of secrets
type LogMasker struct {
	ReplaceWords map[string]string
}

// NewLogMasker creates a new LogMasker loading secrets from the given namespace
func NewLogMasker(kubeClient kubernetes.Interface, ns string) (*LogMasker, error) {
	masker := &LogMasker{}
	resourceList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return masker, err
	}
	for _, secret := range resourceList.Items {
		masker.LoadSecret(&secret)
	}
	return masker, nil
}

// NewLogMaskerFromMap creates a new LogMasker with all the string values in a tree of map
func NewLogMaskerFromMap(m map[string]interface{}) *LogMasker {
	masker := &LogMasker{
		ReplaceWords: map[string]string{},
	}
	masker.replaceMapValues(m)
	return masker
}

// LoadSecrets loads the secrets into the log masker
func (m *LogMasker) LoadSecrets(kubeClient kubernetes.Interface, ns string) error {
	resourceList, err := kubeClient.CoreV1().Secrets(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, secret := range resourceList.Items {
		m.LoadSecret(&secret)
	}
	return nil
}

// LoadSecret loads the secret data into the log masker
func (m *LogMasker) LoadSecret(secret *corev1.Secret) {
	if m.ReplaceWords == nil {
		m.ReplaceWords = map[string]string{}
	}

	if secret.Data != nil {
		for _, v := range secret.Data {
			if v != nil && len(v) > 0 {
				// key := string(k)
				value := string(v)

				m.ReplaceWords[value] = m.replaceValue(value)
			}
		}
	}
}

// MaskLog returns the text with all of the secrets masked out
func (m *LogMasker) MaskLog(text string) string {
	answer := text
	for k, v := range m.ReplaceWords {
		answer = strings.Replace(answer, k, v, -1)
	}
	return answer
}

// MaskLogData masks the log data
func (m *LogMasker) MaskLogData(logData []byte) []byte {
	text := m.MaskLog(string(logData))
	return []byte(text)
}

// replaceMapValues adds all the string values in the given map to the replacer words
func (m *LogMasker) replaceMapValues(values map[string]interface{}) {
	for _, value := range values {
		childMap, ok := value.(map[string]interface{})
		if ok {
			m.replaceMapValues(childMap)
			continue
		}
		text, ok := value.(string)
		if ok {
			m.ReplaceWords[text] = m.replaceValue(text)
		}
	}
}

func (m *LogMasker) replaceValue(value string) string {
	return strings.Repeat("*", len(value))
}
