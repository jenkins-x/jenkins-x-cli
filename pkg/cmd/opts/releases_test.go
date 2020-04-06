// +build unit

package opts_test

import (
	"testing"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	clients_test "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jenkins-x/jx/pkg/cmd/opts"

	"github.com/stretchr/testify/assert"
)

func TestParseDependencyUpdateMessage(t *testing.T) {

	mockFactory := clients_test.NewMockFactory()
	commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)
	mockHelmer := helm_test.NewMockHelmer()
	installerMock := resources_test.NewMockInstaller()
	testhelpers.ConfigureTestOptionsWithResources(&commonOpts,
		[]runtime.Object{},
		[]runtime.Object{},
		gits.NewGitFake(),
		gits.NewFakeProvider(&gits.FakeRepository{
			Owner: "pmuir",
			GitRepo: &gits.GitRepository{
				Name: "brie",
			},
		}),
		mockHelmer,
		installerMock,
	)
	testhelpers.MockFactoryWithKubeClients(mockFactory, &commonOpts)

	t.Run("full", func(t *testing.T) {
		assertParseDependencyUpdateMessage(t, `chore(dependencies): update https://github.com/pmuir/brie from 1.2.3 to 1.2.4

update BRIE_VERSION to 1.2.4`, v1.DependencyUpdate{
			DependencyUpdateDetails: v1.DependencyUpdateDetails{
				Owner:       "pmuir",
				Repo:        "brie",
				ToVersion:   "1.2.4",
				FromVersion: "1.2.3",
				Host:        "github.com",
			},
		}, &commonOpts)
	})
	t.Run("with-dot", func(t *testing.T) {
		assertParseDependencyUpdateMessage(t, `chore(dependencies): update https://github.com/pmuir/brie.git from 1.2.3 to 1.2.4

update BRIE_VERSION to 1.2.4`, v1.DependencyUpdate{
			DependencyUpdateDetails: v1.DependencyUpdateDetails{
				Owner:       "pmuir",
				Repo:        "brie",
				ToVersion:   "1.2.4",
				FromVersion: "1.2.3",
				Host:        "github.com",
			},
		}, &commonOpts)
	})
	t.Run("simple", func(t *testing.T) {
		assertParseDependencyUpdateMessage(t, `chore(dependencies): update pmuir/brie from 1.2.3 to 1.2.4

update BRIE_VERSION to 1.2.4`, v1.DependencyUpdate{
			DependencyUpdateDetails: v1.DependencyUpdateDetails{

				Owner:       "pmuir",
				Repo:        "brie",
				ToVersion:   "1.2.4",
				FromVersion: "1.2.3",
				Host:        "fake.git",
			},
		}, &commonOpts)
	})
	t.Run("fullbump", func(t *testing.T) {
		assertParseDependencyUpdateMessage(t, `chore(deps): bump https://github.com/pmuir/brie from 1.2.3 to 1.2.4

update BRIE_VERSION to 1.2.4`, v1.DependencyUpdate{
			DependencyUpdateDetails: v1.DependencyUpdateDetails{
				Owner:       "pmuir",
				Repo:        "brie",
				ToVersion:   "1.2.4",
				FromVersion: "1.2.3",
				Host:        "github.com",
			},
		}, &commonOpts)
	})
	t.Run("simplebump", func(t *testing.T) {
		assertParseDependencyUpdateMessage(t, `chore(deps): bump pmuir/brie from 1.2.3 to 1.2.4

update BRIE_VERSION to 1.2.4`, v1.DependencyUpdate{
			DependencyUpdateDetails: v1.DependencyUpdateDetails{
				Owner:       "pmuir",
				Repo:        "brie",
				ToVersion:   "1.2.4",
				FromVersion: "1.2.3",
				Host:        "fake.git",
			},
		}, &commonOpts)
	})

}

func assertParseDependencyUpdateMessage(t *testing.T, msg string, expected v1.DependencyUpdate, o *opts.CommonOptions) {
	update, _, err := o.ParseDependencyUpdateMessage(msg, "https://fake.git/acme/cheese")
	assert.NoError(t, err)
	assert.NotNil(t, update)
	assert.Equal(t, expected.Owner, update.Owner)
	assert.Equal(t, expected.Repo, update.Repo)
	assert.Equal(t, expected.ToVersion, update.ToVersion)
	assert.Equal(t, expected.FromVersion, update.FromVersion)
	assert.Equal(t, expected.Host, update.Host)
}
