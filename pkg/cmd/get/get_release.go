package get

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

// GetReleaseOptions containers the CLI options
type GetReleaseOptions struct {
	GetOptions

	Filter    string
	Namespace string
}

var (
	getReleaseLong = templates.LongDesc(`
		Display one or more Releases
`)

	getReleaseExample = templates.Examples(`
		# List the recent releases done by this team
		jx get release

		# Filter the releases 
		jx get release -f myapp
	`)
)

// NewCmdGetRelease creates the new command for: jx get env
func NewCmdGetRelease(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetReleaseOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "releases",
		Short:   "Display the Release or Releases the current user is a member of",
		Aliases: []string{"release"},
		Long:    getReleaseLong,
		Example: getReleaseExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filter the releases with the given text")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to view or defaults to the current namespace")

	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetReleaseOptions) Run() error {
	jxClient, curNs, err := o.JXClient()
	if err != nil {
		return err
	}
	ns := o.Namespace
	if ns == "" {
		ns = curNs
	}
	releases, err := kube.GetOrderedReleases(jxClient, ns, o.Filter)
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		suffix := ""
		if o.Filter != "" {
			suffix = fmt.Sprintf(" for filter: %s", util.ColorInfo(o.Filter))
		}
		log.Logger().Infof("No Releases found in namespace %s%s.", util.ColorInfo(ns), suffix)
		log.Logger().Infof("To create a release try merging code to a master branch to trigger a pipeline or try: %s", util.ColorInfo("jx start build"))
		return nil
	}
	table := o.CreateTable()
	table.AddRow("NAME", "VERSION")
	for _, release := range releases {
		table.AddRow(release.Spec.Name, release.Spec.Version)
	}
	table.Render()
	return nil
}
