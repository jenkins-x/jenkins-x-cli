package e2e

import (
	"errors"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// StepE2ELabelOptions contains the command line flags
type StepE2ELabelOptions struct {
	step.StepOptions
	ProjectID string
	Region    string
	Keep      bool
	Delete    bool
}

var (
	stepE2EApplyLabelLong = templates.LongDesc(`
		Add a label to a cluster used for e2e testing
`)

	stepE2EApplyLabelExample = templates.Examples(`
		# Mark a cluster to not be deleted by the gc
		jx step e2e label --keep clusterName

        # Mark a cluster to be deleted by the gc
		jx step e2e label --delete clusterName

`)
)

// NewCmdStepE2ELabel creates the CLI command
func NewCmdStepE2ELabel(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepE2ELabelOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "label",
		Short:   "Removes unused e2e clusters",
		Aliases: []string{},
		Long:    stepE2EApplyLabelLong,
		Example: stepE2EApplyLabelExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Region, "region", "", "europe-west1-c", "GKE region to use. Default: europe-west1-c")
	cmd.Flags().StringVarP(&options.ProjectID, "project-id", "p", "", "Google Project ID to delete cluster from")
	cmd.Flags().BoolVarP(&options.Keep, "keep", "k", false, "Add a label top mark cluster for non deletion")
	cmd.Flags().BoolVarP(&options.Delete, "delete", "d", false, "Add a label top mark cluster for deletion")

	return cmd
}

// Run runs the command
func (o *StepE2ELabelOptions) Run() error {
	if len(o.Args) == 0 {
		return errors.New("Please specify a cluster name")
	}
	if o.Keep == o.Delete {
		return errors.New("Please specify either --keep or --delete")
	}
	err := o.InstallRequirements(cloud.GKE)
	if err != nil {
		return err
	}
	clusterName := o.Args[0]
	cluster, err := o.GCloud().LoadGkeCluster(o.Region, o.ProjectID, clusterName)
	if err != nil {
		return err
	}
	if cluster != nil {
		labelMap := cluster.ResourceLabels
		labels := make([]string, 0)
		if o.Keep {
			labelMap["keep-me"] = "true"
		} else {
			delete(labelMap, "keep-me")
		}
		if o.Delete {
			labelMap["delete-me"] = "true"
		} else {
			delete(labelMap, "delete-me")
		}
		for key, value := range labelMap {
			labels = append(labels, key+"="+value)
		}
		err := o.GCloud().UpdateGkeClusterLabels(o.Region, o.ProjectID, clusterName, labels)
		if err == nil {
			if o.Keep {
				log.Logger().Infof("%s was marked to be kept", cluster.Name)
			} else {
				log.Logger().Infof("%s was marked to be deleted", cluster.Name)
			}
		} else {
			log.Logger().Error(err)
		}
	}
	return nil
}
