package create

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/packages"

	"io/ioutil"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	pipelineapi "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

var (
	createDevPodWorkpaceLong = templates.LongDesc(`
		Creates the DevPod workspace files
`)

	createDevPodWorkpaceExample = templates.Examples(`
		# create the DevPod Workspace files
		jx step create devpod workspace

			`)
)

// StepCreateDevPodWorkpaceOptions contains the command line flags
type StepCreateDevPodWorkpaceOptions struct {
	step.StepOptions

	Dir            string
	VSCodeSettings string
	VSCodeHome     string
	VSCode         bool
}

// StepCreateDevPodWorkpaceResults stores the generated results
type StepCreateDevPodWorkpaceResults struct {
	Pipeline    *pipelineapi.Pipeline
	Task        *pipelineapi.Task
	PipelineRun *pipelineapi.PipelineRun
}

// NewCmdStepCreateDevPodWorkpace Creates a new Command object
func NewCmdStepCreateDevPodWorkpace(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreateDevPodWorkpaceOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "devpod workspace",
		Short:   "Creates the DevPod workspace files",
		Long:    createDevPodWorkpaceLong,
		Example: createDevPodWorkpaceExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "/workspace", "The workspace directory to write to")
	cmd.Flags().BoolVarP(&options.VSCode, "vscode", "", false, "If enabled also setup the VS Code settings to enable the devpodsh Terminal script")
	cmd.Flags().StringVarP(&options.VSCodeSettings, "vscode-settings", "", ".local/share/code-server/User/settings.json", "The VS Code settings file relative to the workspace home dir")
	cmd.Flags().StringVarP(&options.VSCodeHome, "vscode-home", "", "/root", "The VS Code default home dir file relative to the workspace home dir")
	return cmd
}

// Run implements this command
func (o *StepCreateDevPodWorkpaceOptions) Run() error {
	const kubectl = "kubectl"
	path, err := packages.LookupForBinary(kubectl)
	if err != nil {
		return errors.Wrapf(err, "could not find binary %s on the $PATH", kubectl)
	}
	workspaceDir := o.Dir
	if workspaceDir == "" {
		workspaceDir = "."
	}
	homeDir := filepath.Join(workspaceDir, "idehome")
	outDir := filepath.Join(homeDir, "bin")
	if o.VSCode {
		err = util.CopyDirPreserve(o.VSCodeHome, homeDir)
		if err != nil {
			return errors.Wrapf(err, "failed to copy the VS Code home dir %s to %s", o.VSCodeHome, homeDir)
		}
	} else {
		err = os.MkdirAll(homeDir, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to ensure workspace home directory is created %s", homeDir)
		}
	}
	err = os.MkdirAll(outDir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure workspace bin directory is created %s", outDir)
	}

	destPath := filepath.Join(outDir, kubectl)
	if path != destPath {
		exists, err := util.FileExists(destPath)
		if err != nil {
			return errors.Wrapf(err, "failed to check if file exists %s", destPath)
		}
		if !exists {
			err = util.CopyFile(path, destPath)
			if err != nil {
				return errors.Wrapf(err, "failed to copy %s to %s", path, destPath)
			}
		}
	}

	scriptPath := filepath.Join(outDir, "devpodsh")

	text := fmt.Sprintf(`#!/bin/sh

DIR=$(pwd)
echo "opening a shell inside the DevPod"
%s/kubectl exec -it -c devpod $HOSTNAME bash -- -c "cd $DIR && bash"
`, outDir)

	err = ioutil.WriteFile(scriptPath, []byte(text), util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save script to %s", scriptPath)
	}

	if o.VSCode {
		shell := filepath.Join(outDir, "devpodsh")
		err = o.updateVSCodeSettings(homeDir, shell)
		if err != nil {
			return errors.Wrapf(err, "failed to modify the VS Code Settings")
		}
	}
	return nil
}

func (o *StepCreateDevPodWorkpaceOptions) updateVSCodeSettings(homeDir string, devPodSh string) error {
	jsonFile := filepath.Join(homeDir, o.VSCodeSettings)
	dir, _ := filepath.Split(jsonFile)
	err := os.MkdirAll(dir, util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure the VS Code settings dir is created %s", dir)
	}
	exists, err := util.FileExists(jsonFile)
	if err != nil {
		return errors.Wrapf(err, "failed to check if file exists %s", jsonFile)
	}
	config := map[string]interface{}{}
	if exists {
		data, err := ioutil.ReadFile(jsonFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load VS Code settings file: %s", jsonFile)
		}
		err = json.Unmarshal(data, &config)
		if err != nil {
			return errors.Wrapf(err, "failed to parse VS Code settings JSON: %s", jsonFile)
		}
	}

	const key = "terminal.integrated.shell.linux"

	value := config[key]
	shell, ok := value.(string)
	if !ok || shell != devPodSh {
		config[key] = devPodSh
		data, err := json.Marshal(config)
		if err != nil {
			return errors.Wrap(err, "failed to marshal new VS Code settings to JSON")
		}
		err = ioutil.WriteFile(jsonFile, data, util.DefaultWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to save VS Code settings file: %s", jsonFile)
		}
	}
	return nil
}
