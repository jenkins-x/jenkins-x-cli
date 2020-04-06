package get

import (
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/spf13/cobra"
)

// GetTokenOptions the command line options
type GetTokenOptions struct {
	GetOptions

	Kind string
	Name string
}

// NewCmdGetToken creates the command
func NewCmdGetToken(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetTokenOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "token",
		Short:   "Display the tokens for different kinds of services",
		Aliases: []string{"api-token"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.AddCommand(NewCmdGetTokenAddon(commonOpts))
	return cmd
}

func (o *GetTokenOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Kind, "kind", "k", "", "Filters the services by the kind")
	cmd.Flags().StringVarP(&o.Name, "name", "n", "", "Filters the services by the name")
}

// Run implements this command
func (o *GetTokenOptions) Run() error {
	return o.Cmd.Help()
}

func (o *GetTokenOptions) displayUsersWithTokens(authConfigSvc auth.ConfigService) error {
	config := authConfigSvc.Config()

	filterKind := o.Kind
	filterName := o.Name

	table := o.CreateTable()
	table.AddRow("KIND", "NAME", "URL", "USERNAME", "TOKEN?")

	for _, s := range config.Servers {
		kind := s.Kind
		name := s.Name
		if (filterKind == "" || filterKind == kind) && (filterName == "" || filterName == name) {
			user := ""
			pwd := ""
			if len(s.Users) == 0 {
				table.AddRow(kind, name, kind, s.URL, user, pwd)
			} else {
				for _, u := range s.Users {
					user = u.Username
					pwd = ""
					if u.ApiToken != "" {
						pwd = "yes"
					}
				}
				table.AddRow(kind, name, s.URL, user, pwd)
			}
		}
	}
	table.Render()
	return nil
}
