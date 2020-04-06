package create

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	createAddonIngressControllerLong = templates.LongDesc(`
		Create an Ingress Controller to expose services outside of your remote Staging/Production cluster
`)

	createAddonIngressControllerExample = templates.Examples(`
		# Creates the ingress controller
		jx create addon ingctl
		
	`)
)

// CreateAddonIngressControllerOptions the options for the create spring command
type CreateAddonIngressControllerOptions struct {
	CreateAddonOptions

	InitOptions initcmd.InitOptions
}

// NewCmdCreateAddonIngressController creates a command object for the "create" command
func NewCmdCreateAddonIngressController(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonIngressControllerOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "ingress controller",
		Short:   "Create an Ingress Controller to expose services outside of your remote Staging/Production cluster",
		Aliases: []string{"ingctl"},
		Long:    createAddonIngressControllerLong,
		Example: createAddonIngressControllerExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.InitOptions.AddIngressFlags(cmd)
	return cmd
}

// Run implements the command
func (o *CreateAddonIngressControllerOptions) Run() error {
	jxClient, ns, err := o.JXClient()
	if err != nil {
		return err
	}
	envName := kube.LabelValueThisEnvironment
	env, err := jxClient.JenkinsV1().Environments(ns).Get(envName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "could not find the Environment CRD %s in namespace %s", envName, ns)
	}
	gitRepo := env.Spec.Source.URL
	if gitRepo == "" {
		return fmt.Errorf("could not find the Git Source URL in the Environment CRD %s in namespace %s", envName, ns)
	}

	o.InitOptions.CommonOptions = o.CommonOptions
	err = o.InitOptions.InitIngress()
	if err != nil {
		return err
	}

	// now lets try update the domain
	domain := o.InitOptions.Domain
	if domain == "" {
		log.Logger().Error("No domain could be discovered so we cannot update the domain entry in your GitOps repository")
	}
	log.Logger().Infof("domain is %s", util.ColorInfo(domain))

	log.Logger().Infof("\n\nLets create a Pull Request against %s to modify the domain...\n", util.ColorInfo(gitRepo))

	// now lets make sure we have the latest domain in the git repository
	return o.createPullRequestForDomain(gitRepo, domain)
}

func (o *CreateAddonIngressControllerOptions) createPullRequestForDomain(gitRepoURL string, domain string) error {
	po := &opts.PullRequestDetails{
		RepositoryGitURL: gitRepoURL,
		RepositoryBranch: "master",
	}

	dir, err := ioutil.TempDir("", "create-version-pr")
	if err != nil {
		return err
	}

	po.Dir = dir
	po.BranchNameText = "set-ingress-domain"
	po.Title = "set ingress domain"
	po.RepositoryMessage = "environment repository"
	po.Message = "update the ingress domain"

	return o.CreatePullRequest(po, func() error {
		return o.modifyDomainInValuesFiles(dir, domain)
	})
}

func (o *CreateAddonIngressControllerOptions) modifyDomainInValuesFiles(dir string, domain string) error {
	fileName := filepath.Join(dir, "env", "values.yaml")
	exists, err := util.FileExists(fileName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Could not find helm file in cloned environment git repository: %s", fileName)
	}
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to load helm values file %s", fileName)
	}
	values, err := helm.LoadValues(data)
	if err != nil {
		return errors.Wrapf(err, "failed to parse helm values file %s", fileName)
	}
	util.SetMapValueViaPath(values, "expose.config.domain", domain)

	err = helm.SaveFile(fileName, values)
	if err != nil {
		return errors.Wrapf(err, "failed to save helm values file %s", fileName)
	}
	return nil
}
