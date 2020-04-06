package get

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/spf13/cobra"
)

// GetQuickstartLocationOptions containers the CLI options
type GetQuickstartLocationOptions struct {
	GetOptions
}

var (
	getQuickstartLocationLong = templates.LongDesc(`
		Display one or more Quickstart Locations for the current Team.

		For more documentation see: [https://jenkins-x.io/developing/create-quickstart/#customising-your-teams-quickstarts](https://jenkins-x.io/developing/create-quickstart/#customising-your-teams-quickstarts)

`)

	getQuickstartLocationExample = templates.Examples(`
		# List all the quickstart locations
		jx get quickstartlocations

		# List all the quickstart locations via an alias
		jx get qsloc

	`)
)

// NewCmdGetQuickstartLocation creates the new command for: jx get env
func NewCmdGetQuickstartLocation(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetQuickstartLocationOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     opts.QuickStartLocationCommandName,
		Short:   "Display one or more Quickstart Locations",
		Aliases: opts.QuickStartLocationCommandAliases,
		Long:    getQuickstartLocationLong,
		Example: getQuickstartLocationExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetQuickstartLocationOptions) Run() error {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	locations, err := kube.GetQuickstartLocations(jxClient, ns)
	if err != nil {
		return err
	}

	table := o.CreateTable()
	table.AddRow("GIT SERVER", "KIND", "OWNER", "INCLUDES", "EXCLUDES")

	for _, location := range locations {
		kind := location.GitKind
		if kind == "" {
			kind = gits.KindGitHub
		}
		table.AddRow(location.GitURL, kind, location.Owner, strings.Join(location.Includes, ", "), strings.Join(location.Excludes, ", "))
	}
	table.Render()
	return nil
}
