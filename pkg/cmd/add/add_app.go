package add

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"

	"github.com/jenkins-x/jx/pkg/apps"

	"github.com/jenkins-x/jx/pkg/io/secrets"

	"github.com/jenkins-x/jx/pkg/util"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// AddAppOptions the options for the create spring command
type AddAppOptions struct {
	AddOptions

	GitOps bool
	DevEnv *jenkinsv1.Environment

	Repo        string
	Username    string
	Password    string
	Alias       string
	Prefixes    []string
	Namespace   string
	Version     string
	ReleaseName string
	SetValues   []string
	ValuesFiles []string
	HelmUpdate  bool
	AutoMerge   bool

	// Used for testing
	CloneDir string
}

const (
	optionHelmUpdate = "helm-update"
	optionValues     = "values"
	optionSet        = "set"
	optionAlias      = "alias"
	optionNamespace  = "namespace"
)

var (
	add_app_long = templates.LongDesc(`
		Adds an App to Jenkins X (an app is similar to an addon),
`)
	add_app_example = templates.Examples(`
		# Add an app
		jx add app jx-app-jacoco

		# Add an app from a local path
		jx add app .`)
)

// NewCmdAddApp creates a command object for the "create" command
func NewCmdAddApp(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &AddAppOptions{
		AddOptions: AddOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "app",
		Short:   "Adds an App (an app is similar to an addon)",
		Long:    add_app_long,
		Example: add_app_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, kube.DefaultNamespace)
	return cmd
}

func (o *AddAppOptions) addFlags(cmd *cobra.Command, defaultNamespace string) {

	// Common flags

	cmd.Flags().StringVarP(&o.Version, "version", "v", "",
		"The chart version to install")
	cmd.Flags().StringVarP(&o.Repo, "repository", "", "",
		"The repository from which the app should be installed (default specified in your dev environment)")
	cmd.Flags().StringVarP(&o.Username, "username", "", "",
		"The username for the repository")
	cmd.Flags().StringVarP(&o.Password, "password", "", "",
		"The password for the repository")
	cmd.Flags().StringVarP(&o.Alias, optionAlias, "", "",
		"An alias to use for the app if you wish to install multiple instances of the same app")
	cmd.Flags().BoolVarP(&o.HelmUpdate, optionHelmUpdate, "", true,
		"Should we run helm update first to ensure we use the latest version (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringVarP(&o.Namespace, optionNamespace, "n", "", "The Namespace to install into (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringArrayVarP(&o.ValuesFiles, optionValues, "f", []string{}, "List of locations for values files, "+
		"can be local files or URLs (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringArrayVarP(&o.SetValues, optionSet, "s", []string{},
		"The chart set values (can specify multiple or separate values with commas: key1=val1,key2=val2) (available when NOT using GitOps for your dev environment)")
	cmd.Flags().BoolVarP(&o.AutoMerge, "auto-merge", "", false, "Automatically merge GitOps pull requests that pass CI")
}

// Run implements this command
func (o *AddAppOptions) Run() error {
	o.GitOps, o.DevEnv = o.GetDevEnv()
	if o.Repo == "" {
		o.Repo = o.DevEnv.Spec.TeamSettings.AppsRepository
	}
	if o.Repo == "" {
		o.Repo = kube.DefaultChartMuseumURL
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrapf(err, "getting jx client")
	}
	kubeClient, _, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrapf(err, "getting kubeClient")
	}
	if o.Namespace == "" {
		o.Namespace = ns
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

	if o.GitOps {
		msg := "unable to specify --%s when using GitOps for your dev environment"
		if !o.HelmUpdate {
			return util.InvalidOptionf(optionHelmUpdate, o.HelmUpdate, msg, optionHelmUpdate)
		}
		if o.Namespace != "" && o.Namespace != kube.DefaultNamespace {
			return util.InvalidOptionf(optionNamespace, o.Namespace, msg, optionNamespace)
		}
		if len(o.SetValues) > 0 {
			return util.InvalidOptionf(optionSet, o.SetValues, msg, optionSet)
		}
		if len(o.ValuesFiles) > 1 {
			return util.InvalidOptionf(optionValues, o.SetValues,
				"no more than one --%s can be specified when using GitOps for your dev environment", optionValues)
		}

		gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(o.DevEnv.Spec.Source.URL)
		if err != nil {
			return errors.Wrapf(err, "creating git provider for %s", o.DevEnv.Spec.Source.URL)
		}
		installOpts.GitProvider = gitProvider
		installOpts.Gitter = o.Git()
	}
	if !o.GitOps {
		if o.Alias != "" && o.ReleaseName == "" {
			bin, noTiller, helmTemplate, err := o.TeamHelmBin()
			if err != nil {
				return err
			}
			if bin != "helm" || noTiller || helmTemplate {
				o.ReleaseName = o.Alias
			} else {
				o.ReleaseName = o.Alias + "-" + o.Namespace
			}
		}

	}
	err = o.EnsureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that helm is present")
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

	args := o.Args
	if len(args) == 0 {
		return o.Cmd.Help()
	}
	if len(args) > 1 {
		return o.Cmd.Help()
	}

	if o.Repo == "" {
		return fmt.Errorf("must specify a repository")
	}

	var version string
	if o.Version != "" {
		version = o.Version
	}
	app := args[0]
	return installOpts.AddApp(app, version, o.Repo, o.Username, o.Password, o.ReleaseName, o.ValuesFiles, o.SetValues,
		o.Alias, o.HelmUpdate)
}

// TODO create test for GitOps
