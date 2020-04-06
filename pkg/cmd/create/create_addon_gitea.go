package create

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/helm"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

const (
	optionChart   = "chart"
	optionRelease = "release"

	defaultGiteaReleaseName = "gitea"
	defaultGiteaVersion     = ""
)

var (
	create_addon_gitea_long = templates.LongDesc(`
		Creates the Gitea addon (hosted Git server)
`)

	create_addon_gitea_example = templates.Examples(`
		# Create the Gitea addon 
		jx create addon gitea
	`)
)

// CreateAddonGiteaOptions the options for the create spring command
type CreateAddonGiteaOptions struct {
	CreateAddonOptions

	Chart    string
	Username string
	Password string
	Email    string
	IsAdmin  bool
	NoUser   bool
	NoToken  bool
}

// NewCmdCreateAddonGitea creates a command object for the "create" command
func NewCmdCreateAddonGitea(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonGiteaOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "gitea",
		Short:   "Create a Gitea addon for hosting Git repositories",
		Aliases: []string{"env"},
		Long:    create_addon_gitea_long,
		Example: create_addon_gitea_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, "", defaultGiteaReleaseName, defaultGiteaVersion)

	cmd.Flags().StringVarP(&options.Username, "username", "u", "", "The name for the user to create in Gitea. Note that Gitea tends to reject 'admin'")
	cmd.Flags().StringVarP(&options.Password, "password", "p", "", "The password for the user to create in Gitea. Note that Gitea tends to reject passwords less than 6 characters")
	cmd.Flags().StringVarP(&options.Email, "email", "e", "", "The email address of the new user to create in Gitea")
	cmd.Flags().StringVarP(&options.Chart, optionChart, "c", kube.ChartGitea, "The name of the chart to use")
	cmd.Flags().BoolVarP(&options.IsAdmin, "admin", "", false, "Should the new user created be an admin of the Gitea server")
	cmd.Flags().BoolVarP(&options.NoUser, "no-user", "", false, "If true disable trying to create a new user in the Gitea server")
	cmd.Flags().BoolVarP(&options.NoToken, "no-token", "", false, "If true disable trying to create a new token in the Gitea server")
	return cmd
}

// Run implements the command
func (o *CreateAddonGiteaOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	if o.ReleaseName == "" {
		return util.MissingOption(optionRelease)
	}
	if o.Chart == "" {
		return util.MissingOption(optionChart)
	}

	err := o.EnsureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that helm is present")
	}
	setValues := strings.Split(o.SetValues, ",")
	helmOptions := helm.InstallChartOptions{
		Chart:       o.Chart,
		ReleaseName: o.ReleaseName,
		Version:     o.Version,
		Ns:          o.Namespace,
		SetValues:   setValues,
	}
	err = o.InstallChartWithOptions(helmOptions)
	if err != nil {
		return err
	}
	err = o.createGitServer()
	if err != nil {
		return err
	}
	if !o.NoUser {
		// now to add the Git server + a user
		if !o.BatchMode {
			if o.Username == "" {
				prompt := &survey.Input{
					Message: "Enter the user name to create in Gitea: ",
				}
				err = survey.AskOne(prompt, &o.Username, nil, surveyOpts)
				if err != nil {
					return err
				}
			}
			if o.Username != "" {
				if o.Password == "" {
					prompt := &survey.Password{
						Message: "Enter the password for the new user in Gitea: ",
					}
					err = survey.AskOne(prompt, &o.Password, nil, surveyOpts)
					if err != nil {
						return err
					}
				}
				if o.Password != "" {
					if o.Email == "" {
						prompt := &survey.Input{
							Message: "Enter the email address of the user to create in Gitea: ",
						}
						err = survey.AskOne(prompt, &o.Email, nil, surveyOpts)
						if err != nil {
							return err
						}
					}
				}
			}
		}
		if o.Username != "" && o.Password != "" && o.Email != "" {
			err = o.createGitUser()
			if err != nil {
				return err
			}
		}
	}
	if !o.NoUser && o.Username != "" && o.Password != "" {
		return o.createGitToken()
	}
	return nil
}

func (o *CreateAddonGiteaOptions) createGitServer() error {
	options := &CreateGitServerOptions{
		CreateOptions: o.CreateOptions,
	}
	options.Args = []string{"gitea"}
	return options.Run()
}

func (o *CreateAddonGiteaOptions) createGitUser() error {
	log.Logger().Infof("Generating user: %s with email: %s", util.ColorInfo(o.Username), util.ColorInfo(o.Email))
	options := &CreateGitUserOptions{
		CreateOptions: o.CreateOptions,
		Username:      o.Username,
		Password:      o.Password,
		Email:         o.Email,
		IsAdmin:       o.IsAdmin,
	}
	options.CommonOptions.Args = []string{}
	options.ServerFlags.ServerName = "gitea"
	return options.Run()
}

func (o *CreateAddonGiteaOptions) createGitToken() error {
	log.Logger().Infof("Generating token for user %s with email %s", util.ColorInfo(o.Username), util.ColorInfo(o.Email))
	options := &CreateGitTokenOptions{
		CreateOptions: o.CreateOptions,
		Username:      o.Username,
		Password:      o.Password,
	}
	options.CommonOptions.Args = []string{}
	options.ServerFlags.ServerName = "gitea"
	return options.Run()
}
