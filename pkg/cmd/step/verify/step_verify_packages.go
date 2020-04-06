package verify

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/cmd/upgrade"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/version"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	verifyPackagesLong = templates.LongDesc(`
		Verifies the versions of the required command line packages


` + helper.SeeAlsoText("jx create project"))

	verifyPackagesExample = templates.Examples(`
		Verifies the versions of the required command line packages

		# verify packages and fail if any are not valid:
		jx step verify packages

		# override the error if the 'jx' binary is out of range (e.g. for development)
        export JX_DISABLE_VERIFY_JX="true"
		jx step verify packages
	`)
)

// StepVerifyPackagesOptions contains the command line flags
type StepVerifyPackagesOptions struct {
	step.StepOptions

	Namespace string
	HelmTLS   bool
	Packages  []string
	Dir       string
}

// NewCmdStepVerifyPackages creates the `jx step verify pod` command
func NewCmdStepVerifyPackages(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyPackagesOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "packages",
		Aliases: []string{"package"},
		Short:   "Verifies the versions of the required command line packages",
		Long:    verifyPackagesLong,
		Example: verifyPackagesExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.HelmTLS, "helm-tls", "", false, "Whether to use TLS with helm")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to use to look for helm's tiller")
	cmd.Flags().StringArrayVarP(&options.Packages, "packages", "p", []string{"jx", "kubectl", "git", "helm", "kaniko"}, "The packages to verify")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to recursively look upwards for any 'jx-requirements.yml' file to determine the version stream")
	return cmd
}

// Run implements this command
func (o *StepVerifyPackagesOptions) Run() error {
	packages, table := o.GetPackageVersions(o.Namespace, o.HelmTLS)

	verifyMap := map[string]string{}
	for _, k := range o.Packages {
		verifyMap[k] = packages[k]
	}

	requirements, _, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load boot requirements")
	}
	vs := requirements.VersionStream
	u := vs.URL
	ref := vs.Ref
	log.Logger().Infof("Verifying the CLI packages using version stream URL: %s and git ref: %s\n", u, vs.Ref)

	resolver, err := o.CreateVersionResolver(u, ref)
	if err != nil {
		return errors.Wrapf(err, "failed to create version resolver")
	}

	// lets verify jx separately
	delete(packages, "jx")

	err = resolver.VerifyPackages(verifyMap)
	if err != nil {
		return err
	}
	err = o.verifyJXVersion(resolver)
	if err != nil {
		return err
	}

	log.Logger().Infof("CLI packages %s seem to be setup correctly", util.ColorInfo(strings.Join(o.Packages, ", ")))
	table.Render()

	return nil
}

func (o *StepVerifyPackagesOptions) verifyJXVersion(resolver *versionstream.VersionResolver) error {
	currentVersion, err := version.GetSemverVersion()
	if err != nil {
		return errors.Wrap(err, "getting current jx version")
	}
	newVersion, err := o.GetLatestJXVersion(resolver)
	if err != nil {
		return errors.Wrap(err, "getting latest jx version")
	}
	info := util.ColorInfo
	versionText := newVersion.String()

	if currentVersion.EQ(newVersion) {
		log.Logger().Infof("using version %s of %s", info(versionText), info("jx"))
		return nil
	}

	log.Logger().Info("\n")
	log.Logger().Warnf("jx version specified in the version stream %s is %s. You are using %s. We highly recommend you upgrade to it.", util.ColorInfo(resolver.VersionsDir), util.ColorInfo(newVersion.String()), util.ColorInfo(currentVersion.String()))
	if o.BatchMode {
		log.Logger().Warnf("To upgrade to this new version use: %s then re-run %s", info("jx upgrade cli"), info("jx boot"))
	} else if strings.Contains(currentVersion.String(), "-dev") {
		log.Logger().Warn("Skipping version upgrade since a dev version is used")
	} else {
		log.Logger().Info("\n")
		message := fmt.Sprintf("Would you like to upgrade to the %s version?", info("jx"))
		if answer, err := util.Confirm(message, true, "Please indicate if you would like to upgrade the binary version.", o.GetIOFileHandles()); err != nil {
			return err
		} else if answer {
			options := &upgrade.UpgradeCLIOptions{
				CreateOptions: options.CreateOptions{
					CommonOptions: o.CommonOptions,
				},
			}
			options.Version = versionText
			options.NoBrew = true
			err = options.Run()
			if err != nil {
				return err
			}
			log.Logger().Info("\n")
			log.Logger().Warnf("the version of %s has been updated to %s. Please re-run: %s", info("jx"), info(versionText), info("jx boot"))
			log.Logger().Info("\n")

			return fmt.Errorf("the version of jx has been updated. Please re-run: jx boot")
		}
	}
	return nil
}
