package create

import (
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/helm"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
)

const (
	defaultKubelessNamespace   = "kubeless"
	defaultKubelessReleaseName = "kubeless"
	defaultKubelessVersion     = ""
)

var (
	createAddonKubelessLong = templates.LongDesc(`
		Creates the kubeless addon for serverless on Kubernetes
`)

	createAddonKubelessExample = templates.Examples(`
		# Create the kubeless addon in the ` + defaultKubelessNamespace + ` namespace 
		jx create addon kubeless

		# Create the kubeless addon in a custom namespace
		jx create addon kubeless -n mynamespace
	`)
)

// CreateAddonKubelessOptions the options for the create spring command
type CreateAddonKubelessOptions struct {
	CreateAddonOptions

	Chart string
}

// NewCmdCreateAddonKubeless creates a command object for the "create" command
func NewCmdCreateAddonKubeless(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonKubelessOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "kubeless",
		Short:   "Create a kubeless addon for hosting Git repositories",
		Aliases: []string{"env"},
		Long:    createAddonKubelessLong,
		Example: createAddonKubelessExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, defaultKubelessNamespace, defaultKubelessReleaseName, defaultKubelessVersion)

	cmd.Flags().StringVarP(&options.Chart, optionChart, "c", kube.ChartKubeless, "The name of the chart to use")
	return cmd
}

// Run implements the command
func (o *CreateAddonKubelessOptions) Run() error {
	if o.ReleaseName == "" {
		return util.MissingOption(optionRelease)
	}
	if o.Chart == "" {
		return util.MissingOption(optionChart)
	}
	err := o.EnsureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that Helm is present")
	}
	values := []string{"rbac.create=true"}
	setValues := strings.Split(o.SetValues, ",")
	values = append(values, setValues...)
	helmOptions := helm.InstallChartOptions{
		ReleaseName: o.ReleaseName,
		Chart:       o.Chart,
		Version:     o.Version,
		Ns:          o.Namespace,
		SetValues:   values,
	}
	err = o.InstallChartWithOptions(helmOptions)
	if err != nil {
		return err
	}
	return nil
}
