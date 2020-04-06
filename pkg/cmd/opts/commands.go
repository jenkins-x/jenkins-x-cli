package opts

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// RunCommandFromDir runs a command in the given directory
// Deprecated use util.Command
func (o *CommonOptions) RunCommandFromDir(dir, name string, args ...string) error {
	e := exec.Command(name, args...)
	if dir != "" {
		e.Dir = dir
	}
	e.Stdout = o.Out
	e.Stderr = o.Err
	os.Setenv("PATH", util.PathWithBinary())
	err := e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}
	return err
}

// RunCommand runs a given command command with arguments
// Deprecated use util.Command
func (o *CommonOptions) RunCommand(name string, args ...string) error {
	e := exec.Command(name, args...)
	e.Stdout = o.Out
	e.Stderr = o.Err
	os.Setenv("PATH", util.PathWithBinary())
	err := e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}
	return err
}

// RunCommandVerbose runs a given command with arguments in verbose mode
// Deprecated use util.Command
func (o *CommonOptions) RunCommandVerbose(name string, args ...string) error {
	e := exec.Command(name, args...)
	e.Stdout = o.Out
	e.Stderr = o.Err
	os.Setenv("PATH", util.PathWithBinary())
	err := e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}
	return err
}

// RunCommandVerboseAt runs a given command in a given folder in verbose mode
// Deprecated use util.Command
func (o *CommonOptions) RunCommandVerboseAt(dir string, name string, args ...string) error {
	e := exec.Command(name, args...)
	if dir != "" {
		e.Dir = dir
	}
	e.Stdout = o.Out
	e.Stderr = o.Err
	os.Setenv("PATH", util.PathWithBinary())
	err := e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}
	return err
}

// RunCommandQuietly runs commands and discard the stdout and stderr
// Deprecated use util.Command
func (o *CommonOptions) RunCommandQuietly(name string, args ...string) error {
	e := exec.Command(name, args...)
	e.Stdout = ioutil.Discard
	e.Stderr = ioutil.Discard
	os.Setenv("PATH", util.PathWithBinary())
	return e.Run()
}

// RunCommandInteractive run a given command interactively
// Deprecated use util.Command
func (o *CommonOptions) RunCommandInteractive(interactive bool, name string, args ...string) error {
	e := exec.Command(name, args...)
	e.Stdout = o.Out
	e.Stderr = o.Err
	if interactive {
		e.Stdin = os.Stdin
	}
	os.Setenv("PATH", util.PathWithBinary())
	err := e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}
	return err
}

// RunCommandInteractiveInDir run a given command interactively in a given directory
// Deprecated use util.Command
func (o *CommonOptions) RunCommandInteractiveInDir(interactive bool, dir string, name string, args ...string) error {
	e := exec.Command(name, args...)
	e.Stdout = o.Out
	e.Stderr = o.Err
	if interactive {
		e.Stdin = os.Stdin
	}
	if dir != "" {
		e.Dir = dir
	}
	os.Setenv("PATH", util.PathWithBinary())
	err := e.Run()
	if err != nil {
		log.Logger().Errorf("Error: Command failed  %s %s", name, strings.Join(args, " "))
	}
	return err
}

// GetCommandOutput evaluates the given command and returns the trimmed output
// Deprecated use util.Command
func (o *CommonOptions) GetCommandOutput(dir string, name string, args ...string) (string, error) {
	os.Setenv("PATH", util.PathWithBinary())
	e := exec.Command(name, args...)
	if dir != "" {
		e.Dir = dir
	}
	data, err := e.CombinedOutput()
	text := string(data)
	text = strings.TrimSpace(text)
	if err != nil {
		return "", fmt.Errorf("Command failed '%s %s': %s %s\n", name, strings.Join(args, " "), text, err)
	}
	return text, err
}

// FlagChanged returns true if the given flag was supplied on the command line
func (o *CommonOptions) FlagChanged(name string) bool {
	if o.Cmd != nil {
		f := o.Cmd.Flag(name)
		if f != nil {
			return f.Changed
		}
	}
	return false
}
