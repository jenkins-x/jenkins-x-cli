package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/vault"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type GetSecretOptions struct {
	GetOptions

	Namespace string
	Name      string
}

func (o *GetSecretOptions) VaultName() string {
	return o.Name
}

func (o *GetSecretOptions) VaultNamespace() string {
	return o.Namespace
}

var (
	getSecretLong = templates.LongDesc(`
		Display one or more Vault Secrets	
	`)

	getSecretExample = templates.Examples(`
		# List all secrets
		jx get secrets
	`)
)

// NewCmdGetSecret creates a new command for 'jx get secrets'
func NewCmdGetSecret(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetSecretOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "secrets",
		Short:   "Display one or more Secrets",
		Long:    getSecretLong,
		Example: getSecretExample,
		Run: func(c *cobra.Command, args []string) {
			options.Cmd = c
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)

	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "Namespace from where to list the secrets")
	cmd.Flags().StringVarP(&options.Name, "name", "m", "", "The name of the Vault to use")
	return cmd
}

// Run implements the command
func (o *GetSecretOptions) Run() error {
	var vaultClient vault.Client
	var err error
	if o.Name != "" && o.Namespace != "" {
		vaultClient, err = o.VaultClient(o.Name, o.Namespace)
	} else {
		vaultClient, err = o.SystemVaultClient("")
	}
	if err != nil {
		return errors.Wrap(err, "retrieving the vault client")
	}
	secrets, err := vaultClient.List("")
	if err != nil {
		return errors.Wrap(err, "listing all secrets in vault")
	}

	table := o.CreateTable()
	table.AddRow("KEY")
	for _, secret := range secrets {
		table.AddRow(secret)
	}
	table.Render()

	return nil
}
