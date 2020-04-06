// +build darwin

package system

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetOsVersion returns a human friendly string of the current OS
// in the case of an error this still returns a valid string for the details that can be found.
func GetOsVersion() (string, error) {
	retVal := "unknown OSX version"

	// you can not run the command once to get all this :-(
	pn, err := runCommand("sw_vers", "-productName")
	if err != nil {
		return retVal, err
	}
	pv, err := runCommand("sw_vers", "-productVersion")
	if err != nil {
		retVal = fmt.Sprintf("%s unknown version", pn)
	} else {
		retVal = fmt.Sprintf("%s %s", pn, pv)
	}

	pb, err := runCommand("sw_vers", "-buildVersion")
	if err != nil {
		return fmt.Sprintf("%s unknown build", retVal), err
	}
	return fmt.Sprintf("%s build %s", retVal, pb), nil
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
