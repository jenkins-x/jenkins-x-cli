// +build unit

package create_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/create"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/stretchr/testify/assert"
)

func TestCreateStep(t *testing.T) {
	t.Parallel()

	testData := path.Join("test_data", "create_step")
	_, err := os.Stat(testData)
	assert.NoError(t, err)

	cases := []struct {
		name      string
		sh        string
		pipeline  string
		lifecycle string
		mode      string
	}{
		{
			name: "simple",
			sh:   "echo hello world",
		},
		{
			name:     "with-pipeline",
			sh:       "echo hello world",
			pipeline: "pullrequest",
		},
		{
			name:      "with-lifecycle",
			sh:        "echo hello world",
			lifecycle: "setup",
		},
		{
			name: "mode-pre",
			sh:   "echo hello world",
			mode: "pre",
		},
		{
			name: "mode-replace",
			sh:   "echo hello world",
			mode: "replace",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			caseDir := path.Join(testData, tt.name)
			_, err = os.Stat(caseDir)
			assert.NoError(t, err)

			createStep := &create.CreateStepOptions{
				CreateOptions: options.CreateOptions{
					CommonOptions: &opts.CommonOptions{
						BatchMode: true,
					},
				},
				Dir: caseDir,
				NewStepDetails: create.NewStepDetails{
					Step: syntax.Step{
						Command: tt.sh,
					},
					Pipeline:  tt.pipeline,
					Lifecycle: tt.lifecycle,
					Mode:      tt.mode,
				},
			}

			result, _, err := createStep.AddStepToProjectConfig()
			if err != nil {
				t.Fatalf("Error adding step to project: %s", err)
			}

			expectedFile := path.Join(caseDir, "after-jenkins-x.yml")
			_, err = os.Stat(expectedFile)
			assert.NoError(t, err)

			tempDir, err := ioutil.TempDir("", "test-create-step")
			assert.NoError(t, err)

			resultFile := path.Join(tempDir, "jenkins-x.yml")
			err = result.SaveConfig(resultFile)
			if err != nil {
				t.Fatalf("Error saving modified Pipeline to %s: %s", resultFile, err)
			}
			tests.AssertTextFileContentsEqual(t, expectedFile, resultFile)
		})
	}
}
