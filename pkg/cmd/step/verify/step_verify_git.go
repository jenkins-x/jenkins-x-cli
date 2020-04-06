package verify

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/auth"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const (
	optionGitHubAppOwner = "github-app-owner"
)

// StepVerifyGitOptions contains the command line flags
type StepVerifyGitOptions struct {
	step.StepOptions

	GitHubAppOwner string
}

// NewCmdStepVerifyGit creates the `jx step verify pod` command
func NewCmdStepVerifyGit(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyGitOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use: "git",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.GitHubAppOwner, optionGitHubAppOwner, "g", "", "The owner (organisation or user name) if using GitHub App based tokens")
	return cmd
}

// Run implements this command
func (o *StepVerifyGitOptions) Run() error {
	log.Logger().Infof("Verifying the git config\n")

	gha, err := o.IsGitHubAppMode()
	if err != nil {
		return errors.Wrap(err, "while checking if github app mode is enabled")
	}

	if gha && o.GitHubAppOwner == "" {
		log.Logger().Infof("this command does nothing if using github app mode and no %s option specified", optionGitHubAppOwner)
		return nil
	}

	var authSvc auth.ConfigService
	if gha {
		authSvc, err = o.GitAuthConfigServiceGitHubAppMode("github")
		if err != nil {
			return errors.Wrap(err, "when creating auth config service using GitAuthConfigServiceGitHubAppMode")
		}
	} else {
		authSvc, err = o.GitAuthConfigService()
		if err != nil {
			return errors.Wrap(err, "when creating auth config service using GitAuthConfigService")
		}
	}

	config := authSvc.Config()
	if config == nil {
		return fmt.Errorf("git auth config is empty")
	}

	servers := config.Servers
	if len(servers) == 0 {
		return fmt.Errorf("no git servers found in the auth configuration")
	}
	info := util.ColorInfo
	pipeUserValid := false
	for _, server := range servers {
		for _, userAuth := range server.Users {
			log.Logger().Infof("Verifying username %s at git server %s at %s\n",
				info(userAuth.Username), info(server.Name), info(server.URL))

			provider, err := gits.CreateProvider(server, userAuth, o.Git())
			if err != nil {
				return errors.Wrapf(err, "creating git provider for %s at git server %s",
					userAuth.Username, server.URL)
			}

			if strings.HasSuffix(provider.CurrentUsername(), "[bot]") {
				pipeUserValid = true
				continue
			}

			orgs, err := provider.ListOrganisations()
			if err != nil {
				return errors.Wrapf(err, "listing the organisations for %s at git server %s",
					userAuth.Username, server.URL)
			}
			orgNames := []string{}
			for _, org := range orgs {
				orgNames = append(orgNames, org.Login)
			}
			sort.Strings(orgNames)
			log.Logger().Infof("Found %d organisations in git server %s: %s\n",
				len(orgs), info(server.URL), info(strings.Join(orgNames, ", ")))
			if config.PipeLineServer == server.URL && config.PipeLineUsername == userAuth.Username {
				pipeUserValid = true
			}
		}
	}

	if pipeUserValid {
		log.Logger().Infof("Validated pipeline user %s on git server %s", util.ColorInfo(config.PipeLineUsername), util.ColorInfo(config.PipeLineServer))
	} else {
		return errors.Errorf("pipeline user %s on git server %s not valid", util.ColorError(config.PipeLineUsername), util.ColorError(config.PipeLineServer))
	}

	log.Logger().Infof("Git tokens seem to be setup correctly\n")
	return nil
}
