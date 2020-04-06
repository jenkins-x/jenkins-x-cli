package update

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"gopkg.in/AlecAivazis/survey.v1"

	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// UpdateClusterGKETerraformOptions the flags for updating a cluster on GKE
// using terraform
type UpdateClusterGKETerraformOptions struct {
	UpdateClusterOptions

	Flags UpdateClusterGKETerraformFlags
}

type UpdateClusterGKETerraformFlags struct {
	ClusterName    string
	SkipLogin      bool
	ServiceAccount string
}

var (
	updateClusterGKETerraformLong = templates.LongDesc(`

		Command re-applies the Terraform plan in ~/.jx/clusters/<cluster>/terraform against the specified cluster

`)

	updateClusterGKETerraformExample = templates.Examples(`

		jx update cluster gke terraform

`)
)

// NewCmdUpdateClusterGKETerraform creates a command object for the updating an existing cluster running
// on GKE using terraform.
func NewCmdUpdateClusterGKETerraform(commonOpts *opts.CommonOptions) *cobra.Command {
	options := createUpdateClusterGKETerraformOptions(commonOpts, cloud.GKE)

	cmd := &cobra.Command{
		Use:     "terraform",
		Short:   "Updates an existing Kubernetes cluster on GKE using Terraform: Runs on Google Cloud",
		Long:    updateClusterGKETerraformLong,
		Example: updateClusterGKETerraformExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Flags.ClusterName, opts.OptionClusterName, "n", "", "The name of this cluster")
	cmd.Flags().BoolVarP(&options.Flags.SkipLogin, "skip-login", "", false, "Skip Google auth if already logged in via gcloud auth")
	cmd.Flags().StringVarP(&options.ServiceAccount, "service-account", "", "", "Use a service account to login to GCE")

	return cmd
}

func createUpdateClusterGKETerraformOptions(commonOpts *opts.CommonOptions, cloudProvider string) UpdateClusterGKETerraformOptions {
	options := UpdateClusterGKETerraformOptions{
		UpdateClusterOptions: UpdateClusterOptions{
			UpdateOptions: UpdateOptions{
				CommonOptions: commonOpts,
			},
			Provider: cloudProvider,
		},
	}
	return options
}

func (o *UpdateClusterGKETerraformOptions) Run() error {
	err := o.InstallRequirements(cloud.GKE, "terraform", o.InitOption.HelmBinary())
	if err != nil {
		return err
	}

	err = o.updateClusterGKETerraform()
	if err != nil {
		log.Logger().Errorf("error creating cluster %v", err)
		return err
	}

	return nil
}

func (o *UpdateClusterGKETerraformOptions) updateClusterGKETerraform() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	if !o.BatchMode {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Updating a GKE cluster with Terraform is an experimental feature in jx.  Would you like to continue?",
		}
		survey.AskOne(prompt, &confirm, nil, surveyOpts)

		if !confirm {
			// exit at this point
			return nil
		}
	}

	gcloud := o.GCloud()
	err := gcloud.Login(o.ServiceAccount, o.Flags.SkipLogin)
	if err != nil {
		return err
	}

	if o.Flags.ClusterName == "" {
		log.Logger().Info("No cluster name provided")
		return nil
	}

	serviceAccount := fmt.Sprintf("jx-%s", o.Flags.ClusterName)

	jxHome, err := util.ConfigDir()
	if err != nil {
		return err
	}

	clustersHome := filepath.Join(jxHome, "clusters")
	clusterHome := filepath.Join(clustersHome, o.Flags.ClusterName)
	os.MkdirAll(clusterHome, os.ModePerm)

	var keyPath string
	if o.ServiceAccount == "" {
		keyPath = filepath.Join(clusterHome, fmt.Sprintf("%s.key.json", serviceAccount))
	} else {
		keyPath = o.ServiceAccount
	}

	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		log.Logger().Infof("Unable to find service account key %s", keyPath)
		return nil
	}

	terraformDir := filepath.Join(clusterHome, "terraform")
	if _, err := os.Stat(terraformDir); os.IsNotExist(err) {
		log.Logger().Infof("Unable to find Terraform plan dir %s", terraformDir)
		return nil
	}

	// create .tfvars file in .jx folder
	terraformVars := filepath.Join(terraformDir, "terraform.tfvars")

	args := []string{"init", terraformDir}
	err = o.RunCommand("terraform", args...)
	if err != nil {
		return err
	}

	terraformState := filepath.Join(terraformDir, "terraform.tfstate")

	args = []string{"plan",
		fmt.Sprintf("-state=%s", terraformState),
		fmt.Sprintf("-var-file=%s", terraformVars),
		terraformDir}

	err = o.RunCommandVerbose("terraform", args...)
	if err != nil {
		return err
	}

	if !o.BatchMode {
		confirm := false
		prompt := &survey.Confirm{
			Message: "Would you like to apply this plan",
		}
		survey.AskOne(prompt, &confirm, nil, surveyOpts)

		if !confirm {
			// exit at this point
			return nil
		}
	}

	log.Logger().Info("Applying plan...")

	args = []string{"apply",
		"-auto-approve",
		fmt.Sprintf("-state=%s", terraformState),
		fmt.Sprintf("-var-file=%s", terraformVars),
		terraformDir}

	err = o.RunCommandVerbose("terraform", args...)
	if err != nil {
		return err
	}

	return nil
}
