package create

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/watch"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/spf13/viper"

	"github.com/jenkins-x/jx/pkg/cmd/step/git"

	"github.com/ghodss/yaml"
	jxclient "github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	syntaxstep "github.com/jenkins-x/jx/pkg/cmd/step/syntax"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/jenkinsfile/gitresolver"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeclient "k8s.io/client-go/kubernetes"
)

const (
	kanikoSecretMount = "/kaniko-secret/secret.json" // #nosec
	kanikoSecretName  = kube.SecretKaniko
	kanikoSecretKey   = kube.SecretKaniko

	noApplyOptionName = "no-apply"
	outputOptionName  = "output"
)

var (
	createTaskLong = templates.LongDesc(`
		Creates a Tekton Pipeline Run for a project
`)

	createTaskExample = templates.Examples(`
		# create a Tekton Pipeline Run and render to the console
		jx step create task

		# create a Tekton Pipeline Task
		jx step create task -o mytask.yaml

		# view the steps that would be created
		jx step create task --view

			`)

	lastPipelineRun = time.Now()

	createTaskOutDir  string
	createTaskNoApply bool
)

// StepCreateTaskOptions contains the command line flags
type StepCreateTaskOptions struct {
	step.StepOptions

	Pack                string
	BuildPackURL        string
	BuildPackRef        string
	PipelineKind        string
	Context             string
	CustomLabels        []string
	CustomEnvs          []string
	NoApply             *bool
	DryRun              bool
	InterpretMode       bool
	DisableConcurrent   bool
	StartStep           string
	EndStep             string
	Trigger             string
	TargetPath          string
	SourceName          string
	CustomImage         string
	DefaultImage        string
	CloneGitURL         string
	Branch              string
	Revision            string
	PullRequestNumber   string
	DeleteTempDir       bool
	ViewSteps           bool
	EffectivePipeline   bool
	NoReleasePrepare    bool
	Duration            time.Duration
	FromRepo            bool
	NoKaniko            bool
	SemanticRelease     bool
	KanikoImage         string
	KanikoSecretMount   string
	KanikoSecret        string
	KanikoSecretKey     string
	ProjectID           string
	DockerRegistry      string
	DockerRegistryOrg   string
	AdditionalEnvVars   map[string]string
	PodTemplates        map[string]*corev1.Pod
	UseBranchAsRevision bool

	GitInfo              *gits.GitRepository
	BuildNumber          string
	labels               map[string]string
	Results              tekton.CRDWrapper
	pipelineParams       []pipelineapi.Param
	version              string
	previewVersionPrefix string
	VersionResolver      *versionstream.VersionResolver
	CloneDir             string
}

// NewCmdStepCreateTask Creates a new Command object
func NewCmdStepCreateTask(commonOpts *opts.CommonOptions) *cobra.Command {
	cmd, _ := NewCmdStepCreateTaskAndOption(commonOpts)
	return cmd
}

// NewCmdStepCreateTaskAndOption Creates a new Command object and returns the options
func NewCmdStepCreateTaskAndOption(commonOpts *opts.CommonOptions) (*cobra.Command, *StepCreateTaskOptions) {
	options := &StepCreateTaskOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "task",
		Short:   "Creates a Tekton PipelineRun for the current folder or given build pack",
		Long:    createTaskLong,
		Example: createTaskExample,
		Aliases: []string{"bt"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&createTaskOutDir, outputOptionName, "o", "out", "The directory to write the output to as YAML. Defaults to 'out'")
	cmd.Flags().StringVarP(&options.Branch, "branch", "", "", "The git branch to trigger the build in. Defaults to the current local branch name")
	cmd.Flags().StringVarP(&options.Revision, "revision", "", "", "The git revision to checkout, can be a branch name or git sha")
	cmd.Flags().StringVarP(&options.PipelineKind, "kind", "k", "release", "The kind of pipeline to create such as: "+strings.Join(jenkinsfile.PipelineKinds, ", "))
	cmd.Flags().StringArrayVarP(&options.CustomLabels, "label", "l", nil, "List of custom labels to be applied to resources that are created")
	cmd.Flags().StringArrayVarP(&options.CustomEnvs, "env", "e", nil, "List of custom environment variables to be applied to resources that are created")
	cmd.Flags().StringVarP(&options.CloneGitURL, "clone-git-url", "", "", "Specify the git URL to clone to a temporary directory to get the source code")
	cmd.Flags().StringVarP(&options.CloneDir, "clone-dir", "", "", "Specify the directory of the directory containing the git clone")
	cmd.Flags().StringVarP(&options.PullRequestNumber, "pr-number", "", "", "If a Pull Request this is it's number")
	cmd.Flags().StringVarP(&options.BuildNumber, "build-number", "", "", "The build number")
	cmd.Flags().BoolVarP(&createTaskNoApply, noApplyOptionName, "", false, "Disables creating the Pipeline resources in the kubernetes cluster and just outputs the generated Task to the console or output file")
	cmd.Flags().BoolVarP(&options.DryRun, "dry-run", "", false, "Disables creating the Pipeline resources in the kubernetes cluster and just outputs the generated Task to the console or output file, without side effects")
	cmd.Flags().BoolVarP(&options.InterpretMode, "interpret", "", false, "Enable interpret mode. Rather than spinning up Tekton CRDs to create a Pod just invoke the commands in the current shell directly. Useful for bootstrapping installations of Jenkins X and tekton using a pipeline before you have installed Tekton.")
	cmd.Flags().StringVarP(&options.StartStep, "start-step", "", "", "When in interpret mode this specifies the step to start at")
	cmd.Flags().StringVarP(&options.EndStep, "end-step", "", "", "When in interpret mode this specifies the step to end at")
	cmd.Flags().BoolVarP(&options.ViewSteps, "view", "", false, "Just view the steps that would be created")
	cmd.Flags().BoolVarP(&options.EffectivePipeline, "effective-pipeline", "", false, "Just view the effective pipeline definition that would be created")
	cmd.Flags().BoolVarP(&options.SemanticRelease, "semantic-release", "", false, "Enable semantic releases")
	cmd.Flags().BoolVarP(&options.UseBranchAsRevision, "branch-as-revision", "", false, "Use the provided branch as the revision for release pipelines, not the version tag")

	options.AddCommonFlags(cmd)
	options.setupViper(cmd)
	return cmd, options
}

func (o *StepCreateTaskOptions) setupViper(cmd *cobra.Command) {
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)

	_ = viper.BindEnv(noApplyOptionName)
	_ = viper.BindPFlag(noApplyOptionName, cmd.Flags().Lookup(noApplyOptionName))

	_ = viper.BindEnv(outputOptionName)
	_ = viper.BindPFlag(outputOptionName, cmd.Flags().Lookup(outputOptionName))
}

// AddCommonFlags adds common CLI options
func (o *StepCreateTaskOptions) AddCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Pack, "pack", "p", "", "The build pack name. If none is specified its discovered from the source code")
	cmd.Flags().StringVarP(&o.BuildPackURL, "url", "u", "", "The URL for the build pack Git repository")
	cmd.Flags().StringVarP(&o.BuildPackRef, "ref", "r", "", "The Git reference (branch,tag,sha) in the Git repository to use")
	cmd.Flags().StringVarP(&o.Context, "context", "c", "", "The pipeline context if there are multiple separate pipelines for a given branch")
	cmd.Flags().StringVarP(&o.ServiceAccount, "service-account", "", "tekton-bot", "The Kubernetes ServiceAccount to use to run the pipeline")
	cmd.Flags().StringVarP(&o.TargetPath, "target-path", "", "", "The target path appended to /workspace/${source} to clone the source code")
	cmd.Flags().StringVarP(&o.SourceName, "source", "", "source", "The name of the source repository")
	cmd.Flags().StringVarP(&o.CustomImage, "image", "", "", "Specify a custom image to use for the steps which overrides the image in the PodTemplates")
	cmd.Flags().StringVarP(&o.DefaultImage, "default-image", "", syntax.DefaultContainerImage, "Specify the docker image to use if there is no image specified for a step and there's no Pod Template")
	cmd.Flags().BoolVarP(&o.DeleteTempDir, "delete-temp-dir", "", true, "Deletes the temporary directory of cloned files if using the 'clone-git-url' option")
	cmd.Flags().BoolVarP(&o.NoReleasePrepare, "no-release-prepare", "", false, "Disables creating the release version number and tagging git and triggering the release pipeline from the new tag")
	cmd.Flags().BoolVarP(&o.NoKaniko, "no-kaniko", "", false, "Disables using kaniko directly for building docker images")
	cmd.Flags().StringVarP(&o.KanikoImage, "kaniko-image", "", syntax.KanikoDockerImage, "The docker image for Kaniko")
	cmd.Flags().StringVarP(&o.KanikoSecretMount, "kaniko-secret-mount", "", kanikoSecretMount, "The mount point of the Kaniko secret")
	cmd.Flags().StringVarP(&o.KanikoSecret, "kaniko-secret", "", kanikoSecretName, "The name of the kaniko secret")
	cmd.Flags().StringVarP(&o.KanikoSecretKey, "kaniko-secret-key", "", kanikoSecretKey, "The key in the Kaniko Secret to mount")
	cmd.Flags().StringVarP(&o.ProjectID, "project-id", "", "", "The cloud project ID. If not specified we default to the install project")
	cmd.Flags().StringVarP(&o.DockerRegistry, "docker-registry", "", "", "The Docker Registry host name to use which is added as a prefix to docker images")
	cmd.Flags().StringVarP(&o.DockerRegistryOrg, "docker-registry-org", "", "", "The Docker registry organisation. If blank the git repository owner is used")
	cmd.Flags().DurationVarP(&o.Duration, "duration", "", time.Second*30, "Retry duration when trying to create a PipelineRun")
}

// Run implements this command
func (o *StepCreateTaskOptions) Run() error {
	if o.NoApply == nil {
		b := viper.GetBool(noApplyOptionName)
		o.NoApply = &b
	}

	if o.OutDir == "" {
		s := viper.GetString(outputOptionName)
		o.OutDir = s
	}

	var effectiveProjectConfig *config.ProjectConfig
	var err error

	tektonClient, jxClient, kubeClient, ns, err := o.getClientsAndNamespace()
	if err != nil {
		return err
	}

	if o.CloneDir == "" {
		o.CloneDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	if o.VersionResolver == nil {
		o.VersionResolver, err = o.GetVersionResolver()
		if err != nil {
			return errors.Wrap(err, "Unable to create version resolver")
		}
	}

	pr, err := o.parsePullRefs()
	if err != nil {
		return errors.Wrap(err, "Unable to find or parse PULL_REFS from custom environment")
	}

	exists, err := o.effectiveProjectConfigExists()
	if err != nil {
		return err
	}
	if !exists {
		// TODO this branch all things depending on it can be removed once the meta pipeline is working
		// TODO keeping this to keep existing behavior until then (HF)
		if o.CloneGitURL != "" {
			o.CloneDir = o.cloneGitRepositoryToTempDir(o.CloneGitURL, o.Branch, o.PullRequestNumber, o.Revision)
			if o.DeleteTempDir {
				defer func() {
					log.Logger().Infof("removing the temp directory %s", o.CloneDir)
					err := os.RemoveAll(o.CloneDir)
					if err != nil {
						log.Logger().Warnf("failed to delete dir %s: %s", o.CloneDir, err.Error())
					}
				}()
			}
			// Add the REPO_URL env var
			o.CustomEnvs = append(o.CustomEnvs, fmt.Sprintf("%s=%s", "REPO_URL", o.CloneGitURL))
			err = o.mergePullRefs(pr, o.CloneDir)
			if err != nil {
				return errors.Wrapf(err, "Unable to merge PULL_REFS %s in %s", pr, o.CloneDir)
			}
		}
	}

	o.GitInfo, err = o.FindGitInfo(o.CloneDir)
	if err != nil {
		return errors.Wrapf(err, "failed to find git information from dir %s", o.CloneDir)
	}

	if o.Branch == "" {
		o.Branch, err = o.Git().Branch(o.CloneDir)
		if err != nil {
			return errors.Wrapf(err, "failed to find git branch from dir %s", o.CloneDir)
		}
	}

	o.PodTemplates, err = kube.LoadPodTemplates(kubeClient, ns)
	if err != nil {
		return errors.Wrap(err, "Unable to load pod templates")
	}

	pipelineName := tekton.PipelineResourceNameFromGitInfo(o.GitInfo, o.Branch, o.Context, tekton.BuildPipeline.String())

	exists, err = o.effectiveProjectConfigExists()
	if err != nil {
		return err
	}
	if exists {
		effectiveProjectConfig, err = o.loadEffectiveProjectConfig()
		log.Logger().Debug("loaded effective project configuration from file")
	} else {
		// TODO: This branch also goes away when the metapipeline is actually in place in pipelinerunner (AB)
		log.Logger().Debug("Creating effective project configuration")
		effectiveProjectConfig, err = o.createEffectiveProjectConfigFromOptions(tektonClient, jxClient, kubeClient, ns, pipelineName)
		if err != nil {
			return errors.Wrap(err, "failed to create effective project configuration")
		}
	}

	err = o.setBuildValues()
	if err != nil {
		return err
	}

	log.Logger().Debug("Setting build version")
	err = o.setBuildVersion(effectiveProjectConfig)
	if err != nil {
		return errors.Wrapf(err, "failed to set the version on release pipelines")
	}

	log.Logger().Debug("Creating Tekton CRDs")
	tektonCRDs, err := o.generateTektonCRDs(effectiveProjectConfig, ns, pipelineName)
	if err != nil {
		return errors.Wrap(err, "failed to generate Tekton CRDs")
	}
	log.Logger().Debugf("Tekton CRDs for %s created", tektonCRDs.PipelineRun().Name)
	o.Results = *tektonCRDs

	if o.ViewSteps {
		err = o.viewSteps(tektonCRDs.Tasks()...)
		if err != nil {
			return errors.Wrap(err, "unable to view pipeline steps")
		}
		return nil
	}

	if o.InterpretMode {
		return o.interpretPipeline(ns, effectiveProjectConfig, tektonCRDs)
	}

	if *o.NoApply || o.DryRun {
		log.Logger().Infof("Writing output ")
		err := tektonCRDs.WriteToDisk(o.OutDir, nil)
		if err != nil {
			return errors.Wrapf(err, "Failed to output Tekton CRDs")
		}
	} else {
		activityKey := tekton.GeneratePipelineActivity(o.BuildNumber, o.Branch, o.GitInfo, o.Context, pr)

		log.Logger().Debugf(" PipelineActivity for %s created successfully", tektonCRDs.Name())

		if o.DisableConcurrent {
			o.waitForPreviousPipeline(tektonClient, ns, 10*time.Minute)
		}
		log.Logger().Infof("Applying changes ")
		err := tekton.ApplyPipeline(jxClient, tektonClient, tektonCRDs, ns, activityKey)
		if err != nil {
			return errors.Wrapf(err, "failed to apply Tekton CRDs")
		}
		tektonCRDs.AddLabels(o.labels)

		log.Logger().Debugf(" for %s", tektonCRDs.PipelineRun().Name)
	}
	return nil
}

func (o *StepCreateTaskOptions) waitForPreviousPipeline(tektonClient tektonclient.Interface, ns string, defaultWait time.Duration) {
	fallbackWait := true
	labelSelector := fmt.Sprintf("owner=%s,repository=%s,branch=%s", o.GitInfo.Organisation, o.GitInfo.Name, o.Branch)
	if o.Context != "" {
		labelSelector += fmt.Sprintf(",context=%s", o.Context)
	}

restartWatch:
	prs, err := tektonClient.TektonV1alpha1().PipelineRuns(ns).List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		log.Logger().Errorf("Can't list PipelineRuns %s: %s", labelSelector, err)
	} else {
		pendingPipelineRuns := make(map[string]bool)
		for _, pr := range prs.Items {
			if !(pr.IsDone() || pr.IsCancelled()) {
				pendingPipelineRuns[pr.Name] = true
			}
		}
		if len(pendingPipelineRuns) > 0 {
			log.Logger().Infof("Waiting for pending PipelineRuns %v to finish or be deleted", reflect.ValueOf(pendingPipelineRuns).MapKeys())
			pipelineWatch, err := tektonClient.TektonV1alpha1().PipelineRuns(ns).Watch(metav1.ListOptions{
				LabelSelector:   labelSelector,
				ResourceVersion: prs.ResourceVersion,
			})
			if err != nil {
				log.Logger().Errorf("Can't watch PipelineRun %s (ResourceVersion %s): %s", labelSelector, prs.ResourceVersion, err)
			} else {
				for {
					update := <-pipelineWatch.ResultChan()
					if o.Verbose {
						bytes, err := json.MarshalIndent(update, "", "\t")
						log.Logger().Debugf("PipelineRun watch update: %s %s", bytes, err)
					}
					switch update.Type {
					case watch.Deleted:
						pr := update.Object.(*pipelineapi.PipelineRun)
						if pendingPipelineRuns[pr.Name] {
							log.Logger().Infof("PipelineRun %s is deleted", pr.Name)
							delete(pendingPipelineRuns, pr.Name)
						}
					case watch.Modified:
						pr := update.Object.(*pipelineapi.PipelineRun)
						if pendingPipelineRuns[pr.Name] && (pr.IsDone() || pr.IsCancelled()) {
							log.Logger().Infof("PipelineRun %s is finished", pr.Name)
							delete(pendingPipelineRuns, pr.Name)
						}
					case watch.Added:
						pr := update.Object.(*pipelineapi.PipelineRun)
						if !(pr.IsDone() || pr.IsCancelled()) {
							log.Logger().Infof("PipelineRun %s is added", pr.Name)
							pendingPipelineRuns[pr.Name] = true
						}
					default:
						log.Logger().Errorf("Unknown PipelineRun watch update. Restarting watch.")
						pipelineWatch.Stop()
						goto restartWatch
					}
					if len(pendingPipelineRuns) == 0 {
						pipelineWatch.Stop()
						fallbackWait = false
						break
					}
				}
			}
		} else {
			fallbackWait = false
		}
	}

	// When failing to wait for a pipeline wait for defaultWait.
	if fallbackWait {
		sleepDuration := defaultWait - time.Now().Sub(lastPipelineRun)
		if sleepDuration > 0 {
			log.Logger().Errorf("Can't access previous PipelineRun. Waiting %v to ensure it finishes", sleepDuration)
			time.Sleep(sleepDuration)
		}
		lastPipelineRun = time.Now()
	}
}

func (o *StepCreateTaskOptions) createEffectiveProjectConfigFromOptions(tektonClient tektonclient.Interface, jxClient jxclient.Interface, kubeClient kubeclient.Interface, ns string, pipelineName string) (*config.ProjectConfig, error) {
	if o.InterpretMode {
		// lets allow this command to run in an empty cluster
		o.RemoteCluster = true
	}
	settings, err := o.TeamSettings()
	if err != nil {
		return nil, err
	}

	if o.ProjectID == "" {
		if !o.RemoteCluster {
			data, err := kube.ReadInstallValues(kubeClient, ns)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read install values from namespace %s", ns)
			}
			o.ProjectID = data["projectID"]
		}
		if o.ProjectID == "" {
			o.ProjectID = "todo"
		}
	}
	if o.DefaultImage == "" {
		o.DefaultImage = syntax.DefaultContainerImage
	}

	if o.KanikoImage == "" {
		o.KanikoImage = syntax.KanikoDockerImage
	}
	o.KanikoImage, err = o.VersionResolver.ResolveDockerImage(o.KanikoImage)
	if err != nil {
		return nil, err
	}
	if o.KanikoSecretMount == "" {
		o.KanikoSecretMount = kanikoSecretMount
	}

	if o.DockerRegistry == "" && !o.InterpretMode {
		data, err := kube.GetConfigMapData(kubeClient, kube.ConfigMapJenkinsDockerRegistry, ns)
		if err != nil {
			return nil, fmt.Errorf("could not find ConfigMap %s in namespace %s: %s", kube.ConfigMapJenkinsDockerRegistry, ns, err)
		}
		o.DockerRegistry = data["docker.registry"]
		if o.DockerRegistry == "" {
			return nil, util.MissingOption("docker-registry")
		}
	}

	if o.BuildNumber == "" {
		if *o.NoApply || o.DryRun || o.InterpretMode {
			o.BuildNumber = "1"
		} else {
			log.Logger().Debugf("generating build number...")
			o.BuildNumber, err = tekton.GenerateNextBuildNumber(tektonClient, jxClient, ns, o.GitInfo, o.Branch, o.Duration, o.Context, false)
			if err != nil {
				return nil, err
			}
			log.Logger().Debugf("generated build number %s for %s", o.BuildNumber, o.CloneGitURL)
		}
	}
	projectConfig, projectConfigFile, err := o.loadProjectConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load project config in dir %s", o.CloneDir)
	}
	if o.BuildPackURL == "" || o.BuildPackRef == "" {
		if projectConfig.BuildPackGitURL != "" {
			o.BuildPackURL = projectConfig.BuildPackGitURL
		} else if o.BuildPackURL == "" {
			o.BuildPackURL = settings.BuildPackURL
		}
		if projectConfig.BuildPackGitURef != "" {
			o.BuildPackRef = projectConfig.BuildPackGitURef
		} else if o.BuildPackRef == "" {
			o.BuildPackRef = settings.BuildPackRef
		}
	}
	if o.BuildPackURL == "" {
		return nil, util.MissingOption("url")
	}
	if o.BuildPackRef == "" {
		return nil, util.MissingOption("ref")
	}
	if o.PipelineKind == "" {
		return nil, util.MissingOption("kind")
	}

	if o.Pack == "" {
		o.Pack = projectConfig.BuildPack
	}
	if o.Pack == "" {
		o.Pack, err = o.DiscoverBuildPack(o.CloneDir, projectConfig, o.Pack)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to discover the build pack")
		}
	}

	if o.Pack == "" {
		return nil, util.MissingOption("pack")
	}

	packsDir, err := gitresolver.InitBuildPack(o.Git(), o.BuildPackURL, o.BuildPackRef)
	if err != nil {
		return nil, err
	}

	resolver, err := gitresolver.CreateResolver(packsDir, o.Git())
	if err != nil {
		return nil, err
	}

	log.Logger().Debug("creating effective project configuration")
	effectiveProjectConfig, err := o.createEffectiveProjectConfig(packsDir, projectConfig, projectConfigFile, resolver, ns)
	return effectiveProjectConfig, err
}

// createEffectiveProjectConfig creates the effective parsed pipeline which is then used to generate the Tekton CRDs.
func (o *StepCreateTaskOptions) createEffectiveProjectConfig(packsDir string, projectConfig *config.ProjectConfig, projectConfigFile string, resolver jenkinsfile.ImportFileResolver, ns string) (*config.ProjectConfig, error) {
	createEffective := &syntaxstep.StepSyntaxEffectiveOptions{
		Pack:              o.Pack,
		BuildPackURL:      o.BuildPackURL,
		BuildPackRef:      o.BuildPackRef,
		Context:           o.Context,
		CustomImage:       o.CustomImage,
		DefaultImage:      o.DefaultImage,
		UseKaniko:         !o.NoKaniko,
		KanikoImage:       o.KanikoImage,
		ProjectID:         o.ProjectID,
		DockerRegistry:    o.DockerRegistry,
		DockerRegistryOrg: o.DockerRegistryOrg,
		SourceName:        o.SourceName,
		CustomEnvs:        o.CustomEnvs,
		GitInfo:           o.GitInfo,
		PodTemplates:      o.PodTemplates,
		VersionResolver:   o.VersionResolver,
		ValidateInCluster: !o.InterpretMode,
	}
	commonCopy := *o.CommonOptions
	createEffective.CommonOptions = &commonCopy

	effectiveProjectConfig, err := createEffective.CreateEffectivePipeline(packsDir, projectConfig, projectConfigFile, resolver)
	if err != nil {
		return nil, errors.Wrapf(err, "effective pipeline creation failed")
	}
	// lets allow a `jenkins-x.yml` to specify we want to disable release prepare mode which can be useful for
	// working with custom jenkins pipelines in custom jenkins servers
	if projectConfig.NoReleasePrepare {
		o.NoReleasePrepare = true
	}

	parsed, err := effectiveProjectConfig.GetPipeline(o.PipelineKind)
	if err != nil {
		return nil, err
	}

	if o.EffectivePipeline {
		log.Logger().Info("Successfully generated effective pipeline:")
		effective := &jenkinsfile.PipelineLifecycles{
			Pipeline: parsed,
		}
		effectiveYaml, _ := yaml.Marshal(effective)
		log.Logger().Infof("%s", effectiveYaml)
		return nil, nil
	}
	return effectiveProjectConfig, nil
}

// GenerateTektonCRDs creates the Pipeline, Task, PipelineResource, PipelineRun, and PipelineStructure CRDs that will be applied to actually kick off the pipeline
func (o *StepCreateTaskOptions) generateTektonCRDs(effectiveProjectConfig *config.ProjectConfig, ns string, pipelineName string) (*tekton.CRDWrapper, error) {
	if effectiveProjectConfig == nil {
		return nil, errors.New("effective project config cannot be nil")
	}

	effectivePipeline, err := effectiveProjectConfig.GetPipeline(o.PipelineKind)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to extract the requested pipeline")
	}

	crdParams := syntax.CRDsFromPipelineParams{
		PipelineIdentifier: pipelineName,
		BuildIdentifier:    o.BuildNumber,
		Namespace:          ns,
		PodTemplates:       o.PodTemplates,
		VersionsDir:        o.VersionResolver.VersionsDir,
		TaskParams:         o.getDefaultTaskInputs().Params,
		SourceDir:          o.SourceName,
		Labels:             o.labels,
		DefaultImage:       "",
		InterpretMode:      o.InterpretMode,
	}
	pipeline, tasks, structure, err := effectivePipeline.GenerateCRDs(crdParams)
	if err != nil {
		return nil, errors.Wrapf(err, "generation failed for Pipeline")
	}

	tasks, pipeline = o.enhanceTasksAndPipeline(tasks, pipeline, effectiveProjectConfig.PipelineConfig.Env)
	resources := []*pipelineapi.PipelineResource{tekton.GenerateSourceRepoResource(pipelineName, o.GitInfo, o.Revision)}

	var timeout *metav1.Duration
	if effectivePipeline.Options != nil && effectivePipeline.Options.Timeout != nil {
		timeout, err = effectivePipeline.Options.Timeout.ToDuration()
		if err != nil {
			return nil, errors.Wrapf(err, "parsing of pipeline timeout failed")
		}
	}
	prLabels := util.MergeMaps(o.labels, effectivePipeline.GetPodLabels())
	run := tekton.CreatePipelineRun(resources, pipeline.Name, pipeline.APIVersion, prLabels, o.ServiceAccount, o.pipelineParams, timeout, effectivePipeline.GetPossibleAffinityPolicy(pipeline.Name), effectivePipeline.GetTolerations())

	tektonCRDs, err := tekton.NewCRDWrapper(pipeline, tasks, resources, structure, run)
	if err != nil {
		return nil, err
	}

	return tektonCRDs, nil
}

func (o *StepCreateTaskOptions) loadProjectConfig() (*config.ProjectConfig, string, error) {
	if o.Context != "" {
		fileName := filepath.Join(o.CloneDir, fmt.Sprintf("jenkins-x-%s.yml", o.Context))
		exists, err := util.FileExists(fileName)
		if err != nil {
			return nil, fileName, errors.Wrapf(err, "failed to check if file exists %s", fileName)
		}
		if exists {
			config, err := config.LoadProjectConfigFile(fileName)
			return config, fileName, err
		}
	}
	return config.LoadProjectConfig(o.CloneDir)
}

func (o *StepCreateTaskOptions) effectiveProjectConfigExists() (bool, error) {
	fileName := o.CloneDir

	if o.Context == "" {
		fileName = filepath.Join(fileName, "jenkins-x-effective.yml")
	} else {
		fileName = filepath.Join(fileName, fmt.Sprintf("jenkins-x-%s-effective.yml", o.Context))
	}

	exists, err := util.FileExists(fileName)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check existence of %s", fileName)
	}
	return exists, nil
}

func (o *StepCreateTaskOptions) loadEffectiveProjectConfig() (*config.ProjectConfig, error) {
	fileName := o.CloneDir

	if o.Context == "" {
		fileName = filepath.Join(fileName, "jenkins-x-effective.yml")
	} else {
		fileName = filepath.Join(fileName, fmt.Sprintf("jenkins-x-%s-effective.yml", o.Context))
	}

	projectConfig, err := config.LoadProjectConfigFile(fileName)
	return projectConfig, err
}

// getDefaultTaskInputs gets the base, built-in task parameters as an Input.
func (o *StepCreateTaskOptions) getDefaultTaskInputs() *pipelineapi.Inputs {
	inputs := &pipelineapi.Inputs{}
	taskParams := o.createTaskParams()
	if len(taskParams) > 0 {
		inputs.Params = taskParams
	}
	return inputs
}

func (o *StepCreateTaskOptions) enhanceTaskWithVolumesEnvAndInputs(task *pipelineapi.Task, env []corev1.EnvVar, inputs pipelineapi.Inputs) {
	volumes := task.Spec.Volumes
	for i, step := range task.Spec.Steps {
		volumes = o.modifyVolumes(&step.Container, volumes)
		o.modifyEnvVars(&step.Container, env)
		task.Spec.Steps[i] = step
	}

	task.Spec.Volumes = volumes
	if task.Spec.Inputs == nil {
		task.Spec.Inputs = &inputs
	} else {
		task.Spec.Inputs.Params = inputs.Params
	}
}

// enhanceTasksAndPipeline takes a slice of Tasks and a Pipeline and modifies them to include built-in volumes, environment variables, and parameters
func (o *StepCreateTaskOptions) enhanceTasksAndPipeline(tasks []*pipelineapi.Task, pipeline *pipelineapi.Pipeline, env []corev1.EnvVar) ([]*pipelineapi.Task, *pipelineapi.Pipeline) {
	taskInputs := o.getDefaultTaskInputs()

	for _, t := range tasks {
		o.enhanceTaskWithVolumesEnvAndInputs(t, env, *taskInputs)
	}

	taskParams := o.createPipelineTaskParams()

	for i, pt := range pipeline.Spec.Tasks {
		for _, tp := range taskParams {
			if !hasPipelineParam(pt.Params, tp.Name) {
				pt.Params = append(pt.Params, tp)
				pipeline.Spec.Tasks[i] = pt
			}
		}
	}

	pipeline.Spec.Params = o.createPipelineParams()

	if pipeline.APIVersion == "" {
		pipeline.APIVersion = syntax.TektonAPIVersion
	}
	if pipeline.Kind == "" {
		pipeline.Kind = "Pipeline"
	}

	return tasks, pipeline
}

func (o *StepCreateTaskOptions) createTaskParams() []pipelineapi.ParamSpec {
	taskParams := []pipelineapi.ParamSpec{}
	for _, param := range o.pipelineParams {
		name := param.Name
		description := ""
		defaultValue := ""
		switch name {
		case "version":
			description = "the version number for this pipeline which is used as a tag on docker images and helm charts"
			defaultValue = o.version
		case "build_id":
			description = "the PipelineRun build number"
			defaultValue = o.BuildNumber
		}
		stringParamDefaultValue := syntax.StringParamValue(defaultValue)
		taskParams = append(taskParams, pipelineapi.ParamSpec{
			Name:        name,
			Description: description,
			Default:     &stringParamDefaultValue,
			Type:        pipelineapi.ParamTypeString,
		})
	}
	return taskParams
}

func (o *StepCreateTaskOptions) createPipelineParams() []pipelineapi.ParamSpec {
	answer := []pipelineapi.ParamSpec{}
	taskParams := o.createTaskParams()
	for _, tp := range taskParams {
		answer = append(answer, pipelineapi.ParamSpec{
			Name:        tp.Name,
			Description: tp.Description,
			Default:     tp.Default,
			Type:        tp.Type,
		})
	}
	return answer
}

func (o *StepCreateTaskOptions) createPipelineTaskParams() []pipelineapi.Param {
	ptp := []pipelineapi.Param{}
	for _, p := range o.pipelineParams {
		ptp = append(ptp, pipelineapi.Param{
			Name:  p.Name,
			Value: syntax.StringParamValue(fmt.Sprintf("$(params.%s)", p.Name)),
		})
	}
	return ptp
}

func (o *StepCreateTaskOptions) setBuildValues() error {
	labels := map[string]string{}
	if o.GitInfo != nil {
		labels[tekton.LabelOwner] = o.GitInfo.Organisation
		labels[tekton.LabelRepo] = o.GitInfo.Name
	}
	labels[tekton.LabelBranch] = o.Branch
	if o.Context != "" {
		labels[tekton.LabelContext] = o.Context
	}
	labels[tekton.LabelBuild] = o.BuildNumber
	labels[tekton.LabelType] = tekton.BuildPipeline.String()
	return o.combineLabels(labels)
}

func (o *StepCreateTaskOptions) combineLabels(labels map[string]string) error {
	// add any custom labels
	customLabels, err := util.ExtractKeyValuePairs(o.CustomLabels, "=")
	if err != nil {
		return err
	}
	o.labels = util.MergeMaps(labels, customLabels)
	return nil
}

func (o *StepCreateTaskOptions) getWorkspaceDir() string {
	return filepath.Join("/workspace", o.SourceName)
}

func (o *StepCreateTaskOptions) modifyEnvVars(container *corev1.Container, globalEnv []corev1.EnvVar) {
	envVars := []corev1.EnvVar{}
	for _, e := range container.Env {
		name := e.Name
		if name != "JENKINS_URL" {
			envVars = append(envVars, e)
		}
	}
	if kube.GetSliceEnvVar(envVars, "DOCKER_REGISTRY") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "DOCKER_REGISTRY",
			Value: o.DockerRegistry,
		})
	}
	if kube.GetSliceEnvVar(envVars, "BUILD_NUMBER") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "BUILD_NUMBER",
			Value: o.BuildNumber,
		})
	}
	if o.PipelineKind != "" && kube.GetSliceEnvVar(envVars, "PIPELINE_KIND") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PIPELINE_KIND",
			Value: o.PipelineKind,
		})
	}
	if o.Context != "" && kube.GetSliceEnvVar(envVars, "PIPELINE_CONTEXT") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PIPELINE_CONTEXT",
			Value: o.Context,
		})
	}
	gitUserName := util.DefaultGitUserName
	gitUserEmail := util.DefaultGitUserEmail

	settings, err := o.TeamSettings()
	// If there's an error getting the team settings, just ignore it and keep using the defaults.
	if err == nil {
		if settings.PipelineUsername != "" {
			gitUserName = settings.PipelineUsername
		}
		if settings.PipelineUserEmail != "" {
			gitUserEmail = settings.PipelineUserEmail
		}
	}

	if kube.GetSliceEnvVar(envVars, "GIT_AUTHOR_NAME") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_AUTHOR_NAME",
			Value: gitUserName,
		})
	}
	if kube.GetSliceEnvVar(envVars, "GIT_AUTHOR_EMAIL") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_AUTHOR_EMAIL",
			Value: gitUserEmail,
		})
	}
	if kube.GetSliceEnvVar(envVars, "GIT_COMMITTER_NAME") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_COMMITTER_NAME",
			Value: gitUserName,
		})
	}
	if kube.GetSliceEnvVar(envVars, "GIT_COMMITTER_EMAIL") == nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "GIT_COMMITTER_EMAIL",
			Value: gitUserEmail,
		})
	}

	gitInfo := o.GitInfo
	branch := o.Branch
	if gitInfo != nil {
		u := gitInfo.CloneURL
		if u != "" && kube.GetSliceEnvVar(envVars, "SOURCE_URL") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "SOURCE_URL",
				Value: u,
			})
		}
		repo := gitInfo.Name
		owner := gitInfo.Organisation
		if owner != "" && kube.GetSliceEnvVar(envVars, "REPO_OWNER") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REPO_OWNER",
				Value: owner,
			})
		}
		if repo != "" && kube.GetSliceEnvVar(envVars, "REPO_NAME") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "REPO_NAME",
				Value: repo,
			})
		}
		if owner != "" && repo != "" && branch != "" {
			jobName := fmt.Sprintf("%s/%s/%s", owner, repo, branch)
			if kube.GetSliceEnvVar(envVars, "JOB_NAME") == nil {
				envVars = append(envVars, corev1.EnvVar{
					Name:  "JOB_NAME",
					Value: jobName,
				})
			}
		}

		// lets keep the APP_NAME environment variable we need for previews
		if repo != "" && kube.GetSliceEnvVar(envVars, "APP_NAME") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "APP_NAME",
				Value: repo,
			})
		}
	}
	if branch != "" {
		if kube.GetSliceEnvVar(envVars, util.EnvVarBranchName) == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  util.EnvVarBranchName,
				Value: branch,
			})
		}
	}
	if o.InterpretMode {
		if kube.GetSliceEnvVar(envVars, "JX_INTERPRET_PIPELINE") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "JX_INTERPRET_PIPELINE",
				Value: "true",
			})
		}
	} else {
		if kube.GetSliceEnvVar(envVars, "JX_BATCH_MODE") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "JX_BATCH_MODE",
				Value: "true",
			})
		}
	}

	for _, param := range o.pipelineParams {
		name := strings.ToUpper(param.Name)
		if kube.GetSliceEnvVar(envVars, name) == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  name,
				Value: "$(inputs.params." + param.Name + ")",
			})
		}
	}

	for _, e := range globalEnv {
		if kube.GetSliceEnvVar(envVars, e.Name) == nil && e.ValueFrom != nil {
			envVars = append(envVars, e)
		}
	}

	for i := range envVars {
		if envVars[i].Name == "XDG_CONFIG_HOME" {
			envVars[i].Value = "/workspace/xdg_config"
		}
	}

	if isKanikoExecutorStep(container) && !o.NoKaniko {
		if kube.GetSliceEnvVar(envVars, "GOOGLE_APPLICATION_CREDENTIALS") == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: o.KanikoSecretMount,
			})
		}
	}
	if kube.GetSliceEnvVar(envVars, "PREVIEW_VERSION") == nil && kube.GetSliceEnvVar(envVars, "VERSION") != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "PREVIEW_VERSION",
			Value: "$(inputs.params.version)",
		})
	}
	for k, v := range o.AdditionalEnvVars {
		if kube.GetSliceEnvVar(envVars, k) == nil {
			envVars = append(envVars, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}
	container.Env = envVars
}

func (o *StepCreateTaskOptions) modifyVolumes(container *corev1.Container, volumes []corev1.Volume) []corev1.Volume {
	answer := volumes

	if isKanikoExecutorStep(container) && !o.NoKaniko {
		kubeClient, ns, err := o.KubeClientAndDevNamespace()
		if err != nil {
			log.Logger().Warnf("failed to find kaniko secret: %s", err)
		} else {
			if o.KanikoSecret == "" {
				o.KanikoSecret = kanikoSecretName
			}
			if o.KanikoSecretKey == "" {
				o.KanikoSecretKey = kanikoSecretKey
			}
			secretName := o.KanikoSecret
			key := o.KanikoSecretKey
			secret, err := kubeClient.CoreV1().Secrets(ns).Get(secretName, metav1.GetOptions{})
			if err != nil {
				log.Logger().Warnf("failed to find secret %s in namespace %s: %s", secretName, ns, err)
			} else if secret != nil && secret.Data != nil && secret.Data[key] != nil {
				// lets mount the kaniko secret
				volumeName := "kaniko-secret"
				_, fileName := filepath.Split(o.KanikoSecretMount)

				volume := corev1.Volume{
					Name: volumeName,
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: secretName,
							Items: []corev1.KeyToPath{
								{
									Key:  key,
									Path: fileName,
								},
							},
						},
					},
				}
				if !kube.ContainsVolume(answer, volume) {
					answer = append(answer, volume)
				}

				mountDir, _ := filepath.Split(o.KanikoSecretMount)
				mountDir = strings.TrimSuffix(mountDir, "/")
				volumeMount := corev1.VolumeMount{
					Name:      volumeName,
					MountPath: mountDir,
					ReadOnly:  true,
				}
				if !kube.ContainsVolumeMount(container.VolumeMounts, volumeMount) {
					container.VolumeMounts = append(container.VolumeMounts, volumeMount)
				}
			}
		}
	}

	podInfoName := "podinfo"
	volume := corev1.Volume{
		Name: podInfoName,
		VolumeSource: corev1.VolumeSource{
			DownwardAPI: &corev1.DownwardAPIVolumeSource{
				Items: []corev1.DownwardAPIVolumeFile{
					{
						Path: "labels",
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.labels",
						},
					},
				},
			},
		},
	}
	if !kube.ContainsVolume(volumes, volume) {
		answer = append(answer, volume)
	}
	volumeMount := corev1.VolumeMount{
		Name:      podInfoName,
		MountPath: "/etc/podinfo",
		ReadOnly:  true,
	}
	if !kube.ContainsVolumeMount(container.VolumeMounts, volumeMount) {
		container.VolumeMounts = append(container.VolumeMounts, volumeMount)
	}
	return answer
}

func (o *StepCreateTaskOptions) cloneGitRepositoryToTempDir(gitURL string, branch string, pullRequestNumber string, revision string) string {
	var tmpDir string
	err := o.Retry(3, time.Second*2, func() error {
		var err error
		tmpDir, err = ioutil.TempDir("", "git")
		if err != nil {
			return err
		}
		log.Logger().Infof("shallow cloning repository %s to temp dir %s", gitURL, tmpDir)
		err = o.Git().Init(tmpDir)
		if err != nil {
			return errors.Wrapf(err, "failed to init a new git repository in directory %s", tmpDir)
		}
		log.Logger().Debugf("ran git init in %s", tmpDir)
		err = o.Git().AddRemote(tmpDir, "origin", gitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to add remote origin with url %s in directory %s", gitURL, tmpDir)
		}
		log.Logger().Debugf("ran git add remote origin %s in %s", gitURL, tmpDir)
		commitish := make([]string, 0)
		if pullRequestNumber != "" {
			pr := fmt.Sprintf("pull/%s/head:%s", pullRequestNumber, branch)
			log.Logger().Debugf("will fetch %s for %s in dir %s", pr, gitURL, tmpDir)
			commitish = append(commitish, pr)
		}
		if revision != "" {
			log.Logger().Debugf("will fetch %s for %s in dir %s", revision, gitURL, tmpDir)
			commitish = append(commitish, revision)
		} else {
			commitish = append(commitish, "master")
		}
		err = o.Git().FetchBranchShallow(tmpDir, "origin", commitish...)
		if err != nil {
			return errors.Wrapf(err, "failed to fetch %s from %s in directory %s", commitish, gitURL, tmpDir)
		}
		if revision != "" {
			err = o.Git().Checkout(tmpDir, revision)
			if err != nil {
				return errors.Wrapf(err, "failed to checkout revision %s", revision)
			}
		} else {
			err = o.Git().Checkout(tmpDir, "master")
			if err != nil {
				return errors.Wrapf(err, "failed to checkout revision master")
			}
		}
		return nil
	})

	// if we have failed to clone three times it's likely things wont recover so lets kill the process and let
	// kubernetes reschedule a new pod, however if it's because the revision didn't exist, then it's more likely it's
	// because that object is already deleted by a force-push
	if err != nil {
		if gits.IsUnadvertisedObjectError(err) {
			log.Logger().Warnf("Commit most likely overwritten by force-push, so ignorning underlying error %v", err)
		} else {
			log.Logger().Fatalf("failed to clone three times it's likely things wont recover so lets kill the process; %v", err)
			panic(err)
		}
	}

	return tmpDir
}

// parsePullRefs creates a Prow PullRefs struct from the PULL_REFS environment variable, if it id set.
func (o *StepCreateTaskOptions) parsePullRefs() (*tekton.PullRefs, error) {
	var pr *tekton.PullRefs
	var err error

	for _, envVar := range o.CustomEnvs {
		parts := strings.Split(envVar, "=")
		if parts[0] == "PULL_REFS" {
			pr, err = tekton.ParsePullRefs(parts[1])
			if err != nil {
				return pr, err
			}
		}
	}

	return pr, nil
}

// mergePullRefs merges the pull refs specified into the git repository specified via CloneDir.
func (o *StepCreateTaskOptions) mergePullRefs(pr *tekton.PullRefs, cloneDir string) error {
	if pr == nil {
		return nil
	}
	var shas []string
	for _, sha := range pr.ToMerge {
		shas = append(shas, sha)
	}

	mergeOpts := git.StepGitMergeOptions{
		StepOptions: step.StepOptions{
			CommonOptions: o.CommonOptions,
		},
		Dir:        cloneDir,
		BaseSHA:    pr.BaseSha,
		SHAs:       shas,
		BaseBranch: pr.BaseBranch,
	}
	mergeOpts.Verbose = true
	err := mergeOpts.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to merge git shas %s with base sha %s", shas, pr.BaseSha)
	}
	return nil
}

func (o *StepCreateTaskOptions) viewSteps(tasks ...*pipelineapi.Task) error {
	table := o.CreateTable()
	showTaskName := len(tasks) > 1
	if showTaskName {
		table.AddRow("TASK", "NAME", "COMMAND", "IMAGE")
	} else {
		table.AddRow("NAME", "COMMAND", "IMAGE")
	}
	for _, task := range tasks {
		for _, step := range task.Spec.Steps {
			command := append([]string{}, step.Command...)
			command = append(command, step.Args...)
			commands := strings.Join(command, " ")
			if showTaskName {
				table.AddRow(task.Name, step.Name, commands, step.Image)
			} else {
				table.AddRow(step.Name, commands, step.Image)
			}
		}
	}
	table.Render()
	return nil
}

func getVersionFromFile(dir string) (string, error) {
	var version string
	versionFile := filepath.Join(dir, "VERSION")
	exist, err := util.FileExists(versionFile)
	if err != nil {
		return "", err
	}
	if exist {
		data, err := ioutil.ReadFile(versionFile)
		if err != nil {
			return "", errors.Wrapf(err, "failed to read file %s", versionFile)
		}
		text := strings.TrimSpace(string(data))
		if text == "" {
			log.Logger().Warnf("versions file %s is empty!", versionFile)
		} else {
			version = text
			if version != "" {
				return version, nil
			}
		}
	}
	return "", errors.New("failed to read file " + versionFile)
}

func (o *StepCreateTaskOptions) setBuildVersion(projectConfig *config.ProjectConfig) error {
	if o.NoReleasePrepare || o.ViewSteps || o.EffectivePipeline || projectConfig.NoReleasePrepare {
		return nil
	}
	pipelineConfig := projectConfig.PipelineConfig
	version := ""

	if o.DryRun {
		version, err := getVersionFromFile(o.CloneDir)
		if err != nil {
			log.Logger().Warn("No version file or incorrect content; using 0.0.1 as version")
			version = "0.0.1"
		}
		o.version = version
		o.setRevisionForReleasePipeline(version)
		o.pipelineParams = append(o.pipelineParams, pipelineapi.Param{
			Name:  "version",
			Value: syntax.StringParamValue(o.version),
		})
		log.Logger().Infof("Version used: '%s'", util.ColorInfo(version))

		return nil
	} else if o.PipelineKind == jenkinsfile.PipelineKindRelease {
		release := pipelineConfig.Pipelines.Release
		if release == nil {
			return fmt.Errorf("no Release pipeline available")
		}
		sv := release.SetVersion
		if sv == nil {
			command := "jx step next-version --use-git-tag-only --tag"
			if o.SemanticRelease {
				command = "jx step next-version --semantic-release --tag"
			}
			// lets create a default set version pipeline
			sv = &jenkinsfile.PipelineLifecycle{
				Steps: []*syntax.Step{
					{
						Command: command,
						Name:    "next-version",
						Comment: "tags git with the next version",
					},
				},
			}
		}
		steps := sv.Steps
		err := o.invokeSteps(steps)
		if err != nil {
			return err
		}
		version, err = getVersionFromFile(o.CloneDir)
		if err != nil {
			return err
		}
		o.setRevisionForReleasePipeline(version)
	} else {
		// lets use the branch name if we can find it for the version number
		branch := o.Branch
		if branch == "" {
			branch = o.Revision
		}
		buildNumber := o.BuildNumber
		o.previewVersionPrefix = "0.0.0-SNAPSHOT-" + branch + "-"
		version = o.previewVersionPrefix + buildNumber
	}
	if version != "" {
		if !hasParam(o.pipelineParams, "version") {
			o.pipelineParams = append(o.pipelineParams, pipelineapi.Param{
				Name:  "version",
				Value: syntax.StringParamValue(version),
			})
		}
	}
	o.version = version
	if o.BuildNumber != "" {
		if !hasParam(o.pipelineParams, "build_id") {
			o.pipelineParams = append(o.pipelineParams, pipelineapi.Param{
				Name:  "build_id",
				Value: syntax.StringParamValue(o.BuildNumber),
			})
		}
	}
	return nil
}

func (o *StepCreateTaskOptions) setRevisionForReleasePipeline(version string) {
	if o.UseBranchAsRevision {
		o.Revision = o.Branch
	} else {
		o.Revision = "v" + version
	}
}

func hasParam(params []pipelineapi.Param, name string) bool {
	for _, param := range params {
		if param.Name == name {
			return true
		}
	}
	return false
}

func hasPipelineParam(params []pipelineapi.Param, name string) bool {
	for _, param := range params {
		if param.Name == name {
			return true
		}
	}
	return false
}

func (o *StepCreateTaskOptions) runStepCommand(step *syntax.Step) error {
	c := step.GetFullCommand()
	if c == "" {
		return nil
	}
	log.Logger().Infof("running command: %s", util.ColorInfo(c))

	commandText := strings.Replace(step.GetFullCommand(), "\\$", "$", -1)

	cmd := util.Command{
		Name: "/bin/sh",
		Args: []string{"-c", commandText},
		Out:  o.Out,
		Err:  o.Err,
		Dir:  o.CloneDir,
	}
	result, err := cmd.RunWithoutRetry()
	if err != nil {
		return err
	}
	log.Logger().Infof("%s", result)
	return nil
}

func (o *StepCreateTaskOptions) invokeSteps(steps []*syntax.Step) error {
	for _, s := range steps {
		if s == nil {
			continue
		}
		if len(s.Steps) > 0 {
			err := o.invokeSteps(s.Steps)
			if err != nil {
				return err
			}
		}
		when := strings.TrimSpace(s.When)
		if when == "!prow" || s.GetCommand() == "" {
			continue
		}
		err := o.runStepCommand(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *StepCreateTaskOptions) dockerImage(projectConfig *config.ProjectConfig, gitInfo *gits.GitRepository) string {
	dockerRegistry := o.getDockerRegistry(projectConfig)

	dockerRegistryOrg := o.DockerRegistryOrg
	if dockerRegistryOrg == "" {
		dockerRegistryOrg = o.GetDockerRegistryOrg(projectConfig, gitInfo)
	}
	appName := gitInfo.Name
	return dockerRegistry + "/" + dockerRegistryOrg + "/" + appName
}

func (o *StepCreateTaskOptions) getDockerRegistry(projectConfig *config.ProjectConfig) string {
	dockerRegistry := o.DockerRegistry
	if dockerRegistry == "" {
		dockerRegistry = o.GetDockerRegistry(projectConfig)
	}
	return dockerRegistry
}

func (o *StepCreateTaskOptions) getClientsAndNamespace() (tektonclient.Interface, jxclient.Interface, kubeclient.Interface, string, error) {
	tektonClient, _, err := o.TektonClient()
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to create Tekton client")
	}

	jxClient, _, err := o.JXClient()
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to create JX client")
	}

	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, nil, nil, "", errors.Wrap(err, "unable to create Kube client")
	}

	return tektonClient, jxClient, kubeClient, ns, nil
}

func (o *StepCreateTaskOptions) interpretPipeline(ns string, projectConfig *config.ProjectConfig, crds *tekton.CRDWrapper) error {
	steps := []corev1.Container{}
	for _, task := range crds.Tasks() {
		for _, s := range task.Spec.Steps {
			steps = append(steps, s.Container)
		}
	}

	if o.StartStep != "" {
		found := false
		for i, step := range steps {
			if step.Name == o.StartStep {
				found = true
				steps = steps[i:]
				break
			}
		}
		if !found {
			names := []string{}
			for _, step := range steps {
				names = append(names, step.Name)
			}
			return util.InvalidOption("start-step", o.StartStep, names)
		}
	}

	if o.EndStep != "" {
		found := false
		for i, step := range steps {
			if step.Name == o.EndStep {
				found = true
				steps = steps[:i+1]
				break
			}
		}
		if !found {
			names := []string{}
			for _, step := range steps {
				names = append(names, step.Name)
			}
			return util.InvalidOption("end-step", o.EndStep, names)
		}
	}

	for _, step := range steps {
		err := o.interpretStep(ns, &step)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *StepCreateTaskOptions) interpretStep(ns string, step *corev1.Container) error {
	command := step.Command
	if len(command) == 0 {
		return nil
	}

	// ignore some unnecessary commands
	// TODO is there a nicer way to disable the git-merge step?
	if step.Name == "git-merge" {
		return nil
	}
	commandAndArgs := append(step.Command, step.Args...)
	commandLine := strings.Join(commandAndArgs, " ")
	dir := step.WorkingDir
	if dir != "" {
		workspaceDir := o.getWorkspaceDir()
		if strings.HasPrefix(dir, workspaceDir) {
			curDir := o.CloneDir
			if curDir == "" {
				var err error
				curDir, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			relPath, err := filepath.Rel(workspaceDir, dir)
			if err != nil {
				return err
			}
			dir = filepath.Join(curDir, relPath)
		}
	}

	envMap := createEnvMapForInterpretExecution(step.Env)

	suffix := ""
	if o.Verbose {
		suffix = fmt.Sprintf(" with env: %s", util.ColorInfo(fmt.Sprintf("%#v", envMap)))
	}
	path, err := filepath.Abs(dir)
	if err != nil {
		path = dir
	}
	log.Logger().Infof("\nSTEP: %s command: %s in dir: %s%s\n\n", util.ColorInfo(step.Name), util.ColorInfo(commandLine), util.ColorInfo(path), suffix)

	if !o.DryRun {
		cmd := util.Command{
			Name: commandAndArgs[0],
			Args: commandAndArgs[1:],
			Dir:  dir,
			Out:  os.Stdout,
			Err:  os.Stdout,
			In:   os.Stdin,
			Env:  envMap,
		}
		_, err := cmd.RunWithoutRetry()
		if err != nil {
			return err
		}
	} else {
		log.Logger().Infof("%s", envMap)
	}

	return nil
}

func createEnvMapForInterpretExecution(envVars []corev1.EnvVar) map[string]string {
	m := map[string]string{}
	for _, envVar := range envVars {
		m[envVar.Name] = envVar.Value
	}

	if _, exists := m["JX_LOG_LEVEL"]; !exists {
		m["JX_LOG_LEVEL"] = log.GetLevel()
	}

	return m
}

// isKanikoExecutorStep looks at a container and determines whether its command or args starts with /kaniko/executor.
func isKanikoExecutorStep(container *corev1.Container) bool {
	return strings.HasPrefix(strings.Join(container.Command, " "), "/kaniko/executor") ||
		(len(container.Args) > 0 && strings.HasPrefix(strings.Join(container.Args, " "), "/kaniko/executor"))
}
