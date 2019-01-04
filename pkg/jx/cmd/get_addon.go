package cmd

import (
	"github.com/jenkins-x/jx/pkg/addon"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// GetAddonOptions the command line options
type GetAddonOptions struct {
	GetOptions
}

var (
	get_addon_long = templates.LongDesc(`
		Display the available addons

`)

	get_addon_example = templates.Examples(`
		# List all the possible addons
		jx get addon
	`)
)

// NewCmdGetAddon creates the command
func NewCmdGetAddon(commonOpts *CommonOptions) *cobra.Command {
	options := &GetAddonOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "addons [flags]",
		Short:   "Lists the addons",
		Long:    get_addon_long,
		Example: get_addon_example,
		Aliases: []string{"addon", "add-on"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			CheckErr(err)
		},
	}

	return cmd
}

// Run implements this command
func (o *GetAddonOptions) Run() error {

	addonConfig, err := addon.LoadAddonsConfig()
	if err != nil {
		return err
	}
	addonEnabled := map[string]bool{}
	for _, addon := range addonConfig.Addons {
		if addon.Enabled {
			addonEnabled[addon.Name] = true
		}
	}
	_, ns, err := o.KubeClient()
	if err != nil {
		return err
	}
	statusMap, err := o.Helm().StatusReleases(ns)
	if err != nil {
		log.Warnf("Failed to find Helm installs: %s\n", err)
	}

	charts := kube.AddonCharts

	table := o.CreateTable()
	table.AddRow("NAME", "CHART", "ENABLED", "STATUS", "VERSION")

	keys := util.SortedMapKeys(charts)
	for _, k := range keys {
		chart := charts[k]
		status := statusMap[k].Status
		version := statusMap[k].Version
		enableText := ""
		if addonEnabled[k] {
			enableText = "yes"
		}
		table.AddRow(k, chart, enableText, status, version)
	}

	table.Render()
	return nil
}
