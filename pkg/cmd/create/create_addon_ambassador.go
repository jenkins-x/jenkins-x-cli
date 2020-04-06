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
	defaultAmbassadorReleaseName = "ambassador"
	ambassadorRepoName           = "datawire"
	ambassadorRepoUrl            = "https://www.getambassador.io"
	defaultAmbassadorVersion     = ""
)

var (
	createAddonAmbassadorLong = templates.LongDesc(`
		Creates the ambassador addon for smart load balancing on kubernetes
`)

	createAddonAmbassadorExample = templates.Examples(`
		# Create the ambassador addon 
		jx create addon ambassador

		# Create the ambassador addon in a custom namespace
		jx create addon ambassador -n mynamespace
	`)
)

// CreateAddonAmbassadorOptions the options for the create spring command
type CreateAddonAmbassadorOptions struct {
	CreateAddonOptions

	Chart string
}

// NewCmdCreateAddonAmbassador creates a command object for the "create" command
func NewCmdCreateAddonAmbassador(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonAmbassadorOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "ambassador",
		Short:   "Create an ambassador addon",
		Aliases: []string{"env"},
		Long:    createAddonAmbassadorLong,
		Example: createAddonAmbassadorExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, "", defaultAmbassadorReleaseName, defaultAmbassadorVersion)

	cmd.Flags().StringVarP(&options.Chart, optionChart, "c", kube.ChartAmbassador, "The name of the chart to use")
	return cmd
}

// Run implements the command
func (o *CreateAddonAmbassadorOptions) Run() error {
	if o.ReleaseName == "" {
		return util.MissingOption(optionRelease)
	}
	if o.Chart == "" {
		return util.MissingOption(optionChart)
	}
	_, err := o.AddHelmBinaryRepoIfMissing(ambassadorRepoUrl, ambassadorRepoName, "", "")
	if err != nil {
		return err
	}

	err = o.EnsureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that helm is present")
	}

	values := strings.Split(o.SetValues, ",")
	helmOptions := helm.InstallChartOptions{
		Chart:       o.Chart,
		ReleaseName: o.ReleaseName,
		Version:     o.Version,
		Ns:          o.Namespace,
		SetValues:   values,
		ValueFiles:  o.ValueFiles,
	}
	err = o.InstallChartWithOptions(helmOptions)
	if err != nil {
		return err
	}
	return nil
}
