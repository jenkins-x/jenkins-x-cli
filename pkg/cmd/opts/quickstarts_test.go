// +build unit

package opts_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/clients/fake"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"
	"github.com/jenkins-x/jx/pkg/gits"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	resources_test "github.com/jenkins-x/jx/pkg/kube/resources/mocks"
	"github.com/jenkins-x/jx/pkg/quickstarts"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
)

func TestLoadQuickStarts(t *testing.T) {

	mockFactory := fake.NewFakeFactory()
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

	versionsDir := filepath.Join("test_data", "quickstarts", "version_stream")
	assert.DirExists(t, versionsDir, "no version stream source directory exists")

	resolver := &versionstream.VersionResolver{
		VersionsDir: versionsDir,
	}
	commonOpts.SetVersionResolver(resolver)

	model, err := commonOpts.LoadQuickStartsModel(nil, false)
	require.NoError(t, err, "LoadQuickStartsModel")

	assert.True(t, len(model.Quickstarts) > 0, "quickstart model should not be empty")

	assertQuickStart(t, model, "node-http", "JavaScript")
	assertQuickStart(t, model, "golang-http", "Go")
}

func assertQuickStart(t *testing.T, model *quickstarts.QuickstartModel, name string, language string) {
	owner := "jenkins-x-quickstarts"
	id := fmt.Sprintf("%s/%s", owner, name)

	qs := model.Quickstarts[id]
	require.NotNil(t, qs, "could not find quickstart for id %s", id)

	assert.Equal(t, owner, qs.Owner, "quickstart.Owner")
	assert.Equal(t, name, qs.Name, "quickstart.Name")
	assert.Equal(t, language, qs.Language, "quickstart.Language")
	assert.Equal(t, id, qs.ID, "quickstart.ID")
}
