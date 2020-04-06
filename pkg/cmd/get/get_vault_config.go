package get

import (
	"fmt"
	"runtime"

	"github.com/jenkins-x/jx/pkg/vault"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

type GetVaultConfigOptions struct {
	GetOptions

	Namespace string
	Name      string
	terminal  string
}

func (o *GetVaultConfigOptions) VaultName() string {
	return o.Name
}

func (o *GetVaultConfigOptions) VaultNamespace() string {
	return o.Namespace
}

var (
	getVaultConfigLong = templates.LongDesc(`
		Echoes the configuration required for connecting to a vault using the official vault CLI client	
	`)

	getVaultConfigExample = templates.Examples(`
		# Gets vault config
		jx get vault-config
	`)
)

// NewCmdGetVaultConfig creates a new command for 'jx get secrets'
func NewCmdGetVaultConfig(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetVaultConfigOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "vault-config",
		Short:   "Gets the configuration for using the official vault CLI client",
		Long:    getVaultConfigLong,
		Example: getVaultConfigExample,
		Run: func(c *cobra.Command, args []string) {
			options.Cmd = c
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "Namespace from where to get the vault config")
	cmd.Flags().StringVarP(&options.Name, "name", "m", "", "Name of the vault to get the config for")
	cmd.Flags().StringVarP(&options.terminal, "terminal", "t", "", "terminal type output override. Values: ['sh', 'cmd'].")
	return cmd
}

// Run implements the command
func (o *GetVaultConfigOptions) Run() error {
	vaultClient, err := o.VaultClient(o.Name, o.Namespace) // Will use defaults if empty strings specified
	if err != nil {
		return err
	}

	// Install the vault CLI for the user
	vault.InstallVaultCli()

	url, token, err := vaultClient.Config()
	// Echo the client config out to the command line to be piped into bash
	if o.terminal == "" {
		if runtime.GOOS == "windows" {
			o.terminal = "cmd"
		} else {
			o.terminal = "sh"
		}
	}
	if o.terminal == "cmd" {
		_, _ = fmt.Fprintf(o.Out, "set VAULT_ADDR=%s\nset VAULT_TOKEN=%s\n", url.String(), token)
	} else {
		_, _ = fmt.Fprintf(o.Out, "export VAULT_ADDR=%s\nexport VAULT_TOKEN=%s\n", url.String(), token)
	}

	return err
}
