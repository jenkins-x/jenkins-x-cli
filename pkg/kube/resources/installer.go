package resources

import (
	"github.com/jenkins-x/jx/pkg/util"
)

const kubeCtlBinary = "kubectl"

// Installer provides support for installing Kuberntes resources directly from files
//go:generate pegomock generate github.com/jenkins-x/jx/pkg/kube/resources Installer -o mocks/installer.go
type Installer interface {
	// Install installs the Kubernetes resources provided in the file
	Install(file string) (string, error)
	// InstallDir installs the Kubernetes resources provided in the directory
	InstallDir(dir string) (string, error)
}

// KubeCtlInstaller kubectl based resources installer
type KubeCtlInstaller struct {
	runner util.Commander
}

// NewKubeCtlInstaller creates a new kubectl installer
func NewKubeCtlInstaller(cwd string, wait, validate bool) *KubeCtlInstaller {
	args := []string{"apply"}
	if wait {
		args = append(args, "--wait")
	}
	if validate {
		args = append(args, "--validate=true")
	} else {
		args = append(args, "--validate=false")
	}
	runner := &util.Command{
		Name: kubeCtlBinary,
		Args: args,
		Dir:  cwd,
	}
	return &KubeCtlInstaller{
		runner: runner,
	}
}

// Install installs the resources provided in the file
func (i *KubeCtlInstaller) Install(file string) (string, error) {
	args := i.runner.CurrentArgs()
	args = append(args, "-f", file)
	i.runner.SetArgs(args)
	return i.runner.RunWithoutRetry()
}

// InstallDir installs the resources provided in the directory
func (i *KubeCtlInstaller) InstallDir(dir string) (string, error) {
	args := i.runner.CurrentArgs()
	args = append(args, "--recursive", "-f", dir)
	i.runner.SetArgs(args)
	return i.runner.RunWithoutRetry()
}
