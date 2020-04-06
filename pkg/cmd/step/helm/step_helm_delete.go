package helm

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// StepHelmDeleteOptions contains the command line flags
type StepHelmDeleteOptions struct {
	StepHelmOptions

	Namespace string
	Purge     bool
}

var (
	stepHelmDeleteLong = templates.LongDesc(`
		Deletes a helm release
`)

	stepHelmDeleteExample = templates.Examples(`
		# list all the helm releases in the current namespace
		jx step helm list

`)
)

// NewCmdStepHelmDelete creates the command object
func NewCmdStepHelmDelete(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepHelmDeleteOptions{
		StepHelmOptions: StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "delete [releaseName]",
		Short:   "Deletes the given helm release",
		Aliases: []string{""},
		Long:    stepHelmDeleteLong,
		Example: stepHelmDeleteExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addStepHelmFlags(cmd)
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to look for the helm releases. Defaults to the current namespace")
	cmd.Flags().BoolVarP(&options.Purge, "purge", "", false, "Whether to purge the helm release")

	return cmd
}

// Run performs the CLI command
func (o *StepHelmDeleteOptions) Run() error {
	args := o.Args
	if len(args) == 0 {
		return util.MissingArgument("releaseName")
	}
	releaseName := args[0]
	h := o.Helm()
	if h == nil {
		return fmt.Errorf("no Helmer created")
	}
	ns := o.Namespace
	var err error
	if ns == "" {
		_, ns, err = o.KubeClientAndNamespace()
		if err != nil {
			return err
		}
	}
	err = h.DeleteRelease(ns, releaseName, o.Purge)
	if err != nil {
		return err
	}
	log.Logger().Infof("Deleted release %s in namespace %s", util.ColorInfo(releaseName), util.ColorInfo(ns))
	return nil
}
