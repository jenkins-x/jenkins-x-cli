// +build unit

package util_test

import (
	"strings"
	"testing"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/stretchr/testify/assert"
)

type testStringMatchesAnyData struct {
	input    string
	includes []string
	excludes []string
	expected bool
}

func TestStringMatchesAny(t *testing.T) {
	t.Parallel()
	testCases := []testStringMatchesAnyData{
		{
			"foo", []string{"foo"}, []string{"WIP-*"}, true,
		},
		{
			"foo", []string{"fo*"}, []string{"WIP-*"}, true,
		},
		{
			"foo", []string{"bar"}, []string{"WIP-*"}, false,
		},
		{
			"foo", []string{"*"}, []string{"WIP-*"}, true,
		},
		{
			"WIP-blah", []string{"*"}, []string{"WIP-*"}, false,
		},
		{
			"bar", []string{"foo*"}, []string{"WIP-*"}, false,
		},
	}
	for _, data := range testCases {
		actual := util.StringMatchesAny(data.input, data.includes, data.excludes)
		assert.Equal(t, data.expected, actual, "for StringMatchesAny(%s, %s, %s)", data.input, strings.Join(data.includes, ", "), strings.Join(data.excludes, ", "))
	}
}

func TestStringMatches(t *testing.T) {
	t.Parallel()
	assertStringMatches(t, "foo", "*", true)
	assertStringMatches(t, "foo", "fo*", true)
	assertStringMatches(t, "bar", "fo*", false)
}

func assertStringMatches(t *testing.T, text string, pattern string, expected bool) {
	actual := util.StringMatchesPattern(text, pattern)
	assert.Equal(t, expected, actual, "Failed to evaluate StringMatchesPattern(%s, %s)", text, pattern)
}
