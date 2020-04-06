package kube

import (
	"fmt"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/util"
)

// FieldMap is a map of field:value. It implements fields.Fields.
type fieldMap map[string]interface{}

func newFieldMap(pipelineActivity v1.PipelineActivity) (fieldMap, error) {
	return util.ToObjectMap(pipelineActivity)
}

// Has returns whether the provided field exists in the map.
func (m fieldMap) Has(field string) bool {
	_, exists := m.get(field)
	return exists
}

// Get returns the value in the map for the provided field.
func (m fieldMap) Get(field string) string {
	val, _ := m.get(field)
	return val
}

// Get returns the value in the map for the provided field.
func (m fieldMap) get(field string) (string, bool) {
	pathElements := strings.Split(field, ".")
	valueMap := m
	value := ""
	for i, element := range pathElements {
		tmp, exists := valueMap[element]
		if !exists {
			return "", false
		}

		if i == len(pathElements)-1 {
			value = fmt.Sprintf("%v", tmp)
		} else {
			switch v := tmp.(type) {
			case map[string]interface{}:
				valueMap = v
			default:
				return "", false
			}
		}
	}

	return value, true
}
