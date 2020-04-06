package reportingtools

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"gopkg.in/AlecAivazis/survey.v1"
)

// XUnitViewer is an implementation of the XUnitClient interface
type XUnitViewer struct{}

// EnsureXUnitViewer makes sure `xunit-viewer` is installed, otherwise it attempts to install it. It also checks NPM is installed
func (c XUnitViewer) EnsureXUnitViewer(o *opts.CommonOptions) error {

	cmd := util.Command{
		Name: "xunit-viewer",
	}

	_, err := cmd.RunWithoutRetry()

	if err == nil {
		return nil
	}
	installBinary := true
	if !o.BatchMode {
		surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
		confirm := &survey.Confirm{
			Message: fmt.Sprint("xunit-viewer doesn't seem to be installed, would you like to install it?"),
			Default: true,
		}

		err = survey.AskOne(confirm, &installBinary, nil, surveyOpts)
		if err != nil {
			return err
		}
	}
	if installBinary {
		err := c.EnsureNPMIsInstalled()
		if err != nil {
			log.Logger().Warn("npm is not installed, we can't install xunit-viewer")
			return err
		}

		cmd = util.Command{
			Name:    "npm",
			Args:    []string{"i", "-g", "xunit-viewer"},
			Timeout: time.Minute * 5,
		}

		_, err = cmd.RunWithoutRetry()
		if err != nil {
			log.Logger().Warn("it was impossible to install xunit-viewer, skipping")
			return err
		}
	}

	return nil
}

// EnsureNPMIsInstalled makes sure NPM is installed and fails otherwise
func (c XUnitViewer) EnsureNPMIsInstalled() error {
	cmd := util.Command{
		Name: "npm",
		Args: []string{"--version"},
	}

	_, err := cmd.RunWithoutRetry()

	return err
}

// CreateHTMLReport uses xunit-viewer to create an HTML report into the outputReportName, with a given suitename, from a given xml report
func (c XUnitViewer) CreateHTMLReport(outputReportName, suiteName, targetFileName string) error {
	cmd := util.Command{
		Name: "xunit-viewer",
		Args: []string{
			"--results=" + targetFileName,
			"--output=" + outputReportName,
			"--title=" + suiteName,
		},
	}

	out, err := cmd.RunWithoutRetry()
	fmt.Println(out)
	if err != nil {
		return errors.Wrapf(err, "There was a problem generating the html report")
	}

	return nil
}
