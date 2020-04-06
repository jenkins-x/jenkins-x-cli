// +build unit

package preview_test

import (
	"os"
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/preview"
	"github.com/jenkins-x/jx/pkg/cmd/testhelpers"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/config"
	gits_test "github.com/jenkins-x/jx/pkg/gits/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
)

func TestGetPreviewValuesConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		opts               preview.PreviewOptions
		env                map[string]string
		domain             string
		expectedYAMLConfig string
	}{
		{
			opts: preview.PreviewOptions{
				HelmValuesConfig: config.HelmValuesConfig{
					ExposeController: &config.ExposeController{},
				},
			},
			env: map[string]string{
				preview.DOCKER_REGISTRY: "my.registry",
				preview.ORG:             "my-org",
				preview.APP_NAME:        "my-app",
				preview.PREVIEW_VERSION: "1.0.0",
			},
			expectedYAMLConfig: `expose:
  config: {}
preview:
  image:
    repository: my.registry/my-org/my-app
    tag: 1.0.0
`,
		},
		{
			opts: preview.PreviewOptions{
				HelmValuesConfig: config.HelmValuesConfig{
					ExposeController: &config.ExposeController{
						Config: config.ExposeControllerConfig{
							HTTP:    "false",
							TLSAcme: "true",
						},
					},
				},
			},
			env: map[string]string{
				preview.DOCKER_REGISTRY: "my.registry",
				preview.ORG:             "my-org",
				preview.APP_NAME:        "my-app",
				preview.PREVIEW_VERSION: "1.0.0",
			},
			domain: "jenkinsx.io",
			expectedYAMLConfig: `expose:
  config:
    domain: jenkinsx.io
    http: "false"
    tlsacme: "true"
preview:
  image:
    repository: my.registry/my-org/my-app
    tag: 1.0.0
`,
		},
	}
	co := &opts.CommonOptions{}
	testhelpers.ConfigureTestOptions(co, gits_test.NewMockGitter(), helm_test.NewMockHelmer())

	for i, test := range tests {
		for k, v := range test.env {
			os.Setenv(k, v)
		}

		test.opts.CommonOptions = co
		config, err := test.opts.GetPreviewValuesConfig(nil, test.domain)
		if err != nil {
			t.Errorf("[%d] got unexpected err: %v", i, err)
			continue
		}

		configYAML, err := config.String()
		if err != nil {
			t.Errorf("[%d] %v", i, err)
			continue
		}

		if test.expectedYAMLConfig != configYAML {
			t.Errorf("[%d] expected %#v but got %#v", i, test.expectedYAMLConfig, configYAML)
		}
	}
}
