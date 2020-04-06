package templates

import (
	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/spf13/cobra"
)

// PluginCommandGroup is a group of plugins providing some Commands. The Message is used for describing the group
type PluginCommandGroup struct {
	Message  string
	Commands []*PluginCommand
}

// PluginCommand is a reference to a particular Command provided by a Plugin
type PluginCommand struct {
	jenkinsv1.PluginSpec
	Errors []error
}

// PluginCommandGroups is a slice of PluginCommandGroup
type PluginCommandGroups []PluginCommandGroup

type CommandGroup struct {
	Message  string
	Commands []*cobra.Command
}

type CommandGroups []CommandGroup

func (g CommandGroups) Add(parent *cobra.Command) {
	for _, group := range g {
		for _, c := range group.Commands {
			if !c.HasParent() {
				parent.AddCommand(c)
			}
		}
	}
}

func (g CommandGroups) Has(c *cobra.Command) bool {
	for _, group := range g {
		for _, command := range group.Commands {
			if command == c {
				return true
			}
		}
	}
	return false
}

func AddAdditionalCommands(g CommandGroups, message string, cmds []*cobra.Command) CommandGroups {
	group := CommandGroup{Message: message}
	for _, c := range cmds {
		// Don't show commands that have no short description
		if !g.Has(c) && len(c.Short) != 0 {
			group.Commands = append(group.Commands, c)
		}
	}
	if len(group.Commands) == 0 {
		return g
	}
	return append(g, group)
}
