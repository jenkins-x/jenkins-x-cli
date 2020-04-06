package util

import (
	"os"

	log "github.com/sirupsen/logrus"
)

const (
	// AppName is tge application name for logging.
	AppName = "codegen"
)

var (
	logger = log.WithFields(log.Fields{"app": AppName})
)

func init() {
	// Output to stdout instead of the default stderr
	log.SetOutput(os.Stdout)
}

// AppLogger returns the application logger.
func AppLogger() *log.Entry {
	return logger
}

// SetLevel sets the logging level
func SetLevel(s string) error {
	level, err := log.ParseLevel(s)
	if err != nil {
		return err
	}
	logger.Debugf("logging set to level: %s", level)
	log.SetLevel(level)
	return nil
}
