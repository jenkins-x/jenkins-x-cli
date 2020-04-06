// +build unit

package jenkinsfile_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/stretchr/testify/assert"
)

func TestGetLifecycleReturnsSetup(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	lifecycles.Setup = &jenkinsfile.PipelineLifecycle{}
	lifecycle, _ := lifecycles.GetLifecycle("setup", false)
	assert.Equal(t, lifecycles.Setup, lifecycle)
}

func TestGetLifecycleReturnsSetVersion(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	lifecycles.SetVersion = &jenkinsfile.PipelineLifecycle{}
	lifecycle, _ := lifecycles.GetLifecycle("setversion", false)
	assert.Equal(t, lifecycles.SetVersion, lifecycle)
}

func TestGetLifecycleReturnsPreBuild(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	lifecycles.PreBuild = &jenkinsfile.PipelineLifecycle{}
	lifecycle, _ := lifecycles.GetLifecycle("prebuild", false)
	assert.Equal(t, lifecycles.PreBuild, lifecycle)
}

func TestGetLifecycleReturnsBuild(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	lifecycles.Build = &jenkinsfile.PipelineLifecycle{}
	lifecycle, _ := lifecycles.GetLifecycle("build", false)
	assert.Equal(t, lifecycles.Build, lifecycle)
}

func TestGetLifecycleReturnsPostBuild(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	lifecycles.PostBuild = &jenkinsfile.PipelineLifecycle{}
	lifecycle, _ := lifecycles.GetLifecycle("postbuild", false)
	assert.Equal(t, lifecycles.PostBuild, lifecycle)
}

func TestGetLifecycleReturnsPromote(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	lifecycles.Promote = &jenkinsfile.PipelineLifecycle{}
	lifecycle, _ := lifecycles.GetLifecycle("promote", false)
	assert.Equal(t, lifecycles.Promote, lifecycle)
}

func TestGetLifecycleReturnsEmptyLifecycle(t *testing.T) {
	names := []string{"setup", "setversion", "prebuild", "build", "postbuild", "promote"}
	for _, name := range names {
		lifecycles := jenkinsfile.PipelineLifecycles{}
		lifecycles.Setup = &jenkinsfile.PipelineLifecycle{}
		lifecycle, _ := lifecycles.GetLifecycle(name, true)
		assert.NotNil(t, lifecycle)
	}
}

func TestGetLifecycleReturnsError(t *testing.T) {
	lifecycles := jenkinsfile.PipelineLifecycles{}
	_, err := lifecycles.GetLifecycle("something-else", false)
	assert.Error(t, err)
}
