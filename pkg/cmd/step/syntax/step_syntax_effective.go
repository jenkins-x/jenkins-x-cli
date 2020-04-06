package syntax

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/jenkinsfile/gitresolver"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/tekton/syntax"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// StepSyntaxEffectiveOptions contains the command line flags
type StepSyntaxEffectiveOptions struct {
	step.StepOptions

	Pack              string
	BuildPackURL      string
	BuildPackRef      string
	Context           string
	CustomImage       string
	DefaultImage      string
	UseKaniko         bool
	KanikoImage       string
	ProjectID         string
	DockerRegistry    string
	DockerRegistryOrg string
	SourceName        string
	CustomEnvs        []string
	OutputFile        string
	ShortView         bool

	ValidateInCluster bool

	PodTemplates map[string]*corev1.Pod

	GitInfo         *gits.GitRepository
	VersionResolver *versionstream.VersionResolver
}

var (
	stepSyntaxEffectiveLong = templates.LongDesc(`
		Reads the appropriate jenkins-x.yml, depending on context, from the current directory, if one exists, and outputs an effective representation of the pipelines
`)

	stepSyntaxEffectiveExample = templates.Examples(`
		# view the effective pipeline
		jx step syntax effective

		# view the short version of the effective pipeline
		jx step syntax effective -s

`)
)

// NewCmdStepSyntaxEffective Creates a new Command object
func NewCmdStepSyntaxEffective(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepSyntaxEffectiveOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "effective",
		Short:   "Outputs an effective representation of the pipeline to be executed",
		Long:    stepSyntaxEffectiveLong,
		Example: stepSyntaxEffectiveExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringArrayVarP(&options.CustomEnvs, "env", "e", nil, "List of custom environment variables to be applied to resources that are created")

	options.addFlags(cmd)
	return cmd
}

func (o *StepSyntaxEffectiveOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.OutDir, "output-dir", "", "", "The directory to write the output to as YAML. Defaults to STDOUT if neither --output-dir nor --output-file is specified.")
	cmd.Flags().StringVarP(&o.OutputFile, "output-file", "", "", "The file to write the output to as YAML. If unspecified and --output-dir is specified, the filename defaults to 'jenkins-x[-context]-effective.yml'")
	cmd.Flags().StringVarP(&o.Pack, "pack", "p", "", "The build pack name. If none is specified its discovered from the source code")
	cmd.Flags().StringVarP(&o.BuildPackURL, "url", "u", "", "The URL for the build pack Git repository")
	cmd.Flags().StringVarP(&o.BuildPackRef, "ref", "r", "", "The Git reference (branch,tag,sha) in the Git repository to use")
	cmd.Flags().StringVarP(&o.Context, "context", "c", "", "The pipeline context if there are multiple separate pipelines for a given branch")
	cmd.Flags().StringVarP(&o.ServiceAccount, "service-account", "", "tekton-bot", "The Kubernetes ServiceAccount to use to run the pipeline")
	cmd.Flags().StringVarP(&o.SourceName, "source", "", "source", "The name of the source repository")
	cmd.Flags().StringVarP(&o.CustomImage, "image", "", "", "Specify a custom image to use for the steps which overrides the image in the PodTemplates")
	cmd.Flags().StringVarP(&o.DefaultImage, "default-image", "", syntax.DefaultContainerImage, "Specify the docker image to use if there is no image specified for a step and there's no Pod Template")
	cmd.Flags().BoolVarP(&o.UseKaniko, "use-kaniko", "", true, "Enables using kaniko directly for building docker images")
	cmd.Flags().BoolVarP(&o.ShortView, "short", "s", false, "Use short concise output")
	cmd.Flags().StringVarP(&o.KanikoImage, "kaniko-image", "", syntax.KanikoDockerImage, "The docker image for Kaniko")
	cmd.Flags().StringVarP(&o.ProjectID, "project-id", "", "", "The cloud project ID. If not specified we default to the install project")
	cmd.Flags().StringVarP(&o.DockerRegistry, "docker-registry", "", "", "The Docker Registry host name to use which is added as a prefix to docker images")
	cmd.Flags().StringVarP(&o.DockerRegistryOrg, "docker-registry-org", "", "", "The Docker registry organisation. If blank the git repository owner is used")
	cmd.Flags().BoolVarP(&o.ValidateInCluster, "validate-in-cluster", "", false, "Validate that resources referenced in the effective pipeline, such as volumes, exist in the current context cluster")
}

// Run implements this command
func (o *StepSyntaxEffectiveOptions) Run() error {
	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}

	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "unable to create Kube client")
	}

	if o.ProjectID == "" {
		if !o.RemoteCluster {
			data, err := kube.ReadInstallValues(kubeClient, ns)
			if err != nil {
				return errors.Wrapf(err, "failed to read install values from namespace %s", ns)
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
	if o.VersionResolver == nil {
		o.VersionResolver, err = o.GetVersionResolver()
		if err != nil {
			return err
		}
	}
	if o.KanikoImage == "" {
		o.KanikoImage = syntax.KanikoDockerImage
	}
	o.KanikoImage, err = o.VersionResolver.ResolveDockerImage(o.KanikoImage)
	if err != nil {
		return err
	}
	if o.Verbose {
		log.Logger().Info("setting up docker registry\n")
	}

	if o.DockerRegistry == "" {
		data, err := kube.GetConfigMapData(kubeClient, kube.ConfigMapJenkinsDockerRegistry, ns)
		if err != nil {
			return fmt.Errorf("could not find ConfigMap %s in namespace %s: %s", kube.ConfigMapJenkinsDockerRegistry, ns, err)
		}
		o.DockerRegistry = data["docker.registry"]
		if o.DockerRegistry == "" {
			return util.MissingOption("docker-registry")
		}
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}
	o.GitInfo, err = o.FindGitInfo(workingDir)
	if err != nil {
		return errors.Wrapf(err, "failed to find git information from dir %s", workingDir)
	}
	projectConfig, projectConfigFile, err := o.LoadProjectConfig(workingDir)
	if err != nil {
		return errors.Wrapf(err, "failed to load project config in dir %s", workingDir)
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
		return util.MissingOption("url")
	}
	if o.BuildPackRef == "" {
		return util.MissingOption("ref")
	}

	if o.Pack == "" {
		o.Pack = projectConfig.BuildPack
	}
	if o.Pack == "" {
		o.Pack, err = o.DiscoverBuildPack(workingDir, projectConfig, o.Pack)
		if err != nil {
			return errors.Wrapf(err, "failed to discover the build pack")
		}
	}

	if o.Pack == "" {
		return util.MissingOption("pack")
	}

	o.PodTemplates, err = kube.LoadPodTemplates(kubeClient, ns)
	if err != nil {
		return err
	}

	packsDir, err := gitresolver.InitBuildPack(o.Git(), o.BuildPackURL, o.BuildPackRef)
	if err != nil {
		return err
	}

	resolver, err := gitresolver.CreateResolver(packsDir, o.Git())
	if err != nil {
		return err
	}

	effectiveConfig, err := o.CreateEffectivePipeline(packsDir, projectConfig, projectConfigFile, resolver)
	if err != nil {
		return err
	}

	if o.ShortView {
		effectiveConfig = o.makeConcisePipeline(effectiveConfig)
	}

	effectiveYaml, err := yaml.Marshal(effectiveConfig)
	if err != nil {
		return errors.Wrap(err, "failed to marshal effective pipeline")
	}
	if o.OutDir == "" && o.OutputFile == "" {
		if o.ShortView {
			for _, line := range strings.Split(string(effectiveYaml), "\n") {
				prefix := "command: "
				idx := strings.Index(line, prefix)
				if idx >= 0 {
					line = line[0:idx] + prefix + util.ColorInfo(line[idx+len(prefix):])
				}
				fmt.Printf("%s\n", line)
			}
		} else {
			fmt.Printf("%s\n", effectiveYaml)
		}
	} else {
		outputDir := o.OutDir
		if outputDir == "" {
			outputDir, err = os.Getwd()
			if err != nil {
				return errors.Wrap(err, "failed to get current directory")
			}
		}
		outputFilename := o.OutputFile
		if outputFilename == "" {
			outputFilename = "jenkins-x"
			if o.Context != "" {
				outputFilename += "-" + o.Context
			}
			outputFilename += "-effective.yml"
		}
		outputFile := filepath.Join(outputDir, outputFilename)
		err = ioutil.WriteFile(outputFile, effectiveYaml, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to write effective pipeline to %s", outputFile)
		}
		log.Logger().Infof("Effective pipeline written to %s", outputFile)
	}
	return nil
}

// CreateEffectivePipeline takes a project config and generates the effective version of the pipeline for it, including
// build packs, inheritance, overrides, defaults, etc.
func (o *StepSyntaxEffectiveOptions) CreateEffectivePipeline(packsDir string, projectConfig *config.ProjectConfig, projectConfigFile string, resolver jenkinsfile.ImportFileResolver) (*config.ProjectConfig, error) {
	name := o.Pack
	packDir := filepath.Join(packsDir, name)

	pipelineConfig := projectConfig.PipelineConfig
	if name != "none" {
		pipelineFile := filepath.Join(packDir, jenkinsfile.PipelineConfigFileName)
		exists, err := util.FileExists(pipelineFile)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find build pack pipeline YAML: %s", pipelineFile)
		}
		if !exists {
			return nil, fmt.Errorf("no build pack for %s exists at directory %s", name, packDir)
		}
		pipelineConfig, err = jenkinsfile.LoadPipelineConfig(pipelineFile, resolver, true, false)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load build pack pipeline YAML: %s", pipelineFile)
		}

		localPipelineConfig := projectConfig.PipelineConfig
		if localPipelineConfig != nil {
			err = localPipelineConfig.ExtendPipeline(pipelineConfig, false)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to override PipelineConfig using configuration in file %s", projectConfigFile)
			}
			pipelineConfig = localPipelineConfig
		}
	} else {
		pipelineConfig.PopulatePipelinesFromDefault()
	}

	if pipelineConfig == nil {
		return nil, fmt.Errorf("failed to find PipelineConfig in file %s", projectConfigFile)
	}

	err := o.combineEnvVars(pipelineConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to combine env vars")
	}

	pipelines := pipelineConfig.Pipelines
	// First, handle release.
	if pipelines.Release != nil {
		releaseLifecycles := pipelines.Release

		// lets add a pre-step to setup the credentials
		if releaseLifecycles.Setup == nil {
			releaseLifecycles.Setup = &jenkinsfile.PipelineLifecycle{}
		}
		steps := []*syntax.Step{
			{
				Command: "jx step git credentials",
				Name:    "jx-git-credentials",
			},
		}
		releaseLifecycles.Setup.Steps = append(steps, releaseLifecycles.Setup.Steps...)
		parsed, err := o.createPipelineForKind(jenkinsfile.PipelineKindRelease, releaseLifecycles, pipelines, projectConfig, pipelineConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create effective pipeline for release")
		}
		pipelines.Release = &jenkinsfile.PipelineLifecycles{
			Pipeline:   parsed,
			SetVersion: releaseLifecycles.SetVersion,
		}
	}
	if pipelines.PullRequest != nil {
		prLifecycles := pipelines.PullRequest
		parsed, err := o.createPipelineForKind(jenkinsfile.PipelineKindPullRequest, prLifecycles, pipelines, projectConfig, pipelineConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create effective pipeline for pull request")
		}
		pipelines.PullRequest = &jenkinsfile.PipelineLifecycles{
			Pipeline:   parsed,
			SetVersion: prLifecycles.SetVersion,
		}
	}
	if pipelines.Feature != nil {
		featureLifecycles := pipelines.Feature
		parsed, err := o.createPipelineForKind(jenkinsfile.PipelineKindFeature, featureLifecycles, pipelines, projectConfig, pipelineConfig)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create effective pipeline for pull request")
		}
		pipelines.Feature = &jenkinsfile.PipelineLifecycles{
			Pipeline:   parsed,
			SetVersion: featureLifecycles.SetVersion,
		}
	}

	pipelineConfig.Pipelines = pipelines
	projectConfig.PipelineConfig = pipelineConfig

	return projectConfig, nil
}

func (o *StepSyntaxEffectiveOptions) createPipelineForKind(kind string, lifecycles *jenkinsfile.PipelineLifecycles, pipelines jenkinsfile.Pipelines, projectConfig *config.ProjectConfig, pipelineConfig *jenkinsfile.PipelineConfig) (*syntax.ParsedPipeline, error) {
	var parsed *syntax.ParsedPipeline
	var err error

	if lifecycles != nil && lifecycles.Pipeline != nil {
		parsed = lifecycles.Pipeline
		if projectConfig.BuildPack == "" || projectConfig.BuildPack == "none" {
			for _, override := range pipelines.Overrides {
				if override.MatchesPipeline(kind) {
					// If no step/steps, other overrides, or stage is specified, just remove the whole pipeline.
					// TODO: This is probably pointless functionality.
					if override.Step == nil && len(override.Steps) == 0 && !override.HasNonStepOverrides() && override.Stage == "" {
						return nil, nil
					}
					parsed = syntax.ApplyStepOverridesToPipeline(parsed, override)
				}
			}
		}
	} else {
		args := jenkinsfile.CreatePipelineArguments{
			Lifecycles:        lifecycles,
			PodTemplates:      o.PodTemplates,
			CustomImage:       o.CustomImage,
			DefaultImage:      o.DefaultImage,
			WorkspaceDir:      o.getWorkspaceDir(),
			GitHost:           o.GitInfo.Host,
			GitName:           o.GitInfo.Name,
			GitOrg:            o.GitInfo.Organisation,
			ProjectID:         o.ProjectID,
			DockerRegistry:    o.getDockerRegistry(projectConfig),
			DockerRegistryOrg: o.GetDockerRegistryOrg(projectConfig, o.GitInfo),
			KanikoImage:       o.KanikoImage,
			UseKaniko:         o.UseKaniko,
			// Make sure we don't inject the setversion steps
			NoReleasePrepare: false,
			StepCounter:      0,
		}
		parsed, _, err = pipelineConfig.CreatePipelineForBuildPack(args)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to generate pipeline from build pack")
		}
	}

	// Replace placeholders in directories.
	replacePlaceholderArgs := syntax.StepPlaceholderReplacementArgs{
		WorkspaceDir:      o.getWorkspaceDir(),
		GitName:           o.GitInfo.Name,
		GitOrg:            o.GitInfo.Organisation,
		GitHost:           o.GitInfo.Host,
		ProjectID:         o.ProjectID,
		DockerRegistry:    o.getDockerRegistry(projectConfig),
		DockerRegistryOrg: o.GetDockerRegistryOrg(projectConfig, o.GitInfo),
		KanikoImage:       o.KanikoImage,
		UseKaniko:         o.UseKaniko,
	}
	parsed.ReplacePlaceholdersInStepAndStageDirs(replacePlaceholderArgs)
	parsed.AddContainerEnvVarsToPipeline(pipelineConfig.Env)

	if pipelineConfig.ContainerOptions != nil {
		if parsed.Options == nil {
			parsed.Options = &syntax.RootOptions{}
		}
		mergedContainer, err := syntax.MergeContainers(pipelineConfig.ContainerOptions, parsed.Options.ContainerOptions)
		if err != nil {
			return nil, errors.Wrapf(err, "Could not merge containerOptions from parent")
		}
		parsed.Options.ContainerOptions = mergedContainer
	}

	for _, override := range pipelines.Overrides {
		if override.MatchesPipeline(kind) {
			parsed = syntax.ApplyNonStepOverridesToPipeline(parsed, override)
		}
	}

	var kubeClient kubernetes.Interface
	var ns string

	// If we're validating in the cluster, get the kubeClient. Otherwise it'll be nil and ignored.
	if o.ValidateInCluster {
		kubeClient, ns, err = o.KubeClientAndDevNamespace()
		if err != nil {
			return nil, errors.Wrap(err, "unable to create Kube client")
		}
	}

	// TODO: Seeing weird behavior seemingly related to https://golang.org/doc/faq#nil_error
	// if err is reused, maybe we need to switch return types (perhaps upstream in build-pipeline)?
	ctx := context.Background()
	if validateErr := parsed.ValidateInCluster(ctx, kubeClient, ns); validateErr != nil {
		return nil, errors.Wrapf(validateErr, "validation failed for Pipeline")
	}

	// lets override any container options env vars from any custom injected env vars from the metapipeline client
	if parsed != nil && parsed.Options != nil && parsed.Options.ContainerOptions != nil {
		parsed.Options.ContainerOptions.Env = syntax.CombineEnv(pipelineConfig.Env, parsed.Options.ContainerOptions.Env)
	}
	return parsed, nil
}

func (o *StepSyntaxEffectiveOptions) combineEnvVars(projectConfig *jenkinsfile.PipelineConfig) error {
	// add any custom env vars
	envMap := make(map[string]corev1.EnvVar)
	for _, e := range projectConfig.Env {
		envMap[e.Name] = e
	}
	for _, customEnvVar := range o.CustomEnvs {
		parts := strings.Split(customEnvVar, "=")
		if len(parts) != 2 {
			return errors.Errorf("expected 2 parts to env var but got %v", len(parts))
		}
		e := corev1.EnvVar{
			Name:  parts[0],
			Value: parts[1],
		}
		envMap[e.Name] = e
	}
	projectConfig.Env = syntax.EnvMapToSlice(envMap)
	return nil
}

func (o *StepSyntaxEffectiveOptions) getWorkspaceDir() string {
	return filepath.Join("/workspace", o.SourceName)
}

func (o *StepSyntaxEffectiveOptions) getDockerRegistry(projectConfig *config.ProjectConfig) string {
	dockerRegistry := o.DockerRegistry
	if dockerRegistry == "" {
		dockerRegistry = o.GetDockerRegistry(projectConfig)
	}
	return dockerRegistry
}

// LoadProjectConfig loads the pipeline config from the given workingDir
func (o *StepSyntaxEffectiveOptions) LoadProjectConfig(workingDir string) (*config.ProjectConfig, string, error) {
	if o.Context != "" {
		fileName := filepath.Join(workingDir, fmt.Sprintf("jenkins-x-%s.yml", o.Context))
		exists, err := util.FileExists(fileName)
		if err != nil {
			return nil, fileName, errors.Wrapf(err, "failed to check if file exists %s", fileName)
		}
		if exists {
			config, err := config.LoadProjectConfigFile(fileName)
			return config, fileName, err
		}
	}
	return config.LoadProjectConfig(workingDir)
}

func (o *StepSyntaxEffectiveOptions) makeConcisePipeline(projectConfig *config.ProjectConfig) *config.ProjectConfig {
	for _, pipelines := range projectConfig.PipelineConfig.Pipelines.All() {
		if pipelines != nil {
			if pipelines.Pipeline != nil {
				o.makeConciseStages(pipelines.Pipeline.Stages)
			}
		}
	}
	return projectConfig
}

func (o *StepSyntaxEffectiveOptions) makeConciseStages(stages []syntax.Stage) {
	for i := range stages {
		stage := &stages[i]
		for j := range stage.Steps {
			o.makeConciseStep(&stage.Steps[j])
		}
	}
}

func (o *StepSyntaxEffectiveOptions) makeConciseStep(step *syntax.Step) {
	for _, child := range step.Steps {
		o.makeConciseStep(child)
	}
	c := step.Command
	if c == "" {
		return
	}
	args := step.Arguments
	if len(args) > 0 {
		c = c + " " + strings.Join(args, " ")
		step.Arguments = nil
	}
	step.Command = c
}
