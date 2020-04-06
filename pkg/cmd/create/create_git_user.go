package create

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	create_git_user_long = templates.LongDesc(`
		Creates a new user for a Git Server. Only supported for Gitea so far
`)

	create_git_user_example = templates.Examples(`
		# Creates a new user in the local Gitea server
		jx create git user -n local someUserName -p somepassword -e foo@bar.com
	`)
)

// CreateGitUserOptions the command line options for the command
type CreateGitUserOptions struct {
	options.CreateOptions

	ServerFlags opts.ServerFlags
	Username    string
	Password    string
	ApiToken    string
	Email       string
	IsAdmin     bool
}

// NewCmdCreateGitUser creates a command
func NewCmdCreateGitUser(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateGitUserOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "user [username]",
		Short:   "Adds a new user to the Git server",
		Long:    create_git_user_long,
		Example: create_git_user_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.ServerFlags.AddGitServerFlags(cmd)
	cmd.Flags().StringVarP(&options.ApiToken, "api-token", "t", "", "The API Token for the user")
	cmd.Flags().StringVarP(&options.Password, "password", "p", "", "The User password to try automatically create a new API Token")
	cmd.Flags().StringVarP(&options.Email, "email", "e", "", "The User email address")
	cmd.Flags().BoolVarP(&options.IsAdmin, "admin", "a", false, "Whether the user is an admin user")

	return cmd
}

// Run implements the command
func (o *CreateGitUserOptions) Run() error {
	args := o.Args
	if len(args) > 0 {
		o.Username = args[0]
	}
	if len(args) > 1 {
		o.ApiToken = args[1]
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

	// TODO add the API thingy...
	if o.Username == "" {
		return fmt.Errorf("No Username specified")
	}
	if o.Password == "" && o.ApiToken == "" {
		return fmt.Errorf("No password or APIToken specified")
	}

	client, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	deploymentName := "gitea-gitea"
	log.Logger().Infof("Waiting for pods to be ready for deployment %s", deploymentName)

	err = kube.WaitForDeploymentToBeReady(client, deploymentName, ns, 5*time.Minute)
	if err != nil {
		return err
	}

	pods, err := kube.GetDeploymentPods(client, deploymentName, ns)
	if pods == nil || len(pods) == 0 {
		return fmt.Errorf("No pod found for namespace %s with name %s", ns, deploymentName)
	}

	command := "/app/gitea/gitea admin create-user --admin --name " + o.Username + " --password " + o.Password
	if o.Email != "" {
		command += " --email " + o.Email
	}
	if o.IsAdmin {
		command += " --admin"
	}
	// default to using the first pods found if more than one exists for the deployment
	err = o.RunCommand("kubectl", "exec", "-t", pods[0].Name, "--", "/bin/sh", "-c", command)
	if err != nil {
		return nil
	}

	log.Logger().Infof("Created user %s API Token for Git server %s at %s",
		util.ColorInfo(o.Username), util.ColorInfo(server.Name), util.ColorInfo(server.URL))
	return nil
}
