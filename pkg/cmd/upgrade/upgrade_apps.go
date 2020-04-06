package upgrade

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/add"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/pkg/errors"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

var (
	upgradeAppsLong = templates.LongDesc(`
		Upgrades Apps to newer releases (an app is similar to an addon)
`)

	upgradeAppsExample = templates.Examples(`
		# Upgrade all apps 
		jx upgrade apps
 
        # Upgrade a specific app
        jx upgrade app cheese
	`)
)

const (
	optionVersion    = "version"
	optionHelmUpdate = "helm-update"
	optionSet        = "set"
	optionAlias      = "alias"
)

// UpgradeAppsOptions the options for the create spring command
type UpgradeAppsOptions struct {
	add.AddOptions

	GitOps bool
	DevEnv *jenkinsv1.Environment

	Repo        string
	Alias       string
	Username    string
	Password    string
	ReleaseName string

	HelmUpdate bool
	AskAll     bool
	AutoMerge  bool

	Version string
	All     bool

	Namespace string
	Set       []string

	// Used for testing
	CloneDir string
}

// NewCmdUpgradeApps defines the command
func NewCmdUpgradeApps(commonOpts *opts.CommonOptions) *cobra.Command {
	o := &UpgradeAppsOptions{
		AddOptions: add.AddOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "apps",
		Short:   "Upgrades any Apps to the latest release (an app is similar to an addon)",
		Aliases: []string{"app"},
		Long:    upgradeAppsLong,
		Example: upgradeAppsExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().BoolVarP(&o.BatchMode, opts.OptionBatchMode, "b", false, "Enable batch mode")
	cmd.Flags().StringVarP(&o.Version, "username", "", "",
		"The username for the repository")
	cmd.Flags().StringVarP(&o.Version, "password", "", "",
		"The password for the repository")
	cmd.Flags().StringVarP(&o.Repo, "repository", "", "",
		"The repository from which the app should be installed")
	cmd.Flags().StringVarP(&o.Alias, "alias", "", "", "An alias to use for the app [--gitops]")
	cmd.Flags().StringVarP(&o.Version, "version", "v", "",
		"The chart version to install [--gitops]")
	cmd.Flags().StringVarP(&o.Namespace, opts.OptionNamespace, "", "", "The Namespace to promote to [--no-gitops]")
	cmd.Flags().StringArrayVarP(&o.Set, "set", "s", []string{},
		"The Helm parameters to pass in while upgrading [--no-gitops]")
	cmd.Flags().BoolVarP(&o.HelmUpdate, optionHelmUpdate, "", true,
		"Should we run helm update first to ensure we use the latest version (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringVarP(&o.ReleaseName, opts.OptionRelease, "r", "",
		"The chart release name (by default the name of the app, available when NOT using GitOps for your dev environment)")
	cmd.Flags().BoolVarP(&o.AskAll, "ask-all", "", false, "Ask all configuration questions. "+
		"By default existing answers are reused automatically.")
	cmd.Flags().BoolVarP(&o.AutoMerge, "auto-merge", "", false, "Automatically merge GitOps pull requests that pass CI")
	return cmd
}

// Run implements the command
func (o *UpgradeAppsOptions) Run() error {
	o.GitOps, o.DevEnv = o.GetDevEnv()
	if o.Repo == "" {
		o.Repo = o.DevEnv.Spec.TeamSettings.AppsRepository
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return errors.Wrapf(err, "getting kubeClient")
	}
	jxClient, ns, err := o.JXClient()
	if err != nil {
		return errors.Wrapf(err, "getting jx client")
	}

	installOpts := apps.InstallOptions{
		IOFileHandles: o.GetIOFileHandles(),
		DevEnv:        o.DevEnv,
		Verbose:       o.Verbose,
		GitOps:        o.GitOps,
		BatchMode:     o.BatchMode,
		AutoMerge:     o.AutoMerge,
		SecretsScheme: "vault",

		Helmer:              o.Helm(),
		Namespace:           o.Namespace,
		KubeClient:          kubeClient,
		JxClient:            jxClient,
		InstallTimeout:      opts.DefaultInstallTimeout,
		EnvironmentCloneDir: o.CloneDir,
	}
	if o.Namespace != "" {
		installOpts.Namespace = o.Namespace
	} else {
		installOpts.Namespace = ns
	}

	if o.GitOps {
		msg := "Unable to specify --%s when using GitOps for your dev environment"
		if o.Namespace != "" {
			return util.InvalidOptionf(opts.OptionNamespace, o.ReleaseName, msg, opts.OptionNamespace)
		}
		if len(o.Set) > 0 {
			return util.InvalidOptionf(optionSet, o.ReleaseName, msg, optionSet)
		}
		if o.GetSecretsLocation() != secrets.VaultLocationKind {
			return fmt.Errorf("cannot install apps without a vault when using GitOps for your dev environment")
		}
		if !o.HelmUpdate {
			return util.InvalidOptionf(optionHelmUpdate, o.HelmUpdate, msg, optionHelmUpdate)
		}

		gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(o.DevEnv.Spec.Source.URL)
		if err != nil {
			return errors.Wrapf(err, "creating git provider for %s", o.DevEnv.Spec.Source.URL)
		}

		installOpts.GitProvider = gitProvider
		installOpts.Gitter = o.Git()
	}
	if !o.GitOps {
		msg := "Unable to specify --%s when NOT using GitOps for your dev environment"
		if o.Alias != "" {
			return util.InvalidOptionf(optionAlias, o.ReleaseName, msg, optionAlias)
		}
		if o.Version != "" {
			return util.InvalidOptionf(optionVersion, o.ReleaseName, msg, optionVersion)
		}
	}

	if o.GetSecretsLocation() == secrets.VaultLocationKind {
		teamName, _, err := o.TeamAndEnvironmentNames()
		if err != nil {
			return err
		}
		installOpts.TeamName = teamName
		client, err := o.SystemVaultClient("")
		if err != nil {
			return err
		}

		installOpts.VaultClient = client
	}

	app := ""
	if len(o.Args) > 1 {
		return o.Cmd.Help()
	} else if len(o.Args) == 1 {
		app = o.Args[0]
	}

	var version string
	if o.Version != "" {
		version = o.Version
	}

	return installOpts.UpgradeApp(app, version, o.Repo, o.Username, o.Password, o.ReleaseName, o.Alias, o.HelmUpdate, o.AskAll)

}
