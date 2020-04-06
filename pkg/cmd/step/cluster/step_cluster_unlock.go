package cluster

import (
	"fmt"

	clusters "github.com/jenkins-x/jx/pkg/cluster"
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
	stepClusterUnlockLong    = templates.LongDesc(`Unlocks the given cluster name so it joins the pool of test clusters again.`)
	stepClusterUnlockExample = templates.Examples(`
`)
)

// StepClusterUnlockOptions contains the command line flags and other helper objects
type StepClusterUnlockOptions struct {
	StepClusterOptions
	LockLabel   string
	TestLabel   string
	ClusterName string
}

// NewCmdStepClusterUnlock Creates a new Command object
func NewCmdStepClusterUnlock(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepClusterUnlockOptions{
		StepClusterOptions: StepClusterOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "unlock",
		Short:   "Unlocks the given cluster name so it joins the pool of test clusters again",
		Long:    stepClusterUnlockLong,
		Example: stepClusterUnlockExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.ClusterOptions.AddClusterFlags(cmd)

	cmd.Flags().StringVarP(&options.LockLabel, "label", "l", "locked", "The label name for the lock")
	cmd.Flags().StringVarP(&options.TestLabel, "test-label", "", "test", "The label name for the test")
	cmd.Flags().StringVarP(&options.ClusterName, "name", "n", "", "The name of the cluster to unlock")
	return cmd
}

// Run generates the report
func (o *StepClusterUnlockOptions) Run() error {
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
	resultLabels, err := clusters.RemoveLabels(client, cluster, []string{o.LockLabel, o.TestLabel})
	if err != nil {
		return err
	}
	log.Logger().Infof("unlocked cluster %s so now has labels %s", util.ColorInfo(cluster.Name), util.ColorInfo(util.MapToString(resultLabels)))
	return err
}
