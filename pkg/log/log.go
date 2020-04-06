package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rickar/props"

	"github.com/pkg/errors"

	stackdriver "github.com/TV4/logrus-stackdriver-formatter"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

var (
	// colorStatus returns a new function that returns status-colorized (cyan) strings for the
	// given arguments with fmt.Sprint().
	colorStatus = color.New(color.FgCyan).SprintFunc()

	// colorWarn returns a new function that returns status-colorized (yellow) strings for the
	// given arguments with fmt.Sprint().
	colorWarn = color.New(color.FgYellow).SprintFunc()

	// colorError returns a new function that returns error-colorized (red) strings for the
	// given arguments with fmt.Sprint().
	colorError = color.New(color.FgRed).SprintFunc()

	logger *logrus.Entry

	labelsPath = "/etc/labels"
)

// FormatLayoutType the layout kind
type FormatLayoutType string

const (
	// FormatLayoutJSON uses JSON layout
	FormatLayoutJSON FormatLayoutType = "json"

	// FormatLayoutText uses classic colorful Jenkins X layout
	FormatLayoutText FormatLayoutType = "text"

	// FormatLayoutStackdriver uses a custom formatter for stackdriver
	FormatLayoutStackdriver FormatLayoutType = "stackdriver"
)

func initializeLogger() error {
	if logger == nil {

		// if we are inside a pod, record some useful info
		var fields logrus.Fields
		if exists, err := fileExists(labelsPath); err != nil {
			return errors.Wrapf(err, "checking if %s exists", labelsPath)
		} else if exists {
			f, err := os.Open(labelsPath)
			if err != nil {
				return errors.Wrapf(err, "opening %s", labelsPath)
			}
			labels, err := props.Read(f)
			if err != nil {
				return errors.Wrapf(err, "reading %s as properties", labelsPath)
			}
			app := labels.Get("app")
			if app != "" {
				fields["app"] = app
			}
			chart := labels.Get("chart")
			if chart != "" {
				fields["chart"] = labels.Get("chart")
			}
		}
		logger = logrus.WithFields(fields)

		format := os.Getenv("JX_LOG_FORMAT")
		if format == "json" {
			setFormatter(FormatLayoutJSON)
		} else if format == "stackdriver" {
			setFormatter(FormatLayoutStackdriver)
		} else {
			setFormatter(FormatLayoutText)
		}
	}
	return nil
}

// Logger obtains the logger for use in the jx codebase
// This is the only way you should obtain a logger
func Logger() *logrus.Entry {
	err := initializeLogger()
	if err != nil {
		logrus.Warnf("error initializing logrus %v", err)
	}
	return logger
}

// SetLevel sets the logging level
func SetLevel(s string) error {
	level, err := logrus.ParseLevel(s)
	if err != nil {
		return errors.Errorf("Invalid log level '%s'", s)
	}
	Logger().Debugf("logging set to level: %s", level)
	logrus.SetLevel(level)
	return nil
}

// GetLevel gets the current log level
func GetLevel() string {
	return logrus.GetLevel().String()
}

// GetLevels returns the list of valid log levels
func GetLevels() []string {
	var levels []string
	for _, level := range logrus.AllLevels {
		levels = append(levels, level.String())
	}
	return levels
}

// setFormatter sets the logrus format to use either text or JSON formatting
func setFormatter(layout FormatLayoutType) {
	switch layout {
	case FormatLayoutJSON:
		logrus.SetFormatter(&logrus.JSONFormatter{})
	case FormatLayoutStackdriver:
		logrus.SetFormatter(stackdriver.NewFormatter())
	default:
		logrus.SetFormatter(NewJenkinsXTextFormat())
	}
}

// JenkinsXTextFormat lets use a custom text format
type JenkinsXTextFormat struct {
	ShowInfoLevel   bool
	ShowTimestamp   bool
	TimestampFormat string
}

// NewJenkinsXTextFormat creates the default Jenkins X text formatter
func NewJenkinsXTextFormat() *JenkinsXTextFormat {
	return &JenkinsXTextFormat{
		ShowInfoLevel:   false,
		ShowTimestamp:   false,
		TimestampFormat: "2006-01-02 15:04:05",
	}
}

// Format formats the log statement
func (f *JenkinsXTextFormat) Format(entry *logrus.Entry) ([]byte, error) {
	var b *bytes.Buffer

	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}

	level := strings.ToUpper(entry.Level.String())
	switch level {
	case "INFO":
		if f.ShowInfoLevel {
			b.WriteString(colorStatus(level))
			b.WriteString(": ")
		}
	case "WARNING":
		b.WriteString(colorWarn(level))
		b.WriteString(": ")
	case "DEBUG":
		b.WriteString(colorStatus(level))
		b.WriteString(": ")
	default:
		b.WriteString(colorError(level))
		b.WriteString(": ")
	}
	if f.ShowTimestamp {
		b.WriteString(entry.Time.Format(f.TimestampFormat))
		b.WriteString(" - ")
	}

	b.WriteString(entry.Message)

	if !strings.HasSuffix(entry.Message, "\n") {
		b.WriteByte('\n')
	}
	return b.Bytes(), nil
}

// Blank prints a blank line
func Blank() {
	fmt.Println()
}

// CaptureOutput calls the specified function capturing and returning all logged messages.
func CaptureOutput(f func()) string {
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	f()
	logrus.SetOutput(os.Stderr)
	return buf.String()
}

// SetOutput sets the outputs for the default logger.
func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

// copied from utils to avoid circular import
func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, errors.Wrapf(err, "failed to check if file exists %s", path)
}
