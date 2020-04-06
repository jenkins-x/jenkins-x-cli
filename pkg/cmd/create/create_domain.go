package create

import (
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

var (
	createDomainLong = templates.LongDesc(`
		Create a Domain in a managed DNS service such as GCP 
`)
)

const (
	// Domain to create within managed DNS services
	Domain = "domain"
)

// DomainOptions the options for the create spring command
type DomainOptions struct {
	options.CreateOptions

	SkipMessages bool
}

// NewCmdCreateDomain creates a command object for the "create" command
func NewCmdCreateDomain(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DomainOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "domain",
		Short: "Create a domain in a managed DNS service provider",
		Long:  createDomainLong,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.AddCommand(NewCmdCreateDomainGKE(commonOpts))

	return cmd
}

// AddDomainOptionsArguments adds common Domain flags to the given cobra command
func AddDomainOptionsArguments(cmd *cobra.Command, options *DomainOptions) {
	cmd.Flags().StringVarP(&options.Domain, Domain, "d", "", "The Domain you wish to be managed")
}
