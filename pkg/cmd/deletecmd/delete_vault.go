package deletecmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cloud"

	awsvault "github.com/jenkins-x/jx/pkg/cloud/amazon/vault"
	"github.com/jenkins-x/jx/pkg/cloud/gke"
	gkevault "github.com/jenkins-x/jx/pkg/cloud/gke/vault"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/naming"
	"github.com/jenkins-x/jx/pkg/kube/serviceaccount"
	kubevault "github.com/jenkins-x/jx/pkg/kube/vault"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteVaultOptions keeps the options of delete vault command
type DeleteVaultOptions struct {
	*opts.CommonOptions

	Namespace            string
	RemoveCloudResources bool
	GKEProjectID         string
	GKEZone              string
}

var (
	deleteVaultLong = templates.LongDesc(`
		Deletes a Vault
	`)

	deleteVaultExample = templates.Examples(`
		# Deletes a Vault from namespace my-namespace
		jx delete vault --namespace my-namespace my-vault
	`)
)

// NewCmdDeleteVault builds a new delete vault command
func NewCmdDeleteVault(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteVaultOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "vault",
		Short:   "Deletes a Vault",
		Long:    deleteVaultLong,
		Example: deleteVaultExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "Namespace from where to delete the vault")
	cmd.Flags().BoolVarP(&options.RemoveCloudResources, "remove-cloud-resources", "r", false, "Remove all cloud resource allocated for the Vault")
	cmd.Flags().StringVarP(&options.GKEProjectID, "gke-project-id", "", "", "Google Project ID to use for Vault backend")
	cmd.Flags().StringVarP(&options.GKEZone, "gke-zone", "", "", "The zone (e.g. us-central1-a) where Vault will store the encrypted data")
	return cmd
}

// Run implements the delete vault command
func (o *DeleteVaultOptions) Run() error {
	if len(o.Args) != 1 {
		return fmt.Errorf("Missing vault name")
	}
	vaultName := o.Args[0]

	client, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return errors.Wrap(err, "creating kubernetes client")
	}

	if o.Namespace == "" {
		o.Namespace = ns
	}

	teamSettings, err := o.TeamSettings()
	if err != nil {
		return errors.Wrap(err, "retrieving the team settings")
	}

	vaultOperatorClient, err := o.VaultOperatorClient()
	if err != nil {
		return errors.Wrap(err, "creating vault operator client")
	}

	v, err := kubevault.GetVault(vaultOperatorClient, vaultName, o.Namespace)
	if err != nil {
		return fmt.Errorf("vault '%s' not found in namespace '%s'", vaultName, o.Namespace)
	}

	err = kubevault.DeleteVault(vaultOperatorClient, vaultName, o.Namespace)
	if err != nil {
		return errors.Wrap(err, "deleting the vault resource")
	}

	err = kube.DeleteIngress(client, o.Namespace, vaultName)
	if err != nil {
		return errors.Wrapf(err, "deleting the vault ingress '%s'", vaultName)
	}

	authServiceAccountName := kubevault.GetAuthSaName(*v)
	err = serviceaccount.DeleteServiceAccount(client, o.Namespace, authServiceAccountName)
	if err != nil {
		return errors.Wrapf(err, "deleting the vault auth service account '%s'", authServiceAccountName)
	}

	var secretName string
	if teamSettings.KubeProvider == cloud.GKE {
		secretName = gke.GcpServiceAccountSecretName(vaultName)
	}
	if teamSettings.KubeProvider == cloud.AWS || teamSettings.KubeProvider == cloud.EKS {
		secretName = awsvault.AwsServiceAccountSecretName(vaultName)
	}
	err = client.CoreV1().Secrets(o.Namespace).Delete(secretName, &metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "deleting secret '%s' where GCP service account is stored", secretName)
	}
	err = kube.DeleteClusterRoleBinding(client, vaultName)
	if err != nil {
		return errors.Wrapf(err, "deleting the cluster role binding '%s' for vault", vaultName)
	}

	log.Logger().Infof("Vault %s deleted", util.ColorInfo(vaultName))

	if o.RemoveCloudResources {
		if teamSettings.KubeProvider == cloud.GKE {
			log.Logger().Infof("Removing GCP resources allocated for Vault...")
			err := o.removeGCPResources(vaultName)
			if err != nil {
				return errors.Wrap(err, "removing GCP resource")
			}
			log.Logger().Infof("Cloud resources allocated for vault %s deleted", util.ColorInfo(vaultName))
		}
	}

	return nil
}

func (o *DeleteVaultOptions) removeGCPResources(vaultName string) error {
	err := o.GCloud().Login("", true)
	if err != nil {
		return errors.Wrap(err, "login into GCP")
	}

	if o.GKEProjectID == "" {
		projectID, err := o.GetGoogleProjectID("")
		if err != nil {
			return err
		}
		o.GKEProjectID = projectID
	}
	err = o.RunCommandVerbose("gcloud", "config", "set", "project", o.GKEProjectID)
	if err != nil {
		return err
	}

	if o.GKEZone == "" {
		zone, err := o.GetGoogleZone(o.GKEProjectID, "")
		if err != nil {
			return err
		}
		o.GKEZone = zone
	}

	sa := naming.ToValidGCPServiceAccount(gke.ServiceAccountName(vaultName, gkevault.DefaultVaultAbbreviation))
	err = o.GCloud().DeleteServiceAccount(sa, o.GKEProjectID, gkevault.ServiceAccountRoles)
	if err != nil {
		return errors.Wrapf(err, "deleting the GCP service account '%s'", sa)
	}
	log.Logger().Infof("GCP service account %s deleted", util.ColorInfo(sa))

	bucket := gke.BucketName(vaultName)
	err = o.GCloud().DeleteAllObjectsInBucket(bucket)
	if err != nil {
		return errors.Wrapf(err, "deleting all objects in GCS bucket '%s'", bucket)
	}

	log.Logger().Infof("GCS bucket %s deleted", util.ColorInfo(bucket))

	return nil
}
