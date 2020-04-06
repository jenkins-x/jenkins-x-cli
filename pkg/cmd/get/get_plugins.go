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

package get

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/extensions"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/spf13/cobra"
)

var (
	get_plugins_long = templates.LongDesc(`
		List all available plugin files on a user's PATH.

		Plugins provide extended functionality that is not part of the major command-line distribution.

		Available plugin files are those that are:
		- executable
		- anywhere on the user's PATH
		- begin with "jx-"

		Plugins defined by extensions are automatically installed when the plugin is called.

		Please refer to the documentation and examples for more information about how write your own plugins.

`)
)

type GetPluginsOptions struct {
	*opts.CommonOptions
	Verifier extensions.PathVerifier
}

// NewCmdGetPlugins provides a way to list all plugin executables visible to jx
func NewCmdGetPlugins(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetPluginsOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List all visible plugin executables on a user's PATH",
		Long:  get_plugins_long,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Complete()
			helper.CheckErr(err)
			err = options.Run()

			helper.CheckErr(err)
		},
	}

	return cmd
}

func (o *GetPluginsOptions) Complete() error {
	o.Verifier = &extensions.CommandOverrideVerifier{
		Root:        o.Cmd.Root(),
		SeenPlugins: make(map[string]string, 0),
	}
	return nil
}

// Run implements the get plugins command
func (o *GetPluginsOptions) Run() error {
	return o.printExtensionPlugins()
}

func (o *GetPluginsOptions) printExtensionPlugins() error {
	pcgs, managedPluginsEnabled, err := o.GetPluginCommandGroups(o.Verifier)
	if err != nil {
		return err
	}
	if !managedPluginsEnabled {
		log.Logger().Warnf("Managed Plugins not available")
	}
	maxLength := 0
	for _, pcg := range pcgs {
		for _, pc := range pcg.Commands {

			if len(pc.SubCommand) > maxLength {
				maxLength = len(pc.SubCommand)
			}
		}
	}

	for _, pcg := range pcgs {
		log.Logger().Infof("%s", pcg.Message)
		for _, pc := range pcg.Commands {
			var description string
			url, err := extensions.FindPluginUrl(pc.PluginSpec)
			if err != nil {
				// No-op as this might just be a local plugin
			}
			if pc.Name != "" && pc.Version != "" && url != "" {
				description = fmt.Sprintf("%s (app %s version %s installed from %s)", pc.Description, pc.Name,
					pc.Version, url)
			} else {
				description = pc.Description
			}
			log.Logger().Infof("  %s %s%s", util.ColorInfo(pc.SubCommand), strings.Repeat(" ", maxLength-len(pc.SubCommand)), description)
		}
		log.Logger().Info("")
	}

	if len(pcgs) > 0 {
		// Add a trailing line to make the output more readable
		log.Logger().Info("")
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}

	err = extensions.ValidatePlugins(jxClient, ns)
	if err != nil {
		return err
	}

	return nil
}
