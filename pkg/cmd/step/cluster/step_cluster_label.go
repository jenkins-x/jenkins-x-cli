package cluster

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	stepClusterLabelLong    = templates.LongDesc(`Labels the given cluster.`)
	stepClusterLabelExample = templates.Examples(`
`)
)

// StepClusterLabelOptions contains the command line flags and other helper objects
type StepClusterLabelOptions struct {
	StepClusterOptions
	Labels      []string
	ClusterName string
}

// NewCmdStepClusterLabel Creates a new Command object
func NewCmdStepClusterLabel(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepClusterLabelOptions{
		StepClusterOptions: StepClusterOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "label",
		Short:   "Labels the given cluster",
		Long:    stepClusterLabelLong,
		Example: stepClusterLabelExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.ClusterOptions.AddClusterFlags(cmd)

	cmd.Flags().StringArrayVarP(&options.Labels, "label", "l", nil, "The label key and value to set. Of the form 'label=value'")
	cmd.Flags().StringVarP(&options.ClusterName, "name", "n", "", "The name of the cluster to unlock")
	return cmd
}

// Run generates the report
func (o *StepClusterLabelOptions) Run() error {
	if len(o.Labels) == 0 {
		return util.MissingOption("label")
	}
	client, err := o.ClusterOptions.CreateClient(true)
	if err != nil {
		return err
	}
	clusterName, err := o.GetOrPickClusterName(client, o.ClusterName)
	if err != nil {
		return err
	}

	cluster, err := client.Get(clusterName)
	if err != nil {
		return errors.Wrapf(err, "failed to find cluster name %s using client %s", clusterName, client.String())
	}
	if cluster == nil {
		return fmt.Errorf("there is no cluster called %s using client %s", clusterName, client.String())
	}

	labels := util.MergeMaps(map[string]string{}, cluster.Labels, util.KeyValuesToMap(o.Labels))
	err = client.SetClusterLabels(cluster, labels)
	if err != nil {
		return errors.Wrapf(err, "failed to set cluster %s labels %s with client %s", clusterName, util.MapToString(labels), client.String())
	}

	log.Logger().Infof("the cluster %s now has labels %s", util.ColorInfo(cluster.Name), util.ColorInfo(util.MapToString(labels)))
	return err
}
