package helm

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StepHelmEnvOptions contains the command line flags
type StepHelmEnvOptions struct {
	StepHelmOptions
}

var (
	StepHelmEnvLong = templates.LongDesc(`
		Generates the helm environment variables
`)

	StepHelmEnvExample = templates.Examples(`
		# output the helm environment variables that should be set to use helm directly
		jx step helm env

`)
)

func NewCmdStepHelmEnv(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepHelmEnvOptions{
		StepHelmOptions: StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "env",
		Short:   "Generates the helm environment variables",
		Aliases: []string{""},
		Long:    StepHelmEnvLong,
		Example: StepHelmEnvExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addStepHelmFlags(cmd)

	return cmd
}

func (o *StepHelmEnvOptions) Run() error {
	h := o.Helm()
	if h != nil {
		log.Logger().Info("")
		log.Logger().Info("# helm environment variables")
		envVars := h.Env()
		keys := util.SortedMapKeys(envVars)
		for _, key := range keys {
			if strings.HasPrefix(key, "HELM") {
				log.Logger().Infof("export %s=\"%s\"", key, envVars[key])
			}
		}
		log.Logger().Info("")
	}
	return nil
}
