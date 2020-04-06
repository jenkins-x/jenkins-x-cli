package get

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/users"
	"github.com/spf13/cobra"
)

// GetUserOptions containers the CLI options
type GetUserOptions struct {
	GetOptions

	Pending bool
}

var (
	getUserLong = templates.LongDesc(`
		Display the Users
`)

	getUserExample = templates.Examples(`
		# List the users
		jx get user
	`)
)

// NewCmdGetUser creates the new command for: jx get env
func NewCmdGetUser(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetUserOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "users",
		Short:   "Display the User or Users the current user is a member of",
		Aliases: []string{"user"},
		Long:    getUserLong,
		Example: getUserExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.Pending, "pending", "p", false, "Display only pending Users which are not yet provisioned yet")

	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetUserOptions) Run() error {
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	jxClient, ns, err := o.JXClientAndAdminNamespace()
	if err != nil {
		return err
	}
	users, names, err := users.GetUsers(jxClient, ns)
	if err != nil {
		return err
	}

	if len(names) == 0 {
		log.Logger().Info(`
There are no Users yet. Try create one via: jx create user
`)
		return nil
	}

	table := o.CreateTable()
	table.AddRow("LOGIN", "NAME", "EMAIL", "URL", "ROLES")
	for _, name := range names {
		user := users[name]
		if user != nil {
			spec := &user.Spec
			userKind := user.SubjectKind()
			roleNames, err := kube.GetUserRoles(kubeClient, jxClient, ns, userKind, name)
			if err != nil {
				log.Logger().Warnf("Failed to find User roles in namespace %s for User %s kind %s: %s", ns, name, userKind, err)
			}
			table.AddRow(name, spec.Name, spec.Email, spec.URL, strings.Join(roleNames, ", "))
		}
	}
	table.Render()
	return nil

}
