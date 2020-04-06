package deletecmd

import (
	"errors"

	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// DeleteGkeOptions Options for deleting a GKE cluster
type DeleteGkeOptions struct {
	get.GetOptions
	Region    string
	ProjectID string
}

var (
	deleteGkeLong = templates.LongDesc(`
		Deletes GKE cluster resource.
`)

	deleteGkeExample = templates.Examples(`
		# Delete GKE cluster
		jx delete gke
	`)
)

// NewCmdDeleteGke command for deleting a GKE cluster
func NewCmdDeleteGke(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteGkeOptions{
		GetOptions: get.GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "gke",
		Short:   "Deletes GKE cluster.",
		Long:    deleteGkeLong,
		Example: deleteGkeExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Region, "region", "", "europe-west1-c", "GKE region to use. Default: europe-west1-c")
	cmd.Flags().StringVarP(&options.ProjectID, "project-id", "p", "", "Google Project ID to delete cluster from")
	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *DeleteGkeOptions) Run() error {
	if len(o.Args) == 0 {
		return errors.New("cluster name expected")
	}
	cluster := o.Args[0]
	err := o.InstallRequirements(cloud.GKE)
	if err != nil {
		return err
	}
	args := []string{"container", "clusters", "delete", cluster, "--region=" + o.Region, "--quiet", "--async"}
	if o.ProjectID != "" {
		args = append(args, "--project="+o.ProjectID)
	}
	log.Logger().Info("Deleting cluster ...")
	err = o.RunCommand("gcloud", args...)
	if err != nil {
		return err
	}
	return nil
}
