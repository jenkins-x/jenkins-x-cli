package create

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
)

var (
	createBranchPatternLong = templates.LongDesc(`
		Create a git branch pattern for your team. 

		The pattern should match all the branches you wish to automate CI/CD on when creating or importing projects.

		For more documentation see: [https://jenkins-x.io/docs/using-jx/creating/import/#branch-patterns](https://jenkins-x.io/docs/using-jx/creating/import/#branch-patterns)
`)

	createBranchPatternExample = templates.Examples(`
		# Create a branch pattern for your team 
		jx create branch pattern "master|develop|PR-.*"

	`)
)

// CreateBranchPatternOptions the options for the create spring command
type CreateBranchPatternOptions struct {
	options.CreateOptions

	BranchPattern string
}

// NewCmdCreateBranchPattern creates a command object for the "create" command
func NewCmdCreateBranchPattern(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateBranchPatternOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     opts.BranchPatternCommandName,
		Short:   "Create a git branch pattern for your team",
		Aliases: opts.BranchPatternCommandAliases,
		Long:    createBranchPatternLong,
		Example: createBranchPatternExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	return cmd
}

// Run implements the command
func (o *CreateBranchPatternOptions) Run() error {
	if len(o.Args) == 0 {
		return fmt.Errorf("Missing argument for the branch pattern")
	}
	arg := o.Args[0]

	callback := func(env *v1.Environment) error {
		env.Spec.TeamSettings.BranchPatterns = arg
		log.Logger().Infof("Setting the team branch pattern to: %s", util.ColorInfo(arg))
		return nil
	}
	return o.ModifyDevEnvironment(callback)
}
