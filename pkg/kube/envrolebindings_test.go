package kube

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
)

func TestEnvironmentFilters(t *testing.T) {
	environments := []*v1.Environment{
		NewPermanentEnvironment("staging"),
		NewPermanentEnvironment("production"),
		NewPreviewEnvironment("jstrachan-demo96-pr-1"),
		NewPreviewEnvironment("jstrachan-another-pr-3"),
	}

	assertEnvironmentsFilter(t, environments,
		nil,
		"staging", "production", "jstrachan-demo96-pr-1", "jstrachan-another-pr-3")

	assertEnvironmentsFilter(t, environments,
		[]v1.EnvironmentFilter{{}},
		"staging", "production", "jstrachan-demo96-pr-1", "jstrachan-another-pr-3")

	assertEnvironmentsFilter(t, environments,
		[]v1.EnvironmentFilter{{
			Kind: v1.EnvironmentKindTypePreview,
		}},
		"jstrachan-demo96-pr-1", "jstrachan-another-pr-3")

	assertEnvironmentsFilter(t, environments,
		[]v1.EnvironmentFilter{{
			Kind: v1.EnvironmentKindTypePermanent,
		}},
		"staging", "production")

	assertEnvironmentsFilter(t, environments,
		[]v1.EnvironmentFilter{{
			Kind: v1.EnvironmentKindTypePermanent,
		}, {
			Kind: v1.EnvironmentKindTypePreview,
		}},
		"staging", "production", "jstrachan-demo96-pr-1", "jstrachan-another-pr-3")

	assertEnvironmentsFilter(t, environments,
		[]v1.EnvironmentFilter{{
			Kind:     v1.EnvironmentKindTypePermanent,
			Excludes: []string{"prod*"},
		}},
		"staging")
}

func assertEnvironmentsFilter(t *testing.T, environments []*v1.Environment, filters []v1.EnvironmentFilter, expectedNames ...string) {
	actual := []string{}
	for _, env := range environments {
		if EnvironmentMatchesAny(env, filters) {
			actual = append(actual, env.Name)
		}
	}
	assert.Equal(t, expectedNames, actual, "for filters %#v", filters)
}
