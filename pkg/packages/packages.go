package packages

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"sigs.k8s.io/yaml"

	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
)

// IBMCloudVersion ibm cloud binary version
const IBMCloudVersion = "0.10.1"

// IamAuthenticatorAwsVersion authenticator binary version to use
const IamAuthenticatorAwsVersion = "1.12.7"

// KubectlVersion binary version to use
const KubectlVersion = "1.13.2"

func BinaryWithExtension(binary string) string {
	if runtime.GOOS == "windows" {
		if binary == "gcloud" {
			return binary + ".cmd"
		}
		return binary + ".exe"
	}
	return binary
}

func LookupForBinary(binary string) (string, error) {
	path, err := exec.LookPath(BinaryWithExtension(binary))
	if err != nil {
		return "", err
	}

	return path, nil
}

type VersionExtractor interface {
	arguments() []string

	extractVersion(command string, arguments []string) (string, error)
}

func IsBinaryWithProperVersionInstalled(binary string, expectedVersion string, versionExtractor VersionExtractor) (bool, error) {
	if versionExtractor == nil {
		installedVersions, err := LoadInstalledPackages()
		if err != nil {
			return false, err
		}
		if versionValue, versionExists := installedVersions[binary]; versionExists {
			return versionValue == expectedVersion, nil
		}

		return false, nil
	}

	binaryPath, err := LookupForBinary(binary)
	if err != nil {
		return false, errors.Wrap(err, "looking up the binary")
	}
	if binaryPath != "" {
		currentVersion, err := versionExtractor.extractVersion(binaryPath, versionExtractor.arguments())
		if err != nil {
			return false, errors.Wrap(err, "extracting the version")
		}
		if currentVersion == expectedVersion {
			return true, nil
		}

	}
	return false, nil
}

// Managing installed packages

func LoadInstalledPackages() (map[string]string, error) {
	installedPackagesFile, err := InstalledPackagesFile()
	if err != nil {
		return nil, err
	}
	binariesVersions := map[string]string{}
	if _, err := os.Stat(installedPackagesFile); err == nil {
		binariesBytes, err := ioutil.ReadFile(installedPackagesFile)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(binariesBytes, &binariesVersions)
		if err != nil {
			return nil, err
		}
		return binariesVersions, nil
	}
	return map[string]string{}, nil
}

// RememberInstalledPackage writes the version of package into local file system. This allows to identify
// what version of package is currently installed in ~/.jx/bin .
func RememberInstalledPackage(packageName string, version string) error {
	if packageName == "" {
		return errors.New("package name cannot be empty")
	}
	if version == "" {
		return errors.New("package version cannot be empty")
	}

	versions, err := LoadInstalledPackages()
	if err != nil {
		return err
	}

	versions[packageName] = version

	binariesBytes, err := yaml.Marshal(versions)
	if err != nil {
		return err
	}
	installedPackagesFile, err := InstalledPackagesFile()
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(installedPackagesFile, binariesBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

// InstalledPackagesFile returns absolute path to binaries.yml file used to store version of installed packages.
func InstalledPackagesFile() (string, error) {
	configDir, err := util.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "/binaries.yml"), nil
}

// Downloading

// DownloadFile downloads binary content of given URL into local filesystem.
func DownloadFile(clientURL string, fullPath string) error {
	log.Logger().Infof("Downloading %s to %s...", util.ColorInfo(clientURL), util.ColorInfo(fullPath))
	err := util.DownloadFile(fullPath, clientURL)
	if err != nil {
		return fmt.Errorf("Unable to download file %s from %s due to: %v", fullPath, clientURL, err)
	}
	log.Logger().Infof("Downloaded %s", util.ColorInfo(fullPath))
	return nil
}
