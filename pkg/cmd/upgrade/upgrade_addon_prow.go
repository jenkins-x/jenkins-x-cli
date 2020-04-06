package upgrade

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/hashicorp/go-version"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/environments"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/helm/pkg/proto/hapi/chart"
)

var (
	upgradeAddonProwLong = templates.LongDesc(`
		Upgrades the Jenkins X platform if there is a newer release
`)

	upgradeAddonProwExample = templates.Examples(`
		# Upgrades the Jenkins X platform
		jx upgrade addon prow
	`)
)

// UpgradeAddonProwOptions the options for the create spring command
type UpgradeAddonProwOptions struct {
	UpgradeAddonsOptions

	newKnativeBuildVersion string
	Tekton                 bool
	ExternalDNS            bool

	// Used for testing
	CloneDir string
}

// NewCmdUpgradeAddonProw defines the command
func NewCmdUpgradeAddonProw(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeAddonProwOptions{
		UpgradeAddonsOptions: UpgradeAddonsOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "prow",
		Short:   "Upgrades any AddonProw added to Jenkins X if there are any new releases available",
		Aliases: []string{"addon"},
		Long:    upgradeAddonProwLong,
		Example: upgradeAddonProwExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.UpgradeAddonsOptions.addFlags(cmd)
	options.InstallFlags.AddCloudEnvOptions(cmd)
	cmd.Flags().StringVarP(&options.newKnativeBuildVersion, "new-knative-build-version", "", "0.1.1", "The new kanative build verion that prow needs to work with")
	cmd.Flags().BoolVarP(&options.Tekton, "tekton", "t", true, "Enables Knative Build Pipeline. Otherwise we default to use Knative Build")
	cmd.Flags().BoolVarP(&options.ExternalDNS, "external-dns", "", true, "Installs external-dns into the cluster. ExternalDNS manages service DNS records for your cluster, providing you've setup your domain record")
	return cmd
}

// Run implements the command
func (o *UpgradeAddonProwOptions) Run() error {
	err := o.Helm().UpdateRepo()
	if err != nil {
		return err
	}
	ns := o.Namespace
	if ns == "" {
		_, ns, err = o.JXClientAndDevNamespace()
		o.Namespace = ns
		if err != nil {
			return err
		}
	}

	releases, _, err := o.Helm().ListReleases(ns)
	if err != nil {
		return err
	}

	newKnativeBuildVersion, err := version.NewVersion(o.newKnativeBuildVersion)
	if err != nil {
		return err
	}

	for _, release := range releases {
		if release.ReleaseName == "knative-build" && (release.Status == "DEPLOYED" || release.Status == "FAILED") {
			currentVersion, err := version.NewVersion(release.ChartVersion)
			if err != nil {
				return err
			}
			// if we currently have less than 0.2.x of Knative build chart we need to reinstall as there's issues with
			// the CRDs when upgrading
			if currentVersion.LessThan(newKnativeBuildVersion) {
				message := "The version of Knative Build you are running is too old to support the latest Prow, would " +
					"you like to install the latest Knative Build?\nWARNING: this will remove the previous version and " +
					"install the latest, any existing builds or custom changes to BuildTemplate resources will be lost"

				if answer, err := util.Confirm(message, false, "", o.GetIOFileHandles()); !answer {
					return err
				}

			}
		}
	}
	return o.Upgrade()
}

// Upgrade Prow
func (o *UpgradeAddonProwOptions) Upgrade() error {

	isGitOps, devEnv := o.GetDevEnv()
	if isGitOps {
		return o.UpgradeViaGitOps(devEnv)
	}
	kubeClient, _, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}

	// first lets get the existing hmac and oauth tokens so we can use them when reinstalling
	oauthSecret, err := kubeClient.CoreV1().Secrets(o.Namespace).Get("oauth-token", metav1.GetOptions{})
	if err != nil {
		return err
	}
	oauthToken := string(oauthSecret.Data["oauth"])

	hmacSecret, err := kubeClient.CoreV1().Secrets(o.Namespace).Get("hmac-token", metav1.GetOptions{})
	if err != nil {
		return err
	}
	hmacToken := string(hmacSecret.Data["hmac"])

	// now let's reinstall prow
	err = o.DeleteChart("jx-prow", true)
	if err != nil {
		return err
	}

	o.OAUTHToken = oauthToken
	o.HMACToken = hmacToken

	gitOpsEnvDir := ""

	_, pipelineUser, err := o.GetPipelineGitAuth()
	if err != nil {
		return errors.Wrap(err, "retrieving the pipeline Git Auth")
	}
	pipelineUserName := ""
	if pipelineUser != nil {
		pipelineUserName = pipelineUser.Username
	}

	return o.InstallProw(o.Tekton, o.ExternalDNS, isGitOps, gitOpsEnvDir, pipelineUserName, nil)

}

// UpgradeViaGitOps
func (o *UpgradeAddonProwOptions) UpgradeViaGitOps(devEnv *jenkinsv1.Environment) error {

	gitProvider, _, err := o.CreateGitProviderForURLWithoutKind(devEnv.Spec.Source.URL)
	if err != nil {
		return errors.Wrapf(err, "creating git provider for %s", devEnv.Spec.Source.URL)
	}

	log.Logger().Debugf("Git URL %s", devEnv.Spec.Source.URL)

	prowVersion, err := o.GetVersionNumber(versionstream.KindChart, "jenkins-x/prow", "", "")

	log.Logger().Infof("About to upgrade prow to version %s", prowVersion)

	if err != nil {
		return errors.Wrapf(err, "failed to get latest prow version")
	}

	// use a random string in the branch name to ensure we use a unique git branch and fail to push
	rand, err := util.RandStringBytesMaskImprSrc(5)
	if err != nil {
		return errors.Wrapf(err, "failed to generate a random string for use in branch name")
	}

	versionBranchName := prowVersion
	if versionBranchName == "" {
		versionBranchName = "latest"
	}

	details := &gits.PullRequestDetails{
		BranchName: "upgrade-add-on-prow-" + versionBranchName + "-" + rand,
		Title:      "Prow to " + prowVersion,
		Message:    fmt.Sprintf("Upgrade Prow to version %s", prowVersion),
	}

	modifyChartFn := func(requirements *helm.Requirements, metadata *chart.Metadata, values map[string]interface{},
		templates map[string]string, dir string, info *gits.PullRequestDetails) error {

		requirements.SetAppVersion("prow", prowVersion, "", "prow")
		if o.Tekton {
			tektonVersion, err := o.GetVersionNumber(versionstream.KindChart, "jenkins-x/tekton", "", "")
			log.Logger().Infof("About to upgrade tekton to version %s", tektonVersion)
			if err != nil {
				return errors.Wrapf(err, "failed to get latest tekton version")
			}
			requirements.SetAppVersion("tekton", tektonVersion, "", "tekton")
		}
		return nil
	}

	envDir := ""

	if o.CloneDir != "" {
		envDir = o.CloneDir
	}

	options := environments.EnvironmentPullRequestOptions{
		Gitter:        o.Git(),
		ModifyChartFn: modifyChartFn,
		GitProvider:   gitProvider,
	}
	_, err = options.Create(devEnv, envDir, details, nil, "", false)
	if err != nil {
		return errors.Wrapf(err, "failed to create a pull request to update prow version")
	}
	return err
}
