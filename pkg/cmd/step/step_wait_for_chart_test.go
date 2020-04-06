// +build unit

package step

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/cmd/opts"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	cmd_test "github.com/jenkins-x/jx/pkg/cmd/clients/mocks"
	helm_test "github.com/jenkins-x/jx/pkg/helm/mocks"
	"github.com/jenkins-x/jx/pkg/kube"
	uuid "github.com/satori/go.uuid"
	"k8s.io/helm/pkg/proto/hapi/chart"

	"github.com/stretchr/testify/assert"
)

func TestStepWaitForChart(t *testing.T) {
	nameUUID, err := uuid.NewV4()
	assert.NoError(t, err)

	name := nameUUID.String()
	version := "0.0.1"

	helmer := helm_test.NewMockHelmer()
	helm_test.StubFetchChart(name, "", kube.DefaultChartMuseumURL, &chart.Chart{
		Metadata: &chart.Metadata{
			Name:    name,
			Version: version,
		},
	}, helmer)

	assert.NoError(t, err)

	mockFactory := cmd_test.NewMockFactory()

	commonOpts := opts.NewCommonOptionsWithFactory(mockFactory)
	commonOpts.SetHelm(helmer)
	options := &WaitForChartOptions{
		ChartName:    name,
		ChartVersion: version,
		ChartRepo:    kube.DefaultChartMuseumURL,
		StepOptions: &step.StepOptions{
			CommonOptions: &commonOpts,
		},
	}

	err = options.Run()
	assert.NoError(t, err)
}
