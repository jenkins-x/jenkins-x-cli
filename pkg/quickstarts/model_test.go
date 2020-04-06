// +build unit

package quickstarts_test

import (
	"testing"

	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/quickstarts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuickstartModelFilterText(t *testing.T) {
	t.Parallel()

	quickstart1 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http",
		Name: "node-http",
	}
	quickstart2 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http-watch-pipeline-activity",
		Name: "node-http-watch-pipeline-activity",
	}
	quickstart3 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/ruby",
		Name: "ruby",
	}

	qstarts := make(map[string]*quickstarts.Quickstart)
	qstarts["node-http"] = quickstart1
	qstarts["node-http-watch-pipeline-activity"] = quickstart2
	qstarts["ruby"] = quickstart3

	quickstartModel := &quickstarts.QuickstartModel{
		Quickstarts: qstarts,
	}

	quickstartFilter := &quickstarts.QuickstartFilter{
		Text: "ruby",
	}

	results := quickstartModel.Filter(quickstartFilter)

	assert.Equal(t, 1, len(results))
	assert.Contains(t, results, quickstart3)
}

func TestQuickstartModelFilterTextMatchesMoreThanOne(t *testing.T) {
	t.Parallel()

	quickstart1 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http",
		Name: "node-http",
	}
	quickstart2 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http-watch-pipeline-activity",
		Name: "node-http-watch-pipeline-activity",
	}
	quickstart3 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/ruby",
		Name: "ruby",
	}

	qstarts := make(map[string]*quickstarts.Quickstart)
	qstarts["node-http"] = quickstart1
	qstarts["node-http-watch-pipeline-activity"] = quickstart2
	qstarts["ruby"] = quickstart3

	quickstartModel := &quickstarts.QuickstartModel{
		Quickstarts: qstarts,
	}

	quickstartFilter := &quickstarts.QuickstartFilter{
		Text: "node-htt",
	}

	results := quickstartModel.Filter(quickstartFilter)

	assert.Equal(t, 2, len(results))
	assert.Contains(t, results, quickstart1)
	assert.Contains(t, results, quickstart2)
}

func TestQuickstartModelFilterTextMatchesOneExactly(t *testing.T) {
	t.Parallel()

	quickstart1 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http",
		Name: "node-http",
	}
	quickstart2 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http-watch-pipeline-activity",
		Name: "node-http-watch-pipeline-activity",
	}
	quickstart3 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/ruby",
		Name: "ruby",
	}

	qstarts := make(map[string]*quickstarts.Quickstart)
	qstarts["node-http"] = quickstart1
	qstarts["node-http-watch-pipeline-activity"] = quickstart2
	qstarts["ruby"] = quickstart3

	quickstartModel := &quickstarts.QuickstartModel{
		Quickstarts: qstarts,
	}

	quickstartFilter := &quickstarts.QuickstartFilter{
		Text: "node-http",
	}

	results := quickstartModel.Filter(quickstartFilter)

	assert.Equal(t, 1, len(results))
	assert.Contains(t, results, quickstart1)
}

func TestQuickstartModelFilterExcludesMachineLearning(t *testing.T) {
	t.Parallel()

	quickstart1 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http",
		Name: "node-http",
	}
	quickstart2 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http-watch-pipeline-activity",
		Name: "node-http-watch-pipeline-activity",
	}
	quickstart3 := &quickstarts.Quickstart{
		ID:   "machine-learning-quickstarts/ML-is-a-machine-learning-quickstart",
		Name: "ML-is-a-machine-learning-quickstart",
	}

	qstarts := make(map[string]*quickstarts.Quickstart)
	qstarts["node-http"] = quickstart1
	qstarts["node-http-watch-pipeline-activity"] = quickstart2
	qstarts["ML-is-a-machine-learning-quickstart"] = quickstart3

	quickstartModel := &quickstarts.QuickstartModel{
		Quickstarts: qstarts,
	}

	quickstartFilter := &quickstarts.QuickstartFilter{
		AllowML: false,
	}

	results := quickstartModel.Filter(quickstartFilter)

	assert.Equal(t, 2, len(results))
	assert.Contains(t, results, quickstart1)
	assert.Contains(t, results, quickstart2)
	assert.NotContains(t, results, quickstart3)
}

func TestQuickstartModelFilterIncludesMachineLearning(t *testing.T) {
	t.Parallel()

	quickstart1 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http",
		Name: "node-http",
	}
	quickstart2 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http-watch-pipeline-activity",
		Name: "node-http-watch-pipeline-activity",
	}
	quickstart3 := &quickstarts.Quickstart{
		ID:   "machine-learning-quickstarts/ML-is-a-machine-learning-quickstart",
		Name: "ML-is-a-machine-learning-quickstart",
	}

	qstarts := make(map[string]*quickstarts.Quickstart)
	qstarts["node-http"] = quickstart1
	qstarts["node-http-watch-pipeline-activity"] = quickstart2
	qstarts["ML-is-a-machine-learning-quickstart"] = quickstart3

	quickstartModel := &quickstarts.QuickstartModel{
		Quickstarts: qstarts,
	}

	quickstartFilter := &quickstarts.QuickstartFilter{
		AllowML: true,
	}

	results := quickstartModel.Filter(quickstartFilter)

	assert.Equal(t, 3, len(results))
	assert.Contains(t, results, quickstart1)
	assert.Contains(t, results, quickstart2)
	assert.Contains(t, results, quickstart3)
}

func TestQuickstartModelFilterDefaultsToNoMachineLearning(t *testing.T) {
	t.Parallel()

	quickstart1 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http",
		Name: "node-http",
	}
	quickstart2 := &quickstarts.Quickstart{
		ID:   "jenkins-x-quickstarts/node-http-watch-pipeline-activity",
		Name: "node-http-watch-pipeline-activity",
	}
	quickstart3 := &quickstarts.Quickstart{
		ID:   "machine-learning-quickstarts/ML-is-a-machine-learning-quickstart",
		Name: "ML-is-a-machine-learning-quickstart",
	}

	qstarts := make(map[string]*quickstarts.Quickstart)
	qstarts["node-http"] = quickstart1
	qstarts["node-http-watch-pipeline-activity"] = quickstart2
	qstarts["ML-is-a-machine-learning-quickstart"] = quickstart3

	quickstartModel := &quickstarts.QuickstartModel{
		Quickstarts: qstarts,
	}

	quickstartFilter := &quickstarts.QuickstartFilter{
		Text: "",
	}

	results := quickstartModel.Filter(quickstartFilter)

	assert.Equal(t, 2, len(results))
	assert.Contains(t, results, quickstart1)
	assert.Contains(t, results, quickstart2)
	assert.NotContains(t, results, quickstart3)
}

func TestQuickstartCreateVersion(t *testing.T) {
	t.Parallel()

	sha := "d9e925718"
	v := quickstarts.QuickStartVersion(sha)
	sv, err := semver.Parse(v)
	require.NoError(t, err, "failed to parse semantic version %s for quickstart", v)
	t.Logf("parsed semantic version %s for quickstart", sv.String())
}
