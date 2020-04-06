package upgrade

import (
	"io/ioutil"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/packages"

	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	upgradeBinariesLong = templates.LongDesc(`
		Upgrades the Jenkins X command line binaries (like helm or eksctl) if there is a newer release
`)

	upgradeBInariesExample = templates.Examples(`
		# Upgrades the Jenkins X binaries (like helm or eksctl) 
		jx upgrade binaries
	`)
)

type UpgradeBinariesOptions struct {
	options.CreateOptions
}

func NewCmdUpgradeBinaries(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeBinariesOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "binaries",
		Short:   "Upgrades the command line binaries (like helm or eksctl) - if there are new versions available",
		Long:    upgradeBinariesLong,
		Example: upgradeBInariesExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

func (o *UpgradeBinariesOptions) Run() error {
	binDir, err := util.JXBinLocation()
	if err != nil {
		return err
	}
	binaries, err := ioutil.ReadDir(binDir)
	if err != nil {
		return err
	}

	for _, binary := range binaries {
		if binary.Name() == "eksctl" {
			err = amazon.InstallEksCtl(true)
			if err != nil {
				return err
			}
		} else if binary.Name() == "aws-iam-authenticator" {
			err = amazon.InstallAwsIamAuthenticator(true)
			if err != nil {
				return err
			}
		} else if binary.Name() == "kubectl" {
			err = packages.InstallKubectl(true)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
