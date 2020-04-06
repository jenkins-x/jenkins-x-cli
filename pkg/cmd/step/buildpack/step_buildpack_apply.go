package buildpack

import (
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

var (
	createJenkinsfileLong = templates.LongDesc(`
		Applies the build pack for a project to add any missing files like a Jenkinsfile
`)

	createJenkinsfileExample = templates.Examples(`
		# applies the current build pack for the current team adding any missing files like Jenkinsfile
		jx step buildpack apply

		# applies the 'maven' build pack to the current project
		jx step buildpack apply --pack maven

			`)
)

// StepBuildPackApplyOptions contains the command line flags
type StepBuildPackApplyOptions struct {
	step.StepOptions

	Dir                     string
	Jenkinsfile             string
	DraftPack               string
	DisableJenkinsfileCheck bool
}

// NewCmdStepBuildPackApply Creates a new Command object
func NewCmdStepBuildPackApply(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepBuildPackApplyOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "apply",
		Short:   "Applies the current teams build pack to the project to add any missing resources like a Jenkinsfile",
		Long:    createJenkinsfileLong,
		Example: createJenkinsfileExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The directory to query to find the projects .git directory")
	cmd.Flags().StringVarP(&options.Jenkinsfile, "jenkinsfile", "", "", "The name of the Jenkinsfile to use. If not specified then 'Jenkinsfile' will be used")
	cmd.Flags().StringVarP(&options.DraftPack, "pack", "", "", "The name of the pack to use")
	cmd.Flags().BoolVarP(&options.DisableJenkinsfileCheck, "no-jenkinsfile", "", false, "Disable defaulting a Jenkinsfile if its missing")
	return cmd
}

// Run implements this command
func (o *StepBuildPackApplyOptions) Run() error {
	var err error
	dir := o.Dir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	settings, err := o.CommonOptions.TeamSettings()
	if err != nil {
		return err
	}
	log.Logger().Infof("build pack is %s", settings.BuildPackURL)

	defaultJenkinsfile := filepath.Join(dir, jenkinsfile.Name)
	jenkinsfile := jenkinsfile.Name
	withRename := false
	if o.Jenkinsfile != "" {
		jenkinsfile = o.Jenkinsfile
		withRename = true
	}
	if !filepath.IsAbs(jenkinsfile) {
		jenkinsfile = filepath.Join(dir, jenkinsfile)
	}

	args := &opts.InvokeDraftPack{
		Dir:                     dir,
		CustomDraftPack:         o.DraftPack,
		Jenkinsfile:             jenkinsfile,
		DefaultJenkinsfile:      defaultJenkinsfile,
		WithRename:              withRename,
		InitialisedGit:          true,
		DisableJenkinsfileCheck: o.DisableJenkinsfileCheck,
	}
	_, err = o.InvokeDraftPack(args)
	if err != nil {
		return err
	}
	return nil
}
