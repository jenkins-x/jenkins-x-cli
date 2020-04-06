package create

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

var (
	createDomainGKELong = templates.LongDesc(`
		Create a Domain in GCP so it can be used with GKE  
`)

	createDomainGKEExample = templates.Examples(`
		# Create the Domain in Google Cloud
		jx create domain gke -d foo.bar.io
	`)
)

// ProjectID the Google Cloud Project ID
const ProjectID = "project-id"

// DomainGKEOptions the options for the create spring command
type DomainGKEOptions struct {
	DomainOptions

	ProjectID string
}

// NewCmdCreateDomainGKE creates a command object for the "create" command
func NewCmdCreateDomainGKE(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DomainGKEOptions{
		DomainOptions: DomainOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "gke",
		Short:   "Create a managed domain for GKE",
		Long:    createDomainGKELong,
		Example: createDomainGKEExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.ProjectID, ProjectID, "p", "", "Override the current Project ID")

	AddDomainOptionsArguments(cmd, &options.DomainOptions)

	return cmd
}

// Run implements the command
func (o *DomainGKEOptions) Run() error {

	if o.Domain == "" {
		return util.MissingOption(Domain)
	}

	var err error
	if o.ProjectID == "" {
		if o.BatchMode {
			return errors.Wrapf(err, "please provide a Google Project ID using --%s  when running in batch mode", ProjectID)
		}
		o.ProjectID, err = o.GetGoogleProjectID("")
		if err != nil {
			return errors.Wrap(err, "while trying to get Google Project ID")
		}
	}

	// Checking whether dns api is enabled
	err = o.GCloud().EnableAPIs(o.ProjectID, "dns")
	if err != nil {
		return errors.Wrap(err, "enabling the DNS API")
	}

	// Create domain if it doesn't exist and return name servers list
	_, nameServers, err := o.GCloud().CreateDNSZone(o.ProjectID, o.Domain)
	if err != nil {
		return errors.Wrap(err, "while trying to create the domain zone")
	}

	if !o.BatchMode {
		info := util.ColorInfo
		log.Logger().Info(info("Please update your existing DNS managed servers to use the nameservers below"))
	}

	log.Logger().Infof("%s", strings.Join(nameServers[:], "\n"))

	return nil
}
