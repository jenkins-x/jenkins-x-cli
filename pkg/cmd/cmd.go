/*
Copyright 2018 The Kubernetes Authors & The Jenkins X Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/jenkins-x/jx/pkg/cmd/alpha"
	"github.com/jenkins-x/jx/pkg/cmd/deprecation"
	"github.com/jenkins-x/jx/pkg/cmd/experimental"
	"github.com/jenkins-x/jx/pkg/cmd/profile"
	"github.com/jenkins-x/jx/pkg/cmd/ui"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/cluster"
	"github.com/spf13/viper"

	"github.com/jenkins-x/jx/pkg/cmd/boot"
	"github.com/jenkins-x/jx/pkg/cmd/compliance"
	"github.com/jenkins-x/jx/pkg/cmd/controller"
	"github.com/jenkins-x/jx/pkg/cmd/create"
	"github.com/jenkins-x/jx/pkg/cmd/deletecmd"
	"github.com/jenkins-x/jx/pkg/cmd/edit"
	"github.com/jenkins-x/jx/pkg/cmd/gc"
	"github.com/jenkins-x/jx/pkg/cmd/get"
	"github.com/jenkins-x/jx/pkg/cmd/importcmd"
	"github.com/jenkins-x/jx/pkg/cmd/initcmd"
	"github.com/jenkins-x/jx/pkg/cmd/preview"
	"github.com/jenkins-x/jx/pkg/cmd/rsh"
	"github.com/jenkins-x/jx/pkg/cmd/start"
	"github.com/jenkins-x/jx/pkg/cmd/stop"
	"github.com/jenkins-x/jx/pkg/cmd/sync"
	"github.com/jenkins-x/jx/pkg/cmd/uninstall"
	"github.com/jenkins-x/jx/pkg/cmd/update"
	"github.com/jenkins-x/jx/pkg/cmd/upgrade"

	"github.com/jenkins-x/jx/pkg/cmd/add"
	"github.com/jenkins-x/jx/pkg/cmd/namespace"
	"github.com/jenkins-x/jx/pkg/cmd/promote"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/extensions"

	"github.com/jenkins-x/jx/pkg/features"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/version"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1/terminal"
)

// NewJXCommand creates the `jx` command and its nested children.
// args used to determine binary plugin to run can be overridden (does not affect compiled in commands).
func NewJXCommand(f clients.Factory, in terminal.FileReader, out terminal.FileWriter,
	err io.Writer, args []string) *cobra.Command {

	configureViper()
	rootCommand := &cobra.Command{
		Use:              "jx",
		Short:            "jx is a command line tool for working with Jenkins X",
		PersistentPreRun: setLoggingLevel,
		Run:              runHelp,
	}

	features.Init()

	commonOpts := opts.NewCommonOptionsWithTerm(f, in, out, err)
	commonOpts.AddBaseFlags(rootCommand)

	addCommands := add.NewCmdAdd(commonOpts)
	createCommands := create.NewCmdCreate(commonOpts)
	deleteCommands := deletecmd.NewCmdDelete(commonOpts)

	getCommands := get.NewCmdGet(commonOpts)
	editCommands := edit.NewCmdEdit(commonOpts)
	updateCommands := update.NewCmdUpdate(commonOpts)

	installCommands := []*cobra.Command{
		profile.NewCmdProfile(commonOpts),
		boot.NewCmdBoot(commonOpts),
		create.NewCmdInstall(commonOpts),
		uninstall.NewCmdUninstall(commonOpts),
		upgrade.NewCmdUpgrade(commonOpts),
	}
	installCommands = append(installCommands, findCommands("cluster", createCommands, deleteCommands)...)
	installCommands = append(installCommands, findCommands("cluster", updateCommands)...)
	installCommands = append(installCommands, findCommands("jenkins token", createCommands, deleteCommands)...)
	installCommands = append(installCommands, initcmd.NewCmdInit(commonOpts))

	addProjectCommands := []*cobra.Command{
		importcmd.NewCmdImport(commonOpts),
	}
	addProjectCommands = append(addProjectCommands, findCommands("create spring", createCommands, deleteCommands)...)
	addProjectCommands = append(addProjectCommands, findCommands("create quickstart", createCommands, deleteCommands)...)

	gitCommands := []*cobra.Command{}
	gitCommands = append(gitCommands, findCommands("git server", createCommands, deleteCommands)...)
	gitCommands = append(gitCommands, findCommands("git token", createCommands, deleteCommands)...)
	gitCommands = append(gitCommands, NewCmdRepo(commonOpts))

	addonCommands := []*cobra.Command{}
	addonCommands = append(addonCommands, findCommands("addon", createCommands, deleteCommands)...)
	addonCommands = append(addonCommands, findCommands("app", createCommands, deleteCommands, addCommands)...)

	environmentsCommands := []*cobra.Command{
		preview.NewCmdPreview(commonOpts),
		promote.NewCmdPromote(commonOpts),
	}
	environmentsCommands = append(environmentsCommands, findCommands("environment", createCommands, deleteCommands, editCommands, getCommands)...)

	alphaCommand := alpha.NewCmdAlpha(commonOpts)

	groups := templates.CommandGroups{
		{
			Message:  "Installing:",
			Commands: installCommands,
		},
		{
			Message:  "Adding Projects to Jenkins X:",
			Commands: addProjectCommands,
		},
		{
			Message:  "Apps:",
			Commands: addonCommands,
		},
		{
			Message:  "Git:",
			Commands: gitCommands,
		},
		{
			Message: "Working with Kubernetes:",
			Commands: []*cobra.Command{
				compliance.NewCompliance(commonOpts),
				NewCmdCompletion(commonOpts),
				NewCmdContext(commonOpts),
				NewCmdEnvironment(commonOpts),
				NewCmdTeam(commonOpts),
				namespace.NewCmdNamespace(commonOpts),
				NewCmdPrompt(commonOpts),
				NewCmdScan(commonOpts),
				NewCmdShell(commonOpts),
				NewCmdStatus(commonOpts),
			},
		},
		{
			Message: "Working with Applications:",
			Commands: []*cobra.Command{
				NewCmdConsole(commonOpts),
				NewCmdLogs(commonOpts),
				NewCmdOpen(commonOpts),
				rsh.NewCmdRsh(commonOpts),
				sync.NewCmdSync(commonOpts),
			},
		},
		{
			Message:  "Working with Environments:",
			Commands: environmentsCommands,
		},
		{
			Message: "Working with Jenkins X resources:",
			Commands: []*cobra.Command{
				getCommands,
				editCommands,
				createCommands,
				updateCommands,
				deleteCommands,
				addCommands,
				start.NewCmdStart(commonOpts),
				stop.NewCmdStop(commonOpts),
			},
		},
		{
			Message: "Jenkins X Pipeline Commands:",
			Commands: []*cobra.Command{
				NewCmdStep(commonOpts),
			},
		},
		{
			Message: "Jenkins X services:",
			Commands: []*cobra.Command{
				controller.NewCmdController(commonOpts),
				gc.NewCmdGC(commonOpts),
			},
		},
		{
			Message: "Working with Jenkins X UI:",
			Commands: []*cobra.Command{
				ui.NewCmdUI(commonOpts),
			},
		},
		{
			Message: "Alpha commands:",
			Commands: []*cobra.Command{
				alphaCommand,
			},
		},
	}

	groups.Add(rootCommand)

	filters := []string{"options"}

	getPluginCommandGroups := func() (templates.PluginCommandGroups, bool) {
		verifier := &extensions.CommandOverrideVerifier{
			Root:        rootCommand,
			SeenPlugins: make(map[string]string, 0),
		}
		pluginCommandGroups, managedPluginsEnabled, err := commonOpts.GetPluginCommandGroups(verifier)
		if err != nil {
			log.Logger().Errorf("%v", err)
		}
		return pluginCommandGroups, managedPluginsEnabled
	}
	templates.ActsAsRootCommand(rootCommand, filters, getPluginCommandGroups, groups...)
	rootCommand.AddCommand(NewCmdDocs(commonOpts))
	rootCommand.AddCommand(NewCmdVersion(commonOpts))
	rootCommand.Version = version.GetVersion()
	rootCommand.SetVersionTemplate("{{printf .Version}}\n")
	rootCommand.AddCommand(NewCmdOptions(out))
	rootCommand.AddCommand(NewCmdDiagnose(commonOpts))

	// Mark the deprecated commands
	deprecation.DeprecateCommands(rootCommand)

	// Mark the experimental commands
	experimental.AlphaCommands(rootCommand)
	experimental.BetaCommands(rootCommand)

	managedPlugins := &managedPluginHandler{
		CommonOptions: commonOpts,
	}
	localPlugins := &localPluginHandler{}

	if len(args) == 0 {
		args = os.Args
	}
	if len(args) > 1 {
		cmdPathPieces := args[1:]

		// only look for suitable executables if
		// the specified command does not already exist
		if _, _, err := rootCommand.Find(cmdPathPieces); err != nil {
			if _, managedPluginsEnabled := getPluginCommandGroups(); managedPluginsEnabled {
				if err := handleEndpointExtensions(managedPlugins, cmdPathPieces); err != nil {
					log.Logger().Errorf("%v", err)
					os.Exit(1)
				}
			} else {
				if err := handleEndpointExtensions(localPlugins, cmdPathPieces); err != nil {
					log.Logger().Errorf("%v", err)
					os.Exit(1)
				}
			}

		}
	}
	return rootCommand
}

func configureViper() {
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
}

func findCommands(subCommand string, commands ...*cobra.Command) []*cobra.Command {
	answer := []*cobra.Command{}
	for _, parent := range commands {
		for _, c := range parent.Commands() {
			if commandHasParentName(c, subCommand) {
				answer = append(answer, c)
			} else {
				childCommands := findCommands(subCommand, c)
				if len(childCommands) > 0 {
					answer = append(answer, childCommands...)
				}
			}
		}
	}
	return answer
}

func commandHasParentName(command *cobra.Command, name string) bool {
	path := fullPath(command)
	return strings.Contains(path, name)
}

func fullPath(command *cobra.Command) string {
	name := command.Name()
	parent := command.Parent()
	if parent != nil {
		return fullPath(parent) + " " + name
	}
	return name
}

func setLoggingLevel(cmd *cobra.Command, args []string) {
	verbose, err := strconv.ParseBool(cmd.Flag(opts.OptionVerbose).Value.String())
	if err != nil {
		log.Logger().Errorf("Unable to check if the verbose flag is set")
	}

	level := os.Getenv("JX_LOG_LEVEL")
	if level != "" {
		if verbose {
			log.Logger().Trace("The JX_LOG_LEVEL environment variable took precedence over the verbose flag")
		}

		err := log.SetLevel(level)
		if err != nil {
			log.Logger().Errorf("Unable to set log level to %s", level)
		}
	} else {
		if verbose {
			err := log.SetLevel("debug")
			if err != nil {
				log.Logger().Errorf("Unable to set log level to debug")
			}
		} else {
			err := log.SetLevel("info")
			if err != nil {
				log.Logger().Errorf("Unable to set log level to info")
			}
		}
	}

	logWarningForRemovalOfStaticMasters(cmd, args)
}

func logWarningForRemovalOfStaticMasters(cmd *cobra.Command, args []string) {
	// If we're running in a cluster, just don't do anything. There's no reason to flood the build logs, and BDD tests
	// can depend on parsing specific output.
	if cluster.IsInCluster() {
		return
	}

	// Just return if this is "jx completion (shell)".
	if commandHasParentName(cmd, "completion") {
		return
	}

	shouldWarn := false
	if commandHasParentName(cmd, "create cluster") {
		shouldWarn = isStaticMasterInstall(cmd)
	} else if commandHasParentName(cmd, "install") {
		shouldWarn = isStaticMasterInstall(cmd)
	} else {
		clientFactory := clients.NewFactory()
		jxClient, ns, err := clientFactory.CreateJXClient()

		// If the jxClient is nil or we got an error, don't bother continuing, something else is wrong enough that it's not worth warning about
		// removal.
		if err != nil || jxClient == nil {
			return
		}

		// Try to get the dev env.
		env, err := jxClient.JenkinsV1().Environments(ns).Get(kube.LabelValueDevEnvironment, metav1.GetOptions{})

		// If we get an error or the dev env is nil, just ignore it and move on - something else is wrong enough that it's not worth worrying about
		// whether we should be warning about removal.
		if err != nil || env == nil {
			return
		}

		teamSettings := &env.Spec.TeamSettings
		shouldWarn = !teamSettings.IsJenkinsXPipelines()
	}

	if shouldWarn {
		cmd.Println("WARNING")
		cmd.Println("")
		cmd.Println("Traditional Jenkins masters are being deprecated and removed from all new Jenkins X releases starting on April 20, 2020.")
		cmd.Println("We strongly recommend using the default Tekton-based configuration instead for all Jenkins X activity going forward. We do not recommend using the Jenkins X workflow based around Jenkins masters.")
		cmd.Println("")
		cmd.Println("For more information: https://jenkins-x.io/blog/2020/03/11/tekton/")
	}
}

func isStaticMasterInstall(cmd *cobra.Command) bool {
	isTekton, _ := cmd.Flags().GetBool("tekton")
	isProw, _ := cmd.Flags().GetBool("prow")
	isStatic, _ := cmd.Flags().GetBool("static-jenkins")
	isSkipInstall, _ := cmd.Flags().GetBool("skip-installation")

	return (!isSkipInstall && !(isTekton || isProw)) || isStatic
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

// PluginHandler is capable of parsing command line arguments
// and performing executable filename lookups to search
// for valid plugin files, and execute found plugins.
type PluginHandler interface {
	// Lookup receives a potential filename and returns
	// a full or relative path to an executable, if one
	// exists at the given filename, or an error.
	Lookup(filename string) (string, error)
	// Execute receives an executable's filepath, a slice
	// of arguments, and a slice of environment variables
	// to relay to the executable.
	Execute(executablePath string, cmdArgs, environment []string) error
}

type managedPluginHandler struct {
	*opts.CommonOptions
	localPluginHandler
}

// Lookup implements PluginHandler
func (h *managedPluginHandler) Lookup(filename string) (string, error) {
	jxClient, ns, err := h.JXClientAndDevNamespace()
	if err != nil {
		return "", err
	}

	possibles, err := jxClient.JenkinsV1().Plugins(ns).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", extensions.PluginCommandLabel, filename),
	})
	if err != nil {
		return "", err
	}
	if len(possibles.Items) > 0 {
		found := possibles.Items[0]
		if len(possibles.Items) > 1 {
			// There is a warning about this when you install extensions as well
			log.Logger().Warnf("More than one plugin installed for %s by apps. Selecting the one installed by %s at random.",
				filename, found.Name)

		}
		return extensions.EnsurePluginInstalled(found)
	}
	return h.localPluginHandler.Lookup(filename)
}

// Execute implements PluginHandler
func (h *managedPluginHandler) Execute(executablePath string, cmdArgs, environment []string) error {
	return h.localPluginHandler.Execute(executablePath, cmdArgs, environment)
}

type localPluginHandler struct{}

// Lookup implements PluginHandler
func (h *localPluginHandler) Lookup(filename string) (string, error) {
	// if on Windows, append the "exe" extension
	// to the filename that we are looking up.
	if runtime.GOOS == "windows" {
		filename = filename + ".exe"
	}

	return exec.LookPath(filename)
}

// Execute implements PluginHandler
func (h *localPluginHandler) Execute(executablePath string, cmdArgs, environment []string) error {
	return syscall.Exec(executablePath, cmdArgs, environment)
}

func handleEndpointExtensions(pluginHandler PluginHandler, cmdArgs []string) error {
	remainingArgs := []string{} // all "non-flag" arguments

	for idx := range cmdArgs {
		if strings.HasPrefix(cmdArgs[idx], "-") {
			break
		}
		remainingArgs = append(remainingArgs, strings.Replace(cmdArgs[idx], "-", "_", -1))
	}

	foundBinaryPath := ""

	// attempt to find binary, starting at longest possible name with given cmdArgs
	for len(remainingArgs) > 0 {
		path, err := pluginHandler.Lookup(fmt.Sprintf("jx-%s", strings.Join(remainingArgs, "-")))
		if err != nil || len(path) == 0 {
			/* Usually "executable file not found in $PATH", spams output of jx help subcommand:
			if err != nil {
				log.Logger().Errorf("Error installing plugin for command %s. %v\n", remainingArgs, err)
			}
			*/
			remainingArgs = remainingArgs[:len(remainingArgs)-1]
			continue
		}

		foundBinaryPath = path
		break
	}

	if len(foundBinaryPath) == 0 {
		return nil
	}

	// invoke cmd binary relaying the current environment and args given
	// remainingArgs will always have at least one element.
	// execve will make remainingArgs[0] the "binary name".
	if err := pluginHandler.Execute(foundBinaryPath, append([]string{foundBinaryPath}, cmdArgs[len(remainingArgs):]...), os.Environ()); err != nil {
		return err
	}

	return nil
}
