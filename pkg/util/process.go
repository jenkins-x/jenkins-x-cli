package util

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shirou/gopsutil/process"

	"github.com/jenkins-x/jx/pkg/log"
)

func RunCommandBackground(name string, output io.Writer, verbose bool, args ...string) error {
	e := exec.Command(name, args...)
	e.Stdout = output
	e.Stderr = output
	os.Setenv("PATH", PathWithBinary())
	err := e.Start()
	if err != nil && verbose {
		log.Logger().Errorf("Error: Command failed to start  %s %s", name, strings.Join(args, " "))
	}
	return err
}

func KillProcesses(binary string) error {
	processes, err := process.Processes()
	if err != nil {
		return err
	}
	m := map[int32]bool{}
	_, err = KillProcessesTree(binary, processes, m)
	return err
}

func KillProcessesTree(binary string, processes []*process.Process, m map[int32]bool) (bool, error) {
	var answer error
	done := false
	for _, p := range processes {
		pid := p.Pid
		if pid > 0 && !m[pid] {
			m[pid] = true
			exe, err := p.Name()
			if err == nil && exe != "" {
				_, name := filepath.Split(exe)
				// if windows lets remove .exe
				name = strings.TrimSuffix(name, ".exe")
				if name == binary {
					log.Logger().Infof("killing %s process with pid %d", binary, int(pid))
					err = p.Terminate()
					if err != nil {
						log.Logger().Warnf("Failed to terminate process with pid %d: %s", int(pid), err)
					} else {
						log.Logger().Infof("killed %s process with pid %d", binary, int(pid))
					}
					return true, err
				}
			}
			children, err := p.Children()
			if err == nil {
				done, err = KillProcessesTree(binary, children, m)
				if done {
					return done, err
				}
			}
		}
	}
	return done, answer
}
