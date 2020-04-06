package step

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	optionOutputFile = "output"
)

// StepGpgCredentialsOptions contains the command line flags
type StepGpgCredentialsOptions struct {
	step.StepOptions

	OutputDir string
}

var (
	StepGpgCredentialsLong = templates.LongDesc(`
		This pipeline step generates GPG credentials files from the ` + kube.SecretJenkinsReleaseGPG + ` secret

`)

	StepGpgCredentialsExample = templates.Examples(`
		# generate the GPG credentials file in the canonical location
		jx step gpg credentials

		# generate the git credentials to a output file
		jx step gpg credentials -o /tmp/mycreds

`)
)

func NewCmdStepGpgCredentials(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGpgCredentialsOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "gpg credentials",
		Short:   "Creates the GPG credentials file for GPG signing releases",
		Long:    StepGpgCredentialsLong,
		Example: StepGpgCredentialsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.OutputDir, optionOutputFile, "o", "", "The output directory")
	return cmd
}

func (o *StepGpgCredentialsOptions) Run() error {
	kubeClient, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	ns, _, err := kube.GetDevNamespace(kubeClient, curNs)
	if err != nil {
		return err
	}
	name := kube.SecretJenkinsReleaseGPG
	secret, err := kubeClient.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		if curNs != ns {
			secret2, err2 := kubeClient.CoreV1().Secrets(curNs).Get(name, metav1.GetOptions{})
			if err2 == nil {
				secret = secret2
				err = nil
			} else {
				log.Logger().Warnf("Failed to find secret %s in namespace %s due to: %s", name, curNs, err2)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("Failed to find secret %s in namespace %s due to: %s", name, ns, err)
	}
	return o.GenerateGpgFiles(secret)
}

func (o *StepGpgCredentialsOptions) GenerateGpgFiles(secret *v1.Secret) error {
	outputDir := o.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(util.HomeDir(), ".gnupg")
	}
	if outputDir == "" {
		return util.MissingOption(optionOutputFile)
	}
	err := os.MkdirAll(outputDir, util.DefaultWritePermissions)

	for k, v := range secret.Data {
		fileName := filepath.Join(outputDir, k)
		err = ioutil.WriteFile(fileName, v, util.DefaultWritePermissions)
		if err != nil {
			return err
		}
		log.Logger().Infof("Generated file %s", util.ColorInfo(fileName))
	}
	return nil
}
