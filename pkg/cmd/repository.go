package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

type RepoOptions struct {
	*opts.CommonOptions

	Dir         string
	OnlyViewURL bool
	Quiet       bool
}

var (
	repoLong = templates.LongDesc(`
		Opens the web page for the current Git repository in a browser

		You can use the '--url' argument to just display the URL without opening it`)

	repoExample = templates.Examples(`
		# Open the Git repository in a browser
		jx repo 

		# Print the URL of the Git repository
		jx repo -u

		# Use the git URL in a script/pipeline
		export URL="$(jx repo -q -b)"
`)
)

func NewCmdRepo(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &RepoOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "repository",
		Aliases: []string{"repo"},
		Short:   "Opens the web page for the current Git repository in a browser",
		Long:    repoLong,
		Example: repoExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.OnlyViewURL, "url", "u", false, "Only displays and the URL and does not open the browser")
	cmd.Flags().BoolVarP(&options.Quiet, "quiet", "q", false, "Quiet mode just displays the git URL only for use in scripts")
	return cmd
}

func (o *RepoOptions) Run() error {
	gitInfo, provider, _, err := o.CreateGitProvider(o.Dir)
	if err != nil {
		return err
	}
	if provider == nil {
		return fmt.Errorf("No Git provider could be found. Are you in a directory containing a `.git/config` file?")
	}

	fullURL := gitInfo.HttpsURL()
	if fullURL == "" {
		return fmt.Errorf("Could not find URL from Git repository %s", gitInfo.URL)
	}
	if o.Quiet {
		fmt.Fprintln(o.Out, fullURL)
		return nil
	}
	log.Logger().Infof("repository: %s", util.ColorInfo(fullURL))
	if !o.OnlyViewURL {
		browser.OpenURL(fullURL)
	}
	return nil
}
