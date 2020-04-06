package pipeline

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/step/git/credentials"
	"github.com/jenkins-x/jx/pkg/tekton/metapipeline"
	"github.com/pkg/errors"
	"github.com/spf13/viper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"

	jxclient "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
)

const (
	useMetaPipelineOptionName   = "use-meta-pipeline"
	metaPipelineImageOptionName = "meta-pipeline-image"
	portOptionName              = "port"
	bindOptionName              = "bind"
)

// PipelineRunnerOptions holds the command line arguments
type PipelineRunnerOptions struct {
	*opts.CommonOptions
	BindAddress          string
	Path                 string
	Port                 int
	NoGitCredentialsInit bool
	UseMetaPipeline      bool
	MetaPipelineImage    string
	SemanticRelease      bool
}

var (
	controllerPipelineRunnersLong = templates.LongDesc(`Runs the service to generate Tekton resources from source code webhooks such as from Prow`)

	controllerPipelineRunnersExample = templates.Examples(`
			# run the pipeline runner controller
			jx controller pipelinerunner
		`)
)

// NewCmdControllerPipelineRunner creates the command
func NewCmdControllerPipelineRunner(commonOpts *opts.CommonOptions) *cobra.Command {
	options := PipelineRunnerOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "pipelinerunner",
		Short:   "Runs the service to generate Tekton PipelineRun resources from source code webhooks such as from Prow",
		Long:    controllerPipelineRunnersLong,
		Example: controllerPipelineRunnersExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().IntVar(&options.Port, portOptionName, 8080, "The TCP port to listen on.")
	cmd.Flags().StringVar(&options.BindAddress, bindOptionName, "0.0.0.0", "The interface address to bind to (by default, will listen on all interfaces/addresses).")
	cmd.Flags().StringVar(&options.Path, "path", "/", "The path to listen on for requests to trigger a pipeline run.")
	cmd.Flags().StringVar(&options.ServiceAccount, "service-account", "tekton-bot", "The Kubernetes ServiceAccount to use to run the pipeline.")
	cmd.Flags().BoolVar(&options.NoGitCredentialsInit, "no-git-init", false, "Disables checking we have setup git credentials on startup.")
	cmd.Flags().BoolVar(&options.SemanticRelease, "semantic-release", false, "Enable semantic releases")

	// TODO - temporary flags until meta pipeline is the default
	cmd.Flags().BoolVar(&options.UseMetaPipeline, useMetaPipelineOptionName, true, "Uses the meta pipeline to create the pipeline.")
	cmd.Flags().StringVar(&options.MetaPipelineImage, metaPipelineImageOptionName, "", "Specify the docker image to use if there is no image specified for a step.")

	options.bindViper(cmd)
	return cmd
}

func (o *PipelineRunnerOptions) bindViper(cmd *cobra.Command) {
	_ = viper.BindEnv(useMetaPipelineOptionName)
	_ = viper.BindPFlag(useMetaPipelineOptionName, cmd.Flags().Lookup(useMetaPipelineOptionName))

	_ = viper.BindEnv(metaPipelineImageOptionName)
	_ = viper.BindPFlag(metaPipelineImageOptionName, cmd.Flags().Lookup(metaPipelineImageOptionName))
}

// Run will implement this command
func (o *PipelineRunnerOptions) Run() error {
	useMetaPipeline := viper.GetBool(useMetaPipelineOptionName)

	if !o.NoGitCredentialsInit && !useMetaPipeline {
		err := o.InitGitConfigAndUser()
		if err != nil {
			return err
		}

		err = o.stepGitCredentials()
		if err != nil {
			return err
		}
	}

	jxClient, ns, err := o.getClientsAndNamespace()
	if err != nil {
		return err
	}

	metapipelineClient, err := metapipeline.NewMetaPipelineClient()
	if err != nil {
		return err
	}

	controller := controller{
		bindAddress:        o.BindAddress,
		path:               o.Path,
		port:               o.Port,
		useMetaPipeline:    useMetaPipeline,
		metaPipelineImage:  viper.GetString(metaPipelineImageOptionName),
		semanticRelease:    o.SemanticRelease,
		serviceAccount:     o.ServiceAccount,
		jxClient:           jxClient,
		ns:                 ns,
		metaPipelineClient: metapipelineClient,
	}

	controller.Start()
	return nil
}

func (o *PipelineRunnerOptions) stepGitCredentials() error {
	if !o.NoGitCredentialsInit {
		copy := *o.CommonOptions
		copy.BatchMode = true
		gsc := &credentials.StepGitCredentialsOptions{
			StepOptions: step.StepOptions{
				CommonOptions: &copy,
			},
		}
		err := gsc.Run()
		if err != nil {
			return errors.Wrapf(err, "failed to run: jx step git credentials")
		}
	}
	return nil
}

func (o *PipelineRunnerOptions) getClientsAndNamespace() (jxclient.Interface, string, error) {
	jxClient, _, err := o.JXClient()
	if err != nil {
		return nil, "", errors.Wrap(err, "unable to create JX client")
	}

	_, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, "", errors.Wrap(err, "unable to create Kube client")
	}

	return jxClient, ns, nil
}
