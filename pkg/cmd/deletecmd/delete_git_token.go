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
	delete_git_token_long = templates.LongDesc(`
		Deletes one or more git tokens from your local settings
`)

	delete_git_token_example = templates.Examples(`
		# Deletes a Git user token
		jx delete git token -n local myusername
	`)
)

// DeleteGitTokenOptions the options for the create spring command
type DeleteGitTokenOptions struct {
	options.CreateOptions

	ServerFlags opts.ServerFlags
}

// NewCmdDeleteGitToken defines the command
func NewCmdDeleteGitToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteGitTokenOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Deletes one or more API tokens for a user on a Git server",
		Aliases: []string{"api-token"},
		Long:    delete_git_token_long,
		Example: delete_git_token_example,
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
func (o *DeleteGitTokenOptions) Run() error {
	args := o.Args
	if len(args) == 0 {
		return fmt.Errorf("Missing Git user name")
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	server, err := o.FindGitServer(config, &o.ServerFlags)
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
	log.Logger().Infof("Deleted API tokens for users: %s for Git server %s at %s from local settings",
		util.ColorInfo(strings.Join(args, ", ")), util.ColorInfo(server.Name), util.ColorInfo(server.URL))
	return nil
}
