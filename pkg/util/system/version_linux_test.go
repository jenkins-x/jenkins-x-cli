// +build unit
// +build linux

package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const UbuntuOsRelease = `
NAME="Ubuntu"
VERSION="18.04.4 LTS (Bionic Beaver)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 18.04.4 LTS"
VERSION_ID="18.04"
HOME_URL="https://www.ubuntu.com/"
SUPPORT_URL="https://help.ubuntu.com/"
BUG_REPORT_URL="https://bugs.launchpad.net/ubuntu/"
PRIVACY_POLICY_URL="https://www.ubuntu.com/legal/terms-and-policies/privacy-policy"
VERSION_CODENAME=bionic
UBUNTU_CODENAME=bionic
`

// MockUbuntuReleaseGetter provides to mock Ubuntu file.
type MockUbuntuReleaseGetter struct{}

func (m *MockUbuntuReleaseGetter) GetFileContents(file string) (string, error) {
	switch file {
	case "/etc/os-release":
		return UbuntuOsRelease, nil
	default:
		return "Ubuntu 18.04.4 LTS", nil
	}
}

func TestGetOsVersionReturnsNoErrorWithMock(t *testing.T) {
	t.Parallel()

	ubuntu := &MockUbuntuReleaseGetter{}
	SetReleaseGetter(ubuntu)
	ver, err := GetOsVersion()
	assert.NoError(t, err)
	assert.NotNil(t, ver)
	assert.Equal(t, "Ubuntu 18.04.4 LTS", ver)
}
