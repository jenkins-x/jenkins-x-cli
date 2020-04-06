package gits

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"
)

const (
	GitHubHost = "github.com"
	GitHubURL  = "https://github.com"

	gitPrefix = "git@"
)

func (i *GitRepository) IsGitHub() bool {
	return GitHubHost == i.Host || strings.HasSuffix(i.URL, "https://github.com")
}

// PullRequestURL returns the URL of a pull request of the given name/number
func (i *GitRepository) PullRequestURL(prName string) string {
	return util.UrlJoin("https://"+i.Host, i.Organisation, i.Name, "pull", prName)
}

// HttpCloneURL returns the HTTPS git URL this repository
func (i *GitRepository) HttpCloneURL() string {
	return i.HttpsURL() + ".git"
}

// HttpURL returns the URL to browse this repository in a web browser
func (i *GitRepository) HttpURL() string {
	host := i.Host
	if !strings.Contains(host, ":/") {
		host = "http://" + host
	}
	return util.UrlJoin(host, i.Organisation, i.Name)
}

// HttpsURL returns the URL to browse this repository in a web browser
func (i *GitRepository) HttpsURL() string {
	host := i.Host
	if !strings.Contains(host, ":/") {
		host = "https://" + host
	}
	return util.UrlJoin(host, i.Organisation, i.Name)
}

// HostURL returns the URL to the host
func (i *GitRepository) HostURL() string {
	answer := i.Host
	if !strings.Contains(answer, ":/") {
		// lets find the scheme from the URL
		u := i.URL
		if u != "" {
			u2, err := url.Parse(u)
			if err != nil {
				// probably a git@ URL
				return "https://" + answer
			}
			s := u2.Scheme
			if s != "" {
				if !strings.HasSuffix(s, "://") {
					s += "://"
				}
				return s + answer
			}
		}
		return "https://" + answer
	}
	return answer
}

// URLWithoutUser returns the URL without any user/password
func (i *GitRepository) URLWithoutUser() string {
	u := i.URL
	if u != "" {
		if strings.HasPrefix(u, gitPrefix) {
			return u
		}
		u2, err := url.Parse(u)
		if err == nil {
			u2.User = nil
			return u2.String()
		}

	}
	host := i.Host
	if !strings.Contains(host, ":/") {
		host = "https://" + host
	}
	return host
}

func (i *GitRepository) HostURLWithoutUser() string {
	u := i.URL
	if u != "" {
		u2, err := url.Parse(u)
		if err == nil {
			u2.User = nil
			u2.Path = ""
			return u2.String()
		}

	}
	host := i.Host
	if !strings.Contains(host, ":/") {
		host = "https://" + host
	}
	return host
}

// PipelinePath returns the pipeline path for the master branch which can be used to query
// pipeline logs in `jx get build logs myPipelinePath`
func (i *GitRepository) PipelinePath() string {
	return i.Organisation + "/" + i.Name + "/master"
}

// ParseGitURL attempts to parse the given text as a URL or git URL-like string to determine
// the protocol, host, organisation and name
func ParseGitURL(text string) (*GitRepository, error) {
	answer := GitRepository{
		URL: text,
	}
	u, err := url.Parse(text)
	if err == nil && u != nil {
		answer.Host = u.Host

		// lets default to github
		if answer.Host == "" {
			answer.Host = GitHubHost
		}
		if answer.Scheme == "" {
			answer.Scheme = "https"
		}
		answer.Scheme = u.Scheme
		return parsePath(u.Path, &answer, true)
	}

	// handle git@ kinds of URIs
	if strings.HasPrefix(text, gitPrefix) {
		t := strings.TrimPrefix(text, gitPrefix)
		t = strings.TrimPrefix(t, "/")
		t = strings.TrimPrefix(t, "/")
		t = strings.TrimSuffix(t, "/")
		t = strings.TrimSuffix(t, ".git")

		arr := util.RegexpSplit(t, ":|/")
		if len(arr) >= 3 {
			answer.Scheme = "git"
			answer.Host = arr[0]
			answer.Organisation = arr[1]
			answer.Name = arr[len(arr)-1]
			return &answer, nil
		}
	}
	return nil, fmt.Errorf("Could not parse Git URL %s", text)
}

// ParseGitOrganizationURL attempts to parse the given text as a URL or git URL-like string to determine
// the protocol, host, organisation
func ParseGitOrganizationURL(text string) (*GitRepository, error) {
	answer := GitRepository{
		URL: text,
	}
	u, err := url.Parse(text)
	if err == nil && u != nil {
		answer.Host = u.Host

		// lets default to github
		if answer.Host == "" {
			answer.Host = GitHubHost
		}
		if answer.Scheme == "" {
			answer.Scheme = "https"
		}
		answer.Scheme = u.Scheme
		return parsePath(u.Path, &answer, false)
	}
	// handle git@ kinds of URIs
	if strings.HasPrefix(text, gitPrefix) {
		t := strings.TrimPrefix(text, gitPrefix)
		t = strings.TrimPrefix(t, "/")
		t = strings.TrimPrefix(t, "/")
		t = strings.TrimSuffix(t, "/")
		t = strings.TrimSuffix(t, ".git")

		arr := util.RegexpSplit(t, ":|/")
		if len(arr) >= 3 {
			answer.Scheme = "git"
			answer.Host = arr[0]
			answer.Organisation = arr[1]
			return &answer, nil
		}
	}
	return nil, fmt.Errorf("could not parse Git URL %s", text)
}

func parsePath(path string, info *GitRepository, requireRepo bool) (*GitRepository, error) {

	// This is necessary for Bitbucket Server in some cases.
	trimPath := strings.TrimPrefix(path, "/scm")

	// This is necessary for Bitbucket Server, EG: /projects/ORG/repos/NAME/pull-requests/1/overview
	reOverview := regexp.MustCompile("/pull-requests/[0-9]+/overview$")
	if reOverview.MatchString(trimPath) {
		trimPath = strings.TrimSuffix(trimPath, "/overview")
	}

	// This is necessary for Bitbucket Server in other cases
	trimPath = strings.Replace(trimPath, "/projects/", "/", 1)
	trimPath = strings.Replace(trimPath, "/repos/", "/", 1)
	re := regexp.MustCompile("/pull.*/[0-9]+$")
	trimPath = re.ReplaceAllString(trimPath, "")

	// Remove leading and trailing slashes so that splitting on "/" won't result
	// in empty strings at the beginning & end of the array.
	trimPath = strings.TrimPrefix(trimPath, "/")
	trimPath = strings.TrimSuffix(trimPath, "/")

	trimPath = strings.TrimSuffix(trimPath, ".git")

	arr := strings.Split(trimPath, "/")
	if len(arr) >= 2 {
		// We're assuming the beginning of the path is of the form /<org>/<repo> or /<org>/<subgroup>/.../<repo>
		info.Organisation = arr[0]
		info.Project = arr[0]
		info.Name = arr[len(arr)-1]

		return info, nil
	} else if len(arr) == 1 && !requireRepo {
		// We're assuming the beginning of the path is of the form /<org>/<repo>
		info.Organisation = arr[0]
		info.Project = arr[0]
		return info, nil
	}

	return info, fmt.Errorf("Invalid path %s could not determine organisation and repository name", path)
}

// SaasGitKind returns the kind for SaaS Git providers or "" if the URL could not be deduced
func SaasGitKind(gitServiceUrl string) string {
	gitServiceUrl = strings.TrimSuffix(gitServiceUrl, "/")
	switch gitServiceUrl {
	case "http://github.com":
		return KindGitHub
	case "https://github.com":
		return KindGitHub
	case "https://gitlab.com":
		return KindGitlab
	case "http://bitbucket.org":
		return KindBitBucketCloud
	case BitbucketCloudURL:
		return KindBitBucketCloud
	case "http://fake.git", FakeGitURL:
		return KindGitFake
	default:
		if strings.HasPrefix(gitServiceUrl, "https://github") {
			return KindGitHub
		}
		return ""
	}
}
