package upgrade

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"

	opts_upgrade "github.com/jenkins-x/jx/pkg/cmd/opts/upgrade"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	upgradeIngressLong = templates.LongDesc(`
		Upgrades the Jenkins X Ingress rules
`)

	upgradeIngressExample = templates.Examples(`
		# Upgrades the Jenkins X Ingress rules
		jx upgrade ingress
	`)
)

// NewCmdUpgradeIngress defines the command
func NewCmdUpgradeIngress(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &opts_upgrade.UpgradeIngressOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "ingress",
		Short:   "Upgrades Ingress rules",
		Aliases: []string{"ing"},
		Long:    upgradeIngressLong,
		Example: upgradeIngressExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	addFlags(options, cmd)

	return cmd
}

func addFlags(o *opts_upgrade.UpgradeIngressOptions, cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.Cluster, "cluster", "", false, "Enable cluster wide Ingress upgrade")
	cmd.Flags().StringArrayVarP(&o.Namespaces, "namespaces", "", []string{}, "Namespaces to upgrade")
	cmd.Flags().BoolVarP(&o.SkipCertManager, "skip-certmanager", "", false, "Skips cert-manager installation")
	cmd.Flags().StringArrayVarP(&o.Services, "services", "", []string{}, "Services to upgrade")
	cmd.Flags().BoolVarP(&o.SkipResourcesUpdate, "skip-resources-update", "", false, "Skips the update of jx related resources such as webhook or Jenkins URL")
	cmd.Flags().BoolVarP(&o.Force, "force", "", false, "Forces upgrades of all webooks even if ingress URL has not changed")
	cmd.Flags().BoolVarP(&o.WaitForCerts, "wait-for-certs", "", true, "Waits for TLS certs to be issued by cert-manager")
	cmd.Flags().StringVarP(&o.ConfigNamespace, "config-namespace", "", "", "Namespace where the ingress-config is stored (if empty, it will try to read it from Dev environment namespace)")
	cmd.Flags().StringVarP(&o.IngressConfig.Domain, "domain", "", "", "Domain to expose ingress endpoints (e.g., jenkinsx.io). Leave empty to preserve the current value.")
	cmd.Flags().StringVarP(&o.IngressConfig.UrlTemplate, "urltemplate", "", "", "For ingress; exposers can set the urltemplate to expose. The default value is \"{{.Service}}.{{.Namespace}}.{{.Domain}}\". Leave empty to preserve the current value.")
}
