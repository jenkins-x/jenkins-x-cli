package rsh

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	DefaultShell      = "/bin/sh"
	ShellsFile        = "/etc/shells"
	DefaultRshCommand = "bash"
)

type RshOptions struct {
	*opts.CommonOptions

	Container   string
	Namespace   string
	Pod         string
	Executable  string
	ExecCmd     string
	DevPod      bool
	Username    string
	Environment string
}

var (
	rsh_long = templates.LongDesc(`
		Opens a terminal or runs a command in a pods container

`)

	rsh_example = templates.Examples(`
		# Open a terminal in the first container of the foo deployment's latest pod
		jx rsh foo

		# Opens a terminal in the cheese container in the latest pod in the foo deployment
		jx rsh -c cheese foo

		# To connect to one of your DevPods use:
		jx rsh -d

		# To execute something in the remote shell (like classic rsh or ssh commands)
		jx rsh -e 'do something'
`)
)

func NewCmdRsh(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &RshOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "rsh [deploymentOrPodName]",
		Short:   "Opens a terminal in a pod or runs a command in the pod",
		Long:    rsh_long,
		Example: rsh_example,
		Aliases: []string{"log"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Container, "container", "c", "", "The name of the container to log")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to look for the Deployment. Defaults to the current namespace")
	cmd.Flags().StringVarP(&options.Namespace, "pod", "p", "", "the pod name to use")
	cmd.Flags().StringVarP(&options.Executable, "shell", "s", "", "Path to the shell command")
	cmd.Flags().BoolVarP(&options.DevPod, "devpod", "d", false, "Connect to a DevPod")
	cmd.Flags().StringVarP(&options.ExecCmd, "execute", "e", DefaultRshCommand, "Execute this command on the remote container")
	cmd.Flags().StringVarP(&options.Username, "username", "", "", "The username to create the DevPod. If not specified defaults to the current operating system user or $USER'")
	cmd.Flags().StringVarP(&options.Environment, "environment", "", "", "The environment in which to look for the Deployment. Defaults to the current environment")

	return cmd
}

func (o *RshOptions) Run() error {
	args := o.Args

	client, curNs, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if ns == "" {
		ns = curNs
	}

	if o.Environment != "" {
		ns, err = o.FindEnvironmentNamespace(o.Environment)
		if err != nil {
			return err
		}
	}

	if o.ExecCmd == "" {
		o.ExecCmd = DefaultRshCommand
	}
	filter := ""
	names := []string{}
	podsName := "Pods"
	pods := map[string]*corev1.Pod{}
	if o.DevPod {
		podsName = "DevPods"
		userName, err := o.GetUsername(o.Username)
		if err != nil {
			return err
		}
		names, pods, err = kube.GetDevPodNames(client, ns, userName)
		if err != nil {
			return err
		}
	} else {
		names, err = kube.GetPodNames(client, ns, "")
		if err != nil {
			return err
		}
	}
	if len(names) == 0 {
		if filter == "" {
			return fmt.Errorf("There are no %s", podsName)
		} else {
			return fmt.Errorf("There are no %s matching filter: %s", podsName, filter)
		}
	}
	name := o.Pod
	if len(args) == 0 {
		if util.StringArrayIndex(names, name) < 0 {
			n, err := util.PickName(names, "Pick Pod:", "", o.GetIOFileHandles())
			if err != nil {
				return err
			}
			name = n
		}
	} else {
		name = args[0]
		if util.StringArrayIndex(names, name) < 0 {
			// lets try use the name as a filter
			filteredNames := []string{}
			for _, n := range names {
				if strings.Contains(n, name) {
					filteredNames = append(filteredNames, n)
				}
			}
			n, err := util.PickName(filteredNames, "Pick Pod:", "", o.GetIOFileHandles())
			if err != nil {
				return err
			}
			name = n
		}
	}

	if name == "" {
		return fmt.Errorf("No pod found for namespace %s with name %s", ns, name)
	}

	commandArguments := []string{}
	if o.Executable == "" {
		if o.DevPod {
			workingDir := ""
			pod := pods[name]
			if pod != nil && pod.Annotations != nil {
				workingDir = pod.Annotations[kube.AnnotationWorkingDir]
			}
			if workingDir != "" {
				commandArguments = []string{"--", "/bin/sh", "-c", "mkdir -p " + workingDir + "\ncd " + workingDir + "\n" + o.ExecCmd}
			} else {
				commandArguments = []string{"--", "/bin/sh", "-c", o.ExecCmd}
			}
		} else {
			bash, err := o.detectBash(ns, name, o.Container)
			if err != nil {
				if o.ExecCmd != "" {
					if o.ExecCmd == "bash" {
						commandArguments = []string{DefaultShell}
					} else {
						commandArguments = []string{DefaultShell, "-c", o.ExecCmd}
					}
				} else {
					commandArguments = []string{DefaultShell}
				}
			} else {
				if o.ExecCmd != "" {
					if o.ExecCmd == "bash" {
						commandArguments = []string{bash}
					} else {
						commandArguments = []string{"--", bash, "-c", o.ExecCmd}
					}
				} else {
					commandArguments = []string{bash}
				}
			}
		}
	}

	if len(commandArguments) == 0 {
		commandArguments = []string{o.Executable}
	}

	a := []string{"exec", "-it", "-n", ns}
	if o.Container != "" {
		a = append(a, "-c", o.Container)
	}
	a = append(a, name)
	if len(args) > 1 {
		a = append(a, args[1:]...)
	} else if len(commandArguments) > 0 {
		a = append(a, commandArguments...)
	}
	log.Logger().Debugf("Running command: kubectl %s", strings.Join(a, " "))
	return o.RunCommandInteractive(true, "kubectl", a...)
}

func (o *RshOptions) detectBash(ns string, podName string, container string) (string, error) {
	fileName := "/tmp/pod_" + podName + "_shells"
	args := []string{"cp", ns + "/" + podName + ":" + ShellsFile, fileName}
	if container != "" {
		args = append(args, "-c", container)
	}
	err := o.RunCommandQuietly("kubectl", args...)
	if err != nil {
		return "", errors.Wrapf(err, "failed to copy the shell file form POD '%s'", podName)
	}
	defer os.Remove(fileName)

	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", errors.Wrap(err, "failed to read the copied shell")
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, "bash") {
			return line, nil
		}
	}
	return "", fmt.Errorf("no bash found in POD '%s'", podName)
}
