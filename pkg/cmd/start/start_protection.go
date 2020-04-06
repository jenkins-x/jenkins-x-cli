package start

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/prow"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// StartProtectionOptions contains the command line options
type StartProtectionOptions struct {
	*opts.CommonOptions

	Tail   bool
	Filter string

	Jobs map[string]gojenkins.Job
}

var (
	start_protection_long = templates.LongDesc(`
		Starts protection checking on an app

`)

	start_protection_example = templates.Examples(`
		# Start protection
		jx start protection <context> <org/repo>

		# For example, to enable compliance on a repopository called "example" in the "acme" organization
        jx start protection compliance-check acme/example

	`)
)

// NewCmdStartProtection creates the command
func NewCmdStartProtection(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StartProtectionOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "protection [flags]",
		Short:   "Starts protection",
		Long:    start_protection_long,
		Example: start_protection_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	// TODO once we support get pipelines for Prow we can add support for a selector
	//cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filters all the available jobs by those that contain the given text")

	return cmd
}

// Run implements this command
func (o *StartProtectionOptions) Run() error {
	kClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}
	if len(o.Args) == 0 {
		return fmt.Errorf("No context specified.\n\n%s", start_protection_example)
	}
	if len(o.Args) == 1 {
		return fmt.Errorf("No org/repo specified.\n\n%s", start_protection_example)
	}
	if len(o.Args) > 2 {
		return fmt.Errorf("Too many arguments.\n\n%s", start_protection_example)
	}
	orgrepo := o.Args[1]
	context := o.Args[0]

	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}

	err = prow.AddProtection(kClient, []string{orgrepo}, context, ns, settings)
	if err != nil {
		return err
	}
	log.Logger().Infof("%s enabled for %s", util.ColorInfo(context), util.ColorInfo(orgrepo))
	return nil
}
