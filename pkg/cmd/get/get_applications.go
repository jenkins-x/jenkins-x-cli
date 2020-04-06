package get

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/applications"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/table"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"

	"github.com/spf13/cobra"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"k8s.io/api/apps/v1beta1"
)

// GetApplicationsOptions containers the CLI options
type GetApplicationsOptions struct {
	*opts.CommonOptions

	Namespace   string
	Environment string
	HideUrl     bool
	HidePod     bool
	Previews    bool
}

// Applications is a map indexed by the application name then the environment name
// Applications map[string]map[string]*ApplicationEnvironmentInfo

// EnvApps contains data about app deployments in an environment
type EnvApps struct {
	Environment v1.Environment
	Apps        map[string]v1beta1.Deployment
}

// ApplicationEnvironmentInfo contains the results of an app for an environment
type ApplicationEnvironmentInfo struct {
	Deployment  *v1beta1.Deployment
	Environment *v1.Environment
	Version     string
	URL         string
}

var (
	getVersionLong = templates.LongDesc(`
		Display applications across environments.
`)

	getVersionExample = templates.Examples(`
		# List applications, their URL and pod counts for all environments
		jx get applications

		# List applications only in the Staging environment
		jx get applications -e staging

		# List applications only in the Production environment
		jx get applications -e production

		# List applications only in a specific namespace
		jx get applications -n jx-staging

		# List applications hiding the URLs
		jx get applications -u

		# List applications just showing the versions (hiding urls and pod counts)
		jx get applications -u -p
	`)
)

// NewCmdGetApplications creates the new command for: jx get version
func NewCmdGetApplications(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetApplicationsOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "applications",
		Short:   "Display one or more Applications and their versions",
		Aliases: []string{"application", "version", "versions"},
		Long:    getVersionLong,
		Example: getVersionExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.HideUrl, "url", "u", false, "Hide the URLs")
	cmd.Flags().BoolVarP(&options.HidePod, "pod", "p", false, "Hide the pod counts")
	cmd.Flags().BoolVarP(&options.Previews, "preview", "w", false, "Show preview environments only")
	cmd.Flags().StringVarP(&options.Environment, "env", "e", "", "Filter applications in the given environment")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "Filter applications in the given namespace")
	return cmd
}

// Run implements this command
func (o *GetApplicationsOptions) Run() error {
	if o.Previews {
		fmt.Println("The `--preview` flag has been deprecated from this command, use instead `jx get previews`")
		return nil
	}

	list, err := applications.GetApplications(o.CommonOptions.GetFactory())
	if err != nil {
		return errors.Wrap(err, "fetching applications")
	}
	if len(list.Items) == 0 {
		log.Logger().Infof("No applications found")
		return nil
	}

	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	table := o.generateTable(kubeClient, list)
	table.Render()

	return nil
}

func (o *GetApplicationsOptions) generateTable(kubeClient kubernetes.Interface, list applications.List) table.Table {
	table := o.generateTableHeaders(list)

	for _, a := range list.Items {
		row := []string{}
		name := a.Name()

		if len(a.Environments) > 0 {

			for _, k := range o.sortedKeys(list.Environments()) {

				if ae, ok := a.Environments[k]; ok {
					for _, d := range ae.Deployments {
						name = kube.GetAppName(d.Deployment.Name, k)
						if ae.Environment.Spec.Kind == v1.EnvironmentKindTypeEdit {
							name = kube.GetEditAppName(name)
						} else if ae.Environment.Spec.Kind == v1.EnvironmentKindTypePreview {
							name = ae.Environment.Spec.PullRequestURL
						}
						if !ae.IsPreview() {
							row = append(row, d.Version())
						}
						if !o.HidePod {
							row = append(row, d.Pods())
						}
						if !o.HideUrl {
							row = append(row, d.URL(kubeClient, a))
						}
					}
				} else {
					if !ae.IsPreview() {
						row = append(row, "")
					}
					if !o.HidePod {
						row = append(row, "")
					}
					if !o.HideUrl {
						row = append(row, "")
					}
				}
			}
			row = append([]string{name}, row...)

			table.AddRow(row...)
		}
	}
	return table
}

func envTitleName(e v1.Environment) string {
	if e.Spec.Kind == v1.EnvironmentKindTypeEdit {
		return "Edit"
	}

	return e.Name
}

func (o *GetApplicationsOptions) sortedKeys(envs map[string]v1.Environment) []string {
	keys := make([]string, 0, len(envs))
	for k, env := range envs {
		if (o.Environment == "" || o.Environment == k) && (o.Namespace == "" || o.Namespace == env.Spec.Namespace) {
			keys = append(keys, k)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	return keys
}

func (o *GetApplicationsOptions) generateTableHeaders(list applications.List) table.Table {
	t := o.CreateTable()
	title := "APPLICATION"
	titles := []string{title}

	envs := list.Environments()

	for _, k := range o.sortedKeys(envs) {
		titles = append(titles, strings.ToUpper(envTitleName(envs[k])))

		if !o.HidePod {
			titles = append(titles, "PODS")
		}
		if !o.HideUrl {
			titles = append(titles, "URL")
		}
	}
	t.AddRow(titles...)
	return t
}
