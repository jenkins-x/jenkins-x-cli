package virtualbox

import "github.com/jenkins-x/jx/pkg/log"

// InstallVirtualBox installs virtual box
func InstallVirtualBox() error {
	log.Logger().Warnf("We cannot yet automate the installation of VirtualBox - can you install this manually please?\nPlease see: https://www.virtualbox.org/wiki/Downloads")
	return nil
}
