package get

import (
	"github.com/jenkins-x/jx/pkg/addon"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
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
func NewCmdGetAddon(commonOpts *opts.CommonOptions) *cobra.Command {
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
			helper.CheckErr(err)
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
	_, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	releases, sortedKeys, err := o.Helm().ListReleases(ns)
	if err != nil {
		log.Logger().Warnf("Failed to find Helm installs: %s", err)
	}

	table := o.CreateTable()
	table.AddRow("NAME", "CHART", "ENABLED", "STATUS", "VERSION")
	for _, k := range sortedKeys {
		release := releases[k]
		if addonName, ok := kube.AddonCharts[release.ReleaseName]; ok {
			enableText := ""
			if addonEnabled[release.ReleaseName] {
				enableText = "yes"
			}
			table.AddRow(release.ReleaseName, addonName, enableText, release.Status, release.ChartVersion)
		}
	}
	table.Render()
	return nil
}
