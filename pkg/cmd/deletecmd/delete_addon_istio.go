package deletecmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	deleteAddonIstioLong = templates.LongDesc(`
		Deletes the Istio addon
`)

	deleteAddonIstioExample = templates.Examples(`
		# Deletes the Istio addon
		jx delete addon istio
	`)
)

// DeleteAddonIstioOptions the options for the create spring command
type DeleteAddonIstioOptions struct {
	DeleteAddonOptions

	ReleaseName string
	Namespace   string
}

// NewCmdDeleteAddonIstio defines the command
func NewCmdDeleteAddonIstio(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteAddonIstioOptions{
		DeleteAddonOptions: DeleteAddonOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "istio",
		Short:   "Deletes the Istio addon",
		Long:    deleteAddonIstioLong,
		Example: deleteAddonIstioExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.ReleaseName, opts.OptionRelease, "r", "istio", "The chart release name")
	cmd.Flags().StringVarP(&options.Namespace, opts.OptionNamespace, "n", create.DefaultIstioNamespace, "The Namespace to delete from")
	options.addFlags(cmd)
	return cmd
}

// Run implements the command
func (o *DeleteAddonIstioOptions) Run() error {
	if o.ReleaseName == "" {
		return util.MissingOption(opts.OptionRelease)
	}

	// TODO the official way to delete Istio is to download the release again and run
	// helm template install/kubernetes/helm/istio --name istio --namespace istio-system | kubectl delete -f -
	// but that also means figuring out and downloading the same version that was installed
	// we will just delete most of it using label selectors

	// Delete from the 'istio-system' namespace, not from 'jx'
	err := o.Helm().DeleteRelease(o.Namespace, o.ReleaseName, o.Purge)
	if err != nil {
		return errors.Wrap(err, "Failed to delete istio chart")
	}
	err = o.Helm().DeleteRelease(o.Namespace, o.ReleaseName+"-init", o.Purge)
	if err != nil {
		return errors.Wrap(err, "Failed to delete istio-init chart")
	}

	// delete CRDs
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	selector := fmt.Sprintf("chart=%s,release=%s", o.ReleaseName, o.ReleaseName)
	log.Logger().Infof("Removing Istio CRDs using selector: %s", util.ColorInfo(selector))
	err = apisClient.ApiextensionsV1beta1().CustomResourceDefinitions().DeleteCollection(&metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}

	return err
}
