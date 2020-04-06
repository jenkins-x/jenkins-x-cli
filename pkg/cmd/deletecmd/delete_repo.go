package deletecmd

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"gopkg.in/AlecAivazis/survey.v1"

	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	deleteRepoLong = templates.LongDesc(`
		Deletes one or more repositories.

		This command will require the delete repo role on your Persona Access Token. 

		Note that command will ask for confirmation before doing anything!
`)

	deleteRepoExample = templates.Examples(`
		# Selects the repositories to delete from the given GitHub organisation
		jx delete repo --github --org myname 

        # Selects all the repositories in organisation myname that contain 'foo'
        # you get a chance to select which ones not to delete
		jx delete repo --github --org myname --all --filter foo 
	`)
)

// DeleteRepoOptions the options for the create spring command
type DeleteRepoOptions struct {
	options.CreateOptions

	Organisation string
	Repositories []string
	GitHost      string
	GitHub       bool
	SelectAll    bool
	SelectFilter string
}

// NewCmdDeleteRepo creates a command object for the "delete repo" command
func NewCmdDeleteRepo(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteRepoOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "repo",
		Short:   "Deletes one or more Git repositories",
		Aliases: []string{"repository", "repos"},
		Long:    deleteRepoLong,
		Example: deleteRepoExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	//addDeleteFlags(cmd, &options.CreateOptions)

	cmd.Flags().StringVarP(&options.Organisation, "org", "o", "", "Specify the Git provider organisation that includes the repository to delete")
	cmd.Flags().StringArrayVarP(&options.Repositories, "name", "n", []string{}, "Specify the Git repository names to delete")
	cmd.Flags().StringVarP(&options.GitHost, "git-host", "g", "", "The Git server host if not using GitHub")
	cmd.Flags().BoolVarP(&options.GitHub, "github", "", false, "If you wish to pick the repositories from GitHub to import")
	cmd.Flags().BoolVarP(&options.SelectAll, "all", "a", false, "If selecting projects to delete from a Git provider this defaults to selecting them all")
	cmd.Flags().StringVarP(&options.SelectFilter, "filter", "f", "", "If selecting projects to delete from a Git provider this filters the list of repositories")
	return cmd
}

// Run implements the command
func (o *DeleteRepoOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	var server *auth.AuthServer
	config := authConfigSvc.Config()
	if o.GitHub {
		server = config.GetOrCreateServer(gits.GitHubURL)
	} else {
		if o.GitHost != "" {
			server = config.GetOrCreateServer(o.GitHost)
		} else {
			server, err = config.PickServer("Pick the Git server to search for repositories", o.BatchMode, o.GetIOFileHandles())
			if err != nil {
				return err
			}
		}
	}
	if server == nil {
		return fmt.Errorf("No Git server provided")
	}
	userAuth, err := config.PickServerUserAuth(server, "Git user name", o.BatchMode, "", o.GetIOFileHandles())
	if err != nil {
		return err
	}
	provider, err := gits.CreateProvider(server, userAuth, o.Git())
	if err != nil {
		return err
	}
	username := userAuth.Username
	org := o.Organisation
	if org == "" {
		org, err = gits.PickOrganisation(provider, username, o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	if org == "" {
		org = username
	}

	names := o.Repositories
	if len(names) == 0 {
		repos, err := gits.PickRepositories(provider, org, "Which repositories do you want to delete:", o.SelectAll, o.SelectFilter, o.GetIOFileHandles())
		if err != nil {
			return err
		}

		for _, r := range repos {
			names = append(names, r.Name)
		}
	}

	if !o.BatchMode {
		log.Logger().Warnf("You are about to delete these repositories '%s' on the Git provider. This operation CANNOT be undone!",
			strings.Join(names, ","))

		flag := false
		prompt := &survey.Confirm{
			Message: "Are you sure you want to delete these all these repositories?",
			Default: false,
		}
		err = survey.AskOne(prompt, &flag, nil, surveyOpts)
		if err != nil {
			return err
		}
		if !flag {
			return nil
		}
	}

	owner := org
	if owner == "" {
		owner = username
	}
	info := util.ColorInfo
	for _, name := range names {
		err = provider.DeleteRepository(owner, name)
		if err != nil {
			log.Logger().Warnf("Ensure Git Token has delete repo permissions or manually delete, for GitHub check https://github.com/settings/tokens")
			log.Logger().Warnf("%s", err)
		} else {
			log.Logger().Infof("Deleted repository %s/%s", info(owner), info(name))
		}
	}
	return nil
}
