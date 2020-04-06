package ksync

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jenkins-x/jx/pkg/packages"
	"github.com/jenkins-x/jx/pkg/util"
)

// InstallKSync install ksync
func InstallKSync() (string, error) {
	binDir, err := util.JXBinLocation()
	if err != nil {
		return "", err
	}
	binary := "ksync"
	fileName, flag, err := packages.ShouldInstallBinary(binary)
	if err != nil || !flag {
		// Exec `ksync` to find the version
		ksyncCmd := util.Command{
			Name: fileName,
			Args: []string{
				"version",
			},
		}
		// Explicitly ignore any errors from ksync version, as we just need the output!
		res, _ := ksyncCmd.RunWithoutRetry()
		lines := strings.Split(res, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Git Tag:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "Git Tag:")), nil
			}
		}

		return "", fmt.Errorf("unable to find version of ksync")
	}
	latestVersion, err := util.GetLatestVersionFromGitHub("vapor-ware", "ksync")
	if err != nil {
		return "", err
	}
	clientURL := fmt.Sprintf("https://github.com/vapor-ware/ksync/releases/download/%s/ksync_%s_%s", latestVersion, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		clientURL += ".exe"
	}
	fullPath := filepath.Join(binDir, fileName)
	tmpFile := fullPath + ".tmp"
	err = packages.DownloadFile(clientURL, tmpFile)
	if err != nil {
		return "", err
	}
	err = util.RenameFile(tmpFile, fullPath)
	if err != nil {
		return "", err
	}
	return latestVersion.String(), os.Chmod(fullPath, 0755)
}
