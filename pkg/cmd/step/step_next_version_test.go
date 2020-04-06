// +build unit

package step_test

import (
	"testing"

	step2 "github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/stretchr/testify/assert"
)

func TestMakefile(t *testing.T) {
	t.Parallel()
	o := step.StepNextVersionOptions{
		StepOptions: step2.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		Dir:      "test_data/next_version/make",
		Filename: "Makefile",
	}

	v, err := o.GetVersion()

	assert.NoError(t, err)

	assert.Equal(t, "1.2.0-SNAPSHOT", v, "error with GetVersion for a Makefile")
}

func TestPomXML(t *testing.T) {
	t.Parallel()
	o := step.StepNextVersionOptions{
		StepOptions: step2.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		Dir:      "test_data/next_version/java",
		Filename: "pom.xml",
	}

	v, err := o.GetVersion()

	assert.NoError(t, err)

	assert.Equal(t, "1.0-SNAPSHOT", v, "error with GetVersion for a pom.xml")
}

func TestChart(t *testing.T) {
	t.Parallel()
	o := step.StepNextVersionOptions{
		StepOptions: step2.StepOptions{
			CommonOptions: &opts.CommonOptions{},
		},
		Dir:      "test_data/next_version/helm",
		Filename: "Chart.yaml",
	}

	v, err := o.GetVersion()

	assert.NoError(t, err)

	assert.Equal(t, "0.0.1-SNAPSHOT", v, "error with GetVersion for a Chart.yaml")
}
