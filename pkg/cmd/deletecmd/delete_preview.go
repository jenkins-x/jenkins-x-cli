package deletecmd

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/preview"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/promote"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// DeletePreviewOptions are the flags for delete commands
type DeletePreviewOptions struct {
	preview.PreviewOptions
}

// NewCmdDeletePreview creates a command object
func NewCmdDeletePreview(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeletePreviewOptions{
		PreviewOptions: preview.PreviewOptions{
			PromoteOptions: promote.PromoteOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Deletes a preview environment",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.AddPreviewOptions(cmd)
	return cmd
}

// Run implements this command
func (o *DeletePreviewOptions) Run() error {
	kubeClient, currentNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	ns, _, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}

	err = o.DefaultValues(ns, o.BatchMode)
	if err != nil {
		if !o.BatchMode {
			jxClient, _, err := o.JXClient()
			if err != nil {
				return err
			}

			// lets let the user pick from a list of preview environments to delete
			names, err := kube.GetFilteredEnvironmentNames(jxClient, ns, kube.IsPreviewEnvironment)
			if err != nil {
				return err
			}
			if len(names) == 0 {
				log.Logger().Infof("No preview environments available to delete")
				return nil
			}
			selected := []string{}
			for {
				selected, err = util.PickNames(names, "Pick preview environments to delete: ", "", o.GetIOFileHandles())
				if err != nil {
					return err
				}
				if len(selected) > 0 {
					break
				}
				log.Logger().Warn("\nYou did not select any preview environments to delete\n")
				log.Logger().Infof("Press the %s to select a preview environment to delete\n", util.ColorInfo("[space bar]"))

				if answer, err := util.Confirm("Do you want to pick a preview environment to delete?", true, "Use the space bar to select previews", o.GetIOFileHandles()); !answer {
					return err
				}
			}
			deletePreviews := strings.Join(selected, ", ")
			if answer, err := util.Confirm("You are about to delete the Preview environments: "+deletePreviews, false, "The list of Preview Environments to be deleted", o.GetIOFileHandles()); !answer {
				return err
			}

			for _, name := range selected {
				err = o.DeletePreview(name)
				if err != nil {
					return err
				}
			}
			return nil
		}
		return err
	}

	if o.Name == "" {
		return fmt.Errorf("Could not default the preview environment name")
	}
	return o.DeletePreview(o.Name)
}

func (o *DeletePreviewOptions) DeletePreview(name string) error {
	jxClient, ns, err := o.JXClient()
	if err != nil {
		return err
	}

	environment, err := kube.GetEnvironment(jxClient, ns, name)
	if err != nil {
		return err
	}
	releaseName := kube.GetPreviewEnvironmentReleaseName(environment)
	if len(releaseName) > 0 {
		log.Logger().Infof("Deleting helm release: %s", util.ColorInfo(releaseName))
		err = o.Helm().DeleteRelease(ns, releaseName, true)
		if err != nil {
			return err
		}
	}

	log.Logger().Infof("Deleting preview environment: %s", util.ColorInfo(name))
	deleteOptions := &DeleteEnvOptions{
		CommonOptions:   o.CommonOptions,
		DeleteNamespace: true,
	}
	deleteOptions.Args = []string{name}
	return deleteOptions.Run()
}
