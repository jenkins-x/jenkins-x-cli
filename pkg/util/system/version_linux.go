// +build linux

package system

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"
)

// ReleaseFileGetter defines the interface for read system file
type ReleaseFileGetter interface {
	GetFileContents(string) (string, error)
}

// DefaultReleaseFileGetter is the default implementation of ReleaseFileGetter
type DefaultReleaseFileGetter struct{}

// GetFileContents returns the file contents
func (r *DefaultReleaseFileGetter) GetFileContents(file string) (string, error) {
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return strings.Trim(string(buf), "\n"), nil
}

// SetReleaseGetter sets the current releaseFileGetter
func SetReleaseGetter(r ReleaseFileGetter) {
	releaseGetter = r
}

var releaseGetter ReleaseFileGetter = &DefaultReleaseFileGetter{}

// GetOsVersion returns a human friendly string of the current OS
// in the case of an error this still returns a valid string for the details that can be found.
func GetOsVersion() (string, error) {
	output, err := releaseGetter.GetFileContents("/etc/os-release")
	if err == nil {
		release, err := util.ExtractKeyValuePairs(strings.Split(strings.TrimSpace(output), "\n"), "=")
		if err == nil && len(release["PRETTY_NAME"]) != 0 {
			return strings.ReplaceAll(release["PRETTY_NAME"], "\"", ""), nil
		}
	}
	// fallback
	output, err = runCommand("lsb_release", "-d", "-s")
	if err == nil {
		return output, nil
	}
	// procfs will tell us the kernel version
	output, err = releaseGetter.GetFileContents("/proc/version")
	if err == nil {
		return fmt.Sprintf("Unknown Linux distribution %s", output), nil
	}
	return "Unknown Linux version", fmt.Errorf("Unknown Linux version")
}

func runCommand(command string, args ...string) (string, error) {
	e := exec.Command(command, args...)
	data, err := e.CombinedOutput()
	text := string(data)
	text = strings.TrimSpace(text)
	if err != nil {
		return "", fmt.Errorf("command failed '%s %s': %s %s\n", command, strings.Join(args, " "), text, err)
	}
	return text, err
}
