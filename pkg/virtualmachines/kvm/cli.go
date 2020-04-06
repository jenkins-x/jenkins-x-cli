package kvm

import "github.com/jenkins-x/jx/pkg/log"

// InstallKvm installs kvm
func InstallKvm() error {
	log.Logger().Warnf("We cannot yet automate the installation of KVM - can you install this manually please?\nPlease see: https://www.linux-kvm.org/page/Downloads")
	return nil
}
