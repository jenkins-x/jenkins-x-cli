package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/cve"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// GetGitOptions the command line options
type GetCVEOptions struct {
	GetOptions
	ImageName         string
	ImageID           string
	Version           string
	Env               string
	VulnerabilityType string
}

var (
	getCVELong = templates.LongDesc(`
		Display Common Vulnerabilities and Exposures (CVEs)

`)

	getCVEExample = templates.Examples(`
		# List all Common Vulnerabilities and Exposures (CVEs)

		jx get cve # using current dir as the context for app name
		jx get cve --app foo
		jx get cve --app foo --version 1.0.0
		jx get cve --app foo --environment staging
		jx get cve --environment staging
	`)
)

// NewCmdGetCVE creates the command
func NewCmdGetCVE(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetCVEOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "cve [flags]",
		Short:   "Display Common Vulnerabilities and Exposures (CVEs)",
		Long:    getCVELong,
		Example: getCVEExample,
		Aliases: []string{"cves"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addGetCVEFlags(cmd)

	return cmd
}

func (o *GetCVEOptions) addGetCVEFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ImageName, "image-name", "", "", "Full image name e.g. jenkinsxio/nexus ")
	cmd.Flags().StringVarP(&o.ImageID, "image-id", "", "", "Image ID in CVE engine if already known")
	cmd.Flags().StringVarP(&o.Version, "version", "", "", "Version or tag e.g. 0.0.1")
	cmd.Flags().StringVarP(&o.Env, "environment", "e", "", "The Environment to find running applications")
}

// Run implements this command
func (o *GetCVEOptions) Run() error {

	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return fmt.Errorf("cannot connect to Kubernetes cluster: %v", err)
	}

	jxClient, _, err := o.JXClient()
	if err != nil {
		return fmt.Errorf("cannot create jx client: %v", err)
	}

	externalURL, err := o.EnsureAddonServiceAvailable(kube.AddonServices[create.DefaultAnchoreName])
	if err != nil {
		log.Logger().Warnf("no CVE provider service found, are you in your teams dev environment?  Type `jx env` to switch.")
		return fmt.Errorf("if no CVE provider running, try running `jx create addon anchore` in your teams dev environment: %v", err)
	}

	// if no flags are set try and guess the image name from the current directory
	if o.ImageID == "" && o.ImageName == "" && o.Env == "" {
		return fmt.Errorf("no --image-name, --image-id or --environment flags set\n")
	}

	server, auth, err := o.GetAddonAuthByKind(kube.ValueKindCVE, externalURL)
	if err != nil {
		return fmt.Errorf("error getting anchore engine auth details, %v", err)
	}

	p, err := cve.NewAnchoreProvider(server, auth)
	if err != nil {
		return fmt.Errorf("error creating anchore provider, %v", err)
	}
	table := o.CreateTable()
	table.AddRow("Image", util.ColorInfo("Severity"), "Vulnerability", "URL", "Package", "Fix")

	query := cve.CVEQuery{
		ImageID:     o.ImageID,
		ImageName:   o.ImageName,
		Environment: o.Env,
		Vesion:      o.Version,
	}

	if o.Env != "" {
		targetNamespace, err := kube.GetEnvironmentNamespace(jxClient, currentNamespace, o.Env)
		if err != nil {
			return err
		}
		query.TargetNamespace = targetNamespace
	}

	err = p.GetImageVulnerabilityTable(jxClient, client, &table, query)
	if err != nil {
		return fmt.Errorf("error getting vulnerability table for image %s: %v", query.ImageID, err)
	}

	table.Render()
	return nil
}
