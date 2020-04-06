package releases

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/gits"
)

// ReleaseDownloadCount returns the total number of downloads for the given set of releases
func ReleaseDownloadCount(releases []*gits.GitRelease) int {
	count := 0
	for _, release := range releases {
		count += release.DownloadCount
	}
	return count
}

// GetRelease will find the GitRelease for the given owner/repo, looking for a tag called <version> or v<version>
func GetRelease(version string, owner string, repo string, provider gits.GitProvider) (*gits.GitRelease, error) {
	release, err := provider.GetRelease(owner, repo, version)
	if err != nil {
		// normally tags are v<version> so try that
		tag := fmt.Sprintf("v%s", version)
		release, err = provider.GetRelease(owner, repo, tag)
		if err != nil {
			if ReleaseNotFoundError(err) {
				return nil, nil
			}
			return nil, errors.Wrapf(err, "getting release for %s (tried %s and %s)", version, version, tag)

		}
	}
	return release, nil
}

// ReleaseNotFoundError determines if the reason for the error is that the release is not found
func ReleaseNotFoundError(err error) bool {
	return strings.HasSuffix(err.Error(), "404 Not Found []")
}
