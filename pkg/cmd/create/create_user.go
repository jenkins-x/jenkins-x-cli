package create

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/users"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	optionLogin                = "login"
	optionCreateServiceAccount = "create-service-account"
)

var (
	createUserLong = templates.LongDesc(`
		Creates a user
`)

	createUserExample = templates.Examples(`
		# Create a user
		jx create user -e "user@email.com" --login username --name username
	`)
)

// CreateUserOptions the options for the create spring command
type CreateUserOptions struct {
	options.CreateOptions
	UserSpec v1.UserDetails
}

// NewCmdCreateUser creates a command object for the "create" command
func NewCmdCreateUser(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateUserOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "user",
		Short:   "Create a new User which is then provisioned by the user controller",
		Aliases: []string{"env"},
		Long:    createUserLong,
		Example: createUserExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.UserSpec.Login, optionLogin, "l", "", "The user login name")
	cmd.Flags().StringVarP(&options.UserSpec.Name, "name", "n", "", "The textual full name of the user")
	cmd.Flags().StringVarP(&options.UserSpec.Email, "email", "e", "", "The users email address")
	cmd.Flags().BoolVarP(&options.UserSpec.ExternalUser, optionCreateServiceAccount, "s", false, "Enable ServiceAccount for this external user")

	return cmd
}

// Run implements the command
func (o *CreateUserOptions) Run() error {
	err := o.RegisterUserCRD()
	if err != nil {
		return err
	}
	err = o.RegisterEnvironmentRoleBindingCRD()
	if err != nil {
		return err
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	jxClient, devNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	ns, err := kube.GetAdminNamespace(kubeClient, devNs)
	if err != nil {
		return err
	}

	_, names, err := users.GetUsers(jxClient, ns)
	if err != nil {
		return err
	}

	spec := &o.UserSpec
	login := spec.Login
	if login == "" {
		args := o.Args
		if len(args) > 0 {
			login = args[0]
		}
	}
	if login == "" {
		return util.MissingOption(optionLogin)
	}

	if util.StringArrayIndex(names, login) >= 0 {
		return fmt.Errorf("The User %s already exists!", login)
	}

	name := spec.Name
	if name == "" {
		name = strings.Title(login)
	}
	user := users.CreateUser(ns, login, name, spec.Email)
	user.Spec.ExternalUser = spec.ExternalUser
	_, err = jxClient.JenkinsV1().Users(ns).Create(user)
	if err != nil {
		return fmt.Errorf("failed to create User %s: %s", login, err)
	}
	log.Logger().Infof("Created User: %s", util.ColorInfo(login))
	log.Logger().Infof("You can configure the roles for the user via: %s", util.ColorInfo(fmt.Sprintf("jx edit userrole %s", login)))
	return nil

}
