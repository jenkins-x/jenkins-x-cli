package amazon

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jenkins-x/jx/pkg/packages"
	"github.com/jenkins-x/jx/pkg/util"
)

// InstallAwsIamAuthenticatorWithVersion install a specific version of iam authenticator for AWS
func InstallAwsIamAuthenticatorWithVersion(version string, skipPathScan bool) error {
	return packages.InstallOrUpdateBinary(packages.InstallOrUpdateBinaryOptions{
		Binary:              "aws-iam-authenticator",
		GitHubOrganization:  "",
		DownloadUrlTemplate: "https://amazon-eks.s3-us-west-2.amazonaws.com/{{.version}}/2019-03-27/bin/{{.os}}/{{.arch}}/aws-iam-authenticator",
		Version:             version,
		SkipPathScan:        skipPathScan,
		VersionExtractor:    nil,
	})
}

// InstallAwsIamAuthenticator install iam authenticator for AWS
func InstallAwsIamAuthenticator(skipPathScan bool) error {
	return InstallAwsIamAuthenticatorWithVersion(packages.IamAuthenticatorAwsVersion, skipPathScan)
}

// InstallEksCtlWithVersion install a specific version of eks cli
func InstallEksCtlWithVersion(version string, skipPathScan bool) error {
	return packages.InstallOrUpdateBinary(packages.InstallOrUpdateBinaryOptions{
		Binary:              "eksctl",
		GitHubOrganization:  "weaveworks",
		DownloadUrlTemplate: "https://github.com/weaveworks/eksctl/releases/download/{{.version}}/eksctl_{{.osTitle}}_{{.arch}}.{{.extension}}",
		Version:             version,
		SkipPathScan:        skipPathScan,
		VersionExtractor:    nil,
		Archived:            true,
	})
}

// InstallEksCtl installs eks cli
func InstallEksCtl(skipPathScan bool) error {
	return InstallEksCtlWithVersion("", skipPathScan)
}

// InstallKops installs kops
func InstallKops() error {
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	binary := "kops"
	fileName, flag, err := packages.ShouldInstallBinary(binary)
	if err != nil || !flag {
		return err
	}
	latestVersion, err := util.GetLatestVersionStringFromGitHub("kubernetes", "kops")
	if err != nil {
		return err
	}
	clientURL := fmt.Sprintf("https://github.com/kubernetes/kops/releases/download/%s/kops-%s-%s", latestVersion, runtime.GOOS, runtime.GOARCH)
	fullPath := filepath.Join(binDir, fileName)
	tmpFile := fullPath + ".tmp"
	err = packages.DownloadFile(clientURL, tmpFile)
	if err != nil {
		return err
	}
	err = util.RenameFile(tmpFile, fullPath)
	if err != nil {
		return err
	}
	return os.Chmod(fullPath, 0755)
}
