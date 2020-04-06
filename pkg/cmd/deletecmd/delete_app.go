package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/pkg/errors"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	deleteAppLong = templates.LongDesc(`
		Deletes one or more Apps (an app is similar to an addon)

`)

	deleteAppExample = templates.Examples(`
		# prompt for the available apps to delete
		jx delete apps 

		# delete a specific app 
		jx delete app jx-app-cheese
	`)
)

const (
	optionPurge      = "purge"
	defaultNamespace = "jx"
)

// DeleteAppOptions are the flags for this delete commands
type DeleteAppOptions struct {
	*opts.CommonOptions

	GitOps bool
	DevEnv *jenkinsv1.Environment

	ReleaseName string
	Namespace   string
	Purge       bool
	Alias       string
	AutoMerge   bool

	// Used for testing
	CloneDir string
}

// NewCmdDeleteApp creates a command object for this command
func NewCmdDeleteApp(commonOpts *opts.CommonOptions) *cobra.Command {
	o := &DeleteAppOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "app",
		Short:   "Deletes one or more apps from Jenkins X (an app is similar to an addon)",
		Long:    deleteAppLong,
		Example: deleteAppExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Cmd = cmd
			o.Args = args
			err := o.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&o.ReleaseName, opts.OptionRelease, "r", "",
		"The chart release name (available when NOT using GitOps for your dev environment)")
	cmd.Flags().BoolVarP(&o.Purge, optionPurge, "", true,
		"Should we run helm update first to ensure we use the latest version (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringVarP(&o.Namespace, opts.OptionNamespace, "n", defaultNamespace, "The Namespace to install into (available when NOT using GitOps for your dev environment)")
	cmd.Flags().StringVarP(&o.Alias, opts.OptionAlias, "", "",
		"An alias to use for the app (available when using GitOps for your dev environment)")
	cmd.Flags().BoolVarP(&o.AutoMerge, "auto-merge", "", false, "Automatically merge GitOps pull requests that pass CI")

	return cmd
}

// Run implements this command
func (o *DeleteAppOptions) Run() error {
	o.GitOps, o.DevEnv = o.GetDevEnv()

	installOptions := apps.InstallOptions{
		IOFileHandles:       o.GetIOFileHandles(),
		DevEnv:              o.DevEnv,
		Verbose:             o.Verbose,
		GitOps:              o.GitOps,
		BatchMode:           o.BatchMode,
		Helmer:              o.Helm(),
		AutoMerge:           o.AutoMerge,
		EnvironmentCloneDir: o.CloneDir,
	}

	if o.GitOps {
		msg := "Unable to specify --%s when using GitOps for your dev environment"
		if o.ReleaseName != "" {
			return util.InvalidOptionf(opts.OptionRelease, o.ReleaseName, msg, opts.OptionRelease)
		}
		if o.Namespace != "" && o.Namespace != "jx" {
			return util.InvalidOptionf(opts.OptionNamespace, o.Namespace, msg, opts.OptionNamespace)
		}
		gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(o.DevEnv.Spec.Source.URL)
		if err != nil {
			return errors.Wrapf(err, "creating git provider for %s", o.DevEnv.Spec.Source.URL)
		}
		installOptions.GitProvider = gitProvider
		installOptions.Gitter = o.Git()
	}
	if !o.GitOps {
		err := o.EnsureHelm()
		if err != nil {
			return errors.Wrap(err, "failed to ensure that helm is present")
		}

		if o.Alias != "" {
			return util.InvalidOptionf(opts.OptionAlias, o.Alias,
				"Unable to specify --%s when NOT using GitOps for your dev environment", opts.OptionAlias)
		}
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrapf(err, "getting jx client")
	}
	installOptions.JxClient = jxClient
	if o.Namespace == "" {
		o.Namespace = ns
	}
	installOptions.Namespace = o.Namespace

	args := o.Args
	if len(args) == 0 {
		return o.Cmd.Help()
	}
	if len(args) > 1 {
		return o.Cmd.Help()
	}

	app := args[0]

	return installOptions.DeleteApp(app, o.Alias, o.ReleaseName, o.Purge)
}
