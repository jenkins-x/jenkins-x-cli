package edit

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/jenkins-x/jx/pkg/cmd/edit/requirements"
	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/spf13/cobra"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

// EditOptions contains the CLI options
type EditOptions struct {
	*opts.CommonOptions
}

var (
	exit_long = templates.LongDesc(`
		Edit a resource

`)

	exit_example = templates.Examples(`
		# Lets edit the staging Environment
		jx edit env staging
	`)
)

// NewCmdEdit creates the edit command
func NewCmdEdit(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &EditOptions{
		commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "edit [flags]",
		Short:   "Edit a resource",
		Long:    exit_long,
		Example: exit_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
		SuggestFor: []string{"modify"},
	}

	cmd.AddCommand(NewCmdEditAddon(commonOpts))
	cmd.AddCommand(NewCmdEditAppJenkinsPlugins(commonOpts))
	cmd.AddCommand(NewCmdEditBuildpack(commonOpts))
	cmd.AddCommand(NewCmdEditConfig(commonOpts))
	cmd.AddCommand(NewCmdEditDeployKind(commonOpts))
	cmd.AddCommand(NewCmdEditEnv(commonOpts))
	cmd.AddCommand(NewCmdEditHelmBin(commonOpts))
	cmd.AddCommand(requirements.NewCmdEditRequirements(commonOpts))
	cmd.AddCommand(NewCmdEditStorage(commonOpts))
	cmd.AddCommand(NewCmdEditUserRole(commonOpts))
	cmd.AddCommand(NewCmdEditExtensionsRepository(commonOpts))

	err := addTeamSettingsCommandsFromTags(cmd, options)
	helper.CheckErr(err)

	return cmd
}

// Run implements this command
func (o *EditOptions) Run() error {
	return o.Cmd.Help()
}

func addTeamSettingsCommandsFromTags(baseCmd *cobra.Command, options *EditOptions) error {
	teamSettings := &v1.TeamSettings{}
	value := reflect.ValueOf(teamSettings).Elem()
	t := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		structField := t.Field(i)
		tag := structField.Tag
		command, ok := tag.Lookup("command")
		if !ok {
			continue
		}
		commandUsage, ok := tag.Lookup("commandUsage")
		if !ok {
			continue
		}

		cmd := &cobra.Command{
			Use:   command,
			Short: commandUsage,
			Run: func(cmd *cobra.Command, args []string) {
				var value interface{}
				var err error
				if len(args) > 0 {
					if structField.Type.String() == "string" {
						value = args[0]
					} else if structField.Type.String() == "bool" {
						value, err = strconv.ParseBool(args[0])
						helper.CheckErr(err)
					}
				} else if !options.BatchMode {
					var err error
					if structField.Type.String() == "string" {
						value, err = util.PickValue(commandUsage+":", field.String(), true, "", options.GetIOFileHandles())
					} else if structField.Type.String() == "bool" {
						value, err = util.Confirm(commandUsage+":", field.Bool(), "", options.GetIOFileHandles())
					}
					helper.CheckErr(err)
				} else {
					helper.Fatal(fmt.Sprintf("No value to set %s", command), 1)
				}

				callback := func(env *v1.Environment) error {
					teamSettings := &env.Spec.TeamSettings
					valueField := reflect.ValueOf(teamSettings).Elem().FieldByName(structField.Name)
					switch value.(type) {
					case string:
						valueField.SetString(value.(string))
					case bool:
						valueField.SetBool(value.(bool))
					}
					log.Logger().Infof("Setting the team %s to: %s", util.ColorInfo(command), util.ColorInfo(value))
					return nil
				}
				helper.CheckErr(options.ModifyDevEnvironment(callback))
			},
		}

		baseCmd.AddCommand(cmd)
	}
	return nil
}
