package helm

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// StepHelmListOptions contains the command line flags
type StepHelmListOptions struct {
	StepHelmOptions

	Namespace string
}

var (
	StepHelmListLong = templates.LongDesc(`
		List the helm releases
`)

	StepHelmListExample = templates.Examples(`
		# list all the helm releases in the current namespace
		jx step helm list

`)
)

func NewCmdStepHelmList(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepHelmListOptions{
		StepHelmOptions: StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the helm releases",
		Aliases: []string{""},
		Long:    StepHelmListLong,
		Example: StepHelmListExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addStepHelmFlags(cmd)
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to look for the helm releases. Defaults to the current namespace")

	return cmd
}

func (o *StepHelmListOptions) Run() error {
	h := o.Helm()
	if h == nil {
		return fmt.Errorf("No Helmer created!")
	}
	releases, sortedKeys, err := h.ListReleases(o.Namespace)
	if err != nil {
		return errors.WithStack(err)
	}
	output, err := helm.RenderReleasesAsTable(releases, sortedKeys)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Logger().Info(output)
	return nil
}
