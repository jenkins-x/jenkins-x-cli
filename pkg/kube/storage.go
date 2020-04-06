package kube

import "strings"

const (
	// ClassificationLogs stores build logs
	ClassificationLogs = "logs"

	// ClassificationTests stores test results/reports
	ClassificationTests = "tests"

	// ClassificationCoverage stores code coverage results/reports
	ClassificationCoverage = "coverage"

	// ClassificationReports stores test results, coverage & quality reports
	ClassificationReports = "reports"
)

var (
	// Classifications the common classification names
	Classifications = []string{
		ClassificationCoverage, ClassificationTests, ClassificationLogs, ClassificationReports,
	}

	// ClassificationValues the classification values as a string
	ClassificationValues = strings.Join(Classifications, ", ")
)
