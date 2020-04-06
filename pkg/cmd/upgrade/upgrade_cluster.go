package upgrade

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	upgradeClusterLong = templates.LongDesc(`
		Upgrades the Jenkins X Kubernetes master to the specified version
`)

	upgradeClusterExample = templates.Examples(`
		# Upgrades the Jenkins X Cluster tools 
		jx upgrade cluster
	`)
)

// UpgradeClusterOptions the options for the create spring command
type UpgradeClusterOptions struct {
	UpgradeOptions

	Version     string
	ClusterName string
}

// NewCmdUpgradeCluster defines the command
func NewCmdUpgradeCluster(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeClusterOptions{
		UpgradeOptions: UpgradeOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "cluster",
		Short:   "Upgrades the Kubernetes master to the specified version",
		Long:    upgradeClusterLong,
		Example: upgradeClusterExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Version, "version", "v", "", "The specific version to upgrade to")
	cmd.Flags().StringVarP(&options.ClusterName, "cluster-name", "c", "", "The specific cluster to upgrade")
	return cmd
}

// Run implements the command
func (o *UpgradeClusterOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	confirm := false
	prompt := &survey.Confirm{
		Message: "Upgrading a GKE cluster is an experimental feature in jx.  Would you like to continue?",
	}
	survey.AskOne(prompt, &confirm, nil, surveyOpts)

	if !confirm {
		// exit at this point
		return nil
	}

	// check to see if gcloud is available

	err := o.validateGCloudIsAvailable()
	if err != nil {
		return errors.New("Unable to locate gcloud command")
	}

	// if no cluster name is set, prompt.
	selectedClusterName, err := o.getClusterName()
	if err != nil {
		return err
	}

	// if no version, prompt.
	selectedVersion, err := o.getVersion()
	if err != nil {
		return err
	}

	log.Logger().Infof("Upgrading %s master to %s (this may take a few minutes)", selectedClusterName, selectedVersion)

	err = o.RunCommandVerbose("gcloud", "container", "clusters", "upgrade", selectedClusterName, "--cluster-version", selectedVersion, "--master", "--quiet")
	if err != nil {
		return err
	}

	log.Logger().Infof("Upgrading %s nodes (this may take a few minutes)", selectedClusterName)

	return o.RunCommandVerbose("gcloud", "container", "clusters", "upgrade", selectedClusterName, "--quiet")
}

func (o *UpgradeClusterOptions) getClusterName() (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	selectedClusterName := o.ClusterName
	if selectedClusterName != "" {
		return selectedClusterName, nil
	}

	out, err := o.GetCommandOutput("", "gcloud", "container", "clusters", "list")
	if err != nil {
		return "", err
	}

	lines := strings.Split(out, "\n")
	var existingClusters []string
	for _, l := range lines {
		if strings.Contains(l, "MASTER_VERSION") {
			continue
		}
		fields := strings.Fields(l)
		existingClusters = append(existingClusters, fields[0])
	}

	if len(existingClusters) == 0 {
		return "", errors.New("Could not find a cluster to upgrade, please manually create one and rerun the wizard")
	} else if len(existingClusters) == 1 {
		selectedClusterName = existingClusters[0]
		log.Logger().Infof("Using the only GKE cluster %s", util.ColorInfo(selectedClusterName))
	} else {
		prompts := &survey.Select{
			Message: "GKE Cluster:",
			Options: existingClusters,
			Help:    "Select a GKE cluster to upgrade",
		}

		err := survey.AskOne(prompts, &selectedClusterName, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}

	if selectedClusterName == "" {
		return "", errors.New("no GKE cluster found, please manual create one and rerun this wizard")
	}

	return selectedClusterName, nil
}

func (o *UpgradeClusterOptions) getVersion() (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	selectedVersion := o.Version
	if selectedVersion != "" {
		return selectedVersion, nil
	}

	out, err := o.GetCommandOutput("", "gcloud", "container", "get-server-config", "--format=json")
	if err != nil {
		return "", err
	}

	startOfJSON := strings.Index(out, "{")
	if startOfJSON == -1 {
		return "", errors.New("does not appear to be a valid JSON response")
	}

	sc := &serverConfig{}
	err = json.Unmarshal([]byte(out[startOfJSON:]), sc)
	if err != nil {
		return "", err
	}

	prompts := &survey.Select{
		Message: "New Cluster Version:",
		Options: sc.ValidMasterVersions,
		Help:    "Select a GKE cluster version to upgrade to",
	}

	err = survey.AskOne(prompts, &selectedVersion, nil, surveyOpts)
	if err != nil {
		return "", err
	}

	return selectedVersion, nil
}

func (o *UpgradeClusterOptions) validateGCloudIsAvailable() error {
	_, err := o.GetCommandOutput("", "gcloud", "version")
	if err != nil {
		return err
	}
	return nil
}

// internal type to help unmarshal Kubernetes upgrade versions
type serverConfig struct {
	ValidMasterVersions []string `json:"validMasterVersions"`
}
