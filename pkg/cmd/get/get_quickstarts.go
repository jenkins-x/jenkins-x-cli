package get

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/quickstarts"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

//GetQuickstartsOptions -  the command line options
type GetQuickstartsOptions struct {
	GetOptions
	GitHubOrganisations []string
	Filter              quickstarts.QuickstartFilter
	ShortFormat         bool
	IgnoreTeam          bool
}

var (
	getQuickstartsLong = templates.LongDesc(`
		Display the available quickstarts

`)

	getQuickstartsExample = templates.Examples(`
		# List all the available quickstarts
		jx get quickstarts
	`)
)

//NewCmdGetQuickstarts creates the command
func NewCmdGetQuickstarts(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetQuickstartsOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "quickstarts [flags]",
		Short:   "Lists the available quickstarts",
		Long:    getQuickstartsLong,
		Example: getQuickstartsExample,
		Aliases: []string{"quickstart", "qs"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringArrayVarP(&options.GitHubOrganisations, "organisations", "g", []string{}, "The GitHub organisations to query for quickstarts")
	cmd.Flags().StringArrayVarP(&options.Filter.Tags, "tag", "t", []string{}, "The tags on the quickstarts to filter")
	cmd.Flags().StringVarP(&options.Filter.Text, "filter", "f", "", "The text filter")
	cmd.Flags().StringVarP(&options.Filter.Owner, "owner", "", "", "The owner to filter on")
	cmd.Flags().StringVarP(&options.Filter.Language, "language", "l", "", "The language to filter on")
	cmd.Flags().StringVarP(&options.Filter.Framework, "framework", "", "", "The framework to filter on")
	cmd.Flags().BoolVarP(&options.Filter.AllowML, "machine-learning", "", false, "Allow machine-learning quickstarts in results")
	cmd.Flags().BoolVarP(&options.ShortFormat, "short", "s", false, "return minimal details")
	cmd.Flags().BoolVarP(&options.IgnoreTeam, "ignore-team", "", false, "ignores the quickstarts added to the Team Settings")

	return cmd
}

// Run implements this command
func (o *GetQuickstartsOptions) Run() error {
	model, err := o.LoadQuickStartsModel(o.GitHubOrganisations, o.IgnoreTeam)
	if err != nil {
		return fmt.Errorf("failed to load quickstarts: %s", err)
	}

	//output list of available quickstarts and exit
	filteredQuickstarts := model.Filter(&o.Filter)
	table := o.CreateTable()
	if o.ShortFormat {
		table.AddRow("NAME")
	} else {
		table.AddRow("NAME", "OWNER", "VERSION", "LANGUAGE", "URL")
	}

	for _, qs := range filteredQuickstarts {
		if o.ShortFormat {
			table.AddRow(qs.Name)
		} else {
			table.AddRow(qs.Name, qs.Owner, qs.Version, qs.Language, qs.DownloadZipURL)
		}
	}
	table.Render()
	return nil
}
