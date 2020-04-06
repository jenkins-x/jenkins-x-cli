package kube

import (
	"sort"
	"strings"

	"github.com/blang/semver"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetOrCreateRelease creates or updates the given release resource
func GetOrCreateRelease(jxClient versioned.Interface, ns string, release *v1.Release) (*v1.Release, error) {
	releaseInterface := jxClient.JenkinsV1().Releases(ns)
	name := release.Name
	old, err := releaseInterface.Get(name, metav1.GetOptions{})
	if err == nil {
		old.Spec = release.Spec
		answer, err := releaseInterface.PatchUpdate(old)
		if err != nil {
			return answer, errors.Wrapf(err, "Failed to update Release %s in namespace %s", name, ns)
		}
		return answer, nil
	}
	answer, err := releaseInterface.Create(release)
	if err != nil {
		return answer, errors.Wrapf(err, "Failed to create Release %s in namespace %s", name, ns)
	}
	return answer, nil
}

type ReleaseOrder []v1.Release

func (a ReleaseOrder) Len() int      { return len(a) }
func (a ReleaseOrder) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ReleaseOrder) Less(i, j int) bool {
	r1 := a[i]
	r2 := a[j]

	n1 := r1.Spec.Name
	n2 := r2.Spec.Name
	if n1 != n2 {
		return n1 < n2
	}

	// lets return newest releases first
	v1 := r1.Spec.Version
	v2 := r2.Spec.Version

	if v1 == "" || v2 == "" {
		return v1 > v2
	}

	sv1, err1 := semver.Parse(v1)
	sv2, err2 := semver.Parse(v2)

	if err1 != nil && err2 != nil {
		return v1 > v2
	}
	if err1 != nil && err2 == nil {
		return false
	}
	if err1 == nil && err2 != nil {
		return true
	}
	return sv1.Compare(sv2) > 0
}

// SortReleases sorts the releases in name order then latest version first
func SortReleases(releases []v1.Release) {
	sort.Sort(ReleaseOrder(releases))
}

// GetOrderedReleases returns the releases sorted in newest release first
func GetOrderedReleases(jxClient versioned.Interface, ns string, filter string) ([]v1.Release, error) {
	releaseInterface := jxClient.JenkinsV1().Releases(ns)
	answer := []v1.Release{}
	list, err := releaseInterface.List(metav1.ListOptions{})
	if err != nil {
		return answer, err
	}
	for _, release := range list.Items {
		if filter == "" || strings.Index(release.Name, filter) >= 0 {
			answer = append(answer, release)
		}
	}
	SortReleases(answer)
	return answer, nil
}
