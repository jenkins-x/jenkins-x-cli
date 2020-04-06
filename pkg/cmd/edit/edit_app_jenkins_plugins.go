package edit

import (
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	editAppJenkinsPluginsLong = templates.LongDesc(`
		Edits a Jenkins App's plugins
`)

	editAppJenkinsPluginsExample = templates.Examples(`
		# Edits the plugins for a Jenkins App
		jx edit app jenkins plugins
	`)
)

// EditAppJenkinsPluginsOptions the options for the create spring command
type EditAppJenkinsPluginsOptions struct {
	EditOptions

	Name    string
	Enabled string

	IssuesAuthConfigSvc auth.ConfigService
}

// NewCmdEditAppJenkinsPlugins creates a command object for the "create" command
func NewCmdEditAppJenkinsPlugins(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &EditAppJenkinsPluginsOptions{
		EditOptions: EditOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "app jenkins plugins",
		Short:   "Edits the Jenkins Plugins for a Jenkins App",
		Long:    editAppJenkinsPluginsLong,
		Example: editAppJenkinsPluginsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Enabled, optionEnabled, "e", "", "Enables or disables the addon")

	return cmd
}

// Run implements the command
func (o *EditAppJenkinsPluginsOptions) Run() error {
	data, err := jenkins.LoadUpdateCenterURL(jenkins.DefaultUpdateCenterURL)
	if err != nil {
		return errors.Wrapf(err, "failed to load URL %s", jenkins.DefaultUpdateCenterURL)
	}

	// TODO load from the GitOps values.yaml folder
	currentValues := []string{"jx-resources:1.0.0"}

	selection, err := data.PickPlugins(currentValues, o.GetIOFileHandles())
	if err != nil {
		return err
	}

	log.Logger().Infof("chosen selection:")
	for _, sel := range selection {
		log.Logger().Infof("    %s", util.ColorInfo(sel))
	}
	// TODO update the GitOps values.yaml folder
	return nil
}
