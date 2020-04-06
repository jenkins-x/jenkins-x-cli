package helm

import (
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// GetTillerAddress returns the address that tiller is listening on
func GetTillerAddress() string {
	tillerAddress := os.Getenv("TILLER_ADDR")
	if tillerAddress == "" {
		tillerAddress = ":44134"
	}
	return tillerAddress
}

// StartLocalTiller starts local tiller server
func StartLocalTiller(lazy bool) error {
	tillerAddress := GetTillerAddress()
	tillerArgs := os.Getenv("TILLER_ARGS")
	args := []string{"-listen", tillerAddress, "-alsologtostderr"}
	if tillerArgs != "" {
		args = append(args, tillerArgs)
	}
	logsDir, err := util.LogsDir()
	if err != nil {
		return err
	}
	logFile := filepath.Join(logsDir, "tiller.log")
	f, err := os.Create(logFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to create tiller log file %s: %s", logFile, err)
	}
	err = util.RunCommandBackground("tiller", f, !lazy, args...)
	if err == nil {
		log.Logger().Infof("running tiller locally and logging to file: %s", util.ColorInfo(logFile))
	} else if lazy {
		// lets assume its because the process is already running so lets ignore
		return nil
	}
	return err
}

// RestartLocalTiller resttarts locall tiller
func RestartLocalTiller() error {
	log.Logger().Info("checking if we need to kill a local tiller process")
	util.KillProcesses("tiller")
	return StartLocalTiller(false)
}

// StartLocalTillerIfNotRunning starts local tiller if not running
func StartLocalTillerIfNotRunning() error {
	return StartLocalTiller(true)
}
