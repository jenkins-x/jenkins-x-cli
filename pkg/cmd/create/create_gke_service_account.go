package create

import (
	"errors"
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

type CreateGkeServiceAccountFlags struct {
	Name      string
	Project   string
	SkipLogin bool
}

type CreateGkeServiceAccountOptions struct {
	options.CreateOptions
	Flags CreateGkeServiceAccountFlags
}

var (
	createGkeServiceAccountExample = templates.Examples(`
		jx create gke-service-account

		# to specify the options via flags
		jx create gke-service-account --name my-service-account --project my-gke-project

`)
)

// NewCmdCreateGkeServiceAccount creates a command object for the "create" command
func NewCmdCreateGkeServiceAccount(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateGkeServiceAccountOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "gke-service-account",
		Short:   "Creates a GKE service account",
		Example: createGkeServiceAccountExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, true)

	return cmd
}

func (options *CreateGkeServiceAccountOptions) addFlags(cmd *cobra.Command, addSharedFlags bool) {
	cmd.Flags().StringVarP(&options.Flags.Name, "name", "n", "", "The name of the service account to create")
	cmd.Flags().StringVarP(&options.Flags.Project, "project", "p", "", "The GCP project to create the service account in")
	if addSharedFlags {
		cmd.Flags().BoolVarP(&options.Flags.SkipLogin, "skip-login", "", false, "Skip Google auth if already logged in via gcloud auth")
	}
}

// Run implements this command
func (o *CreateGkeServiceAccountOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	if !o.Flags.SkipLogin {
		err := o.RunCommandVerbose("gcloud", "auth", "login", "--brief")
		if err != nil {
			return err
		}
	}

	if o.Flags.Name == "" {
		prompt := &survey.Input{
			Message: "Name for the service account",
		}

		err := survey.AskOne(prompt, &o.Flags.Name, func(val interface{}) error {
			// since we are validating an Input, the assertion will always succeed
			if str, ok := val.(string); !ok || len(str) < 6 {
				return errors.New("Service Account name must be longer than 5 characters")
			}
			return nil
		}, surveyOpts)

		if err != nil {
			return err
		}

	}

	if o.Flags.Project == "" {
		projectId, err := o.getGoogleProjectId()
		if err != nil {
			return err
		}
		o.Flags.Project = projectId
	}

	path, err := o.GCloud().GetOrCreateServiceAccount(o.Flags.Name, o.Flags.Project, util.HomeDir(), gke.RequiredServiceAccountRoles)
	if err != nil {
		return err
	}

	log.Logger().Infof("Created service account key %s", util.ColorInfo(path))

	return nil
}

// asks to chose from existing projects or optionally creates one if none exist
func (o *CreateGkeServiceAccountOptions) getGoogleProjectId() (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	existingProjects, err := gke.GetGoogleProjects()
	if err != nil {
		return "", err
	}

	var projectId string
	if len(existingProjects) == 0 {
		confirm := &survey.Confirm{
			Message: fmt.Sprintf("No existing Google Projects exist, create one now?"),
			Default: true,
		}
		flag := true
		err = survey.AskOne(confirm, &flag, nil, surveyOpts)
		if err != nil {
			return "", err
		}
		if !flag {
			return "", errors.New("no google project to create cluster in, please manual create one and rerun this wizard")
		}

		if flag {
			return "", errors.New("auto creating projects not yet implemented, please manually create one and rerun the wizard")
		}
	} else if len(existingProjects) == 1 {
		projectId = existingProjects[0]
		log.Logger().Infof("Using the only Google Cloud Project %s to create the cluster", util.ColorInfo(projectId))
	} else {
		prompts := &survey.Select{
			Message: "Google Cloud Project:",
			Options: existingProjects,
			Help:    "Select a Google Project to create the cluster in",
		}

		err := survey.AskOne(prompts, &projectId, nil, surveyOpts)
		if err != nil {
			return "", err
		}
	}

	if projectId == "" {
		return "", errors.New("no Google Cloud Project to create cluster in, please manual create one and rerun this wizard")
	}

	return projectId, nil
}
