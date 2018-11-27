package cmd

import (
	"fmt"
	"github.com/jenkins-x/jx/pkg/jx/cmd/templates"
	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
	"io"
	"runtime"
)

type GetVaultConfigOptions struct {
	GetOptions

	namespace string
	name      string
	terminal  string
}

func (o *GetVaultConfigOptions) VaultName() string {
	return o.name
}

func (o *GetVaultConfigOptions) VaultNamespace() string {
	return o.namespace
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
func NewCmdGetVaultConfig(f Factory, in terminal.FileReader, out terminal.FileWriter, errOut io.Writer) *cobra.Command {
	options := &GetVaultConfigOptions{
		GetOptions: GetOptions{
			CommonOptions: CommonOptions{
				Factory: f,
				In:      in,
				Out:     out,
				Err:     errOut,
			},
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
			CheckErr(err)
		},
	}

	options.addGetFlags(cmd)

	cmd.Flags().StringVarP(&options.namespace, "namespace", "n", "", "Namespace from where to get the vault config")
	cmd.Flags().StringVarP(&options.name, "name", "m", "", "Name of the vault to get the config for")
	cmd.Flags().StringVarP(&options.terminal, "terminal", "t", "", "terminal type output override. Values: ['sh', 'cmd'].")
	return cmd
}

// Run implements the command
func (o *GetVaultConfigOptions) Run() error {
	client, err := vault.NewVaulter(o)
	if err != nil {
		return err
	}

	url, token, err := client.Config()
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
