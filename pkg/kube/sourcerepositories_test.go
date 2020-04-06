// +build unit

package kube_test

import (
	"testing"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ns = "jx"
)

func TestFindSourceRepository(t *testing.T) {
	existingRepos := &v1.SourceRepositoryList{
		Items: []v1.SourceRepository{
			// Standard auto-created source repository
			createSourceRepository("first-org-first-repo", "first-org", "first-repo", "https://github.com", false),
			// Arbitrary name
			createSourceRepository("random-name", "second-org", "second-repo", "https://github.com", false),
			// Unlabeled, to verify proper behavior with legacy autocreated source repositories without labels
			createSourceRepository("third-org-third-repo", "third-org", "third-repo", "https://github.com", true),
		},
	}
	jxClient := fake.NewSimpleClientset(existingRepos)

	// Test the standard auto-created
	firstSr, err := kube.FindSourceRepository(jxClient, ns, "first-org", "first-repo", "github")
	assert.NoError(t, err)
	assert.NotNil(t, firstSr)
	assert.Equal(t, "first-org-first-repo", firstSr.Name)

	// Test the arbitrary name
	secondSr, err := kube.FindSourceRepository(jxClient, ns, "second-org", "second-repo", "github")
	assert.NoError(t, err)
	assert.NotNil(t, secondSr)
	assert.Equal(t, "random-name", secondSr.Name)

	// Test the unlabeled case
	thirdSr, err := kube.FindSourceRepository(jxClient, ns, "third-org", "third-repo", "github")
	assert.NoError(t, err)
	assert.NotNil(t, thirdSr)
	assert.Equal(t, "third-org-third-repo", thirdSr.Name)
	assert.Equal(t, "", thirdSr.Labels[v1.LabelOwner])
}

func TestGetOrCreateSourceRepositories(t *testing.T) {
	tests := []struct {
		name         string
		org          string
		repo         string
		providerURL  string
		existingName string
	}{
		{
			name:        "no existing repo",
			org:         "some-org",
			repo:        "some-repo",
			providerURL: "https://github.com",
		},
		{
			name:         "existing repo, standard name",
			org:          "some-org",
			repo:         "some-repo",
			providerURL:  "https://github.com",
			existingName: "some-org-some-repo",
		},
		{
			name:         "existing repo, different name",
			org:          "some-org",
			repo:         "some-repo",
			providerURL:  "https://github.com",
			existingName: "some-arbitrary-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var existingRepo v1.SourceRepository
			if tt.existingName != "" {
				existingRepo = createSourceRepository(tt.existingName, tt.org, tt.repo, tt.providerURL, false)
			}
			var jxClient *fake.Clientset
			if tt.existingName != "" {
				jxClient = fake.NewSimpleClientset(&existingRepo)
			} else {
				jxClient = fake.NewSimpleClientset()
			}

			createdOrExisting, err := kube.GetOrCreateSourceRepository(jxClient, ns, tt.repo, tt.org, tt.providerURL)
			assert.NoError(t, err)
			assert.NotNil(t, createdOrExisting)

			if tt.existingName == "" {
				createdRepoName := naming.ToValidName(tt.org + "-" + tt.repo)
				assert.Equal(t, createdRepoName, createdOrExisting.Name, "new SourceRepository name should be %s but is %s", createdRepoName, createdOrExisting.Name)
			} else {
				assert.Equal(t, existingRepo.Name, createdOrExisting.Name, "existing or updated repository name should be %s but is %s", existingRepo.Name, createdOrExisting.Name)
			}

			assert.Equal(t, tt.org, createdOrExisting.Spec.Org)
			assert.Equal(t, tt.repo, createdOrExisting.Spec.Repo)
			assert.Equal(t, kube.ToProviderName(tt.providerURL), createdOrExisting.Spec.ProviderName)
		})
	}
}

func createSourceRepository(name, org, repo, providerURL string, skipLabels bool) v1.SourceRepository {
	labels := make(map[string]string)
	if !skipLabels {
		labels[v1.LabelOwner] = org
		labels[v1.LabelRepository] = repo
		labels[v1.LabelProvider] = kube.ToProviderName(providerURL)
	}

	return v1.SourceRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SourceRepository",
			APIVersion: jenkinsio.GroupName + "/" + jenkinsio.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: v1.SourceRepositorySpec{
			Org:          org,
			Provider:     providerURL,
			ProviderName: kube.ToProviderName(providerURL),
			Repo:         repo,
		},
	}

}
