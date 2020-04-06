// +build unit

package upgrade

import (
	"fmt"
	"testing"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/brew"
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/version"
	"github.com/stretchr/testify/assert"
)

var sampleBrewInfo = `[  
   {  
      "name":"jx",
      "full_name":"jenkins-x/jx/jx",
      "oldname":null,
      "aliases":[  

      ],
      "versioned_formulae":[  

      ],
      "desc":"A tool to install and interact with Jenkins X on your Kubernetes cluster.",
      "homepage":"https://jenkins-x.github.io/jenkins-x-website/",
      "versions":{  
         "stable":"2.0.181",
         "devel":null,
         "head":null,
         "bottle":false
      },
      "revision":0,
      "version_scheme":0,
      "bottle":{  

      },
      "keg_only":false,
      "bottle_disabled":false,
      "options":[  

      ],
      "build_dependencies":[  

      ],
      "dependencies":[  

      ],
      "recommended_dependencies":[  

      ],
      "optional_dependencies":[  

      ],
      "requirements":[  

      ],
      "conflicts_with":[  

      ],
      "caveats":null,
      "installed":[  
         {  
            "version":"2.0.181",
            "used_options":[  

            ],
            "built_as_bottle":false,
            "poured_from_bottle":false,
            "runtime_dependencies":[  

            ],
            "installed_as_dependency":false,
            "installed_on_request":true
         }
      ],
      "linked_keg":"2.0.181",
      "pinned":false,
      "outdated":false
   }
]`

func TestLatestJxBrewVersion(t *testing.T) {
	version, err := brew.LatestJxBrewVersion(sampleBrewInfo)
	assert.NoError(t, err)
	assert.Equal(t, "2.0.181", version)
}

func TestNeedsUpgrade(t *testing.T) {
	type testData struct {
		current               string
		latest                string
		expectedUpgradeNeeded bool
		expectedMessage       string
	}

	testCases := []testData{
		{
			"1.0.0", "1.0.0", false, "You are already on the latest version of jx 1.0.0\n",
		},
		{
			"1.0.0", "1.0.1", true, "",
		},
		{
			"1.0.0", "0.0.99", true, "",
		},
	}

	o := UpgradeCLIOptions{}
	for _, data := range testCases {
		currentVersion, _ := semver.New(data.current)
		latestVersion, _ := semver.New(data.latest)
		actualMessage := log.CaptureOutput(func() {
			actualUpgradeNeeded := o.needsUpgrade(*currentVersion, *latestVersion)
			assert.Equal(t, data.expectedUpgradeNeeded, actualUpgradeNeeded, fmt.Sprintf("Unexpected upgrade flag for %v", data))
		})
		assert.Equal(t, data.expectedMessage, actualMessage, fmt.Sprintf("Unexpected message for %v", data))
	}
}

func TestVersionCheckWhenCurrentVersionIsGreaterThanReleaseVersion(t *testing.T) {
	jxVersion := semver.Version{Major: 1, Minor: 3, Patch: 153}
	version.Map["version"] = "1.4.0"
	opts := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	update, err := opts.ShouldUpdate(jxVersion)
	assert.NoError(t, err, "should check version without failure")
	assert.False(t, update, "should not update")
}

func TestVersionCheckWhenCurrentVersionIsEqualToReleaseVersion(t *testing.T) {
	jxVersion := semver.Version{Major: 1, Minor: 2, Patch: 3}
	version.Map["version"] = "1.2.3"
	opts := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	update, err := opts.ShouldUpdate(jxVersion)
	assert.NoError(t, err, "should check version without failure")
	assert.False(t, update, "should not update")
}

func TestVersionCheckWhenCurrentVersionIsLessThanReleaseVersion(t *testing.T) {
	jxVersion := semver.Version{Major: 1, Minor: 3, Patch: 153}
	version.Map["version"] = "1.0.0"
	opts := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	update, err := opts.ShouldUpdate(jxVersion)
	assert.NoError(t, err, "should check version without failure")
	assert.True(t, update, "should update")
}

func TestVersionCheckWhenCurrentVersionIsEqualToReleaseVersionWithPatch(t *testing.T) {
	prVersions := []semver.PRVersion{}
	prVersions = append(prVersions, semver.PRVersion{VersionStr: "dev"})
	jxVersion := semver.Version{Major: 1, Minor: 2, Patch: 3, Pre: prVersions, Build: []string(nil)}
	version.Map["version"] = "1.2.3"
	opts := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	update, err := opts.ShouldUpdate(jxVersion)
	assert.NoError(t, err, "should check version without failure")
	assert.False(t, update, "should not update")
}

func TestVersionCheckWhenCurrentVersionWithPatchIsEqualToReleaseVersion(t *testing.T) {
	jxVersion := semver.Version{Major: 1, Minor: 2, Patch: 3}
	version.Map["version"] = "1.2.3-dev+6a8285f4"
	opts := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	update, err := opts.ShouldUpdate(jxVersion)
	assert.NoError(t, err, "should check version without failure")
	assert.False(t, update, "should not update")
}

func TestVersionCheckWhenCurrentVersionWithPatchIsLessThanReleaseVersion(t *testing.T) {
	jxVersion := semver.Version{Major: 1, Minor: 2, Patch: 3}
	version.Map["version"] = "1.2.2-dev+6a8285f4"
	opts := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	update, err := opts.ShouldUpdate(jxVersion)
	assert.NoError(t, err, "should check version without failure")
	assert.False(t, update, "should not update")
}
