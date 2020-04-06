// +build !linux
// +build !darwin
// +build !windows

package system

import (
	"fmt"
	"runtime"
)

// GetOsVersion returns "unknown platform runtime.GOOS runtime.GOARCH" as a string and error.
// We don't have support for knowing how to get more details for this platform
func GetOsVersion() (string, error) {
	str := fmt.Sprintf("unknown platform %s %s", runtime.GOOS, runtime.GOARCH)
	return str, fmt.ErrorF(str)
}
