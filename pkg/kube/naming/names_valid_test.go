// +build unit

package naming_test

import (
	"testing"

	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/stretchr/testify/assert"
)

func TestToValidName(t *testing.T) {
	t.Parallel()
	assertToValidName(t, "foo", "foo")
	assertToValidName(t, "foo[bot]", "foo-bot")
	assertToValidName(t, "foo-bar", "foo-bar")
	assertToValidName(t, "foo-bar-", "foo-bar")
	assertToValidName(t, "foo-bar-0.1.0", "foo-bar-0-1-0")
	assertToValidName(t, "---foo-bar-", "foo-bar")
	assertToValidName(t, "foo/bar_*123", "foo-bar-123")
	assertToValidName(t, "", "")
}

func TestToValidValue(t *testing.T) {
	t.Parallel()
	assertToValidValue(t, "1", "1")
	assertToValidValue(t, "1.2.3", "1.2.3")
	assertToValidValue(t, "foo/bar", "foo/bar")
	assertToValidValue(t, "Foo/Bar", "Foo/Bar")
	assertToValidValue(t, "foo", "foo")
	assertToValidValue(t, "foo[bot]", "foo-bot")
	assertToValidValue(t, "foo-bar", "foo-bar")
	assertToValidValue(t, "foo-bar-", "foo-bar")
	assertToValidValue(t, "foo-bar-0.1.0", "foo-bar-0.1.0")
	assertToValidValue(t, "---foo-bar-", "-foo-bar")
	assertToValidValue(t, "foo/bar_*123", "foo/bar-123")
	assertToValidValue(t, "", "")
}

func TestToValidNameWithDots(t *testing.T) {
	t.Parallel()
	assertToValidNameWithDots(t, "foo-bar-0.1.0", "foo-bar-0.1.0")
	assertToValidNameWithDots(t, "foo/bar_.123", "foo-bar-.123")
}

func TestToValidNameTruncated(t *testing.T) {
	t.Parallel()
	assertToValidNameTruncated(t, "foo", 7, "foo")
	assertToValidNameTruncated(t, "foo", 3, "foo")
	assertToValidNameTruncated(t, "foo", 2, "fo")
	assertToValidNameTruncated(t, "foo-bar", 4, "foo")
	assertToValidNameTruncated(t, "foo-bar", 5, "foo-b")
	assertToValidNameTruncated(t, "foo-bar-0.1.0", 10, "foo-bar-0")
	assertToValidNameTruncated(t, "---foo-bar-", 10, "foo-bar")
	assertToValidNameTruncated(t, "foo/bar_*123", 11, "foo-bar-123")
}

func TestToValidGCPServiceAccount(t *testing.T) {
	t.Parallel()
	assertToValidGCPServiceAccount(t, "f", "f-jx-[-a-z0-9]")
	assertToValidGCPServiceAccount(t, "foo", "foo-jx-[-a-z0-9]")
	assertToValidGCPServiceAccount(t, "fo-ko", "fo-ko-jx-[-a-z0-9]")
	assertToValidGCPServiceAccount(t, "foo-ko", "foo-ko")
	assertToValidGCPServiceAccount(t, "foo-bar", "foo-bar")
	assertToValidGCPServiceAccount(t, "foo-bar-0.1.0", "foo-bar-0")
	assertToValidGCPServiceAccount(t, "---foo-bar-", "foo-bar")
	assertToValidGCPServiceAccount(t, "foo/bar_*123", "foo-bar-123")
}

func assertToValidNameWithDots(t *testing.T, input string, expected string) {
	actual := naming.ToValidNameWithDots(input)
	assert.Equal(t, expected, actual, "ToValidNameWithDots for input %s", input)
}

func assertToValidName(t *testing.T, input string, expected string) {
	actual := naming.ToValidName(input)
	assert.Equal(t, expected, actual, "ToValidName for input %s", input)
}

func assertToValidValue(t *testing.T, input string, expected string) {
	actual := naming.ToValidValue(input)
	assert.Equal(t, expected, actual, "ToValidValue for input %s", input)
}

func assertToValidNameTruncated(t *testing.T, input string, maxLength int, expected string) {
	actual := naming.ToValidNameTruncated(input, maxLength)
	assert.Equal(t, expected, actual, "ToValidNameTruncated for input %s", input)
}

func assertToValidGCPServiceAccount(t *testing.T, input string, expected string) {
	actual := naming.ToValidGCPServiceAccount(input)
	assert.Regexp(t, expected, actual, "ToValidGCPServiceAccount for input %s", input)
	assert.Regexp(t, "[a-z]([-a-z0-9]*[a-z0-9])", actual, "GCP SA valid regex for input %s", input)
	assert.Regexp(t, "[-a-z0-9]{6,30}", actual, "GCP SA length for input %s", input)
}
