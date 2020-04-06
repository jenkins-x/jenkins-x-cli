package upgrade

import (
	"github.com/blang/semver"
	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/version"
	"github.com/spf13/cobra"
)

var (
	upgradeCLILong = templates.LongDesc(`
		Upgrades the Jenkins X command line tools if there is a different version stored in the version stream.

		The exact version used for the version stream is stored in the Team Settings on the 'dev' Environment CRD.

		For more information on Version Streams see: [https://jenkins-x.io/docs/concepts/version-stream/](https://jenkins-x.io/docs/concepts/version-stream/)
`)

	upgradeCLIExample = templates.Examples(`
		# Upgrades the Jenkins X CLI tools 
		jx upgrade cli
	`)
)

// UpgradeCLIOptions the options for the create spring command
type UpgradeCLIOptions struct {
	options.CreateOptions

	Version string
}

// NewCmdUpgradeCLI defines the command
func NewCmdUpgradeCLI(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeCLIOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "cli",
		Short:   "Upgrades the jx command line application if there is a new version available in the version stream",
		Aliases: []string{"client", "clients"},
		Long:    upgradeCLILong,
		Example: upgradeCLIExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Version, "version", "v", "", "The specific version to upgrade to (requires --no-brew on macOS)")
	cmd.Flags().BoolVar(&options.CommonOptions.NoBrew, opts.OptionNoBrew, false, "Disables brew package manager on MacOS when installing binary dependencies")
	return cmd
}

// Run implements the command
func (o *UpgradeCLIOptions) Run() error {
	// upgrading to a specific version is not yet supported in brew so lets disable it for upgrades
	o.NoBrew = true
	candidateInstallVersion, err := o.candidateInstallVersion()
	if err != nil {
		return err
	}

	currentVersion, err := version.GetSemverVersion()
	if err != nil {
		return errors.Wrap(err, "failed to determine version of currently install jx release")
	}

	log.Logger().Debugf("Current version of jx: %s", util.ColorInfo(currentVersion))

	if o.needsUpgrade(currentVersion, candidateInstallVersion) {
		shouldUpgrade, err := o.ShouldUpdate(candidateInstallVersion)
		if err != nil {
			return errors.Wrap(err, "failed to determine if we should upgrade")
		}
		if shouldUpgrade {
			return o.InstallJx(true, candidateInstallVersion.String())
		}
	}

	return nil
}

func (o *UpgradeCLIOptions) candidateInstallVersion() (semver.Version, error) {
	if o.Version == "" {
		versionResolver, err := o.GetVersionResolver()
		if err != nil {
			return semver.Version{}, err
		}
		latestVersion, err := o.GetLatestJXVersion(versionResolver)
		if err != nil {
			return semver.Version{}, errors.Wrap(err, "failed to determine version of latest jx release")
		}
		return latestVersion, nil
	}

	requestedVersion, err := semver.New(o.Version)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "invalid version requested: %s", o.Version)
	}
	return *requestedVersion, nil
}

func (o *UpgradeCLIOptions) needsUpgrade(currentVersion semver.Version, latestVersion semver.Version) bool {
	if latestVersion.EQ(currentVersion) {
		log.Logger().Infof("You are already on the latest version of jx %s", util.ColorInfo(currentVersion.String()))
		return false
	}
	return true
}

// ShouldUpdate checks if CLI version should be updated
func (o *UpgradeCLIOptions) ShouldUpdate(newVersion semver.Version) (bool, error) {
	log.Logger().Debugf("Checking if should upgrade %s", newVersion)
	currentVersion, err := version.GetSemverVersion()
	if err != nil {
		return false, err
	}

	if newVersion.GT(currentVersion) {
		// Do not ask to update if we are using a dev build...
		for _, x := range currentVersion.Pre {
			if x.VersionStr == "dev" {
				log.Logger().Debugf("Ignoring possible update as it appears you are using a dev build - %s", currentVersion)
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
}
