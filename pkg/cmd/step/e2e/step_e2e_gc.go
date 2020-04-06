package e2e

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cluster"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cluster/factory"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	"github.com/jenkins-x/jx/pkg/cmd/deletecmd"
	"github.com/jenkins-x/jx/pkg/cmd/gc"
	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// StepE2EGCOptions contains the command line flags
type StepE2EGCOptions struct {
	step.StepOptions
	ProjectID string
	Providers []string
	Region    string
	Duration  int
}

var (
	stepE2EGCLong = templates.LongDesc(`
		This pipeline step removes stale E2E test clusters
`)

	stepE2EGCExample = templates.Examples(`
		# delete stale E2E test clusters
		jx step e2e gc

`)
)

// NewCmdStepE2EGC creates the CLI command
func NewCmdStepE2EGC(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepE2EGCOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "gc",
		Short:   "Removes unused e2e clusters",
		Aliases: []string{},
		Long:    stepE2EGCLong,
		Example: stepE2EGCExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Region, "region", "", "europe-west1-c", "GKE region to use. Default: europe-west1-c")
	cmd.Flags().StringVarP(&options.ProjectID, "project-id", "p", "", "Google Project ID to delete cluster from")
	cmd.Flags().IntVarP(&options.Duration, "duration", "d", 2, "How many hours old a cluster should be before it is deleted if it does not have a --delete tag")
	cmd.Flags().StringArrayVarP(&options.Providers, "providers", "", []string{"gke"}, "The providers to run the cleanup for")

	return cmd
}

// Run runs the command
func (o *StepE2EGCOptions) Run() error {
	// Until https://github.com/jenkins-x/jx/issues/6206 is done, we are going to be using different approaches to run this for different providers
	for _, pr := range o.Providers {
		switch strings.ToLower(pr) {
		case cloud.GKE:
			return o.gcpGarbageCollection()
		case cloud.AWS:
			fallthrough
		case cloud.EKS:
			return o.eksGarbageCollection()
		default:
			return fmt.Errorf("provider %s doesn't have an E2E GC implementation defined", pr)
		}
	}
	return nil
}

func (o *StepE2EGCOptions) eksGarbageCollection() error {
	eksClient, err := factory.NewClientForProvider(cloud.EKS)
	if err != nil {
		return errors.Wrap(err, "could not obtain an EKS cluster client to ")
	}
	eksClusters, err := eksClient.List()
	if err != nil {
		return errors.Wrap(err, "there was a problem obtaining every eksClient in the current account")
	}

	for _, eksCluster := range eksClusters {
		if eksCluster.Status == "ACTIVE" {
			if !o.ShouldDeleteMarkedEKSCluster(eksCluster) {
				if !o.ShouldDeleteOlderThanDurationEKS(eksCluster) {
					if o.ShouldDeleteDueToNewerRunEKS(eksCluster, eksClusters) {
						err = o.deleteEksCluster(eksCluster, eksClient)
					}
				} else {
					err = o.deleteEksCluster(eksCluster, eksClient)
				}
			} else {
				err = o.deleteEksCluster(eksCluster, eksClient)
			}
		}
		if err != nil {
			log.Logger().Errorf("error deleting cluster %s: %s", eksCluster.Name, err.Error())
		}
	}
	return nil
}

func (o *StepE2EGCOptions) gcpGarbageCollection() error {
	err := o.InstallRequirements(cloud.GKE)
	if err != nil {
		return err
	}
	gkeSa := os.Getenv("GKE_SA_KEY_FILE")
	if gkeSa != "" {
		err = o.GCloud().Login(gkeSa, true)
		if err != nil {
			return err
		}
	}

	clusters, err := o.GCloud().ListClusters(o.Region, o.ProjectID)
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		if cluster.Status == "RUNNING" {
			// Marked for deletion
			if !o.ShouldDeleteMarkedCluster(&cluster) {
				// Older than duration in hours
				if !o.ShouldDeleteOlderThanDuration(&cluster) {
					// Delete build that has been replaced by a newer version
					if o.ShouldDeleteDueToNewerRun(&cluster, clusters) {
						o.deleteGkeCluster(&cluster)
					}
				} else {
					o.deleteGkeCluster(&cluster)
				}
			} else {
				o.deleteGkeCluster(&cluster)
			}
		}
	}
	gkeGCOpts := gc.GCGKEOptions{
		CommonOptions: &opts.CommonOptions{},
	}
	gkeGCOpts.Err = o.Err
	gkeGCOpts.Out = o.Out
	gkeGCOpts.Flags.ProjectID = o.ProjectID
	gkeGCOpts.Flags.RunNow = true
	return gkeGCOpts.Run()
}

// GetBuildNumberFromClusterEKS gets the build number from the cluster labels
func (o *StepE2EGCOptions) GetBuildNumberFromClusterEKS(cluster *cluster.Cluster) (int, error) {
	if branch, ok := cluster.Labels["branch"]; ok {
		if clusterType, ok := cluster.Labels["cluster"]; ok {
			buildNumStr := strings.Replace(strings.Replace(cluster.Name, branch+"-", "", -1), "-"+clusterType, "", -1)
			return strconv.Atoi(buildNumStr)
		}
	}
	return 0, fmt.Errorf("finding build number for cluster " + cluster.Name)
}

// GetBuildNumberFromCluster gets the build number from the cluster labels
func (o *StepE2EGCOptions) GetBuildNumberFromCluster(cluster *gke.Cluster) (int, error) {
	if branch, ok := cluster.ResourceLabels["branch"]; ok {
		if clusterType, ok := cluster.ResourceLabels["cluster"]; ok {
			buildNumStr := strings.Replace(strings.Replace(cluster.Name, branch+"-", "", -1), "-"+clusterType, "", -1)
			return strconv.Atoi(buildNumStr)
		}
	}
	return 0, fmt.Errorf("finding build number for cluster " + cluster.Name)
}

// ShouldDeleteMarkedCluster returns true if the cluster has a delete label
func (o *StepE2EGCOptions) ShouldDeleteMarkedCluster(cluster *gke.Cluster) bool {
	if deleteLabel, ok := cluster.ResourceLabels["delete-me"]; ok {
		if deleteLabel == "true" {
			return true
		}
	}
	return false
}

// ShouldDeleteMarkedEKSCluster returns true if the cluster has a delete label
func (o *StepE2EGCOptions) ShouldDeleteMarkedEKSCluster(cluster *cluster.Cluster) bool {
	if deleteLabel, ok := cluster.Labels["delete-me"]; ok {
		if deleteLabel == "true" {
			return true
		}
	}
	return false
}

// ShouldDeleteOlderThanDurationEKS returns true if the cluster is older than the delete duration and does not have a keep label
func (o *StepE2EGCOptions) ShouldDeleteOlderThanDurationEKS(cluster *cluster.Cluster) bool {
	if createdTime, ok := cluster.Labels["create-time"]; ok {
		createdDate, err := time.Parse("Mon-Jan-2-2006-15-04-05", createdTime)
		if err != nil {
			log.Logger().Errorf("Error parsing date for cluster %s", createdTime)
			log.Logger().Error(err)
		} else {
			ttlExceededDate := createdDate.Add(time.Duration(o.Duration) * time.Hour)
			now := time.Now().UTC()
			if now.After(ttlExceededDate) {
				if _, ok := cluster.Labels["keep-me"]; !ok {
					return true
				}
			}
		}
	}
	return false
}

// ShouldDeleteOlderThanDuration returns true if the cluster is older than the delete duration and does not have a keep label
func (o *StepE2EGCOptions) ShouldDeleteOlderThanDuration(cluster *gke.Cluster) bool {
	if createdTime, ok := cluster.ResourceLabels["create-time"]; ok {
		createdDate, err := time.Parse("Mon-Jan-2-2006-15-04-05", createdTime)
		if err != nil {
			log.Logger().Errorf("Error parsing date for cluster %s", createdTime)
			log.Logger().Error(err)
		} else {
			ttlExceededDate := createdDate.Add(time.Duration(o.Duration) * time.Hour)
			now := time.Now().UTC()
			if now.After(ttlExceededDate) {
				if _, ok := cluster.ResourceLabels["keep-me"]; !ok {
					return true
				}
			}
		}
	}
	return false
}

// ShouldDeleteDueToNewerRunEKS returns true if a cluster with a higher build number exists
func (o *StepE2EGCOptions) ShouldDeleteDueToNewerRunEKS(cluster *cluster.Cluster, clusters []*cluster.Cluster) bool {
	if branchLabel, ok := cluster.Labels["branch"]; ok {
		if strings.Contains(branchLabel, "pr-") {
			currentBuildNumber, err := o.GetBuildNumberFromClusterEKS(cluster)
			if err == nil {
				if clusterType, ok := cluster.Labels["cluster"]; ok {
					for _, existingCluster := range clusters {
						// Check for same PR & Cluster type
						if existingClusterType, ok := existingCluster.Labels["cluster"]; ok {
							if strings.Contains(existingCluster.Name, branchLabel) && existingClusterType == clusterType {
								existingBuildNumber, err := o.GetBuildNumberFromClusterEKS(existingCluster)
								if err == nil {
									// Delete the older build
									if currentBuildNumber < existingBuildNumber {
										if _, ok := cluster.Labels["keep-me"]; !ok {
											return true
										}
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

// ShouldDeleteDueToNewerRun returns true if a cluster with a higher build number exists
func (o *StepE2EGCOptions) ShouldDeleteDueToNewerRun(cluster *gke.Cluster, clusters []gke.Cluster) bool {
	if branchLabel, ok := cluster.ResourceLabels["branch"]; ok {
		if strings.Contains(branchLabel, "pr-") {
			currentBuildNumber, err := o.GetBuildNumberFromCluster(cluster)
			if err == nil {
				if clusterType, ok := cluster.ResourceLabels["cluster"]; ok {
					for _, existingCluster := range clusters {
						// Check for same PR & Cluster type
						if existingClusterType, ok := existingCluster.ResourceLabels["cluster"]; ok {
							if strings.Contains(existingCluster.Name, branchLabel) && existingClusterType == clusterType {
								existingBuildNumber, err := o.GetBuildNumberFromCluster(&existingCluster)
								if err == nil {
									// Delete the older build
									if currentBuildNumber < existingBuildNumber {
										if _, ok := cluster.ResourceLabels["keep-me"]; !ok {
											return true
										}
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return false
}

func (o *StepE2EGCOptions) deleteEksCluster(cluster *cluster.Cluster, client cluster.Client) error {
	err := client.Delete(cluster)
	if err != nil {
		return errors.Wrapf(err, "error deleting EKS cluster %s", cluster.Name)
	}
	return nil
}

func (o *StepE2EGCOptions) deleteGkeCluster(cluster *gke.Cluster) {
	deleteOptions := &deletecmd.DeleteGkeOptions{
		GetOptions: get.GetOptions{
			CommonOptions: &opts.CommonOptions{},
		},
	}
	deleteOptions.Args = []string{cluster.Name}
	deleteOptions.ProjectID = o.ProjectID
	deleteOptions.Region = o.Region
	err := deleteOptions.Run()
	if err != nil {
		log.Logger().Error(err)
	} else {
		log.Logger().Infof("Deleted cluster %s", cluster.Name)
	}
}
