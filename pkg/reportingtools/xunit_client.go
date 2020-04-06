package reportingtools

import "github.com/jenkins-x/jx/pkg/cmd/opts"

// XUnitClient is the interface defined for jx interactions with the xunit-viewer client
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/reportingtools XUnitClient -o mocks/xunitclient.go
type XUnitClient interface {
	EnsureXUnitViewer(o *opts.CommonOptions) error
	EnsureNPMIsInstalled() error
	CreateHTMLReport(outputReportName, suiteName, targetFileName string) error
}
