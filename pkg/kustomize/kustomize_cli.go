package kustomize

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// KustomizeCLI implements common kustomize actions based on kustomize CLI
type KustomizeCLI struct {
	Runner util.Commander
}

// NewKustomizeCLI creates a new KustomizeCLI instance configured to use the provided kustomize CLI in
// the given current working directory
func NewKustomizeCLI() *KustomizeCLI {
	runner := &util.Command{
		Name: "kustomize",
	}
	cli := &KustomizeCLI{
		Runner: runner,
	}
	return cli
}

// Version executes the Kustomize version command and returns its output
func (k *KustomizeCLI) Version(extraArgs ...string) (string, error) {
	args := []string{"version", "--short"}
	args = append(args, extraArgs...)
	version, err := k.runKustomizeWithOutput(args...)
	if err != nil {
		return "", err
	}
	return extractSemanticVersion(version)
}

func (k *KustomizeCLI) runKustomizeWithOutput(args ...string) (string, error) {
	k.Runner.SetArgs(args)
	return k.Runner.RunWithoutRetry()
}

// extractSemanticVersion return the semantic version string out of given version cli output.
// currently tested on {Version:3.5.4 GitCommit ....} and {Version:kustomize/v3.5.4 GitCommit: ...}
func extractSemanticVersion(version string) (string, error) {
	regex, err := regexp.Compile(`[0-9]+\.[0-9]+\.[0-9]+`)
	if err != nil {
		return "", errors.Wrapf(err, "not able to extract a semantic version of kustomize version output")
	}
	return regex.FindString(version), nil
}

// ContainsKustomizeConfig finds out if there is any kustomize resource in the cwd or subdirectories
func (k *KustomizeCLI) ContainsKustomizeConfig(dir string) bool {
	if len(k.FindKustomizationYamlPaths(dir)) != 0 {
		return true
	}

	return false
}

// FindKustomizationYamlPaths looks for the kustomization.yaml i.e. kustomize resources in present and sub-directories
func (k *KustomizeCLI) FindKustomizationYamlPaths(dir string) []string {
	fp, err := filepath.Abs(dir)
	var resources []string
	err = filepath.Walk(fp, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, "kustomization.yaml") {
			resources = append(resources, path)
		}
		return nil
	})
	if err != nil {
		log.Logger().Errorf("problem finding kustomize resources %s ", err)
	}
	return resources
}
