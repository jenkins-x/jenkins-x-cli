package kube

import (
	corev1 "k8s.io/api/core/v1"
)

// CombineVolumes combines all the given volumes together ignoring duplicates
func CombineVolumes(volumes []corev1.Volume, otherVolumes ...corev1.Volume) []corev1.Volume {
	answer := append([]corev1.Volume{}, volumes...)
	for _, v := range otherVolumes {
		if !ContainsVolume(answer, v) {
			answer = append(answer, v)
		}
	}
	return answer
}

// ContainsVolume returns true if the given volume slice contains the given volume
func ContainsVolume(volumes []corev1.Volume, volume corev1.Volume) bool {
	for _, v := range volumes {
		if v.Name == volume.Name {
			return true
		}
	}
	return false
}

// ContainsVolumeMount returns true if the given volume mount slice contains the given volume
func ContainsVolumeMount(volumes []corev1.VolumeMount, volume corev1.VolumeMount) bool {
	for _, v := range volumes {
		if v.Name == volume.Name {
			return true
		}
	}
	return false
}
