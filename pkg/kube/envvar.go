package kube

import (
	corev1 "k8s.io/api/core/v1"
)

// GetSliceEnvVar returns the EnvVar for the given name or nil if none exists in the slice
func GetSliceEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, envVar := range envVars {
		if envVar.Name == name {
			copy := envVar
			return &copy
		}
	}
	return nil
}

// SetEnvVar returns the env vars with the env var of the given name updated or appended
func SetEnvVar(envVars []corev1.EnvVar, name string, value string) []corev1.EnvVar {
	for i := range envVars {
		if envVars[i].Name == name {
			envVars[i].Value = value
			return envVars
		}
	}
	envVars = append(envVars, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
	return envVars
}
