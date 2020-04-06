package util

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"

	"github.com/blang/semver"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var githubClient *github.Client

// Download a file from the given URL
func DownloadFile(filepath string, url string) (err error) {
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := GetClientWithTimeout(time.Hour * 2).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("download of %s failed with return code %d", url, resp.StatusCode)
		return err
	}

	// Writer the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// make it executable
	os.Chmod(filepath, 0755)
	if err != nil {
		return err
	}
	return nil
}

func GetLatestVersionFromGitHub(githubOwner, githubRepo string) (semver.Version, error) {
	text, err := GetLatestVersionStringFromGitHub(githubOwner, githubRepo)
	if err != nil {
		return semver.Version{}, err
	}
	if text == "" {
		return semver.Version{}, fmt.Errorf("No version found")
	}
	return semver.Make(text)
}

func GetLatestVersionStringFromGitHub(githubOwner, githubRepo string) (string, error) {
	latestVersionString, err := GetLatestReleaseFromGitHub(githubOwner, githubRepo)
	if err != nil {
		return "", err
	}
	if latestVersionString != "" {
		return strings.TrimPrefix(latestVersionString, "v"), nil
	}
	return "", fmt.Errorf("Unable to find the latest version for github.com/%s/%s", githubOwner, githubRepo)
}

// GetLatestVersionStringFromBucketURLs return the latest version from a list of buckets with the version at the end of the path
func GetLatestVersionStringFromBucketURLs(versionStrings []string) (semver.Version, error) {
	versions := make([]semver.Version, 0)
	for _, versionStr := range versionStrings {
		versionPaths := strings.Split(versionStr, "/")
		version, err := semver.New(versionPaths[len(versionPaths)-2])
		if err != nil {
			return semver.Version{}, err
		}
		versions = append(versions, *version)
	}
	semver.Sort(versions)
	return versions[len(versions)-1], nil
}

// GetLatestReleaseFromGitHub gets the latest Release from a specific github repo
func GetLatestReleaseFromGitHub(githubOwner, githubRepo string) (string, error) {
	// Github has low (60/hour) unauthenticated limits from a single IP address. Try to get the latest release via HTTP
	// first to avoid hitting this limit (eg, small company behind one IP address)
	version := ""
	var err error
	version, err = getLatestReleaseFromGithubUsingHttpRedirect(githubOwner, githubRepo)

	if version == "" || err != nil {
		log.Logger().Warnf("getting latest release using HTTP redirect (%v) - using API instead", err)
		version, err = getLatestReleaseFromGithubUsingApi(githubOwner, githubRepo)
	}

	return version, err
}

// GetLatestReleaseFromGitHubURL returns the latest release version for the git URL
func GetLatestReleaseFromGitHubURL(gitURL string) (string, error) {
	const gitHubPrefix = "https://github.com/"
	if !strings.HasPrefix(gitURL, gitHubPrefix) {
		log.Logger().Warnf("cannot determine the latest release of version stream git URL %s\n", gitURL)
		return "", nil
	}
	name := strings.TrimPrefix(gitURL, gitHubPrefix)
	paths := strings.Split(name, "/")
	if len(paths) <= 1 {
		log.Logger().Warnf("cannot parse git URL %s so cannot determine the latest release\n", gitURL)
		return "", nil
	}
	owner := paths[0]
	repo := strings.TrimSuffix(paths[1], ".git")
	return GetLatestReleaseFromGitHub(owner, repo)
}

func getLatestReleaseFromGithubUsingApi(githubOwner, githubRepo string) (string, error) {
	client, release, resp, err := preamble()
	release, resp, err = client.Repositories.GetLatestRelease(context.Background(), githubOwner, githubRepo)
	if err != nil {
		return "", errors.Wrapf(err, "getting latest version for github.com/%s/%s", githubOwner, githubRepo)
	}
	defer resp.Body.Close()
	latestVersionString := release.TagName
	if latestVersionString != nil {
		return *latestVersionString, nil
	}
	return "", fmt.Errorf("unable to find the latest version for github.com/%s/%s", githubOwner, githubRepo)
}

func getLatestReleaseFromGithubUsingHttpRedirect(githubOwner, githubRepo string) (string, error) {
	return getLatestReleaseFromHostUsingHttpRedirect("https://github.com", githubOwner, githubRepo)
}

func getLatestReleaseFromHostUsingHttpRedirect(host, githubOwner, githubRepo string) (string, error) {
	// Github will redirect "https://github.com/organisation/repo/releases/latest" to the latest release, eg
	// https://github.com/jenkins-x/jx/releases/tag/v1.3.696
	// We can use this to get the latest release without affecting any API limits.
	url := fmt.Sprintf("%s/%s/%s/releases/latest", host, githubOwner, githubRepo)

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { // Don't follow redirects
			// We want to follow 301 permanent redirects (eg, repo renames like kubernetes/helm --> helm/helm)
			// but not temporary 302 temporary redirects (as these point to the latest tag)
			if req.Response.StatusCode == 302 {
				return http.ErrUseLastResponse
			} else {
				return nil
			}
		},
	}

	response, err := client.Get(url)
	if err != nil {
		return "", errors.Wrapf(err, "getting %s", url)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode <= 399 {
		location := response.Header.Get("Location")
		if location == "" {
			return "", fmt.Errorf("no location header in repsponse")
		}

		arr := strings.Split(location, "releases/tag/")
		if len(arr) == 2 {
			return arr[1], nil
		} else {
			return "", fmt.Errorf("unexpected location header: %s", location)
		}
	} else {
		return "", fmt.Errorf("could not determine redirect for %s. Got a %v response", url, response.StatusCode)
	}
}

// GetLatestFullTagFromGithub gets the latest 'full' tag from a specific github repo. This (at present) ignores releases
// with a hyphen in it, usually used with -SNAPSHOT, or -RC1 or -beta
func GetLatestFullTagFromGithub(githubOwner, githubRepo string) (string, error) {
	tags, err := GetTagsFromGithub(githubOwner, githubRepo)
	if err == nil {
		// Iterate over the tags to find the first that doesn't contain any hyphens in it (so is just x.y.z)
		for _, tag := range tags {
			name := *tag.Name
			if !strings.ContainsRune(name, '-') {
				return name, nil
			}
		}
		return "", errors.Errorf("No Full releases found for %s/%s", githubOwner, githubRepo)
	}
	return "", err
}

// GetLatestTagFromGithub gets the latest (in github order) tag from a specific github repo
func GetLatestTagFromGithub(githubOwner, githubRepo string) (string, error) {
	tags, err := GetTagsFromGithub(githubOwner, githubRepo)
	if err == nil {
		return *tags[0].Name, nil
	}
	return "", err
}

// GetTagsFromGithub gets the list of tags on a specific github repo
func GetTagsFromGithub(githubOwner, githubRepo string) ([]*github.RepositoryTag, error) {
	client, _, resp, err := preamble()

	tags, resp, err := client.Repositories.ListTags(context.Background(), githubOwner, githubRepo, nil)
	defer resp.Body.Close()
	if err != nil {
		return []*github.RepositoryTag{}, fmt.Errorf("Unable to get tags for github.com/%s/%s %v", githubOwner, githubRepo, err)
	}

	return tags, nil
}

func preamble() (*github.Client, *github.RepositoryRelease, *github.Response, error) {
	if githubClient == nil {
		token := os.Getenv("GH_TOKEN")
		var tc *http.Client
		if len(token) > 0 {
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: token},
			)
			tc = oauth2.NewClient(oauth2.NoContext, ts)
		}
		githubClient = github.NewClient(tc)
	}
	client := githubClient
	var (
		release *github.RepositoryRelease
		resp    *github.Response
		err     error
	)
	return client, release, resp, err
}

// untargz a tarball to a target, from
// http://blog.ralch.com/tutorial/golang-working-with-tar-and-gzipf
func UnTargz(tarball, target string, onlyFiles []string) error {
	zreader, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer zreader.Close()

	reader, err := gzip.NewReader(zreader)
	defer reader.Close()
	if err != nil {
		panic(err)
	}

	tarReader := tar.NewReader(reader)

	for {
		inkey := false
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		for _, value := range onlyFiles {
			if value == "*" || value == path.Base(header.Name) {
				inkey = true
				break
			}
		}

		if !inkey && len(onlyFiles) > 0 {
			continue
		}

		path := filepath.Join(target, path.Base(header.Name))
		UnTarFile(header, path, tarReader)
	}
	return nil
}

// untargz a tarball to a target including any folders inside the tarball
// http://blog.ralch.com/tutorial/golang-working-with-tar-and-gzipf
func UnTargzAll(tarball, target string) error {
	zreader, err := os.Open(tarball)
	if err != nil {
		return err
	}
	defer zreader.Close()

	reader, err := gzip.NewReader(zreader)
	defer reader.Close()
	if err != nil {
		panic(err)
	}

	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(target, header.Name)
		UnTarFile(header, path, tarReader)
	}
	return nil
}

// UnTarFile extracts one file from the tar, or creates a directory
func UnTarFile(header *tar.Header, target string, tarReader io.Reader) error {
	info := header.FileInfo()
	if info.IsDir() {
		if err := os.MkdirAll(target, info.Mode()); err != nil {
			return err
		}
		return nil
	}
	// In a normal archive, directories are mentionned before their files
	// But in an archive generated by helm, no directories are mentionned
	if err := os.MkdirAll(path.Dir(target), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, tarReader)
	return err
}
