package experimental

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/spf13/cobra"
)

// alphaCommands list of deprecated commands along with some more deprecation details
var alphaCommands = map[string]info{
	"step create helmfile": {
		info:        "** EXPERIMENTAL COMMAND ** Generates a helmfile from a jx-apps.yml see enhancement https://github.com/jenkins-x/enhancements/pull/1",
		createdDate: "Jan 28 2020",
	},
}

// betaCommands list of deprecated commands along with some more deprecation details
var betaCommands = map[string]info{}

// info keeps experiment details related to a command
type info struct {
	createdDate string
	info        string
}

// AlphaCommands runs recursively over all commands and set the message
// on every command defined in the commands map.
func AlphaCommands(cmd *cobra.Command) {
	path := commandPath(cmd)
	if alpha, ok := alphaCommands[path]; ok {
		msg := fmt.Sprintf("Alpha command, expect this to change or be removed, created: %s", alpha.createdDate)
		cmd.Short = util.ColorWarning(msg)

		cmd.Long = util.ColorWarning(msg + "\n" + alpha.info)
	}
	if !cmd.HasSubCommands() {
		return
	}
	for _, c := range cmd.Commands() {
		AlphaCommands(c)
	}
}

// BetaCommands runs recursively over all commands and set the message
// on every command defined in the commands map.
func BetaCommands(cmd *cobra.Command) {
	path := commandPath(cmd)
	if beta, ok := betaCommands[path]; ok {
		msg := fmt.Sprintf("Beta command, still experimental but maturing towards being GA, created: %s", beta.createdDate)
		cmd.Short = util.ColorWarning(msg)
		cmd.Long = util.ColorWarning(msg + "\n" + beta.info)
	}
	if !cmd.HasSubCommands() {
		return
	}
	for _, c := range cmd.Commands() {
		BetaCommands(c)
	}
}

func commandPath(cmd *cobra.Command) string {
	parentText := ""
	parent := cmd.Parent()
	if parent != nil {
		parentText = commandPath(parent)
		if parentText != "" {
			parentText += " "
		}
	}
	return strings.TrimPrefix(parentText, "jx ") + cmd.Name()
}
