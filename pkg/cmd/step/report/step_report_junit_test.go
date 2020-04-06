// +build unit

package report

import (
	"encoding/xml"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/acarl005/stripansi"
	"github.com/google/uuid"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	log2 "github.com/jenkins-x/jx/pkg/log"
	reportingtools_test "github.com/jenkins-x/jx/pkg/reportingtools/mocks"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/petergtz/pegomock"
	"github.com/stretchr/testify/assert"
)

func TestReportFromSingleFile(t *testing.T) {
	mock := reportingtools_test.NewMockXUnitClient(pegomock.WithT(t))

	dirName, err := ioutil.TempDir("", uuid.New().String())
	defer os.Remove(dirName)
	assert.NoError(t, err, "there shouldn't be any problem creating a temp dir")

	reportName := uuid.New().String() + ".html"
	o := StepReportJUnitOptions{
		XUnitClient:      mock,
		ReportsDir:       filepath.Join("test_data", "junit", "single_report"),
		TargetReport:     "import_applications.junit.xml",
		DeleteReportFn:   func(reportName string) (err error) { return },
		OutputReportName: reportName,
		StepReportOptions: StepReportOptions{
			OutputDir: dirName,
		},
	}

	err = o.Run()
	assert.NoError(t, err)

	mock.VerifyWasCalledOnce().EnsureXUnitViewer(AnyCommonOptions())
	_, _, targetFileName := mock.VerifyWasCalledOnce().CreateHTMLReport(pegomock.EqString(filepath.Join(dirName,
		reportName)), pegomock.EqString(""), pegomock.AnyString()).GetCapturedArguments()
	defer util.DeleteFile(targetFileName)

	resultingReportBytes, err := ioutil.ReadFile(targetFileName)
	assert.NoError(t, err)

	initialReportBytes, err := ioutil.ReadFile(filepath.Join(o.ReportsDir, o.TargetReport))
	assert.NoError(t, err)

	assert.Equal(t, initialReportBytes, resultingReportBytes)
}

func TestReportFromTestSuites(t *testing.T) {
	mock := reportingtools_test.NewMockXUnitClient(pegomock.WithT(t))

	dirName, err := ioutil.TempDir("", uuid.New().String())
	defer os.Remove(dirName)
	assert.NoError(t, err, "there shouldn't be any problem creating a temp dir")

	reportName := uuid.New().String() + ".html"
	o := StepReportJUnitOptions{
		XUnitClient:      mock,
		ReportsDir:       filepath.Join("test_data", "junit", "testsuites_report"),
		TargetReport:     "ui_smoke.junit.xml",
		DeleteReportFn:   func(reportName string) (err error) { return },
		OutputReportName: reportName,
		StepReportOptions: StepReportOptions{
			OutputDir: dirName,
		},
	}

	err = o.Run()
	assert.NoError(t, err)

	mock.VerifyWasCalledOnce().EnsureXUnitViewer(AnyCommonOptions())
	_, _, targetFileName := mock.VerifyWasCalledOnce().CreateHTMLReport(pegomock.EqString(filepath.Join(dirName,
		reportName)), pegomock.EqString(""), pegomock.AnyString()).GetCapturedArguments()
	defer util.DeleteFile(targetFileName)

	resultingReportBytes, err := ioutil.ReadFile(targetFileName)
	assert.NoError(t, err)

	initialReportBytes, err := ioutil.ReadFile(filepath.Join(o.ReportsDir, o.TargetReport))
	assert.NoError(t, err)

	assert.Equal(t, initialReportBytes, resultingReportBytes)
}

func TestReportWithMultipleFiles(t *testing.T) {
	mock := reportingtools_test.NewMockXUnitClient(pegomock.WithT(t))

	dirName, err := ioutil.TempDir("", uuid.New().String())
	defer os.Remove(dirName)
	assert.NoError(t, err, "there shouldn't be any problem creating a temp dir")

	reportName := uuid.New().String() + ".html"
	o := StepReportJUnitOptions{
		XUnitClient:      mock,
		ReportsDir:       filepath.Join("test_data", "junit", "multiple_reports"),
		MergeReports:     true,
		DeleteReportFn:   func(reportName string) (err error) { return },
		OutputReportName: reportName,
		StepReportOptions: StepReportOptions{
			OutputDir: dirName,
		},
	}

	err = o.Run()
	assert.NoError(t, err)

	_, _, targetFileName := mock.VerifyWasCalledOnce().CreateHTMLReport(pegomock.EqString(filepath.Join(dirName,
		reportName)), pegomock.EqString(""), pegomock.AnyString()).GetCapturedArguments()
	defer util.DeleteFile(targetFileName)

	reportBytes, err := ioutil.ReadFile(targetFileName)
	assert.NoError(t, err)

	var testSuites TestSuites
	err = xml.Unmarshal(reportBytes, &testSuites)
	assert.NoError(t, err, "There shouldn't be an error Unmarshalling the resulting merged report")

	assert.Len(t, testSuites.TestSuites, 4, "there should be two suites in the merged report")
	knownSuitesNamesFound := 0
	for _, v := range testSuites.TestSuites {
		if v.Name == "Jenkins X E2E tests: create_quickstarts" {
			knownSuitesNamesFound++
		}
		if v.Name == "Jenkins X E2E tests: import_applications" {
			knownSuitesNamesFound++
		}
		if v.Name == "Jenkins X E2E tests: ui_smoke_can_list_projects" {
			knownSuitesNamesFound++
		}
		if v.Name == "Jenkins X E2E tests: ui_smoke_can_list_builds" {
			knownSuitesNamesFound++
		}
	}
	assert.Equal(t, len(testSuites.TestSuites), knownSuitesNamesFound,
		"the number of known suites must match the number of merged suites")
}

func TestUnableToEnsureXUnitViewer(t *testing.T) {
	mock := reportingtools_test.NewMockXUnitClient(pegomock.WithT(t))

	o := StepReportJUnitOptions{
		XUnitClient: mock,
	}

	pegomock.When(mock.EnsureXUnitViewer(AnyCommonOptions())).ThenReturn(errors.New("error from EnsureXUnitViewer"))

	output := log2.CaptureOutput(func() {
		err := o.Run()
		assert.NoError(t, err, "the result of the step failing should never be an error, we want to avoid the step from failing a pipeline")
	})

	assert.Equal(t, "ERROR: there was a problem ensuring the presence of xunit-viewer: error from EnsureXUnitViewer\n", output)
}

func TestErrorNoMatchingFilesFound(t *testing.T) {
	mock := reportingtools_test.NewMockXUnitClient(pegomock.WithT(t))
	o := StepReportJUnitOptions{
		XUnitClient: mock,
		ReportsDir:  filepath.Join("test_data", "junit", "empty_dir"),
	}

	output := log2.CaptureOutput(func() {
		err := o.Run()
		assert.NoError(t, err, "the result of the step failing should never be an error, we want to avoid the step from failing a pipeline")
	})

	assert.Equal(t, "ERROR: there was a problem obtaining the matching report files: no report files to parse in test_data/junit/empty_dir, skipping\n", stripansi.Strip(output))
}

func AnyCommonOptions() *opts.CommonOptions {
	pegomock.RegisterMatcher(pegomock.NewAnyMatcher(reflect.TypeOf((**opts.CommonOptions)(nil)).Elem()))
	return nil
}
