package get

import (
	"os"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/spf13/cobra"
)

// GetLangOptions containers the CLI options
type GetLangOptions struct {
	GetOptions
	StepOptions step.StepOptions

	Pending bool
}

var (
	getPackLong = templates.LongDesc(`
		Display the pack of the current directory
`)

	getPackExample = templates.Examples(`
		# Print the lang
		jx get lang
	`)
)

// NewCmdGetLang creates the new command for: jx get env
func NewCmdGetLang(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetLangOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "lang",
		Short:   "Display the pack of the current working directory",
		Aliases: []string{"lang"},
		Long:    getPackLong,
		Example: getPackExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetLangOptions) Run() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	projectConfig, _, err := config.LoadProjectConfig(dir)

	//args := &opts.InvokeDraftPack{
	//	Dir:                dir,
	//	CustomDraftPack:    "",
	//	DisableAddFiles:    true,
	//	UseNextGenPipeline: false,
	//}
	_, err = o.StepOptions.DiscoverBuildPack(dir, projectConfig, "")
	//_, err = o.InvokeDraftPack(args)
	return err
}
