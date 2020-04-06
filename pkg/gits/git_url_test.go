// +build unit

package gits_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/stretchr/testify/assert"
)

type parseGitURLData struct {
	url          string
	host         string
	organisation string
	name         string
}

type parseGitOrganizationURLData struct {
	url          string
	host         string
	organisation string
}

func TestParseGitOrganizationURL(t *testing.T) {
	t.Parallel()
	testCases := []parseGitOrganizationURLData{
		{
			"git://host.xz/org/repo", "host.xz", "org",
		},
		{
			"git://host.xz/org", "host.xz", "org",
		},
		{
			"git://host.xz/org/repo.git", "host.xz", "org",
		},
		{
			"git://host.xz/org/repo.git/", "host.xz", "org",
		},
		{
			"git://github.com/jstrachan/npm-pipeline-test-project.git", "github.com", "jstrachan",
		},
		{
			"https://github.com/fabric8io/foo.git", "github.com", "fabric8io",
		},
		{
			"https://github.com/fabric8io/foo", "github.com", "fabric8io",
		},
		{
			"git@github.com:jstrachan/npm-pipeline-test-project.git", "github.com", "jstrachan",
		},
		{
			"git@github.com:bar/foo.git", "github.com", "bar",
		},
		{
			"git@github.com:bar/foo", "github.com", "bar",
		},
		{
			"bar/foo", "github.com", "bar",
		},
		{
			"bar", "github.com", "bar",
		},
		{
			"http://test-user@auth.example.com/scm/bar/foo.git", "auth.example.com", "bar",
		},
		{
			"https://bitbucketserver.com/projects/myproject/repos/foo/pull-requests/1", "bitbucketserver.com", "myproject",
		},
	}
	for _, data := range testCases {
		info, err := gits.ParseGitOrganizationURL(data.url)
		assert.Nil(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, data.host, info.Host, "Host does not match for input %s", data.url)
		assert.Equal(t, data.organisation, info.Organisation, "Organisation does not match for input %s", data.url)
	}
}

func TestParseGitURL(t *testing.T) {
	t.Parallel()
	testCases := []parseGitURLData{
		{
			"git://host.xz/org/repo", "host.xz", "org", "repo",
		},
		{
			"git://host.xz/org/repo.git", "host.xz", "org", "repo",
		},
		{
			"git://host.xz/org/repo.git/", "host.xz", "org", "repo",
		},
		{
			"git://github.com/jstrachan/npm-pipeline-test-project.git", "github.com", "jstrachan", "npm-pipeline-test-project",
		},
		{
			"https://github.com/fabric8io/foo.git", "github.com", "fabric8io", "foo",
		},
		{
			"https://github.com/fabric8io/foo", "github.com", "fabric8io", "foo",
		},
		{
			"git@github.com:jstrachan/npm-pipeline-test-project.git", "github.com", "jstrachan", "npm-pipeline-test-project",
		},
		{
			"git@github.com:bar/foo.git", "github.com", "bar", "foo",
		},
		{
			"git@github.com:bar/foo", "github.com", "bar", "foo",
		},
		{
			"git@github.com:bar/overview", "github.com", "bar", "overview",
		},
		{
			"git@gitlab.com:bar/subgroup/foo", "gitlab.com", "bar", "foo",
		},
		{
			"https://gitlab.com/bar/subgroup/foo", "gitlab.com", "bar", "foo",
		},
		{
			"https://gitlab.com/bar/subgroup/overview", "gitlab.com", "bar", "overview",
		},
		{
			"bar/foo", "github.com", "bar", "foo",
		},
		{
			"http://test-user@auth.example.com/scm/bar/foo.git", "auth.example.com", "bar", "foo",
		},
		{
			"https://bitbucketserver.com/projects/myproject/repos/foo/pull-requests/1", "bitbucketserver.com", "myproject", "foo",
		},
		{
			"https://bitbucketserver.com/projects/myproject/repos/foo/pull-requests/1/overview", "bitbucketserver.com", "myproject", "foo",
		},
	}
	for _, data := range testCases {
		info, err := gits.ParseGitURL(data.url)
		assert.Nil(t, err)
		assert.NotNil(t, info)
		assert.Equal(t, data.host, info.Host, "Host does not match for input %s", data.url)
		assert.Equal(t, data.organisation, info.Organisation, "Organisation does not match for input %s", data.url)
		assert.Equal(t, data.name, info.Name, "Name does not match for input %s", data.url)
	}
}

func TestSaasKind(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		gitURL string
		kind   string
	}{
		"GitHub": {
			gitURL: "https://github.com/test",
			kind:   gits.KindGitHub,
		},
		"GitHub Enterprise": {
			gitURL: "https://github.test.com",
			kind:   gits.KindGitHub,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			k := gits.SaasGitKind(tc.gitURL)
			assert.Equal(t, tc.kind, k)
		})
	}
}

func TestGitInfoProviderURL(t *testing.T) {
	for _, u := range []string{"https://github.com/jenkins-x/x.git", "git@github.com:jenkins-x/jx.git"} {
		info, err := gits.ParseGitURL(u)
		require.NoError(t, err, "for URL %s", u)
		assert.Equal(t, "https://github.com", info.ProviderURL(), "ProviderURL() for %s", u)
	}
}
