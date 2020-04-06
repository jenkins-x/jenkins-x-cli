// +build windows

package system

import (
	"fmt"
	//	"fmt"
	"golang.org/x/sys/windows/registry"
)

// GetOsVersion returns a human friendly string of the current OS
// in the case of an error this still returns a valid string for the details that can be found.
func GetOsVersion() (string, error) {
	// we can not use GetVersion as that returns a version that the app sees (from its manifest) and not the current version
	// and is missing some info.  So use the registry e.g.
	// HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\CurrentVersion  -> 6.3 (used on older platforms)
	// HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\ProductName  -> Windows 10 Pro
	// HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NcT\CurrentVersion\ReleaseId  -> 1803
	// HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Windows NT\CurrentVersion\CurrentBuild -> 17134

	retVal := "unknown windows version"

	regkey, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return retVal, err
	}
	defer regkey.Close()

	pn, _, err := regkey.GetStringValue("ProductName")
	if err != nil {
		pn, err = getMajorMinorVersion(&regkey)
		if err != nil {
			return retVal, err
		}
		pn = fmt.Sprintf("Windows %s", pn)
	}

	rel, _, err := regkey.GetStringValue("ReleaseId")
	if err != nil {
		retVal = fmt.Sprintf("%s %s", pn, "unkown release")
	} else {
		retVal = fmt.Sprintf("%s %s", pn, rel)
	}

	build, _, err := regkey.GetStringValue("CurrentBuild")
	if err != nil {
		retVal = fmt.Sprintf("%s build %s", retVal, "unknown")
	} else {
		retVal = fmt.Sprintf("%s build %s", retVal, build)
	}
	return retVal, nil
}

func getMajorMinorVersion(regkey *registry.Key) (string, error) {
	major, _, err := regkey.GetIntegerValue("CurrentMajorVersionNumber")
	if err != nil {
		// try currentVersion which will only be up to 8.1
		ver, _, err := regkey.GetStringValue("CurrentVersion")
		return ver, err
	}
	minor, _, err := regkey.GetIntegerValue("CurrentMinorVersionNumber")
	if err != nil {
		return fmt.Sprintf("unknwown windows (%d.unknown)", major), nil
	}
	return fmt.Sprintf("unknwown windows (%d.%d)", major, minor), nil
}
