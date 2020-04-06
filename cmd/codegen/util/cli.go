package util

import (
	"fmt"
	"os"
	"strings"
)

const (
	// DefaultErrorExitCode is the default exit code in case of an error
	DefaultErrorExitCode = 1
)

var (
	fatalErrHandler = fatal
	// ErrExit can be used to exit with a non 0 exit code without any error message
	ErrExit = fmt.Errorf("exit")
)

// InvalidOptionf returns an error that shows the invalid option.
func InvalidOptionf(option string, value interface{}, message string, a ...interface{}) error {
	text := fmt.Sprintf(message, a...)
	return fmt.Errorf("invalid option: --%s %v\n%s", option, value, text)
}

// MissingOption reports a missing command line option using the full name expression.
func MissingOption(name string) error {
	return fmt.Errorf("missing option: --%s", name)
}

// CheckErr prints a user friendly error to STDERR and exits with a non-zero exit code.
func CheckErr(err error) {
	checkErr(err, fatalErrHandler)
}

// checkErr formats a given error as a string and calls the passed handleErr func.
func checkErr(err error, handleErr func(string, int)) {
	switch {
	case err == nil:
		return
	case err == ErrExit:
		handleErr("", DefaultErrorExitCode)
		return
	default:
		handleErr(err.Error(), DefaultErrorExitCode)
	}
}

func fatal(msg string, code int) {
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}
		fmt.Fprint(os.Stderr, msg)
	}
	os.Exit(code)
}
