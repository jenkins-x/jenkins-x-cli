package deletecmd

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	deleteTrackerServer_long = templates.LongDesc(`
		Deletes one or more issue tracker servers from your local settings
`)

	deleteTrackerServer_example = templates.Examples(`
		# Deletes an issue tracker server
		jx delete tracker server MyProvider
	`)
)

// DeleteTrackerServerOptions the options for the create spring command
type DeleteTrackerServerOptions struct {
	*opts.CommonOptions

	IgnoreMissingServer bool
}

// NewCmdDeleteTrackerServer defines the command
func NewCmdDeleteTrackerServer(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteTrackerServerOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "server",
		Short:   "Deletes one or more issue tracker server(s)",
		Long:    deleteTrackerServer_long,
		Example: deleteTrackerServer_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.IgnoreMissingServer, "ignore-missing", "i", false, "Silently ignore attempts to remove an issue tracker server name that does not exist")
	return cmd
}

// Run implements the command
func (o *DeleteTrackerServerOptions) Run() error {
	args := o.Args
	if len(args) == 0 {
		return fmt.Errorf("Missing issue tracker server name argument")
	}
	authConfigSvc, err := o.CreateIssueTrackerAuthConfigService("")
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	serverNames := config.GetServerNames()
	for _, arg := range args {
		idx := config.IndexOfServerName(arg)
		if idx < 0 {
			if o.IgnoreMissingServer {
				return nil
			}
			return util.InvalidArg(arg, serverNames)
		}
		config.Servers = append(config.Servers[0:idx], config.Servers[idx+1:]...)
	}
	err = authConfigSvc.SaveConfig()
	if err != nil {
		return err
	}
	log.Logger().Infof("Deleted issue tracker servers: %s from local settings", util.ColorInfo(strings.Join(args, ", ")))
	return nil
}
