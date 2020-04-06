package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// GetGitOptions the command line options
type GetGitOptions struct {
	GetOptions
}

var (
	get_git_long = templates.LongDesc(`
		Display the Git server URLs.

`)

	get_git_example = templates.Examples(`
		# List all registered Git server URLs
		jx get git
	`)
)

// NewCmdGetGit creates the command
func NewCmdGetGit(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetGitOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "git [flags]",
		Short:   "Display the current registered Git service URLs",
		Long:    get_git_long,
		Example: get_git_example,
		Aliases: []string{"gitserver"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	return cmd
}

// Run implements this command
func (o *GetGitOptions) Run() error {
	authConfigSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	config := authConfigSvc.Config()

	table := o.CreateTable()
	table.AddRow("Name", "Kind", "URL")

	for _, s := range config.Servers {
		kind := s.Kind
		if kind == "" {
			kind = "github"
		}
		table.AddRow(s.Name, kind, s.URL)
	}
	table.Render()
	return nil
}
