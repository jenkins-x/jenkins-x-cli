package alpha

import (
	"io/ioutil"
	"os"
	"strings"

	"github.com/ghodss/yaml"
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/extensions"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options are the options to execute the alpha command
type Options struct {
	*opts.CommonOptions
}

const (
	alphaYamlURL = "https://raw.githubusercontent.com/jenkins-x-labs/jxl/master/alpha/plugins.yml"
)

var (
	alphaLong = templates.LongDesc(`
		Provides alpha versions of existing commands or adds new alpha commands.
`)
	alphaExample = templates.Examples(`
		# Run the new helm3 / helmfile based version of boot
		jx alpha boot
`)
)

// NewCmdAlpha creates the "jx alpha" command
func NewCmdAlpha(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &Options{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "alpha",
		Short:   "Provides alpha versions of existing commands or adds new alpha commands",
		Long:    alphaLong,
		Example: alphaExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	plugins, err := options.getPluginSpecs()
	if err != nil {
		log.Logger().Warnf("failed to discover alpha commands from github: %s", err.Error())
	}
	for i := range plugins {
		c := plugins[i]
		subCmd := &cobra.Command{
			Use:                c.SubCommand,
			Short:              c.Description,
			DisableFlagParsing: true,
			Run: func(cmd *cobra.Command, args []string) {
				err := options.RunPlugin(c, args)
				helper.CheckErr(err)
			},
		}
		cmd.AddCommand(subCmd)
	}
	return cmd
}

// Run implements this command
func (o *Options) Run() error {
	return o.Cmd.Help()
}

// getPluginSpecs returns the plugins
func (o *Options) getPluginSpecs() ([]jenkinsv1.PluginSpec, error) {
	pluginSpecs := []jenkinsv1.PluginSpec{}
	httpClient := util.GetClient()
	resp, err := httpClient.Get(alphaYamlURL)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get YAML from %s", alphaYamlURL)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read YAML from %s", alphaYamlURL)
	}

	err = yaml.Unmarshal(data, &pluginSpecs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmmarshal plugins YAML")
	}

	// lets see if there are any version expressions in the URLs
	for i := range pluginSpecs {
		p := &pluginSpecs[i]
		for j := range p.Binaries {
			b := &p.Binaries[j]
			b.URL = strings.ReplaceAll(b.URL, "$VERSION", p.Version)
		}
	}
	return pluginSpecs, nil
}

// RunPlugin runs the given plugin command
func (o *Options) RunPlugin(pluginSpec jenkinsv1.PluginSpec, args []string) error {
	plugin := jenkinsv1.Plugin{}
	plugin.Spec = pluginSpec
	plugin.Name = pluginSpec.Name

	path, err := extensions.EnsurePluginInstalled(plugin)
	if err != nil {
		return errors.Wrapf(err, "failed to ensure plugin is installed %s", pluginSpec.Name)
	}

	log.Logger().Debugf("running plugin %s with args %#v", path, args)

	c := util.Command{
		Name: path,
		Args: args,
		Env: map[string]string{
			"BINARY_NAME":       "jx alpha " + pluginSpec.SubCommand,
			"TOP_LEVEL_COMMAND": "jx-alpha-" + pluginSpec.SubCommand,
		},
		Out: os.Stdin,
		Err: os.Stderr,
		In:  os.Stdin,
	}
	_, err = c.RunWithoutRetry()
	if err != nil {
		return err
	}
	return nil
}
