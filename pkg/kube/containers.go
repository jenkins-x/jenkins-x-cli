package kube

import (
	corev1 "k8s.io/api/core/v1"
)

// GetEnvVar returns the env var if its defined for the given name
func GetEnvVar(container *corev1.Container, name string) *corev1.EnvVar {
	if container == nil {
		return nil
	}
	return GetSliceEnvVar(container.Env, name)
}

func GetVolumeMount(volumenMounts *[]corev1.VolumeMount, name string) *corev1.VolumeMount {
	if volumenMounts != nil {
		for idx, v := range *volumenMounts {
			if v.Name == name {
				return &(*volumenMounts)[idx]
			}
		}
	}
	return nil
}

func GetVolume(volumes *[]corev1.Volume, name string) *corev1.Volume {
	if volumes != nil {
		for idx, v := range *volumes {
			if v.Name == name {
				return &(*volumes)[idx]
			}
		}
	}
	return nil

}

func entrypointIndex(args []string) *int {
	for i, a := range args {
		if a == "-entrypoint" {
			return &i
		}
	}

	return nil
}

// GetCommandAndArgs extracts the command and arguments for the container, taking into account the
// entrypoint invocation if it's not an init container
func GetCommandAndArgs(container *corev1.Container, isInit bool) ([]string, []string) {
	if isInit {
		return container.Command, container.Args
	}

	idx := entrypointIndex(container.Args)

	if idx == nil {
		// TODO: Logging here probably
		return nil, nil
	} else if *idx >= len(container.Args) {
		// TODO: Logging here probably
		return nil, nil
	}

	// Args ends up as [..., "-entrypoint", COMMAND, "--", ARGS...]
	return []string{container.Args[*idx+1]}, container.Args[*idx+3:]
}
