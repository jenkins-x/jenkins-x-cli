package pre

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/kube"

	"github.com/jenkins-x/jx/pkg/cloud/amazon"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	optionImage = "image"
)

// StepPreBuildOptions contains the command line flags
type StepPreBuildOptions struct {
	step.StepOptions

	Image string
}

var (
	StepPreBuildLong = templates.LongDesc(`
		This pipeline step performs pre build actions such as ensuring that a Docker registry is available in the cloud
`)

	StepPreBuildExample = templates.Examples(`
		jx step pre build ${DOCKER_REGISTRY}/someorg/myapp
`)
)

func NewCmdStepPreBuild(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepPreBuildOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "build",
		Short:   "Performs actions before a build happens in a pipeline",
		Long:    StepPreBuildLong,
		Example: StepPreBuildExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Image, optionImage, "i", "", "The image name that is about to be built")
	return cmd
}

func (o *StepPreBuildOptions) Run() error {
	imageName := o.Image
	if imageName == "" {
		args := o.Args
		if len(args) == 0 {
			return util.MissingOption(optionImage)
		} else {
			imageName = args[0]
		}
	}
	paths := strings.Split(imageName, "/")
	l := len(paths)
	if l > 2 {
		dockerRegistry := paths[0]
		orgName := paths[l-2]
		appName := paths[l-1]

		log.Logger().Infof("Docker registry host: %s app name %s/%s", util.ColorInfo(dockerRegistry), util.ColorInfo(orgName), util.ColorInfo(appName))

		kubeClient, currentNamespace, err := o.KubeClientAndNamespace()
		if err != nil {
			return err
		}
		region, _ := kube.ReadRegion(kubeClient, currentNamespace)
		if strings.HasSuffix(dockerRegistry, ".amazonaws.com") && strings.Index(dockerRegistry, ".ecr.") > 0 {
			return amazon.LazyCreateRegistry(kubeClient, currentNamespace, region, dockerRegistry, orgName, appName)
		}
	}
	return nil
}
