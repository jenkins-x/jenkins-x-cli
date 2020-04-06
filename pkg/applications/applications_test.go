// +build unit

package applications

import (
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppendMatchingDeployments(t *testing.T) {
	tests := []struct {
		name             string
		list             List
		environments     map[string]*v1.Environment
		deployments      map[string]map[string]appsv1.Deployment
		wantApplications int
		wantEnvironments int
		wantDeployments  int
	}{
		{
			"No source repositories found",
			List{},
			make(map[string]*v1.Environment),
			make(map[string]map[string]appsv1.Deployment),
			0, 0, 0,
		},
		{
			"Source repository doesn't have a matching deployment",
			List{
				[]Application{
					{
						&v1.SourceRepository{
							Spec: v1.SourceRepositorySpec{
								Repo: "my-repo-name",
							},
						},
						make(map[string]Environment),
					},
				},
			},
			make(map[string]*v1.Environment),
			make(map[string]map[string]appsv1.Deployment),
			1, 0, 0,
		},
		{
			"Source repository matches a single deployment",
			List{
				[]Application{
					{
						&v1.SourceRepository{
							Spec: v1.SourceRepositorySpec{
								Repo: "my-repo-name",
							},
						},
						make(map[string]Environment),
					},
				},
			},
			map[string]*v1.Environment{
				"staging": {
					Spec: v1.EnvironmentSpec{
						Namespace: "jx-staging",
						Kind:      v1.EnvironmentKindTypePermanent,
					},
				},
			},
			map[string]map[string]appsv1.Deployment{
				"staging": {
					"jx-staging": appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "my-repo-name",
								},
							},
						},
					},
				},
			},
			1, 1, 1,
		},
		{
			"Source repository matches multiple deployments",
			List{
				[]Application{
					{
						&v1.SourceRepository{
							Spec: v1.SourceRepositorySpec{
								Repo: "my-repo-name",
							},
						},
						make(map[string]Environment),
					},
				},
			},
			map[string]*v1.Environment{
				"staging": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "staging",
					},
					Spec: v1.EnvironmentSpec{
						Namespace: "jx-staging",
						Kind:      v1.EnvironmentKindTypePermanent,
					},
				},
				"production": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "production",
					},
					Spec: v1.EnvironmentSpec{
						Namespace: "jx-production",
						Kind:      v1.EnvironmentKindTypePermanent,
					},
				},
			},
			map[string]map[string]appsv1.Deployment{
				"staging": {
					"jx-staging": appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "jx-my-repo-name",
								},
							},
						},
					},
				},
				"production": {
					"jx-production": appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app": "jx-my-repo-name",
								},
							},
						},
					},
				},
			},
			1, 2, 1,
		},
	}

	for _, test := range tests {
		err := test.list.appendMatchingDeployments(test.environments, test.deployments)

		assert.NoError(t, err, test.name)
		assert.Equal(t, test.wantApplications, len(test.list.Items), test.name)

		envs := test.list.Environments()
		assert.Equal(t, test.wantEnvironments, len(envs), test.name)

		for env := range envs {
			assert.Equal(t, test.wantDeployments, len(test.list.Items[0].Environments[env].Deployments), test.name)
		}
	}
}
