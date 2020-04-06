// +build unit

package kube_test

import (
	"testing"

	jenkinsio_v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	versiond_mocks "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureDevEnvironmentSetup(t *testing.T) {
	t.Parallel()

	// mock versiond interface
	versiondInterface := versiond_mocks.NewSimpleClientset()

	// fixture
	envFixture := &jenkinsio_v1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name: kube.LabelValueDevEnvironment,
		},
		Spec: jenkinsio_v1.EnvironmentSpec{
			Namespace:         "jx-testing",
			Label:             "Development",
			PromotionStrategy: jenkinsio_v1.PromotionStrategyTypeNever,
			Kind:              jenkinsio_v1.EnvironmentKindTypeDevelopment,
			TeamSettings: jenkinsio_v1.TeamSettings{
				UseGitOps:           true,
				AskOnCreate:         false,
				QuickstartLocations: kube.DefaultQuickstartLocations,
				PromotionEngine:     jenkinsio_v1.PromotionEngineJenkins,
				AppsRepository:      kube.DefaultChartMuseumURL,
			},
		},
	}

	env, err := kube.EnsureDevEnvironmentSetup(versiondInterface, "jx-testing")

	assert.NoError(t, err, "Should not error")
	assert.Equal(t, envFixture.ObjectMeta.Name, env.ObjectMeta.Name)
	assert.Equal(t, envFixture.Spec.Namespace, env.Spec.Namespace)
	assert.Equal(t, envFixture.Spec.Label, env.Spec.Label)
	assert.Equal(t, jenkinsio_v1.PromotionStrategyType("Never"), env.Spec.PromotionStrategy)
	assert.Equal(t, jenkinsio_v1.EnvironmentKindType("Development"), env.Spec.Kind)
	assert.Equal(t, true, env.Spec.TeamSettings.UseGitOps)
	assert.Equal(t, false, env.Spec.TeamSettings.AskOnCreate)
	assert.Equal(t, []jenkinsio_v1.QuickStartLocation{{GitURL: "https://github.com", GitKind: "github", Owner: "jenkins-x-quickstarts", Includes: []string{"*"}, Excludes: []string{"WIP-*"}}}, env.Spec.TeamSettings.QuickstartLocations)
	assert.Equal(t, jenkinsio_v1.PromotionEngineType("Jenkins"), env.Spec.TeamSettings.PromotionEngine)
	assert.Equal(t, envFixture.Spec.TeamSettings.AppsRepository, env.Spec.TeamSettings.AppsRepository)
}
