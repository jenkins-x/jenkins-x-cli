package opts

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	jenkinsio "github.com/jenkins-x/jx/pkg/apis/jenkins.io"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/extensions"

	"github.com/jenkins-x/jx/pkg/cmd/templates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
)

func (o *CommonOptions) isManagedPluginsEnabled() bool {
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		log.Logger().Warnf("Unable to load managed plugins because %v", err)
		return false
	}
	_, err = apisClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get("plugins."+jenkinsio.GroupName,
		metav1.GetOptions{})
	if err != nil {
		log.Logger().Warnf("Unable to load managed plugins because %v", err)
		return false
	}
	return true
}

// GetPluginCommandGroups returns the plugin groups
func (o *CommonOptions) GetPluginCommandGroups(verifier extensions.PathVerifier) (templates.PluginCommandGroups, bool,
	error) {

	otherCommands := templates.PluginCommandGroup{
		Message: "Other Commands",
	}
	groups := make(map[string]templates.PluginCommandGroup, 0)

	// Managed plugins
	managedPluginsEnabled := o.isManagedPluginsEnabled()
	if managedPluginsEnabled {
		jxClient, ns, err := o.JXClientAndDevNamespace()
		if err != nil {
			return nil, false, err
		}
		plugs := jxClient.JenkinsV1().Plugins(ns)

		pluginList, err := plugs.List(metav1.ListOptions{})
		if err != nil {
			return nil, false, err
		}
		for _, plugin := range pluginList.Items {
			pluginCommand := &templates.PluginCommand{
				PluginSpec: plugin.Spec,
			}
			if plugin.Spec.Group == "" {
				otherCommands.Commands = append(otherCommands.Commands, pluginCommand)
			} else {
				if g, ok := groups[plugin.Spec.Group]; !ok {
					groups[plugin.Spec.Group] = templates.PluginCommandGroup{
						Message: fmt.Sprintf("%s:", plugin.Spec.Group),
						Commands: []*templates.PluginCommand{
							pluginCommand,
						},
					}
				} else {
					g.Commands = append(g.Commands, pluginCommand)
				}
			}
		}
	}

	pathCommands := templates.PluginCommandGroup{
		Message: "Locally Available Commands:",
	}

	path := "PATH"
	if runtime.GOOS == "windows" {
		path = "path"
	}

	paths := sets.NewString(filepath.SplitList(os.Getenv(path))...)
	for _, dir := range paths.List() {
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() {
				continue
			}
			if !strings.HasPrefix(f.Name(), "jx-") {
				continue
			}

			pluginPath := filepath.Join(dir, f.Name())
			subCommand := strings.TrimPrefix(strings.Replace(filepath.Base(pluginPath), "-", " ", -1), "jx ")
			pc := &templates.PluginCommand{
				PluginSpec: jenkinsv1.PluginSpec{
					SubCommand:  subCommand,
					Description: pluginPath,
				},
				Errors: make([]error, 0),
			}
			pathCommands.Commands = append(pathCommands.Commands, pc)
			if errs := verifier.Verify(filepath.Join(dir, f.Name())); len(errs) != 0 {
				for _, err := range errs {
					pc.Errors = append(pc.Errors, err)
				}
			}
		}
	}

	pcgs := templates.PluginCommandGroups{}
	for _, g := range groups {
		pcgs = append(pcgs, g)
	}
	if len(otherCommands.Commands) > 0 {
		pcgs = append(pcgs, otherCommands)
	}
	if len(pathCommands.Commands) > 0 {
		pcgs = append(pcgs, pathCommands)
	}
	return pcgs, managedPluginsEnabled, nil
}
