package create

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
)

const (
	DefaultEnvCtrlReleaseName = "jxet"
	DefaultEnvCtrlNamespace   = "jx"
)

var (
	createAddonEnvironmentControllerLong = templates.LongDesc(`
		Create an Environment Controller to handle webhooks and promote changes from GitOps
`)

	createAddonEnvironmentControllerExample = templates.Examples(`
		# Creates the environment controller using a specific environment git repository, project, git user, chart repo
		jx create addon envctl -s https://github.com/myorg/env-production.git --project-id myproject --docker-registry gcr.io --cluster-rbac true --user mygituser --token mygittoken

	`)
)

// CreateAddonEnvironmentControllerOptions the options for the create spring command
type CreateAddonEnvironmentControllerOptions struct {
	CreateAddonOptions

	Namespace   string
	Version     string
	ReleaseName string
	SetValues   string
	Timeout     int
	InitOptions initcmd.InitOptions

	// chart parameters
	WebHookURL        string
	GitSourceURL      string
	GitKind           string
	GitUser           string
	GitToken          string
	BuildPackURL      string
	BuildPackRef      string
	ClusterRBAC       bool
	NoClusterAdmin    bool
	ProjectID         string
	DockerRegistry    string
	DockerRegistryOrg string
}

// NewCmdCreateAddonEnvironmentController creates a command object for the "create" command
func NewCmdCreateAddonEnvironmentController(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonEnvironmentControllerOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "environment controller",
		Short:   "Create an Environment Controller to handle webhooks and promote changes from GitOps",
		Aliases: []string{"envctl"},
		Long:    createAddonEnvironmentControllerLong,
		Example: createAddonEnvironmentControllerExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to install the controller")
	cmd.Flags().StringVarP(&options.ReleaseName, optionRelease, "r", DefaultEnvCtrlReleaseName, "The chart release name")
	cmd.Flags().StringVarP(&options.SetValues, "set", "", "", "The chart set values (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().StringVarP(&options.Version, "version", "", "", "The version of the chart to use - otherwise the latest version is used")
	cmd.Flags().IntVarP(&options.Timeout, "timeout", "", 600000, "The timeout value for how long to wait for the install to succeed")
	cmd.Flags().StringVarP(&options.GitSourceURL, "source-url", "s", "", "The git URL of the environment repository to promote from")
	cmd.Flags().StringVarP(&options.GitKind, "git-kind", "", "", "The kind of git repository. Should be one of: "+strings.Join(gits.KindGits, ", "))
	cmd.Flags().StringVarP(&options.GitUser, "user", "u", "", "The git user to use to clone and tag the git repository")
	cmd.Flags().StringVarP(&options.GitToken, "token", "t", "", "The git token to clone and tag the git repository")
	cmd.Flags().StringVarP(&options.WebHookURL, "webhook-url", "w", "", "The webhook URL used to expose the exposecontroller and register with the git provider's webhooks")
	cmd.Flags().StringVarP(&options.BuildPackURL, "buildpack-url", "", "", "The URL for the build pack Git repository")
	cmd.Flags().StringVarP(&options.BuildPackRef, "buildpack-ref", "", "", "The Git reference (branch,tag,sha) in the Git repository to use")
	cmd.Flags().StringVarP(&options.ProjectID, "project-id", "", "", "The cloud project ID")
	cmd.Flags().BoolVarP(&options.ClusterRBAC, "cluster-rbac", "", false, "Whether to enable cluster level RBAC on Tekton")
	cmd.Flags().StringVarP(&options.InitOptions.Flags.UserClusterRole, "cluster-role", "", "cluster-admin", "The cluster role for the current user to be able to install Cluster RBAC based Environment Controller")
	cmd.Flags().BoolVarP(&options.NoClusterAdmin, "no-cluster-admin", "", false, "If using cluster RBAC the current user needs 'cluster-admin' karma which this command will add if its possible")
	cmd.Flags().StringVarP(&options.DockerRegistry, "docker-registry", "", "", "The Docker Registry host name to use which is added as a prefix to docker images")
	cmd.Flags().StringVarP(&options.DockerRegistryOrg, "docker-registry-org", "", "", "The Docker registry organisation. If blank the git repository owner is used")
	return cmd
}

// Run implements the command
func (o *CreateAddonEnvironmentControllerOptions) Run() error {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return errors.Wrap(err, "failed to create the API extensions client")
	}
	err = kube.RegisterPipelineCRDs(apisClient)
	if err != nil {
		return errors.Wrap(err, "failed to register the Jenkins X Pipeline CRDs")
	}

	// validate the git URL
	if o.GitSourceURL == "" && !o.BatchMode {
		o.GitSourceURL, err = util.PickValue("git repository to promote from: ", "", true, "please specify the GitOps repository used to store the kubernetes applications to deploy to this cluster", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}
	if o.GitSourceURL == "" {
		return util.MissingOption("source-url")
	}
	gitInfo, err := gits.ParseGitURL(o.GitSourceURL)
	if err != nil {
		return err
	}
	serverURL := gitInfo.ProviderURL()
	if o.GitKind == "" {
		o.GitKind = gits.SaasGitKind(serverURL)
	}
	if o.GitKind == "" && !o.BatchMode {
		o.GitKind, err = util.PickName(gits.KindGits, "kind of git repository: ", "please specify the GitOps repository used to store the kubernetes applications to deploy to this cluster", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}
	if o.GitKind == "" {
		return util.MissingOption("git-kind")
	}

	_, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	if o.Namespace == "" {
		o.Namespace = ns
	}

	// lets ensure there's a dev environment setup for no-tiller mode
	err = o.ModifyDevEnvironment(func(env *v1.Environment) error {
		env.Spec.TeamSettings.HelmTemplate = true
		env.Spec.TeamSettings.PromotionEngine = v1.PromotionEngineProw
		env.Spec.TeamSettings.ProwEngine = v1.ProwEngineTypeTekton
		env.Spec.WebHookEngine = v1.WebHookEngineProw
		return nil
	})
	if err != nil {
		return err
	}
	err = o.ModifyEnvironment(kube.LabelValueThisEnvironment, func(env *v1.Environment) error {
		env.Spec.TeamSettings.HelmTemplate = true
		env.Spec.TeamSettings.PromotionEngine = v1.PromotionEngineProw
		env.Spec.TeamSettings.ProwEngine = v1.ProwEngineTypeTekton
		env.Spec.WebHookEngine = v1.WebHookEngineProw
		env.Spec.Kind = v1.EnvironmentKindTypePermanent
		env.Spec.Order = 100
		env.Spec.PromotionStrategy = v1.PromotionStrategyTypeAutomatic
		env.Spec.Source.URL = o.GitSourceURL
		return nil
	})
	if err != nil {
		return err
	}

	if o.ClusterRBAC && !o.NoClusterAdmin {
		io := &o.InitOptions
		io.CommonOptions = o.CommonOptions
		err = io.EnableClusterAdminRole()
		if err != nil {
			return err
		}
	}

	// avoid needing a dev cluster
	o.EnableRemoteKubeCluster()

	authSvc, err := o.GitAuthConfigService()
	if err != nil {
		return err
	}
	config, err := authSvc.LoadConfig()
	if err != nil {
		return err
	}
	server := config.GetOrCreateServer(serverURL)

	if o.GitUser == "" {
		auth, err := o.PickPipelineUserAuth(config, server)
		if err != nil {
			return err
		}
		if auth == nil {
			return fmt.Errorf("no user found for git server %s", serverURL)
		}
		o.GitUser = auth.Username
		if o.GitToken == "" {
			o.GitToken = auth.ApiToken
		}
	}
	if o.GitToken == "" {
		auth := server.GetUserAuth(o.GitUser)
		if auth != nil {
			o.GitToken = auth.ApiToken
		} else {
			return util.MissingOption("token")
		}
	}
	if o.GitUser == "" {
		return util.MissingOption("user")
	}
	if o.GitToken == "" {
		return util.MissingOption("token")
	}

	setValues := []string{}
	if o.SetValues != "" {
		setValues = strings.Split(o.SetValues, ",")
	}
	if o.WebHookURL != "" {
		setValues = append(setValues, "webhookUrl="+o.WebHookURL)
	}
	setValues = append(setValues, "source.owner="+gitInfo.Organisation)
	setValues = append(setValues, "source.repo="+gitInfo.Name)
	setValues = append(setValues, "source.serverUrl="+serverURL)
	setValues = append(setValues, "tekton.auth.git.url="+serverURL)
	setValues = append(setValues, "source.gitKind="+o.GitKind)
	setValues = append(setValues, "source.user="+o.GitUser)
	setValues = append(setValues, "tekton.auth.git.username="+o.GitUser)
	setValues = append(setValues, "source.token="+o.GitToken)
	setValues = append(setValues, "tekton.auth.git.password="+o.GitToken)
	if o.ProjectID != "" {
		setValues = append(setValues, "projectId="+o.ProjectID)
	}
	if o.BuildPackURL != "" {
		setValues = append(setValues, "buildPackURL="+o.BuildPackURL)
	}
	if o.BuildPackRef != "" {
		setValues = append(setValues, "buildPackRef="+o.BuildPackRef)
	}
	if o.DockerRegistry != "" {
		setValues = append(setValues, "dockerRegistry="+o.DockerRegistry)
	}
	if o.DockerRegistryOrg != "" {
		setValues = append(setValues, "dockerRegistryOrg="+o.DockerRegistryOrg)
	}
	setValues = append(setValues, "tekton.rbac.cluster="+strconv.FormatBool(o.ClusterRBAC))

	log.Logger().Infof("installing the Environment Controller with values: %s", util.ColorInfo(strings.Join(setValues, ",")))
	helmOptions := helm.InstallChartOptions{
		Chart:       "environment-controller",
		ReleaseName: o.ReleaseName,
		Version:     o.Version,
		Ns:          o.Namespace,
		SetValues:   setValues,
		HelmUpdate:  true,
		Repository:  kube.DefaultChartMuseumURL,
	}
	err = o.InstallChartWithOptions(helmOptions)
	if err != nil {
		return err
	}
	log.Logger().Infof("installed the Environment Controller!")
	return nil
}
