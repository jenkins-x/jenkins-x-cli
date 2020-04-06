// +build unit

package util_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestUrlJoin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "http://foo.bar/whatnot/thingy", util.UrlJoin("http://foo.bar", "whatnot", "thingy"))
	assert.Equal(t, "http://foo.bar/whatnot/thingy/", util.UrlJoin("http://foo.bar/", "/whatnot/", "/thingy/"))
}

func TestUrlHostNameWithoutPort(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"hostname":                         "hostname",
		"1.2.3.4":                          "1.2.3.4",
		"1.2.3.4:123":                      "1.2.3.4",
		"https://1.2.3.4:123":              "1.2.3.4",
		"https://1.2.3.4:123/":             "1.2.3.4",
		"https://1.2.3.4:123/foo/bar":      "1.2.3.4",
		"http://user:password@1.2.3.4":     "1.2.3.4",
		"http://user:password@1.2.3.4/foo": "1.2.3.4",
	}

	for rawURI, expected := range tests {
		actual, err := util.UrlHostNameWithoutPort(rawURI)
		assert.NoError(t, err, "for input: %s", rawURI)
		assert.Equal(t, expected, actual, "for input: %s", rawURI)
	}
}

func TestSanitizeURL(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"http://test.com":                 "http://test.com",
		"http://user:test@github.com":     "http://github.com",
		"https://user:test@github.com":    "https://github.com",
		"https://user:@github.com":        "https://github.com",
		"https://:pass@github.com":        "https://github.com",
		"git@github.com:jenkins-x/jx.git": "git@github.com:jenkins-x/jx.git",
		"invalid/url":                     "invalid/url",
	}

	for test, expected := range tests {
		t.Run(test, func(t *testing.T) {
			actual := util.SanitizeURL(test)
			assert.Equal(t, expected, actual, "for url: %s", test)
		})
	}
}
