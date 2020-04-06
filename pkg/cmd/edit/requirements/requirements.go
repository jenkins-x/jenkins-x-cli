package requirements

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
)

// RequirementsOptions the CLI options for this command
type RequirementsOptions struct {
	*opts.CommonOptions

	Dir string

	Requirements  config.RequirementsConfig
	SecretStorage string
	Webhook       string
	Flags         RequirementBools
}

// RequirementBools for the boolean flags we only update if specified on the CLI
type RequirementBools struct {
	AutoUpgrade, EnvironmentGitPublic, GitOps, Kaniko, Terraform bool
	VaultRecreateBucket, VaultDisableURLDiscover                 bool
}

var (
	requirementsLong = templates.LongDesc(`
		Edits the local 'jx-requirements.yml file for 'jx boot'
`)

	requirementsExample = templates.Examples(`
		# edits the local 'jx-requirements.yml' file used for 'jx boot'
		jx edit requirements --domain foo.com --tls --provider eks
`)
)

// NewCmdEditRequirements creates the new command
func NewCmdEditRequirements(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &RequirementsOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "requirements",
		Short:   "Edits the local 'jx-requirements.yml file for 'jx boot'",
		Long:    requirementsLong,
		Example: requirementsExample,
		Aliases: []string{"req", "require", "requirement"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			options.Cmd = cmd
			options.Args = args
			return options.Run()
		},
	}
	cmd.Flags().StringVarP(&options.Dir, "dir", "", ".", "the directory to search for the 'jx-requirements.yml' file")

	// bools
	cmd.Flags().BoolVarP(&options.Flags.AutoUpgrade, "autoupgrade", "", false, "enables or disables auto upgrades")
	cmd.Flags().BoolVarP(&options.Flags.EnvironmentGitPublic, "env-git-public", "", false, "enables or disables whether the environment repositories should be public")
	cmd.Flags().BoolVarP(&options.Flags.GitOps, "gitops", "g", false, "enables or disables the use of gitops")
	cmd.Flags().BoolVarP(&options.Flags.Kaniko, "kaniko", "", false, "enables or disables the use of kaniko")
	cmd.Flags().BoolVarP(&options.Flags.Terraform, "terraform", "", false, "enables or disables the use of terraform")
	cmd.Flags().BoolVarP(&options.Flags.VaultRecreateBucket, "vault-recreate-bucket", "", false, "enables or disables whether to rereate the secret bucket on boot")
	cmd.Flags().BoolVarP(&options.Flags.VaultDisableURLDiscover, "vault-disable-url-discover", "", false, "override the default lookup of the Vault URL, could be incluster service or external ingress")

	// requirements
	cmd.Flags().StringVarP(&options.Requirements.BootConfigURL, "boot-config-url", "", "", "specify the boot configuration git URL")
	cmd.Flags().StringVarP(&options.SecretStorage, "secret", "s", "", fmt.Sprintf("configures the kind of secret storage. Values: %s", strings.Join(config.SecretStorageTypeValues, ", ")))
	cmd.Flags().StringVarP(&options.Webhook, "webhook", "w", "", fmt.Sprintf("configures the kind of webhook. Values %s", strings.Join(config.WebhookTypeValues, ", ")))

	// auto upgrade
	cmd.Flags().StringVarP(&options.Requirements.AutoUpdate.Schedule, "autoupdate-schedule", "", "", "the cron schedule for auto upgrading your cluster")

	// cluster
	cmd.Flags().StringVarP(&options.Requirements.Cluster.ClusterName, "cluster", "c", "", "configures the cluster name")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.Namespace, "namespace", "n", "", "configures the namespace to use")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.Provider, "provider", "p", "", "configures the kubernetes provider")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.ProjectID, "project", "", "", "configures the Google Project ID")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.Registry, "registry", "", "", "configures the host name of the container registry")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.Region, "region", "r", "", "configures the cloud region")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.Zone, "zone", "z", "", "configures the cloud zone")

	cmd.Flags().StringVarP(&options.Requirements.Cluster.ExternalDNSSAName, "extdns-sa", "", "", "configures the External DNS service account name")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.KanikoSAName, "kaniko-sa", "", "", "configures the Kaniko service account name")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.HelmMajorVersion, "helm-version", "", "", "configures the Helm major version. e.g. 3 to try helm 3")

	// git
	cmd.Flags().StringVarP(&options.Requirements.Cluster.GitKind, "git-kind", "", "", fmt.Sprintf("the kind of git repository to use. Possible values: %s", strings.Join(gits.KindGits, ", ")))
	cmd.Flags().StringVarP(&options.Requirements.Cluster.GitName, "git-name", "", "", "the name of the git repository")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.GitServer, "git-server", "", "", "the git server host such as https://github.com or https://gitlab.com")
	cmd.Flags().StringVarP(&options.Requirements.Cluster.EnvironmentGitOwner, "env-git-owner", "", "", "the git owner (organisation or user) used to own the git repositories for the environments")

	// ingress
	cmd.Flags().StringVarP(&options.Requirements.Ingress.Domain, "domain", "d", "", "configures the domain name")
	cmd.Flags().StringVarP(&options.Requirements.Ingress.TLS.Email, "tls-email", "", "", "the TLS email address to enable TLS on the domain")

	// storage
	cmd.Flags().StringVarP(&options.Requirements.Storage.Logs.URL, "bucket-logs", "", "", "the bucket URL to store logs")
	cmd.Flags().StringVarP(&options.Requirements.Storage.Backup.URL, "bucket-backups", "", "", "the bucket URL to store backups")
	cmd.Flags().StringVarP(&options.Requirements.Storage.Repository.URL, "bucket-repo", "", "", "the bucket URL to store repository artifacts")
	cmd.Flags().StringVarP(&options.Requirements.Storage.Reports.URL, "bucket-reports", "", "", "the bucket URL to store reports. If not specified default to te logs bucket")

	// vault
	cmd.Flags().StringVarP(&options.Requirements.Vault.Name, "vault-name", "", "", "specify the vault name")
	cmd.Flags().StringVarP(&options.Requirements.Vault.Bucket, "vault-bucket", "", "", "specify the vault bucket")
	cmd.Flags().StringVarP(&options.Requirements.Vault.Keyring, "vault-keyring", "", "", "specify the vault key ring")
	cmd.Flags().StringVarP(&options.Requirements.Vault.Key, "vault-key", "", "", "specify the vault key")
	cmd.Flags().StringVarP(&options.Requirements.Vault.ServiceAccount, "vault-sa", "", "", "specify the vault Service Account name")

	// velero
	cmd.Flags().StringVarP(&options.Requirements.Velero.ServiceAccount, "velero-sa", "", "", "specify the Velero Service Account name")
	cmd.Flags().StringVarP(&options.Requirements.Velero.Namespace, "velero-ns", "", "", "specify the Velero Namespace")

	// version stream
	cmd.Flags().StringVarP(&options.Requirements.VersionStream.URL, "version-stream-url", "", "", "specify the Version Stream git URL")
	cmd.Flags().StringVarP(&options.Requirements.VersionStream.Ref, "version-stream-ref", "", "", "specify the Version Stream git reference (branch, tag, sha)")
	return cmd
}

// Run runs the command
func (o *RequirementsOptions) Run() error {
	requirements, fileName, err := config.LoadRequirementsConfig(o.Dir)
	if err != nil {
		return err
	}
	if fileName == "" {
		fileName = filepath.Join(o.Dir, config.RequirementsConfigFileName)
	}
	o.Requirements = *requirements

	// lets re-parse the CLI arguments to re-populate the loaded requirements
	err = o.Cmd.Flags().Parse(os.Args)
	if err != nil {
		return errors.Wrap(err, "failed to reparse arguments")
	}

	err = o.applyDefaults()
	if err != nil {
		return err
	}

	err = o.Requirements.SaveConfig(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to save %s", fileName)
	}

	log.Logger().Infof("saved file: %s", util.ColorInfo(fileName))
	return nil
}

func (o *RequirementsOptions) applyDefaults() error {
	r := &o.Requirements

	gitKind := r.Cluster.GitKind
	if gitKind != "" && util.StringArrayIndex(gits.KindGits, gitKind) < 0 {
		return util.InvalidOption("git-kind", gitKind, gits.KindGits)
	}

	// override boolean flags if specified
	if o.FlagChanged("autoupgrade") {
		r.AutoUpdate.Enabled = o.Flags.AutoUpgrade
	}
	if o.FlagChanged("env-git-public") {
		r.Cluster.EnvironmentGitPublic = o.Flags.EnvironmentGitPublic
	}
	if o.FlagChanged("gitops") {
		r.GitOps = o.Flags.GitOps
	}
	if o.FlagChanged("kaniko") {
		r.Kaniko = o.Flags.Kaniko
	}
	if o.FlagChanged("terraform") {
		r.Terraform = o.Flags.Terraform
	}
	if o.FlagChanged("vault-disable-url-discover") {
		r.Vault.DisableURLDiscovery = o.Flags.VaultDisableURLDiscover
	}
	if o.FlagChanged("vault-recreate-bucket") {
		r.Vault.RecreateBucket = o.Flags.VaultRecreateBucket
	}

	// custom string types...
	if o.SecretStorage != "" {
		switch o.SecretStorage {
		case "local":
			r.SecretStorage = config.SecretStorageTypeLocal
		case "vault":
			r.SecretStorage = config.SecretStorageTypeVault
		default:
			return util.InvalidOption("secret", o.SecretStorage, config.SecretStorageTypeValues)
		}
	}
	if o.Webhook != "" {
		switch o.Webhook {
		case "jenkins":
			r.Webhook = config.WebhookTypeJenkins
		case "lighthouse":
			r.Webhook = config.WebhookTypeLighthouse
		case "prow":
			r.Webhook = config.WebhookTypeProw
		default:
			return util.InvalidOption("webhook", o.Webhook, config.WebhookTypeValues)
		}
	}

	// default flags if associated values
	if r.AutoUpdate.Schedule != "" {
		r.AutoUpdate.Enabled = true
	}
	if r.Ingress.TLS.Email != "" {
		r.Ingress.TLS.Enabled = true
	}

	// enable storage if we specify a URL
	storage := &r.Storage
	if storage.Logs.URL != "" && storage.Reports.URL == "" {
		storage.Reports.URL = storage.Logs.URL
	}
	o.defaultStorage(&storage.Backup)
	o.defaultStorage(&storage.Logs)
	o.defaultStorage(&storage.Reports)
	o.defaultStorage(&storage.Repository)
	return nil
}

func (o *RequirementsOptions) defaultStorage(storage *config.StorageEntryConfig) {
	if storage.URL != "" {
		storage.Enabled = true
	}
}
