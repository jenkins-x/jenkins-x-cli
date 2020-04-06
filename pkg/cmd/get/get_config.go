package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// GetConfigOptions the command line options
type GetConfigOptions struct {
	GetOptions

	Dir string
}

var (
	getConfigLong = templates.LongDesc(`
		Display the project configuration

`)

	getConfigExample = templates.Examples(`
		# View the project configuration
		jx get config
	`)
)

// NewCmdGetConfig creates the command
func NewCmdGetConfig(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetConfigOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "config [flags]",
		Short:   "Display the project configuration",
		Long:    getConfigLong,
		Example: getConfigExample,
		Aliases: []string{"url"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addGetConfigFlags(cmd)
	return cmd
}

func (o *GetConfigOptions) addGetConfigFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", "", "The root project directory")
}

// Run implements this command
func (o *GetConfigOptions) Run() error {
	pc, _, err := config.LoadProjectConfig(o.Dir)
	if err != nil {
		return err
	}
	if pc.IsEmpty() {
		log.Logger().Info("No project configuration for this directory.")
		log.Logger().Infof("To edit the configuration use: %s", util.ColorInfo("jx edit config"))
		return nil
	}
	table := o.CreateTable()
	table.AddRow("SERVICE", "KIND", "URL", "NAME")

	t := pc.IssueTracker
	if t != nil {
		table.AddRow("Issue Tracker", t.Kind, t.URL, t.Project)
	}
	w := pc.Wiki
	if w != nil {
		table.AddRow("Wiki", w.Kind, w.URL, w.Space)
	}
	ch := pc.Chat
	if ch != nil {
		if ch.DeveloperChannel != "" {
			table.AddRow("Developer Chat", ch.Kind, ch.URL, ch.DeveloperChannel)
		}
		if ch.UserChannel != "" {
			table.AddRow("User Chat", ch.Kind, ch.URL, ch.UserChannel)
		}
	}
	table.Render()
	return nil
}
