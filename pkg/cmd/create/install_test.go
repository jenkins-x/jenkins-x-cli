// +build unit

package create_test

import (
	"errors"
	"os"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"
	"github.com/jenkins-x/jx/pkg/kube/cluster"

	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	configio "github.com/jenkins-x/jx/pkg/io"
	"github.com/jenkins-x/jx/pkg/util"

	//. "github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
)

func TestInstall(t *testing.T) {
	t.Parallel()
	testDir := path.Join("test_data", "install_jenkins_x_versions")
	_, err := os.Stat(testDir)
	assert.NoError(t, err)

	configStore := configio.NewFileStore()
	version, err := create.LoadVersionFromCloudEnvironmentsDir(testDir, configStore)
	assert.NoError(t, err)

	assert.Equal(t, "0.0.3321", version, "For stable version in dir %s", testDir)
}

func TestGenerateProwSecret(t *testing.T) {
	fmt.Println(util.RandStringBytesMaskImprSrc(41))
}

func TestGetSafeUsername(t *testing.T) {
	t.Parallel()
	username := `Your active configuration is: [cloudshell-16392]
tutorial@bamboo-depth-206411.iam.gserviceaccount.com`
	assert.Equal(t, cluster.GetSafeUsername(username), "tutorial@bamboo-depth-206411.iam.gserviceaccount.com")

	username = `tutorial@bamboo-depth-206411.iam.gserviceaccount.com`
	assert.Equal(t, cluster.GetSafeUsername(username), "tutorial@bamboo-depth-206411.iam.gserviceaccount.com")
}

func TestCheckFlags(t *testing.T) {

	var tests = []struct {
		name           string
		in             *create.InstallFlags
		nextGeneration bool
		tekton         bool
		prow           bool
		staticJenkins  bool
		kaniko         bool
		provider       string
		dockerRegistry string
		err            error
	}{
		{
			name: "default",
			in: &create.InstallFlags{
				Provider: cloud.GKE,
			},
			nextGeneration: false,
			tekton:         false,
			prow:           false,
			staticJenkins:  true,
			kaniko:         false,
			dockerRegistry: "",
			err:            nil,
		},
		{
			name: "next_generation",
			in: &create.InstallFlags{
				NextGeneration: true,
				Provider:       cloud.GKE,
			},
			nextGeneration: true,
			tekton:         true,
			prow:           true,
			staticJenkins:  false,
			kaniko:         true,
			dockerRegistry: "gcr.io",
			err:            nil,
		},
		{
			name: "prow",
			in: &create.InstallFlags{
				Prow:     true,
				Provider: cloud.GKE,
			},
			nextGeneration: false,
			tekton:         true,
			prow:           true,
			staticJenkins:  false,
			kaniko:         true,
			dockerRegistry: "gcr.io",
			err:            nil,
		},
		{
			name: "tekton_and_gke",
			in: &create.InstallFlags{
				Tekton:   true,
				Provider: cloud.GKE,
			},
			nextGeneration: false,
			tekton:         true,
			prow:           true,
			staticJenkins:  false,
			kaniko:         true,
			dockerRegistry: "gcr.io",
			err:            nil,
		},
		{
			name: "tekton_and_eks",
			in: &create.InstallFlags{
				Tekton:   true,
				Provider: cloud.EKS,
			},
			nextGeneration: false,
			tekton:         true,
			prow:           true,
			staticJenkins:  false,
			kaniko:         false,
			dockerRegistry: "",
			err:            nil,
		},
		{
			name: "tekton_and_eks_and_kaniko",
			in: &create.InstallFlags{
				Tekton:   true,
				Provider: cloud.EKS,
				Kaniko:   true,
			},
			nextGeneration: false,
			tekton:         true,
			prow:           true,
			staticJenkins:  false,
			kaniko:         true,
			dockerRegistry: "",
			err:            nil,
		},
		{
			name: "tekton_with_a_custom_docker_registry",
			in: &create.InstallFlags{
				Tekton:         true,
				Provider:       cloud.GKE,
				DockerRegistry: "my.docker.registry.io",
			},
			nextGeneration: false,
			tekton:         true,
			prow:           true,
			staticJenkins:  false,
			kaniko:         true,
			dockerRegistry: "my.docker.registry.io",
			err:            nil,
		},
		{
			name: "next_generation_and_static_jenkins",
			in: &create.InstallFlags{
				NextGeneration: true,
				StaticJenkins:  true,
			},
			err: errors.New("incompatible options '--ng' and '--static-jenkins'. Please pick only one of them. We recommend --ng as --static-jenkins is deprecated"),
		},
		{
			name: "tekton_and_static_jenkins",
			in: &create.InstallFlags{
				Tekton:        true,
				StaticJenkins: true,
			},
			err: errors.New("incompatible options '--tekton' and '--static-jenkins'. Please pick only one of them. We recommend --tekton as --static-jenkins is deprecated"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := create.InstallOptions{
				CommonOptions: &opts.CommonOptions{
					BatchMode: true,
				},
				Flags:       *tt.in,
				InitOptions: initcmd.InitOptions{},
			}

			err := opts.CheckFlags()
			if tt.err != nil {
				assert.Equal(t, tt.err, err)
			} else {

				assert.NoError(t, err)

				assert.Equal(t, tt.nextGeneration, opts.Flags.NextGeneration, "NextGeneration flag is not as expected")
				assert.Equal(t, tt.tekton, opts.Flags.Tekton, "Tekton flag is not as expected")
				assert.Equal(t, tt.prow, opts.Flags.Prow, "Prow flag is not as expected")
				assert.Equal(t, tt.staticJenkins, opts.Flags.StaticJenkins, "StaticJenkins flag is not as expected")
				assert.Equal(t, tt.kaniko, opts.Flags.Kaniko, "Kaniko flag is not as expected")
				if tt.dockerRegistry != "" {
					assert.Equal(t, tt.dockerRegistry, opts.Flags.DockerRegistry, "DockerRegistry flag is not as expected")
				}
			}
		})
	}
}
