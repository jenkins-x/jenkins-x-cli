package kustomize

// Kustomizer defines common kustomize actions used within Jenkins X
type Kustomizer interface {
	Version(extraArgs ...string) (string, error)
	ContainsKustomizeConfig(dir string) bool
	FindKustomizationYamlPaths(dir string) (resource []string)
}
