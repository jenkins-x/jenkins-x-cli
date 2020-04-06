package release

import (
	"os"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

var (
	updateReleaseGithubLong = templates.LongDesc(`
		updates the status of a release to be either a prerelease or a release
	`)

	updateReleaseGitHubExample = templates.Examples(`
        jx step update release github -o jenkins-x -r jx -v 1.2.3 -p=false
	`)
)

// StepUpdateReleaseGitHubOptions contains the command line flags
type StepUpdateReleaseGitHubOptions struct {
	StepUpdateReleaseOptions
	PreRelease bool

	State StepUpdateReleaseStatusState
}

// StepUpdateReleaseStatusState contains the state information
type StepUpdateReleaseStatusState struct {
	GitInfo     *gits.GitRepository
	GitProvider gits.GitProvider
	Release     *v1.Release
}

// NewCmdStepUpdateReleaseGitHub Creates a new Command object
func NewCmdStepUpdateReleaseGitHub(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepUpdateReleaseGitHubOptions{
		StepUpdateReleaseOptions: StepUpdateReleaseOptions{
			StepUpdateOptions: step.StepUpdateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "github",
		Short:   "Updates a release to either a prerelease or a release",
		Long:    updateReleaseGithubLong,
		Example: updateReleaseGitHubExample,
		Aliases: []string{"gh"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	AddStepUpdateReleaseFlags(cmd, &options.StepUpdateReleaseOptions)
	cmd.Flags().BoolVarP(&options.PreRelease, "prerelease", "p", false, "The release state of that version release of the repository")
	return cmd
}

// ValidateGitHubOptions validates the common options for brew pr steps
func (o *StepUpdateReleaseGitHubOptions) ValidateGitHubOptions() error {
	if err := o.ValidateOptions(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Run implements this command
func (o *StepUpdateReleaseGitHubOptions) Run() error {
	if err := o.ValidateGitHubOptions(); err != nil {
		return errors.WithStack(err)
	}

	releaseInfo := &gits.GitRelease{
		PreRelease: o.PreRelease,
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	gitDir, gitConfDir, err := o.Git().FindGitConfigDir(dir)
	if err != nil {
		return err
	}
	if gitDir == "" || gitConfDir == "" {
		log.Logger().Warnf("No git directory could be found from dir %s", dir)
		return nil
	}
	gitURL, err := o.Git().DiscoverUpstreamGitURL(gitConfDir)
	if err != nil {
		return err
	}
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return err
	}
	o.State.GitInfo = gitInfo

	gitKind, err := o.GitServerKind(gitInfo)
	if err != nil {
		return err
	}
	ghOwner, err := o.GetGitHubAppOwner(gitInfo)
	if err != nil {
		return err
	}

	gitProvider, err := o.State.GitInfo.CreateProvider(o.InCluster(), authConfigSvc, gitKind, ghOwner, o.Git(), false, o.GetIOFileHandles())
	if err != nil {
		return errors.Wrap(err, "Could not create GitProvider, unable to update the release state %s")
	}
	o.State.GitProvider = gitProvider
	err = gitProvider.UpdateReleaseStatus(o.Owner, o.Repository, o.Version, releaseInfo)
	if err != nil {
		return err
	}
	return nil
}
