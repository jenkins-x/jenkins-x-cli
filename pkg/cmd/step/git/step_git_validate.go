package git

import (
	"os"
	"os/user"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StepGitValidateOptions contains the command line flags
type StepGitValidateOptions struct {
	step.StepOptions
}

var (
	stepGitValidateLong = templates.LongDesc(`
		This pipeline step validates that the .gitconfig is correctly configured

`)

	stepGitValidateExample = templates.Examples(`
		# validates the user.name & user.email values are set in the .gitconfig
		jx step git validate
`)
)

// NewCmdStepGitValidate creates a command to validate gitconfig
func NewCmdStepGitValidate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGitValidateOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "validate",
		Short:   "Validates the .gitconfig is correctly configured",
		Long:    stepGitValidateLong,
		Example: stepGitValidateExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

// Run validates git config
func (o *StepGitValidateOptions) Run() error {
	// lets ignore errors which indicate no value set
	userName, _ := o.Git().Username("")
	userEmail, _ := o.Git().Email("")
	var err error
	if userName == "" {
		// check the OS first
		userName = os.Getenv("GIT_AUTHOR_NAME")
		if userName == "" {
			if !o.BatchMode {
				userName, err = util.PickValue("Please enter the name you wish to use with git: ", "", true, "", o.GetIOFileHandles())
				if err != nil {
					return err
				}
			}
		}
		if userName == "" {
			user, err := user.Current()
			if err == nil && user != nil {
				userName = user.Username
			}
		}
		if userName == "" {
			userName = util.DefaultGitUserName
		}
		err = o.Git().SetUsername("", userName)
		if err != nil {
			return err
		}
	}
	if userEmail == "" {
		// check the OS first
		userEmail = os.Getenv("GIT_AUTHOR_EMAIL")
		if userEmail == "" {
			if !o.BatchMode {
				userEmail, err = util.PickValue("Please enter the email address you wish to use with git: ", "", true, "", o.GetIOFileHandles())
				if err != nil {
					return err
				}
			}
		}
		if userEmail == "" {
			userEmail = util.DefaultGitUserEmail
		}
		err = o.Git().SetEmail("", userEmail)
		if err != nil {
			return err
		}
	}
	log.Logger().Infof("Git configured for user: %s and email %s", util.ColorInfo(userName), util.ColorInfo(userEmail))
	return nil

}
