// +build unit

package reports_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/reports"
	"github.com/stretchr/testify/assert"
)

func TestProjectHistory(t *testing.T) {
	t.Parallel()
	_, history, err := reports.NewProjectHistoryService("test_data/projectHistory.yml")
	assert.Equal(t, "Jan 2 2018", history.LastReportDate, "history.LastReportDate")

	assert.Nil(t, err, "Failed to create the ProjectHistoryService")

	reportDate := "April 13 2018"
	report := history.GetOrCreateReport(reportDate)
	assert.NotNil(t, report, "Did not create a new report for %s", reportDate)

	assert.Equal(t, 2, len(history.Reports), "len(history.Reports)")

	previous := history.FindPreviousReport(reportDate)
	assert.NotNil(t, previous, "Did not create a previous report for %s", reportDate)
	assert.Equal(t, 10, previous.StarsMetrics.Count, "previous.StarsMetrics.Count")
	assert.Equal(t, 10, previous.StarsMetrics.Count, "previous.StarsMetrics.Count")

	report = history.StarsMetrics(reportDate, 50)
	assert.Equal(t, 30, report.StarsMetrics.Count, "report.StarsMetrics.Count")
	assert.Equal(t, 50, report.StarsMetrics.Total, "report.StarsMetrics.Total")

	assert.Equal(t, 2, len(history.Reports), "len(history.Reports)")

	report = history.IssueMetrics(reportDate, 20)
	assert.Equal(t, 20, report.IssueMetrics.Count, "report.IssueMetrics.Count")
	assert.Equal(t, 32, report.IssueMetrics.Total, "report.IssueMetrics.Total")

}
