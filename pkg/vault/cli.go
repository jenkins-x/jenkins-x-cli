package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/jenkins-x/jx/pkg/packages"
	"github.com/jenkins-x/jx/pkg/util"
)

// InstallVaultCli installs vault cli
func InstallVaultCli() error {
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	binary := "vault"
	fileName, flag, err := packages.ShouldInstallBinary(binary)
	if err != nil || !flag {
		return err
	}
	latestVersion, err := util.GetLatestFullTagFromGithub("hashicorp", "vault")
	if err != nil {
		return err
	}
	// Strip the v off the beginning of the version number
	latestVersion = strings.Replace(latestVersion, "v", "", 1)

	clientURL := fmt.Sprintf("https://releases.hashicorp.com/vault/%s/vault_%s_%s_%s.zip", latestVersion, latestVersion, runtime.GOOS, runtime.GOARCH)
	fullPath := filepath.Join(binDir, fileName)
	tarFile := fullPath + ".zip"
	err = packages.DownloadFile(clientURL, tarFile)
	if err != nil {
		return err
	}
	err = util.UnzipSpecificFiles(tarFile, binDir, fileName)
	if err != nil {
		return err
	}
	err = os.Remove(tarFile)
	if err != nil {
		return err
	}
	err = os.Chmod(fullPath, 0755)
	return err
}
