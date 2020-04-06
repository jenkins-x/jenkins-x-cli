package create

import (
	"fmt"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/jenkinsfile/gitresolver"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	createVariableLong = templates.LongDesc(`
		Creates an environment variable in the Jenkins X Pipeline
`)

	createVariableExample = templates.Examples(`
		# Create a new environment variable with a name and value
		jx create var -n CHEESE -v Edam

		# Create a new environment variable with a name and ask the user for the value
		jx create var -n CHEESE 

		# Overrides an environment variable from the build pack
		jx create var 
	`)
)

// CreateVariableOptions the options for the create spring command
type CreateVariableOptions struct {
	options.CreateOptions

	Dir   string
	Name  string
	Value string
}

// NewCmdCreateVariable creates a command object for the "create" command
func NewCmdCreateVariable(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateVariableOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "variable",
		Short:   "Creates an environment variable in the Jenkins X Pipeline",
		Aliases: []string{"var", "envvar"},
		Long:    createVariableLong,
		Example: createVariableExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Name, "name", "n", "", "The name of the environment variable to set")
	cmd.Flags().StringVarP(&options.Value, "value", "v", "", "The value of the environment variable to set")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The root project directory. Defaults to the current dir")
	return cmd
}

// Run implements the command
func (o *CreateVariableOptions) Run() error {
	dir := o.Dir
	var err error
	if dir == "" {
		dir, _, err := o.Git().FindGitConfigDir(o.Dir)
		if err != nil {
			return err
		}
		if dir == "" {
			dir = "."
		}
	}
	projectConfig, fileName, err := config.LoadProjectConfig(dir)
	if err != nil {
		return err
	}
	enrichedProjectConfig, _, err := config.LoadProjectConfig(dir)
	if err != nil {
		return err
	}

	name := o.Name
	value := o.Value
	if o.BatchMode {
		if name == "" {
			return util.MissingOption("name")
		}
		if value == "" {
			return util.MissingOption("value")
		}
	}

	defaultValues, err := o.loadEnvVars(enrichedProjectConfig)
	if err != nil {
		return err
	}
	keys := util.SortedMapKeys(defaultValues)

	if name == "" {
		message := "environment variable name: "
		help := "the name of the environment variable which is usually upper case without spaces or dashes"
		if len(keys) == 0 {
			name, err = util.PickValue(message, "", true, help, o.GetIOFileHandles())
		} else {
			name, err = util.PickName(keys, message, help, o.GetIOFileHandles())
		}
		if err != nil {
			return err
		}
		if name == "" {
			return util.MissingOption("name")
		}
	}
	if value == "" {
		message := "environment variable value: "
		value, err = util.PickValue(message, defaultValues[name], true, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		if name == "" {
			return util.MissingOption("name")
		}
	}

	if projectConfig.PipelineConfig == nil {
		projectConfig.PipelineConfig = &jenkinsfile.PipelineConfig{}
	}
	projectConfig.PipelineConfig.Env = kube.SetEnvVar(projectConfig.PipelineConfig.Env, name, value)

	err = projectConfig.SaveConfig(fileName)
	if err != nil {
		return err
	}
	log.Logger().Infof("Updated Jenkins X Pipeline file: %s", util.ColorInfo(fileName))
	return nil

}

func (o *CreateVariableOptions) loadEnvVars(projectConfig *config.ProjectConfig) (map[string]string, error) {
	answer := map[string]string{}

	teamSettings, err := o.TeamSettings()
	if err != nil {
		return answer, err
	}

	packsDir, err := gitresolver.InitBuildPack(o.Git(), teamSettings.BuildPackURL, teamSettings.BuildPackRef)
	if err != nil {
		return answer, err
	}

	resolver, err := gitresolver.CreateResolver(packsDir, o.Git())
	if err != nil {
		return answer, err
	}

	name := projectConfig.BuildPack
	if name == "" {
		name, err = o.DiscoverBuildPack(o.Dir, projectConfig, name)
		if err != nil {
			return answer, err
		}
	}
	if projectConfig.PipelineConfig == nil {
		projectConfig.PipelineConfig = &jenkinsfile.PipelineConfig{}
	}
	pipelineConfig := projectConfig.PipelineConfig
	if name != "none" {
		packDir := filepath.Join(packsDir, name)
		pipelineFile := filepath.Join(packDir, jenkinsfile.PipelineConfigFileName)
		exists, err := util.FileExists(pipelineFile)
		if err != nil {
			return answer, errors.Wrapf(err, "failed to find build pack pipeline YAML: %s", pipelineFile)
		}
		if !exists {
			return answer, fmt.Errorf("no build pack for %s exists at directory %s", name, packDir)
		}
		buildPackPipelineConfig, err := jenkinsfile.LoadPipelineConfig(pipelineFile, resolver, true, false)
		if err != nil {
			return answer, errors.Wrapf(err, "failed to load build pack pipeline YAML: %s", pipelineFile)
		}
		err = pipelineConfig.ExtendPipeline(buildPackPipelineConfig, false)
		if err != nil {
			return answer, errors.Wrapf(err, "failed to override PipelineConfig using configuration in file %s", pipelineFile)
		}
	}

	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return answer, err
	}

	answer = pipelineConfig.GetAllEnvVars()

	podTemplates, err := kube.LoadPodTemplates(kubeClient, ns)
	if err != nil {
		return answer, err
	}
	containerName := pipelineConfig.Agent.GetImage()
	if containerName != "" && podTemplates != nil && podTemplates[containerName] != nil {
		podTemplate := podTemplates[containerName]
		if len(podTemplate.Spec.Containers) > 0 {
			container := podTemplate.Spec.Containers[0]
			for _, env := range container.Env {
				if env.Value != "" || answer[env.Name] == "" {
					answer[env.Name] = env.Value
				}
			}
		}
	}
	return answer, nil
}
