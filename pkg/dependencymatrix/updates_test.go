// +build unit

package dependencymatrix_test

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/dependencymatrix"
	"github.com/stretchr/testify/assert"
)

func TestCanLoadDependencyUpdates(t *testing.T) {
	data := `updates:
- component: ""
  fromReleaseHTMLURL: https://github.com/cloudbees/jx-tenant-service/releases/tag/v0.0.262
  fromReleaseName: v0.0.262
  fromVersion: v0.0.262
  host: github.com
  owner: cloudbees
  repo: jx-tenant-service
  toReleaseHTMLURL: https://github.com/cloudbees/jx-tenant-service/releases/tag/v0.0.263
  toReleaseName: v0.0.263
  toVersion: 0.0.263
  url: https://github.com/cloudbees/jx-tenant-service`

	var updates dependencymatrix.DependencyUpdates
	err := yaml.Unmarshal([]byte(data), &updates)
	assert.NoError(t, err, "error unmarshalling yaml")

}

func TestCanLoadDependencyUpdates2(t *testing.T) {
	data := `updates:
- component: ""
  fromReleaseHTMLURL: https://github.com/jenkins-x/jx/releases/tag/v2.0.1029
  fromReleaseName: 2.0.1029
  fromVersion: 2.0.1029
  host: github.com
  owner: jenkins-x
  repo: jx
  toReleaseHTMLURL: https://github.com/jenkins-x/jx/releases/tag/v2.0.1030
  toReleaseName: 2.0.1030
  toVersion: 2.0.1030
  url: https://github.com/jenkins-x/jx`

	var updates dependencymatrix.DependencyUpdates
	err := yaml.Unmarshal([]byte(data), &updates)
	assert.NoError(t, err, "error unmarshalling yaml")

}
