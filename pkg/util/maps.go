package util

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
)

// StringMapHasValue returns true if the given map contains the given value
func StringMapHasValue(m map[string]string, value string) bool {
	if m == nil {
		return false
	}
	for _, v := range m {
		if v == value {
			return true
		}
	}
	return false
}

// MapKeys returns the keys of a given map
func MapKeys(m map[string]string) []string {
	keys := []string{}
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// MergeMaps merges all the maps together with the entries in the last map overwriting any earlier values
//
// so if you want to add some annotations to a resource you can do
// resource.Annotations = kube.MergeMaps(resource.Annotations, myAnnotations)
func MergeMaps(maps ...map[string]string) map[string]string {
	answer := map[string]string{}
	for _, m := range maps {
		if m != nil {
			for k, v := range m {
				answer[k] = v
			}
		}
	}
	return answer
}

// CombineMapTrees recursively copies all the values from the input map into the destination map preserving any missing entries in the destination
func CombineMapTrees(destination map[string]interface{}, input map[string]interface{}) {
	for k, v := range input {
		old, exists := destination[k]
		if exists {
			vm, ok := v.(map[string]interface{})
			if ok {
				oldm, ok := old.(map[string]interface{})
				if ok {
					// if both entries are maps lets combine them
					// otherwise we assume that the input entry is correct
					CombineMapTrees(oldm, vm)
					continue
				}
			}
		}
		destination[k] = v
	}
}

// GetMapValueViaPath returns the value at the given path
// mean `m["foo"]["bar"]["whatnot"]`
func GetMapValueViaPath(m map[string]interface{}, path string) interface{} {
	dest := m
	paths := strings.Split(path, ".")

	last := len(paths) - 1
	for i, key := range paths {
		if i == last {
			return dest[key]
		}
		entry := dest[key]
		entryMap, ok := entry.(map[string]interface{})
		if ok {
			dest = entryMap
		} else {
			entryMap = map[string]interface{}{}
			dest[key] = entryMap
			dest = entryMap
		}
	}
	return nil
}

// GetMapValueAsStringViaPath returns the string value at the given path
// mean `m["foo"]["bar"]["whatnot"]`
func GetMapValueAsStringViaPath(m map[string]interface{}, path string) string {
	value := GetMapValueViaPath(m, path)
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

// GetMapValueAsIntViaPath returns the int value at the given path
// mean `m["foo"]["bar"]["whatnot"]`
func GetMapValueAsIntViaPath(m map[string]interface{}, path string) int {
	value := GetMapValueViaPath(m, path)
	n, ok := value.(int)
	if !ok {
		f, ok := value.(float64)
		if ok {
			return int(f)
		}
		return 0
	}
	return n
}

// GetMapValueAsMapViaPath returns the map value at the given path
// mean `m["foo"]["bar"]["whatnot"]`
func GetMapValueAsMapViaPath(m map[string]interface{}, path string) map[string]interface{} {
	value := GetMapValueViaPath(m, path)
	answer, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}
	return answer
}

// SetMapValueViaPath sets the map key using the given path which supports the form `foo.bar.whatnot` to
// mean `m["foo"]["bar"]["whatnot"]` lazily creating maps as the path is navigated
func SetMapValueViaPath(m map[string]interface{}, path string, value interface{}) {
	dest := m
	paths := strings.Split(path, ".")

	last := len(paths) - 1
	for i, key := range paths {
		if i == last {
			dest[key] = value
		} else {
			entry := dest[key]
			entryMap, ok := entry.(map[string]interface{})
			if ok {
				dest = entryMap
			} else {
				entryMap = map[string]interface{}{}
				dest[key] = entryMap
				dest = entryMap
			}
		}
	}
}

// ToObjectMap converts the given object into a map of strings/maps using YAML marshalling
func ToObjectMap(object interface{}) (map[string]interface{}, error) {
	answer := map[string]interface{}{}
	data, err := yaml.Marshal(object)
	if err != nil {
		return answer, err
	}
	err = yaml.Unmarshal(data, &answer)
	return answer, err
}

// KeyValuesToMap converts the set of values of the form "foo=abc" into a map
func KeyValuesToMap(values []string) map[string]string {
	answer := map[string]string{}
	for _, kv := range values {
		tokens := strings.SplitN(kv, "=", 2)
		if len(tokens) > 1 {
			k := tokens[0]
			v := tokens[1]
			answer[k] = v
		}
	}
	return answer
}

// MapToKeyValues converts the the map into a sorted array of key/value pairs
func MapToKeyValues(values map[string]string) []string {
	answer := []string{}
	for k, v := range values {
		answer = append(answer, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(answer)
	return answer
}

// MapToString converts the map to a string
func MapToString(m map[string]string) string {
	builder := strings.Builder{}
	for k, v := range m {
		if builder.Len() > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(k)
		builder.WriteString("=")
		builder.WriteString(v)
	}
	return builder.String()
}
