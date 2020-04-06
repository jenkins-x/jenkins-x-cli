package kvm2

import "github.com/jenkins-x/jx/pkg/log"

// InstallKvm2 install kvm2
func InstallKvm2() error {
	log.Logger().Warnf("We cannot yet automate the installation of KVM with KVM2 driver - can you install this manually please?\nPlease see: https://www.linux-kvm.org/page/Downloads " +
		"and https://github.com/kubernetes/minikube/blob/master/docs/drivers.md#kvm2-driver")
	return nil
}
