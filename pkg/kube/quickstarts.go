package kube

import (
	"fmt"
	"reflect"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/gits"
)

var (
	DefaultQuickstartLocations = []v1.QuickStartLocation{
		{
			GitURL:   gits.GitHubURL,
			GitKind:  gits.KindGitHub,
			Owner:    "jenkins-x-quickstarts",
			Includes: []string{"*"},
			Excludes: []string{"WIP-*"},
		},
	}
)

// GetQuickstartLocations returns the current quickstart locations. If no locations are defined
// yet lets return the defaults
func GetQuickstartLocations(jxClient versioned.Interface, ns string) ([]v1.QuickStartLocation, error) {
	var answer []v1.QuickStartLocation
	env, err := EnsureDevEnvironmentSetup(jxClient, ns)
	if err != nil {
		return answer, err
	}
	if env == nil {
		return answer, fmt.Errorf("No Development environment found for namespace %s", ns)
	}
	answer = env.Spec.TeamSettings.QuickstartLocations
	return answer, nil
}

// IsDefaultQuickstartLocation checks whether the given quickstart location is a default one, and if so returns true
func IsDefaultQuickstartLocation(location v1.QuickStartLocation) bool {
	for _, l := range DefaultQuickstartLocations {
		if reflect.DeepEqual(l, location) {
			return true
		}
	}
	return false
}
