package deletecmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"strings"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	delete_jenkins_user_long = templates.LongDesc(`
		Deletes one or more Jenkins user tokens from your local settings
`)

	delete_jenkins_user_example = templates.Examples(`
		# Deletes the current Jenkins token
		jx delete jenkins user admin
	`)
)

// DeleteJenkinsTokenOptions the options for the create spring command
type DeleteJenkinsTokenOptions struct {
	options.CreateOptions

	JenkinsSelector opts.JenkinsSelectorOptions

	ServerFlags opts.ServerFlags
}

// NewCmdDeleteJenkinsToken defines the command
func NewCmdDeleteJenkinsToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteJenkinsTokenOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Deletes one or more Jenkins user API tokens",
		Aliases: []string{"user"},
		Long:    delete_jenkins_user_long,
		Example: delete_jenkins_user_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.ServerFlags.AddGitServerFlags(cmd)
	options.JenkinsSelector.AddFlags(cmd)
	return cmd
}

// Run implements the command
func (o *DeleteJenkinsTokenOptions) Run() error {
	args := o.Args
	if len(args) == 0 {
		return fmt.Errorf("Missing Jenkins user name")
	}
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	authConfigSvc, err := o.JenkinsAuthConfigService(ns, &o.JenkinsSelector)
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	var server *auth.AuthServer
	if o.ServerFlags.IsEmpty() {
		url, err := o.CustomJenkinsURL(&o.JenkinsSelector, kubeClient, ns)
		if err != nil {
			return err
		}
		server = config.GetOrCreateServer(url)
	} else {
		server, err = o.FindServer(config, &o.ServerFlags, "jenkins server", "Try installing one via: jx create team", false)
		if err != nil {
			return err
		}
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
