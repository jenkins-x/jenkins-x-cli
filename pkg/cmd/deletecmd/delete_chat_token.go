package deletecmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	deleteChatTokenLong = templates.LongDesc(`
		Deletes one or more API tokens for your chat server from your local settings
`)

	deleteChatTokenExample = templates.Examples(`
		# Deletes a chat user token
		jx delete chat token -n slack myusername
	`)
)

// DeleteChatTokenOptions the options for the create spring command
type DeleteChatTokenOptions struct {
	options.CreateOptions

	ServerFlags opts.ServerFlags
}

// NewCmdDeleteChatToken defines the command
func NewCmdDeleteChatToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteChatTokenOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Deletes one or more API tokens for a user on a chat server",
		Aliases: []string{"api-token"},
		Long:    deleteChatTokenLong,
		Example: deleteChatTokenExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.ServerFlags.AddGitServerFlags(cmd)
	return cmd
}

// Run implements the command
func (o *DeleteChatTokenOptions) Run() error {
	args := o.Args
	if len(args) == 0 {
		return fmt.Errorf("Missing chat server user name")
	}
	authConfigSvc, err := o.CreateChatAuthConfigService("")
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	server, err := o.FindChatServer(config, &o.ServerFlags)
	if err != nil {
		return err
	}
	for _, username := range args {
		err = server.DeleteUser(username)
		if err != nil {
			return err
		}
	}
	err = authConfigSvc.SaveConfig()
	if err != nil {
		return err
	}
	log.Logger().Infof("Deleted API tokens for users: %s for chat server server %s at %s from local settings",
		util.ColorInfo(strings.Join(args, ", ")), util.ColorInfo(server.Name), util.ColorInfo(server.URL))
	return nil
}
