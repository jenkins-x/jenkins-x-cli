package kube

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/kserving"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetVersion returns the version from the labels on the deployment if it can be deduced
func GetVersion(r *metav1.ObjectMeta) string {
	if r != nil {
		labels := r.Labels
		if labels != nil {
			v := labels["version"]
			if v != "" {
				return v
			}
			v = labels["chart"]
			if v != "" {
				arr := strings.Split(v, "-")
				last := arr[len(arr)-1]
				if last != "" {
					return last
				}
				return v
			}

			// find the kserve revision
			kversion := labels[kserving.RevisionLabel]
			if kversion != "" {
				idx := strings.LastIndex(kversion, "-")
				if idx > 0 {
					kversion = kversion[idx+1:]
				}
				return kversion
			}
		}
	}
	return ""
}

// GetPodVersion returns the version for the given app name
func GetPodVersion(pod *corev1.Pod, appName string) string {
	v := GetVersion(&pod.ObjectMeta)
	if v != "" {
		return v
	}
	if appName == "" {
		appName = GetName(&pod.ObjectMeta)
	}
	if appName != "" {
		for _, c := range pod.Spec.Containers {
			image := c.Image
			idx := strings.LastIndex(image, ":")
			if idx > 0 {
				version := image[idx+1:]
				prefix := image[0:idx]
				if prefix == appName || strings.HasSuffix(prefix, "/"+appName) {
					return version
				}
			}
		}
	}
	return ""
}

// GetName returns the app name
func GetName(r *metav1.ObjectMeta) string {
	if r != nil {
		ns := r.Namespace
		labels := r.Labels
		if labels != nil {
			name := labels["app"]
			if name != "" {
				prefix := ns + "-"
				if !strings.HasPrefix(name, prefix) {
					prefix = "jx-"
				}

				// for helm deployments which prefix the namespace in the name lets strip it
				if strings.HasPrefix(name, prefix) {
					name = strings.TrimPrefix(name, prefix)

					// we often have the app name repeated twice!
					l := len(name) / 2
					if name[l] == '-' {
						first := name[0:l]
						if name[l+1:] == first {
							return first
						}
					}
				}
				return name
			}
		}
		name := r.Name

		if ns != "" {
			prefix := ns + "-"
			if !strings.HasPrefix(name, prefix) {
				prefix = "jx-"
			}

			// for helm deployments which prefix the namespace in the name lets strip it
			if strings.HasPrefix(name, prefix) {
				name = strings.TrimPrefix(name, prefix)
				return name
			}
		}
		return name
	}
	return ""
}

// GetAppName returns the app name
func GetAppName(name string, namespaces ...string) string {
	if name != "" {
		for _, ns := range namespaces {
			// for helm deployments which prefix the namespace in the name lets strip it
			prefix := ns + "-"
			if strings.HasPrefix(name, prefix) {
				name = strings.TrimPrefix(name, prefix)
			}
		}

		// we often have the app name repeated twice - particularly when using helm 3
		l := len(name) / 2
		if name[l] == '-' {
			first := name[0:l]
			if name[l+1:] == first {
				return first
			}
		}

		// The applications seems to be prefixed with jx regardless of the namespace
		// where they are deployed. Let's remove this prefix.
		prefix := "jx-"
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

// GetCommitSha returns the git commit sha
func GetCommitSha(r *metav1.ObjectMeta) string {
	if r != nil {
		annotations := r.Annotations
		if annotations != nil {
			return annotations["jenkins.io/git-sha"]
		}
	}
	return ""
}

// GetCommitURL returns the git commit URL
func GetCommitURL(r *metav1.ObjectMeta) string {
	if r != nil {
		annotations := r.Annotations
		if annotations != nil {
			return annotations["jenkins.io/git-url"]
		}
	}
	return ""
}

func GetEditAppName(name string) string {
	// we often have the app name repeated twice!
	l := len(name) / 2
	if name[l] == '-' {
		first := name[0:l]
		if name[l+1:] == first {
			return first
		}
	}
	return name
}
