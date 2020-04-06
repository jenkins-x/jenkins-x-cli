package kube

import (
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ObjectReference represents a reference to a k8s resource
type ObjectReference struct {
	APIVersion string `json:"apiVersion" protobuf:"bytes,5,opt,name=apiVersion"`
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	Kind string `json:"kind" protobuf:"bytes,1,opt,name=kind"`
	// Name of the referent.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
}

// IsResourceVersionNewer returns true if the first resource version is newer than the second
func IsResourceVersionNewer(v1 string, v2 string) bool {
	if v1 == v2 || v1 == "" {
		return false
	}
	if v2 == "" {
		return true
	}
	i1, e1 := strconv.Atoi(v1)
	i2, e2 := strconv.Atoi(v2)

	if e1 == nil && e2 != nil {
		return true
	}
	if e1 != nil {
		return false
	}
	return i1 > i2
}

// CreateObjectReference create an ObjectReference from the typed and object meta stuff
func CreateObjectReference(t metav1.TypeMeta, o metav1.ObjectMeta) ObjectReference {
	return ObjectReference{
		APIVersion: t.APIVersion,
		Kind:       t.Kind,
		Name:       o.Name,
	}
}
