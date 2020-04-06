package get

import (
	"encoding/json"
	"fmt"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/apps"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/table"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// GetAppsOptions containers the CLI options
type GetAppsOptions struct {
	GetOptions
	Namespace  string
	ShowStatus bool
	GitOps     bool
	DevEnv     *v1.Environment
}

type appsResult struct {
	AppOutput []appOutput `json:"items"`
}

type appOutput struct {
	Name            string `yaml:"appName" json:"appName"`
	Version         string `yaml:"version" json:"version"`
	Description     string `yaml:"description" json:"description"`
	ChartRepository string `yaml:"chartRepository" json:"chartRepository"`
	Status          string `yaml:"status" json:"status"`
	Namespace       string `yaml:"namespace" json:"namespace"`
}

// HelmOutput is the representation of the Helm status command
type HelmOutput struct {
	helmInfo `yaml:"info" json:"info"`
}

type helmInfo struct {
	helmInfoStatus `yaml:"status" json:"status"`
}

type helmInfoStatus struct {
	Resources string `json:"resources" json:"resources"`
}

var (
	getAppsLong = templates.LongDesc(`
		Display installed Apps (an app is similar to an addon)
`)

	getAppsExample = templates.Examples(`
		# List all apps
		jx get apps

		# Display details about the app called cheese
		jx get app cheese
		
		# Display detailed status info about the app called cheese
		jx get app cheese --status

		# Display detailed status info about the app called cheese in 'json' format
		jx get app cheese --status -o json

		# Display details about the app called cheese in 'yaml' format
		jx get app cheese -o yaml
	`)
)

// NewCmdGetApps creates the new command for: jx get version
func NewCmdGetApps(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetAppsOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "apps",
		Short:   "Display one or more installed Apps (an app is similar to an addon)",
		Aliases: []string{"app"},
		Long:    getAppsLong,
		Example: getAppsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.AddGetFlags(cmd)
	cmd.Flags().StringVarP(&options.Namespace, opts.OptionNamespace, "n", "", "The namespace where you want to search the apps in")
	return cmd
}

// Run implements this command
func (o *GetAppsOptions) Run() error {
	o.GitOps, o.DevEnv = o.GetDevEnv()
	kubeClient, err := o.GetOptions.KubeClient()
	if err != nil {
		return err
	}
	jxClient, _, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrapf(err, "getting jx client")
	}
	envsDir, err := o.GetOptions.EnvironmentsDir()
	if err != nil {
		return errors.Wrap(err, "getting the the GitOps environments dir")
	}
	installOptions := apps.InstallOptions{
		IOFileHandles:       o.GetIOFileHandles(),
		DevEnv:              o.DevEnv,
		Verbose:             o.Verbose,
		GitOps:              o.GitOps,
		BatchMode:           o.BatchMode,
		Helmer:              o.Helm(),
		JxClient:            jxClient,
		EnvironmentCloneDir: envsDir,
	}

	if o.GetSecretsLocation() == secrets.VaultLocationKind {
		teamName, _, err := o.TeamAndEnvironmentNames()
		if err != nil {
			return err
		}
		installOptions.TeamName = teamName
		client, err := o.SystemVaultClient("")
		if err != nil {
			return err
		}
		installOptions.VaultClient = client
	}

	if o.GitOps {
		msg := "Unable to specify --%s when using GitOps for your dev environment"
		if o.Namespace != "" {
			return util.InvalidOptionf(opts.OptionNamespace, o.ReleaseName, msg, opts.OptionNamespace)
		}
		gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(o.DevEnv.Spec.Source.URL)
		if err != nil {
			return errors.Wrapf(err, "creating git provider for %s", o.DevEnv.Spec.Source.URL)
		}
		installOptions.GitProvider = gitProvider
		installOptions.Gitter = o.Git()
	}

	apps, err := installOptions.GetApps(o.Args)
	if err != nil {
		return err
	}

	if len(apps.Items) == 0 {
		if len(o.Args) > 0 {
			return errors.New("No Apps found")
		}

		fmt.Fprint(o.Out, "No Apps found\n")
		return nil
	}

	if o.Output != "" {
		appsResult := o.generateTableFormatted(apps)
		return o.renderResult(appsResult, o.Output)
	}
	table := o.generateTable(apps, kubeClient)
	table.Render()
	return nil
}

func (o *GetAppsOptions) generateAppStatusOutput(app *v1.App) error {
	name := app.Labels[helm.LabelReleaseName]
	output, err := o.Helm().StatusReleaseWithOutput(o.Namespace, name, "json")
	if err != nil {
		return err
	}
	return o.printHelmResourcesWithFormat(output)
}

func (o *GetAppsOptions) generateTableFormatted(apps *v1.AppList) appsResult {
	releases, err := o.getAppsStatus(o.GitOps, o.Namespace, apps)
	if err != nil {
		log.Logger().Warnf("There was a problem obtaining the app status: %v", err)
	}
	results := appsResult{}
	for _, app := range apps.Items {
		if app.Labels != nil {
			name := app.Labels[helm.LabelAppName]
			if name != "" && app.Annotations != nil {
				var status string
				if releaseStatus, ok := releases[name]; ok {
					status = releaseStatus
				}
				results.AppOutput = append(results.AppOutput, appOutput{
					Name:            name,
					Version:         app.Labels[helm.LabelAppVersion],
					ChartRepository: app.Annotations[helm.AnnotationAppRepository],
					Namespace:       app.Namespace,
					Status:          status,
					Description:     app.Annotations[helm.AnnotationAppDescription],
				})
			}
		}
	}
	return results
}

func (o *GetAppsOptions) generateTable(apps *v1.AppList, kubeClient kubernetes.Interface) table.Table {
	table := o.generateTableHeaders(apps)
	releases, err := o.getAppsStatus(o.GitOps, o.Namespace, apps)
	if err != nil {
		log.Logger().Warnf("There was a problem obtaining the app status: %v", err)
	}
	for _, app := range apps.Items {
		if app.Labels != nil {
			name := app.Labels[helm.LabelAppName]
			if name != "" && app.Annotations != nil {
				version := app.Labels[helm.LabelAppVersion]
				description := app.Annotations[helm.AnnotationAppDescription]
				repository := app.Annotations[helm.AnnotationAppRepository]
				var status string
				if releaseStatus, ok := releases[name]; ok {
					status = releaseStatus
				}
				namespace := app.Namespace
				row := []string{name, version, repository, namespace, status, description}
				table.AddRow(row...)
			}
		}

	}
	return table
}

func (o *GetAppsOptions) generateTableHeaders(apps *v1.AppList) table.Table {
	t := o.CreateTable()
	t.Out = o.CommonOptions.Out
	titles := []string{"Name", "Version", "Chart Repository", "Namespace", "Status", "Description"}
	t.AddRow(titles...)
	return t
}

func (o *GetAppsOptions) printHelmResourcesWithFormat(helmOutputJSON string) error {
	h := HelmOutput{}
	err := json.Unmarshal([]byte(helmOutputJSON), &h)
	if err != nil {
		return err
	}
	if o.Output == "" {
		fmt.Fprintln(o.Out, h.helmInfoStatus.Resources)
		return nil
	}
	return o.renderResult(h, o.Output)

}

func (o *GetAppsOptions) getAppsStatus(gitOps bool, namespace string, apps *v1.AppList) (map[string]string, error) {
	appsStatus := make(map[string]string)
	if o.GitOps {
		for _, a := range apps.Items {
			//In gitops, we can assume that if they app has a namespace, it has been deployed
			//Otherwise, it has just been defined in the requirements.yaml and not deployed yet
			appName := a.Labels[helm.LabelAppName]
			if a.Namespace == "" {
				appsStatus[appName] = "READY FOR DEPLOYMENT"
			} else {
				appsStatus[appName] = "DEPLOYED"
			}
		}
		return appsStatus, nil
	}

	statusReleases, _, err := o.Helm().ListReleases(namespace)
	if err != nil {
		return nil, errors.Wrap(err, "there was a problem getting the status of the apps")
	}
	for _, v := range statusReleases {
		appsStatus[v.Chart] = v.Status
	}
	return appsStatus, nil
}
