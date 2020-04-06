package iks

import (
	"runtime"

	"github.com/jenkins-x/jx/pkg/packages"
)

// InstallIBMCloudWithVersion  installs a specific version of IBM cloud CLI
func InstallIBMCloudWithVersion(version string, skipPathScan bool) error {
	if runtime.GOOS == "darwin" {
		return packages.InstallOrUpdateBinary(packages.InstallOrUpdateBinaryOptions{
			Binary:              "ibmcloud",
			GitHubOrganization:  "",
			DownloadUrlTemplate: "https://public.dhe.ibm.com/cloud/bluemix/cli/bluemix-cli/{{.version}}/binaries/IBM_Cloud_CLI_{{.version}}_macos.tgz",
			Version:             version,
			SkipPathScan:        skipPathScan,
			VersionExtractor:    nil,
			Archived:            true,
			ArchiveDirectory:    "IBM_Cloud_CLI",
		})
	}
	return packages.InstallOrUpdateBinary(packages.InstallOrUpdateBinaryOptions{
		Binary:              "ibmcloud",
		GitHubOrganization:  "",
		DownloadUrlTemplate: "https://public.dhe.ibm.com/cloud/bluemix/cli/bluemix-cli/{{.version}}/binaries/IBM_Cloud_CLI_{{.version}}_{{.os}}_{{.arch}}.{{.extension}}",
		Version:             version,
		SkipPathScan:        skipPathScan,
		VersionExtractor:    nil,
		Archived:            true,
		ArchiveDirectory:    "IBM_Cloud_CLI",
	})
}

func InstallIBMCloud(skipPathScan bool) error {
	return InstallIBMCloudWithVersion(packages.IBMCloudVersion, skipPathScan)
}
