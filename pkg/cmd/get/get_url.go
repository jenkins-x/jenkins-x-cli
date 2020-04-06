package get

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// GetURLOptions the command line options
type GetURLOptions struct {
	GetOptions

	Namespace    string
	Environment  string
	OnlyViewHost bool
}

var (
	get_url_long = templates.LongDesc(`
		Display one or more URLs from the running services.

`)

	get_url_example = templates.Examples(`
		# List all URLs in this namespace
		jx get url
	`)
)

// NewCmdGetURL creates the command
func NewCmdGetURL(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetURLOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "urls [flags]",
		Short:   "Display one or more URLs",
		Long:    get_url_long,
		Example: get_url_example,
		Aliases: []string{"url"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.AddGetUrlFlags(cmd)
	return cmd
}

func (o *GetURLOptions) AddGetUrlFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Specifies the namespace name to look inside")
	cmd.Flags().StringVarP(&o.Environment, "env", "e", "", "Specifies the Environment name to look inside")
	cmd.Flags().BoolVarP(&o.OnlyViewHost, "host", "", false, "Only displays host names of the URLs and does not open the browser")
}

// Run implements this command
func (o *GetURLOptions) Run() error {
	client, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	if o.Namespace != "" {
		ns = o.Namespace
	} else if o.Environment != "" {
		ns, err = o.FindEnvironmentNamespace(o.Environment)
		if err != nil {
			return err
		}
	}
	urls, err := services.FindServiceURLs(client, ns)
	if err != nil {
		return err
	}
	table := o.CreateTable()
	header := "URL"
	if o.OnlyViewHost {
		header = "HOST"
	}
	table.AddRow("NAME", header)

	for _, u := range urls {
		text := u.URL
		if o.OnlyViewHost {
			text = util.URLToHostName(text)
		}
		table.AddRow(u.Name, text)
	}
	table.Render()
	return nil
}
