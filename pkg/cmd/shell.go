package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"k8s.io/client-go/tools/clientcmd"

	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"
)

const (
	defaultRcFile = `
if [ -f /etc/bashrc ]; then
    source /etc/bashrc
fi
if [ -f ~/.bashrc ]; then
    source ~/.bashrc
fi
if type -t dh_bash-completion >/dev/null; then
    if type -t __start_jx >/dev/null; then true; else
        source <(jx completion bash)
    fi
fi
`

	zshRcFile = `
if [ -f /etc/zshrc ]; then
    source /etc/zshrc
fi
if [ -f ~/.zshrc ]; then
    source ~/.zshrc
fi
`
)

type ShellOptions struct {
	*opts.CommonOptions

	Filter string
}

var (
	shell_long = templates.LongDesc(`
		Create a sub shell so that changes to the Kubernetes context, namespace or environment remain local to the shell.`)
	shell_example = templates.Examples(`
		# create a new shell where the context changes are local to the shell only
		jx shell

		# create a new shell using a specific named context
		jx shell prod-cluster

		# ends the current shell and returns to the previous Kubernetes context
		exit
`)
)

func NewCmdShell(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &ShellOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "shell",
		Aliases: []string{"sh"},
		Short:   "Create a sub shell so that changes to the Kubernetes context, namespace or environment remain local to the shell",
		Long:    shell_long,
		Example: shell_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Filter the list of contexts to switch between using the given text")
	return cmd
}

func (o *ShellOptions) Run() error {
	config, _, err := o.Kube().LoadConfig()
	if err != nil {
		return err
	}

	if config == nil || config.Contexts == nil || len(config.Contexts) == 0 {
		return fmt.Errorf("No Kubernetes contexts available! Try create or connect to cluster?")
	}

	contextNames := []string{}
	for k, v := range config.Contexts {
		if k != "" && v != nil {
			if o.Filter == "" || strings.Index(k, o.Filter) >= 0 {
				contextNames = append(contextNames, k)
			}
		}
	}
	sort.Strings(contextNames)

	ctxName := ""
	args := o.Args
	if len(args) > 0 {
		ctxName = args[0]
		if util.StringArrayIndex(contextNames, ctxName) < 0 {
			return util.InvalidArg(ctxName, contextNames)
		}
	}

	if ctxName == "" && !o.BatchMode {
		defaultCtxName := config.CurrentContext
		pick, err := o.PickContext(contextNames, defaultCtxName)
		if err != nil {
			return err
		}
		ctxName = pick
	}
	if ctxName == "" {
		ctxName = config.CurrentContext
	}
	newConfig := *config
	newConfig.CurrentContext = ctxName

	tmpDirName, err := ioutil.TempDir("", ".jx-shell-")
	if err != nil {
		return err
	}
	tmpConfigFileName := filepath.Join(tmpDirName, "/config")
	err = clientcmd.WriteToFile(newConfig, tmpConfigFileName)
	if err != nil {
		return err
	}

	fullShell := os.Getenv("SHELL")
	shell := filepath.Base(fullShell)
	if fullShell == "" && runtime.GOOS == "windows" {
		// SHELL is set by git-bash but not cygwin :-(
		shell = "cmd.exe"
	}

	prompt := o.createNewBashPrompt(os.Getenv("PS1"))
	rcFile := defaultRcFile + "\nexport PS1=" + prompt + "\nexport KUBECONFIG=\"" + tmpConfigFileName + "\"\n"
	tmpRCFileName := tmpDirName + "/.bashrc"

	if shell == "zsh" {
		prompt = o.createNewZshPrompt(os.Getenv("PS1"))
		rcFile = zshRcFile + "\nexport PS1=" + prompt + "\nexport KUBECONFIG=\"" + tmpConfigFileName + "\"\n"
		tmpRCFileName = tmpDirName + "/.zshrc"
	}
	err = ioutil.WriteFile(tmpRCFileName, []byte(rcFile), util.DefaultWritePermissions)
	if err != nil {
		return err
	}

	info := util.ColorInfo
	log.Logger().Infof("Creating a new shell using the Kubernetes context %s", info(ctxName))
	if shell != "cmd.exe" {
		log.Logger().Infof("Shell RC file is %s\n", tmpRCFileName)
	}
	log.Logger().Infof("All changes to the Kubernetes context like changing environment, namespace or context will be local to this shell")
	log.Logger().Infof("To return to the global context use the command: exit\n")

	e := exec.Command(shell, "-rcfile", tmpRCFileName, "-i")
	if shell == "zsh" {
		env := os.Environ()
		env = append(env, fmt.Sprintf("ZDOTDIR=%s", tmpDirName))
		e = exec.Command(shell, "-i")
		e.Env = env
	} else if shell == "cmd.exe" {
		env := os.Environ()
		env = append(env, fmt.Sprintf("KUBECONFIG=%s", tmpConfigFileName))
		e = exec.Command(shell)
		e.Env = env
	}

	e.Stdout = o.Out
	e.Stderr = o.Err
	e.Stdin = os.Stdin
	err = e.Run()
	if deleteError := os.RemoveAll(tmpDirName); deleteError != nil {
		panic(err)
	}
	return err

}

func (o *ShellOptions) PickContext(names []string, defaultValue string) (string, error) {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	if len(names) == 0 {
		return "", nil
	}
	if len(names) == 1 {
		return names[0], nil
	}
	name := ""
	prompt := &survey.Select{
		Message: "Change Kubernetes context:",
		Options: names,
		Default: defaultValue,
	}
	err := survey.AskOne(prompt, &name, nil, surveyOpts)
	return name, err
}

func (o *ShellOptions) createNewBashPrompt(prompt string) string {
	if prompt == "" {
		return "'[\\u@\\h \\W $(jx prompt) ]\\$ '"
	}
	if strings.Contains(prompt, "jx prompt") {
		return prompt
	}
	if prompt[0] == '"' {
		return prompt[0:1] + "$(jx prompt) " + prompt[1:]
	}
	if prompt[0] == '\'' {
		return prompt[0:1] + "$(jx prompt) " + prompt[1:]
	}
	return "'$(jx prompt) " + prompt + "'"
}

func (o *ShellOptions) createNewZshPrompt(prompt string) string {
	if prompt == "" {
		return "'[$(jx prompt) %n@%m %c]\\$ '"
	}
	if strings.Contains(prompt, "jx prompt") {
		return prompt
	}
	if prompt[0] == '"' {
		return prompt[0:1] + "$(jx prompt) " + prompt[1:]
	}
	if prompt[0] == '\'' {
		return prompt[0:1] + "$(jx prompt) " + prompt[1:]
	}
	return "'$(jx prompt) " + prompt + "'"
}
