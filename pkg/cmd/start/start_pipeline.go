package start

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/tekton/metapipeline"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/kube"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/prow"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	prowjobv1 "k8s.io/test-infra/prow/apis/prowjobs/v1"
)

const (
	releaseBranchName = "master"
)

// StartPipelineOptions contains the command line options
type StartPipelineOptions struct {
	*opts.CommonOptions

	Output          string
	Tail            bool
	Filter          string
	Branch          string
	PipelineKind    string
	JenkinsSelector opts.JenkinsSelectorOptions

	Jobs map[string]gojenkins.Job

	ProwOptions prow.Options

	// meta pipeline options
	Context      string
	CustomLabels []string
	CustomEnvs   []string
}

var (
	startPipelineLong = templates.LongDesc(`
		Starts the pipeline build.

`)

	startPipelineExample = templates.Examples(`
		# Start a pipeline
		jx start pipeline foo

		# Select the pipeline to start
		jx start pipeline

		# Select the pipeline to start and tail the log
		jx start pipeline -t
	`)
)

// NewCmdStartPipeline creates the command
func NewCmdStartPipeline(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StartPipelineOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "pipeline [flags]",
		Short:   "Starts one or more pipelines",
		Long:    startPipelineLong,
		Example: startPipelineExample,
		Aliases: []string{"pipe", "pipeline", "build", "run"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.Tail, "tail", "t", false, "Tails the build log to the current terminal")
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filters all the available jobs by those that contain the given text")
	cmd.Flags().StringVarP(&options.Context, "context", "c", "", "An optional Prow pipeline context")
	cmd.Flags().StringVarP(&options.Branch, "branch", "", "", "The branch to start. If not specified defaults to master")
	cmd.Flags().StringVarP(&options.PipelineKind, "kind", "", "", "The kind of pipeline such as release or pullrequest")
	cmd.Flags().StringVar(&options.ServiceAccount, "service-account", "tekton-bot", "The Kubernetes ServiceAccount to use to run the meta pipeline")
	cmd.Flags().StringArrayVarP(&options.CustomLabels, "label", "l", nil, "List of custom labels to be applied to the generated PipelineRun (can be use multiple times)")
	cmd.Flags().StringArrayVarP(&options.CustomEnvs, "env", "e", nil, "List of custom environment variables to be applied to the generated PipelineRun that are created (can be use multiple times)")

	options.JenkinsSelector.AddFlags(cmd)

	return cmd
}

// Run implements this command
func (o *StartPipelineOptions) Run() error {
	kubeClient, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	_, _, err = o.JXClient()
	if err != nil {
		return err
	}

	devEnv, _, err := o.DevEnvAndTeamSettings()
	if err != nil {
		return err
	}

	isProw := devEnv.Spec.IsProwOrLighthouse()

	args := o.Args
	names := []string{}
	o.ProwOptions = prow.Options{
		KubeClient: kubeClient,
		NS:         currentNamespace,
	}
	if o.JenkinsSelector.IsCustom() {
		isProw = false
	}
	if isProw {
		names, err = o.ProwOptions.GetReleaseJobs()
		if err != nil {
			return err
		}
		names = util.StringsContaining(names, o.Filter)
	} else {
		jobMap, err := o.GetJenkinsJobs(&o.JenkinsSelector, o.Filter)
		if err != nil {
			return err
		}
		o.Jobs = jobMap

		for k := range o.Jobs {
			names = append(names, k)
		}
	}

	if len(args) == 0 {
		if len(names) == 0 {
			return errors.New("no jobs found to trigger")
		}
		sort.Strings(names)

		defaultName := ""
		for _, n := range names {
			if strings.HasSuffix(n, "/master") {
				defaultName = n
				break
			}
		}
		name, err := util.PickNameWithDefault(names, "Which pipeline do you want to start: ", defaultName, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		args = []string{name}
	}
	for _, a := range args {
		if devEnv.Spec.IsLighthouse() {
			err = o.createMetaPipeline(a)
			if err != nil {
				return err
			}
		} else if isProw {
			err = o.createProwJob(a)
			if err != nil {
				return err
			}
		} else {
			err = o.startJenkinsJob(a)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *StartPipelineOptions) createMetaPipeline(jobName string) error {
	parts := strings.Split(jobName, "/")
	if len(parts) != 3 {
		return fmt.Errorf("job name [%s] does not match org/repo/branch format", jobName)
	}
	owner := parts[0]
	repo := parts[1]
	branch := parts[2]
	if o.Branch != "" {
		branch = o.Branch
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create JX client")
	}

	sr, err := kube.FindSourceRepositoryWithoutProvider(jxClient, ns, owner, repo)
	if err != nil {
		return errors.Wrap(err, "cannot determine git source URL")
	}
	if sr == nil {
		return fmt.Errorf("could not find existing SourceRepository for owner %s and repo %s", owner, repo)
	}

	sourceURL, err := kube.GetRepositoryGitURL(sr)
	if err != nil {
		return errors.Wrapf(err, "cannot generate the git URL from SourceRepository %s", sr.Name)
	}
	if sourceURL == "" {
		return fmt.Errorf("no git URL returned from SourceRepository %s", sr.Name)
	}

	log.Logger().Debug("creating meta pipeline client")
	client, err := metapipeline.NewMetaPipelineClient()
	if err != nil {
		return errors.Wrap(err, "unable to create meta pipeline client")
	}

	pullRef := metapipeline.NewPullRef(sourceURL, branch, "")
	pipelineKind := o.determinePipelineKind(branch)
	envVarMap, err := util.ExtractKeyValuePairs(o.CustomEnvs, "=")
	if err != nil {
		return errors.Wrap(err, "unable to parse env variables")
	}

	labelMap, err := util.ExtractKeyValuePairs(o.CustomLabels, "=")
	if err != nil {
		return errors.Wrap(err, "unable to parse label variables")
	}

	pipelineCreateParam := metapipeline.PipelineCreateParam{
		PullRef:        pullRef,
		PipelineKind:   pipelineKind,
		Context:        o.Context,
		EnvVariables:   envVarMap,
		Labels:         labelMap,
		ServiceAccount: o.ServiceAccount,
	}

	pipelineActivity, tektonCRDs, err := client.Create(pipelineCreateParam)
	if err != nil {
		return errors.Wrap(err, "unable to create Tekton CRDs")
	}

	err = client.Apply(pipelineActivity, tektonCRDs)
	if err != nil {
		return errors.Wrap(err, "unable to apply Tekton CRDs")
	}

	err = client.Close()
	if err != nil {
		log.Logger().Errorf("unable to close meta pipeline client: %s", err.Error())
	}

	return nil
}

func (o *StartPipelineOptions) createProwJob(jobname string) error {
	parts := strings.Split(jobname, "/")
	if len(parts) != 3 {
		return fmt.Errorf("job name [%s] does not match org/repo/branch format", jobname)
	}
	org := parts[0]
	repo := parts[1]
	branch := parts[2]

	postSubmitJob, err := o.ProwOptions.GetPostSubmitJob(org, repo, branch)
	if err != nil {
		return err
	}
	agent := prowjobv1.ProwJobAgent(prow.TektonAgent)
	jobSpec := prowjobv1.ProwJobSpec{
		BuildSpec: postSubmitJob.BuildSpec,
		Agent:     agent,
	}
	jobSpec.Type = prowjobv1.PostsubmitJob

	// TODO prow only supports github.com
	// if you want to use anything but github.com you should use
	// lighthouse: https://jenkins-x.io/docs/reference/components/lighthouse/
	sourceURL := fmt.Sprintf("https://github.com/%s/%s.git", org, repo)

	provider, _, err := o.CreateGitProviderForURLWithoutKind(sourceURL)
	if err != nil {
		return errors.Wrapf(err, "creating git provider for %s", sourceURL)
	}
	gitBranch, err := provider.GetBranch(org, repo, branch)
	if err != nil {
		return errors.Wrapf(err, "getting branch %s on %s/%s", branch, org, repo)
	}

	if gitBranch != nil && gitBranch.Commit != nil {
		if jobSpec.Refs == nil {
			jobSpec.Refs = &prowjobv1.Refs{}
		}
		jobSpec.Refs.BaseSHA = gitBranch.Commit.SHA
		jobSpec.Refs.Repo = repo
		jobSpec.Refs.Org = org
		jobSpec.Refs.BaseRef = branch
	}

	p := prow.NewProwJob(jobSpec, nil)
	p.Status = prowjobv1.ProwJobStatus{
		State: prowjobv1.PendingState,
	}
	p.Spec.Refs = &prowjobv1.Refs{
		BaseRef: branch,
		Org:     org,
		Repo:    repo,
	}

	if gitBranch != nil && gitBranch.Commit != nil {
		p.Spec.Refs.BaseSHA = gitBranch.Commit.SHA
	}

	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	_, err = prow.CreateProwJob(client, currentNamespace, p)
	return err
}

func (o *StartPipelineOptions) startJenkinsJob(name string) error {
	job := o.Jobs[name]

	jenkinsClient, err := o.CreateCustomJenkinsClient(&o.JenkinsSelector)
	if err != nil {
		return err
	}
	job.Url = jenkins.SwitchJenkinsBaseURL(job.Url, jenkinsClient.BaseURL())

	// ignore errors as it could be there's no last build yet
	previous, _ := jenkinsClient.GetLastBuild(job)

	params := url.Values{}
	err = jenkinsClient.Build(job, params)
	if err != nil {
		return err
	}

	i := 0
	for {
		last, err := jenkinsClient.GetLastBuild(job)

		// lets ignore the first query in case there's no build yet
		if i > 0 && err != nil {
			return err
		}
		i++

		if last.Number != previous.Number {
			last.Url = jenkins.SwitchJenkinsBaseURL(last.Url, jenkinsClient.BaseURL())

			log.Logger().Infof("Started build of %s at %s", util.ColorInfo(name), util.ColorInfo(last.Url))
			log.Logger().Infof("%s %s", util.ColorStatus("view the log at:"), util.ColorInfo(util.UrlJoin(last.Url, "/console")))
			if o.Tail {
				return o.TailJenkinsBuildLog(&o.JenkinsSelector, name, &last)
			}
			return nil
		}
		time.Sleep(time.Second)
	}
}

func (o *StartPipelineOptions) determinePipelineKind(branch string) metapipeline.PipelineKind {
	if o.PipelineKind != "" {
		return metapipeline.StringToPipelineKind(o.PipelineKind)
	}
	var kind metapipeline.PipelineKind

	// `jx start pipeline` will only always trigger a release or feature pipeline. Not sure whether there is a way
	// to configure your release branch atm. Using a constant here (HF)
	if branch == releaseBranchName {
		kind = metapipeline.ReleasePipeline
	} else {
		kind = metapipeline.FeaturePipeline
	}
	return kind
}
