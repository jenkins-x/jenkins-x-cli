package edit

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/users"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	optionLogin = "login"
)

var (
	editUserRoleLong = templates.LongDesc(`
		Edits the Roles associated with a User
`)

	editUserRoleExample = templates.Examples(`
		# Prompt the CLI to pick a User from the list then select which Roles to update for the user
		jx edit userrole


		# Update a user to a given set of roles
		jx edit userrole --l mylogin -r foo -r bar
"
	`)
)

// EditUserRoleOptions the options for the create spring command
type EditUserRoleOptions struct {
	EditOptions

	Login string
	Roles []string
}

// NewCmdEditUserRole creates a command object for the "create" command
func NewCmdEditUserRole(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &EditUserRoleOptions{
		EditOptions: EditOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "userroles",
		Short:   "Edits the roles associated with a User",
		Aliases: []string{"userrole"},
		Long:    editUserRoleLong,
		Example: editUserRoleExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Login, optionLogin, "l", "", "The user login name")
	cmd.Flags().StringArrayVarP(&options.Roles, "role", "r", []string{}, "The roles to set on a user")

	return cmd
}

// Run implements the command
func (o *EditUserRoleOptions) Run() error {
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

	jxClient, teamNs, adminNs, err := o.JXClientDevAndAdminNamespace()
	if err != nil {
		return err
	}

	users, names, err := users.GetUsers(jxClient, adminNs)
	if err != nil {
		return err
	}

	name := o.Login
	if name == "" {
		args := o.Args
		if len(args) > 0 {
			name = args[0]
		}
	}
	if name == "" {
		if o.BatchMode {
			return util.MissingOption(optionLogin)
		}
		name, err = util.PickName(names, "Pick the user to edit", "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		if name == "" {
			return util.MissingOption(optionLogin)
		}
	}
	user := users[name]
	if user == nil {
		return fmt.Errorf("Could not find user %s", name)
	}
	userKind := user.SubjectKind()

	roles, roleNames, err := kube.GetTeamRoles(kubeClient, teamNs)
	if err != nil {
		return err
	}

	if len(roleNames) == 0 {
		log.Logger().Warnf("No Team roles for team %s", teamNs)
		return nil
	}

	userRoles := o.Roles
	if !o.BatchMode && len(userRoles) == 0 {

		currentRoles, err := kube.GetUserRoles(kubeClient, jxClient, teamNs, userKind, name)
		if err != nil {
			return err
		}
		userRoles, err = util.PickNamesWithDefaults(roleNames, currentRoles, "Roles for user: "+name, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	rolesText := strings.Join(userRoles, ", ")
	log.Logger().Infof("updating user %s for roles %s", name, rolesText)

	err = kube.UpdateUserRoles(kubeClient, jxClient, teamNs, userKind, name, userRoles, roles)
	if err != nil {
		return errors.Wrapf(err, "Failed to update user roles for user %s kind %s and roles %s", name, userKind, rolesText)
	}
	log.Logger().Infof("Updated roles for user: %s kind: %s roles: %s", util.ColorInfo(name), util.ColorInfo(userKind), util.ColorInfo(rolesText))
	return nil

}
