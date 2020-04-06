package deletecmd

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// DeleteOptions are the flags for delete commands
type DeleteOptions struct {
	*opts.CommonOptions
}

var (
	delete_long = templates.LongDesc(`
		Deletes one or more resources.

`)

	delete_example = templates.Examples(`
		# Delete an environment
		jx delete env staging
	`)
)

// NewCmdDelete creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdDelete(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteOptions{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "delete TYPE [flags]",
		Short:   "Deletes one or more resources",
		Long:    delete_long,
		Example: delete_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		SuggestFor: []string{"remove", "rm", "del"},
	}

	cmd.AddCommand(NewCmdDeleteAddon(commonOpts))
	cmd.AddCommand(NewCmdDeleteApplication(commonOpts))
	cmd.AddCommand(NewCmdDeleteApp(commonOpts))
	cmd.AddCommand(NewCmdDeleteBranch(commonOpts))
	cmd.AddCommand(NewCmdDeleteChat(commonOpts))
	cmd.AddCommand(NewCmdDeleteContext(commonOpts))
	cmd.AddCommand(NewCmdDeleteDevPod(commonOpts))
	cmd.AddCommand(NewCmdDeleteEnv(commonOpts))
	cmd.AddCommand(NewCmdDeleteGit(commonOpts))
	cmd.AddCommand(NewCmdDeleteGke(commonOpts))
	cmd.AddCommand(NewCmdDeleteJenkins(commonOpts))
	cmd.AddCommand(NewCmdDeleteNamespace(commonOpts))
	cmd.AddCommand(NewCmdDeletePostPreviewJob(commonOpts))
	cmd.AddCommand(NewCmdDeletePreview(commonOpts))
	cmd.AddCommand(NewCmdDeleteQuickstartLocation(commonOpts))
	cmd.AddCommand(NewCmdDeleteRepo(commonOpts))
	cmd.AddCommand(NewCmdDeleteToken(commonOpts))
	cmd.AddCommand(NewCmdDeleteTeam(commonOpts))
	cmd.AddCommand(NewCmdDeleteTracker(commonOpts))
	cmd.AddCommand(NewCmdDeleteUser(commonOpts))
	cmd.AddCommand(NewCmdDeleteVault(commonOpts))
	cmd.AddCommand(NewCmdDeleteExtension(commonOpts))
	return cmd
}

// Run implements this command
func (o *DeleteOptions) Run() error {
	return o.Cmd.Help()
}
