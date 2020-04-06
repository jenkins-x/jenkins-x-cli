package get

import (
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"
)

// GetBuildPodsOptions the command line options
type GetBuildPodsOptions struct {
	GetOptions

	Namespace   string
	BuildFilter builds.BuildPodInfoFilter
}

var (
	getBiuldPodsLong = templates.LongDesc(`
		Display the Tekton build pods

`)

	getBiuldPodsExample = templates.Examples(`
		# List all the Tekton build pods
		jx get build pods

		# List all the pending Tekton build pods 
		jx get build pods -p

		# List all the Tekton build pods for a given repository
		jx get build pods --repo cheese

		# List all the pending Tekton build pods for a given repository
		jx get build pods --repo cheese -p

		# List all the Tekton build pods for a given Pull Request
		jx get build pods --repo cheese --branch PR-1234
	`)
)

// NewCmdGetBuildPods creates the command
func NewCmdGetBuildPods(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetBuildPodsOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "pods [flags]",
		Short:   "Displays the build pods and their details",
		Long:    getBiuldPodsLong,
		Example: getBiuldPodsExample,
		Aliases: []string{"pod"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to look for the build pods. Defaults to the current namespace")
	cmd.Flags().BoolVarP(&options.BuildFilter.Pending, "pending", "p", false, "Filter builds which are currently pending or running")
	cmd.Flags().StringVarP(&options.BuildFilter.Filter, "filter", "f", "", "Filters the build name by the given text")
	cmd.Flags().StringVarP(&options.BuildFilter.Owner, "owner", "o", "", "Filters the owner (person/organisation) of the repository")
	cmd.Flags().StringVarP(&options.BuildFilter.Repository, "repo", "r", "", "Filters the build repository")
	cmd.Flags().StringVarP(&options.BuildFilter.Branch, "branch", "", "", "Filters the branch")
	cmd.Flags().StringVarP(&options.BuildFilter.Build, "build", "", "", "Filter a specific build number")
	cmd.Flags().StringVarP(&options.BuildFilter.Context, "context", "", "", "Filters the context of the build")
	cmd.Flags().StringVarP(&options.BuildFilter.GitURL, "giturl", "g", "", "The git URL to filter on. If you specify a link to a github repository or PR we can filter the query of build pods accordingly")
	return cmd
}

// Run implements this command
func (o *GetBuildPodsOptions) Run() error {
	err := o.BuildFilter.Validate()
	if err != nil {
		return err
	}
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}
	if o.Namespace != "" {
		ns = o.Namespace
	}
	pods, err := builds.GetBuildPods(kubeClient, ns)
	if err != nil {
		log.Logger().Warnf("Failed to query pods %s", err)
		return err
	}

	table := o.CreateTable()
	table.AddRow("OWNER", "REPOSITORY", "BRANCH", "BUILD", "CONTEXT", "AGE", "STATUS", "POD", "GIT URL")

	buildInfos := []*builds.BuildPodInfo{}
	for _, pod := range pods {
		buildInfo := builds.CreateBuildPodInfo(pod)
		if o.BuildFilter.BuildMatches(buildInfo) {
			buildInfos = append(buildInfos, buildInfo)
		}
	}
	builds.SortBuildPodInfos(buildInfos)

	now := time.Now()
	for _, build := range buildInfos {
		duration := strings.TrimSuffix(now.Sub(build.CreatedTime).Round(time.Minute).String(), "0s")

		table.AddRow(build.Organisation, build.Repository, build.Branch, build.Build, build.Context, duration, build.Status(), build.PodName, build.GitURL)
	}
	table.Render()
	return nil
}
