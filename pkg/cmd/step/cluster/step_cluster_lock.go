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
	defaultLockLabel = "locked"
	defaultTestLabel = "test"

	stepClusterLockLong    = templates.LongDesc(`Locks and joins a cluster using a lock label and optional label filters.`)
	stepClusterLockExample = templates.Examples(`
`)
)

// StepClusterLockOptions contains the command line flags and other helper objects
type StepClusterLockOptions struct {
	StepClusterOptions
	LockLabel string
	TestLabel string
	TestName  string
	Filters   []string
}

// NewCmdStepClusterLock Creates a new Command object
func NewCmdStepClusterLock(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepClusterLockOptions{
		StepClusterOptions: StepClusterOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "lock",
		Short:   "Locks and joins a cluster using a lock label and optional label filters",
		Long:    stepClusterLockLong,
		Example: stepClusterLockExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.ClusterOptions.AddClusterFlags(cmd)

	cmd.Flags().StringVarP(&options.LockLabel, "label", "l", defaultLockLabel, "The label name for the lock")
	cmd.Flags().StringVarP(&options.TestLabel, "test-label", "", defaultTestLabel, "The label name for the test")
	cmd.Flags().StringVarP(&options.TestName, "test", "t", "", "The name of the test to label on the cluster")
	cmd.Flags().StringArrayVarP(&options.Filters, "filter", "f", nil, "The labels of the form 'key=value' to filter the clusters to choose from")
	return cmd
}

// Run generates the report
func (o *StepClusterLockOptions) Run() error {
	client, err := o.ClusterOptions.CreateClient(true)
	if err != nil {
		return err
	}
	lockValue, err := clusters.NewLabelValue()
	if err != nil {
		return errors.Wrapf(err, "failed to create lock value")
	}
	lockLabels := map[string]string{
		o.LockLabel: lockValue,
	}
	if o.TestName != "" {
		lockLabels[o.TestLabel] = o.TestName
	}
	filterLabels := util.KeyValuesToMap(o.Filters)
	cluster, err := clusters.LockCluster(client, lockLabels, filterLabels)
	if err != nil {
		return errors.Wrapf(err, "failed to lock cluster with label %s and filters %#v", lockLabels, filterLabels)
	}
	if cluster == nil {
		return fmt.Errorf("could not find a cluster without lock label %#v and filters %#v", lockLabels, filterLabels)
	}

	log.Logger().Infof("to unlock the cluster again run: %s", util.ColorInfo(o.createUnlockCommand(cluster.Name)))

	return o.verifyClusterConnect(client, cluster)
}

func (o *StepClusterLockOptions) verifyClusterConnect(client clusters.Client, cluster *clusters.Cluster) error {
	name := cluster.Name

	currentContext, err := o.currentKubeContext()
	if err != nil {
		return err
	}
	log.Logger().Infof("connecting to cluster %s", util.ColorInfo(name))
	log.Logger().Infof("to switch back to the previous cluster run: %s", util.ColorInfo(fmt.Sprintf("jx ctx %s", currentContext)))

	err = client.Connect(cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to cluster %s using client %s", name, client.String())
	}

	newContext, err := o.currentKubeContext()
	if err != nil {
		return err
	}

	if currentContext == newContext {
		return fmt.Errorf("did not change the kubernetes context. The context is still %s", currentContext)
	}

	log.Logger().Infof("now connected to cluster %s with kubernetes context %s", util.ColorInfo(name), util.ColorInfo(newContext))
	return nil
}

func (o *StepClusterLockOptions) currentKubeContext() (string, error) {
	config, _, err := o.Kube().LoadConfig()
	if err != nil {
		return "", err
	}
	return config.CurrentContext, nil
}

func (o *StepClusterLockOptions) createUnlockCommand(name string) string {
	answer := "jx step cluster unlock -n " + name
	if o.LockLabel != defaultLockLabel {
		answer += " --label " + o.LockLabel
	}
	if o.TestLabel != defaultTestLabel {
		answer += " --label " + o.TestLabel
	}
	return answer
}
