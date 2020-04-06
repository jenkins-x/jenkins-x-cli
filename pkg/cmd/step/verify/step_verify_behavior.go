package verify

import (
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/tekton/metapipeline"
	"github.com/sirupsen/logrus"

	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/importcmd"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/start"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
)

// BehaviorOptions contains the command line options
type BehaviorOptions struct {
	*opts.CommonOptions

	SourceGitURL      string
	Branch            string
	NoImport          bool
	CredentialsSecret string
	GitOrganisation   string
	UseGoProxy        bool
	TestSuite         string
}

var (
	verifyBehaviorLong = templates.LongDesc(`
		Verifies the cluster behaves correctly by running the BDD tests to verify we can create quickstarts, previews and promote applications.

`)

	verifyBehaviorExample = templates.Examples(`
		# runs the BDD tests on the current cluster to verify it behaves nicely
		jx step verify behavior
	`)
)

// NewCmdStepVerifyBehavior creates the command
func NewCmdStepVerifyBehavior(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &BehaviorOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "behavior [flags]",
		Short:   "Verifies the cluster behaves correctly by running the BDD tests to verify we can create quickstarts, previews and promote applications",
		Long:    verifyBehaviorLong,
		Example: verifyBehaviorExample,
		Aliases: []string{"tck", "bdd", "behavior", "behave"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.SourceGitURL, "git-url", "u", "https://github.com/jenkins-x/bdd-jx.git", "The git URL of the BDD tests pipeline")
	cmd.Flags().StringVarP(&options.Branch, "branch", "", "master", "The git branch to use to run the BDD tests")
	cmd.Flags().BoolVarP(&options.NoImport, "no-import", "", false, "Create the pipeline directly, don't import the repository")
	cmd.Flags().StringVarP(&options.CredentialsSecret, "credentials-secret", "", "", "The name of the secret to generate the bdd credentials from, if not specified, the default git auth will be used")
	cmd.Flags().StringVarP(&options.GitOrganisation, "git-organisation", "", "", "Override the git org for the tests rather than reading from teamSettings")
	cmd.Flags().BoolVarP(&options.UseGoProxy, "use-go-proxy", "", false, "Enable the GoProxy for the bdd tests")
	cmd.Flags().StringVarP(&options.TestSuite, "test-suite", "", "", "Override the default test suite ")

	return cmd
}

// Run implements this command
func (o *BehaviorOptions) Run() error {
	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	if o.NoImport {
		owner := "jenkins-x"
		repo := "bdd-jx"
		err = o.runPipelineDirectly(owner, repo, o.SourceGitURL)
		if err != nil {
			return errors.Wrapf(err, "unable to run job directly %s/%s", owner, repo)
		}
		// let sleep a little bit to give things a head start
		time.Sleep(time.Second * 3)

		return o.followLogs(owner, repo)
	}

	list, err := jxClient.JenkinsV1().SourceRepositories(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to load SourceRepositories in namespace '%s'", ns)
	}
	gitInfo, err := gits.ParseGitURL(o.SourceGitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL '%s'", o.SourceGitURL)
	}
	sr, err := o.findSourceRepository(list.Items, o.SourceGitURL, gitInfo)
	if err != nil {
		return errors.Wrapf(err, "failed to find SourceRepository for URL '%s'", o.SourceGitURL)
	}
	owner := ""
	repo := ""
	trigger := true
	if sr == nil {
		err = o.importSourceRepository(gitInfo)
		if err != nil {
			return errors.Wrapf(err, "failed to import SourceRepository for URL '%s'", o.SourceGitURL)
		}
		trigger = false
	} else {
		owner = sr.Spec.Org
		repo = sr.Spec.Repo
	}
	if owner == "" {
		owner = gitInfo.Organisation
	}
	if owner == "" {
		owner = gitInfo.Organisation
	}
	if trigger {
		err = o.triggerPipeline(owner, repo)
		if err != nil {
			return errors.Wrapf(err, "failed to trigger Pipeline for URL '%s/%s'", owner, repo)
		}
	}

	// let sleep a little bit to give things a head start
	time.Sleep(time.Second * 3)

	return o.followLogs(owner, repo)
}

func (o *BehaviorOptions) followLogs(owner string, repo string) error {
	commonOptions := *o.CommonOptions
	commonOptions.BatchMode = true
	lo := &get.GetBuildLogsOptions{
		GetOptions: get.GetOptions{
			CommonOptions: &commonOptions,
		},
		Tail:           true,
		Wait:           true,
		FailIfPodFails: true,
		BuildFilter: builds.BuildPodInfoFilter{
			Owner:      owner,
			Repository: repo,
			Branch:     o.Branch,
		},
	}
	return lo.Run()
}

func (o *BehaviorOptions) findSourceRepository(repositories []v1.SourceRepository, url string, gitInfo *gits.GitRepository) (*v1.SourceRepository, error) {
	for i := range repositories {
		repo := &repositories[i]
		u2, _ := kube.GetRepositoryGitURL(repo)
		if url == u2 || strings.TrimSuffix(url, ".git") == strings.TrimSuffix(u2, ".git") {
			return repo, nil
		}
	}
	for i := range repositories {
		repo := &repositories[i]
		if repo.Spec.Org == gitInfo.Organisation && repo.Spec.Repo == gitInfo.Name {
			return repo, nil
		}
	}
	return nil, nil
}

func (o *BehaviorOptions) importSourceRepository(gitInfo *gits.GitRepository) error {
	log.Logger().Infof("importing project %s", util.ColorInfo(o.SourceGitURL))

	io := &importcmd.ImportOptions{
		CommonOptions:           o.CommonOptions,
		RepoURL:                 o.SourceGitURL,
		DisableDraft:            true,
		DisableJenkinsfileCheck: true,
		DisableMaven:            true,
		DisableWebhooks:         true,
		Organisation:            gitInfo.Organisation,
		Repository:              gitInfo.Name,
		AppName:                 gitInfo.Name,
	}
	err := io.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to import project %s", o.SourceGitURL)
	}
	return nil
}

func (o *BehaviorOptions) triggerPipeline(owner string, repo string) error {
	pipeline := owner + "/" + repo + "/" + o.Branch
	log.Logger().Infof("triggering pipeline %s", util.ColorInfo(pipeline))

	so := &start.StartPipelineOptions{
		CommonOptions: o.CommonOptions,
		Filter:        pipeline,
		Branch:        o.Branch,
	}
	err := so.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to start pipeline %s", pipeline)
	}
	return nil
}

func (o *BehaviorOptions) runPipelineDirectly(owner string, repo string, sourceURL string) error {
	pullRefs := ""
	branch := "master"
	pullRefs = branch + ":"
	kind := metapipeline.ReleasePipeline
	sa := "tekton-bot"

	l := logrus.WithFields(logrus.Fields(map[string]interface{}{
		"Owner":     owner,
		"Name":      repo,
		"SourceURL": sourceURL,
		"Branch":    branch,
		"PullRefs":  pullRefs,
		//"Job":       job,
	}))
	l.Info("about to start Jenkinx X meta pipeline")

	pullRefData := metapipeline.NewPullRef(sourceURL, branch, "")
	envVars := map[string]string{}
	if o.CredentialsSecret != "" {
		envVars["JX_CREDENTIALS_FROM_SECRET"] = o.CredentialsSecret
	}

	if o.GitOrganisation != "" {
		envVars["GIT_ORGANISATION"] = o.GitOrganisation
	}

	if o.UseGoProxy {
		envVars["GOPROXY"] = "https://proxy.golang.org"
	}

	if o.TestSuite != "" {
		envVars["SUITE"] = o.TestSuite
	}

	pipelineCreateParam := metapipeline.PipelineCreateParam{
		PullRef:      pullRefData,
		PipelineKind: kind,
		Context:      "",
		// No equivalent to https://github.com/jenkins-x/jx/blob/bb59278c2707e0e99b3c24be926745c324824388/pkg/cmd/controller/pipeline/pipelinerunner_controller.go#L236
		//   for getting environment variables from the prow job here, so far as I can tell (abayer)
		// Also not finding an equivalent to labels from the PipelineRunRequest
		ServiceAccount: sa,
		// I believe we can use an empty string default image?
		DefaultImage:        "",
		EnvVariables:        envVars,
		UseBranchAsRevision: true,
		NoReleasePrepare:    true,
	}

	c, err := metapipeline.NewMetaPipelineClient()
	if err != nil {
		return errors.Wrap(err, "unable to create metapipeline client")
	}

	pipelineActivity, tektonCRDs, err := c.Create(pipelineCreateParam)
	if err != nil {
		return errors.Wrap(err, "unable to create Tekton CRDs")
	}

	err = c.Apply(pipelineActivity, tektonCRDs)
	if err != nil {
		return errors.Wrap(err, "unable to apply Tekton CRDs")
	}
	return nil
}
