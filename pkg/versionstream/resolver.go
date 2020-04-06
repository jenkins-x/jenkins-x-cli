package versionstream

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// VersionResolver resolves versions of charts, packages or docker images
type VersionResolver struct {
	VersionsDir string
}

// ResolveDockerImage ensures the given docker image has a valid version if there is one in the version stream
func (v *VersionResolver) ResolveDockerImage(image string) (string, error) {
	return ResolveDockerImage(v.VersionsDir, image)
}

// StableVersion returns the stable version of the given kind name
func (v *VersionResolver) StableVersion(kind VersionKind, name string) (*StableVersion, error) {
	return LoadStableVersion(v.VersionsDir, kind, name)
}

// StableVersionNumber returns the stable version number of the given kind name
func (v *VersionResolver) StableVersionNumber(kind VersionKind, name string) (string, error) {
	return LoadStableVersionNumber(v.VersionsDir, kind, name)
}

// ResolveGitVersion resolves the version to use for the given git repository using the version stream
func (v *VersionResolver) ResolveGitVersion(gitURL string) (string, error) {
	answer, err := v.StableVersionNumber(KindGit, gitURL)
	if err != nil {
		return answer, err
	}
	if answer == "" {
		path := GitURLToName(gitURL)
		log.Logger().Warnf("could not find a stable version for git repository: %s in %s", gitURL, v.VersionsDir)
		log.Logger().Warn("for background see: https://jenkins-x.io/docs/concepts/version-stream/")
		log.Logger().Infof("please lock this version down via the command: %s", util.ColorInfo(fmt.Sprintf("jx step create pr versions -k git -n %s -v 1.2.3", path)))
	}
	return answer, nil
}

// VerifyPackages verifies that the package keys and current version numbers are at the required minimum versions
func (v *VersionResolver) VerifyPackages(packages map[string]string) error {
	errs := []error{}
	keys := util.SortedMapKeys(packages)
	for _, p := range keys {
		version := packages[p]
		if version == "" {
			continue
		}
		err := v.VerifyPackage(p, version)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return util.CombineErrors(errs...)
}

// VerifyPackage verifies the package is of a sufficient version
func (v *VersionResolver) VerifyPackage(name string, currentVersion string) error {
	data, err := LoadStableVersion(v.VersionsDir, KindPackage, name)
	if err != nil {
		return err
	}
	return data.VerifyPackage(name, currentVersion, v.VersionsDir)
}

// GetRepositoryPrefixes loads the repository prefixes for the version stream
func (v *VersionResolver) GetRepositoryPrefixes() (*RepositoryPrefixes, error) {
	return GetRepositoryPrefixes(v.VersionsDir)
}
