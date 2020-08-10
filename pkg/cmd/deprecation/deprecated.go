package deprecation

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/spf13/cobra"
)

// deprecatedCommands list of deprecated commands along with some more deprecation details
var deprecatedCommands = map[string]deprecationInfo{
	"install": {
		replacement: "jx boot",
		date:        "Sep 1 2020",
		info: fmt.Sprintf("Please check %s for more details.",
			util.ColorStatus("https://jenkins-x.io/docs/getting-started/setup/boot/")),
	},
	// Todo: Implementation is very tightly coupled with jx install, better to remove it once we remove jx install
	"init": {
		replacement: "jx boot",
		date:        "Sep 1 2020",
		info: fmt.Sprintf("Please check %s for more details.",
			util.ColorStatus("https://jenkins-x.io/docs/getting-started/setup/boot/")),
	},
	"create spring": {
		replacement: "jx create project",
		date:        "Sep 1 2020",
		info: fmt.Sprintf("Please check %s for more details.",
			util.ColorStatus("https://jenkins-x.io/commands/jx_create_project/")),
	},
	"upgrade ingress": {
		replacement: "jx boot",
		date:        "Sep 1 2020",
		info: fmt.Sprintf("Please check %s for more details.",
			util.ColorStatus("https://jenkins-x.io/docs/getting-started/setup/boot/")),
	},
	"create vault": {
		date: "Sep 1 2020",
		info: "This commands will have no replacement.",
	},
	"delete vault": {
		date: "Sep 1 2020",
		info: "This commands will have no replacement.",
	},
	"create addon": {
		date:        "Sep 1 2020",
		replacement: "jx add app",
	},
	"create addon ambassador": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-ambassador",
	},
	"create addon anchore": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-anchore",
	},
	"create addon envctl": {
		date: "Sep 1 2020",
		info: "This commands will have no replacement.",
	},
	"create addon flagger": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-flagger",
	},
	"create addon gloo": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-gloo",
	},
	"create addon ingctl": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-ingress",
	},
	"create addon istio": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-istio",
	},
	"create addon kubeless": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-kubeless",
	},
	"create addon owasp-zap": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-owasp-zap",
	},
	"create addon pipeline-events": {
		date: "Sep 1 2020",
		info: "This commands will have no replacement.",
	},
	"create addon prometheus": {
		date:        "Sep 1 2020",
		replacement: "jx add app jx-app-prometheus",
	},
	"create addon prow": {
		date: "Sep 1 2020",
		info: "This commands will have no replacement.",
	},
}

// deprecateInfo keeps some deprecation details related to a command
type deprecationInfo struct {
	replacement string
	date        string
	info        string
}

// DeprecateCommands runs recursively over all commands and set the deprecation message
// on every command defined the deprecated commands map.
func DeprecateCommands(cmd *cobra.Command) {
	path := commandPath(cmd)
	if deprecation, ok := deprecatedCommands[path]; ok {
		cmd.Deprecated = deprecationMessage(deprecation)
	}
	if !cmd.HasSubCommands() {
		return
	}
	for _, c := range cmd.Commands() {
		DeprecateCommands(c)
	}
}

// GetRemovalDate returns the date when the command is planned to be removed
func GetRemovalDate(cmd *cobra.Command) string {
	path := commandPath(cmd)
	if deprecation, ok := deprecatedCommands[path]; ok {
		return deprecation.date
	}
	return ""
}

// GetReplacement returns the command replacement if any available
func GetReplacement(cmd *cobra.Command) string {
	path := commandPath(cmd)
	if deprecation, ok := deprecatedCommands[path]; ok {
		return deprecation.replacement
	}
	return ""
}

func deprecationMessage(dep deprecationInfo) string {
	var date string
	if dep.date != "" {
		date = fmt.Sprintf("it will be removed on %s.", util.ColorInfo(dep.date))
	} else {
		date = "it will be soon removed."
	}
	var replacement string
	if dep.replacement != "" {
		replacement = fmt.Sprintf("We now highly recommend you use %s instead.", util.ColorInfo(dep.replacement))
	}
	msg := fmt.Sprintf("%s %s", date, replacement)
	if dep.info != "" {
		return fmt.Sprintf("%s %s", msg, dep.info)
	}
	return msg
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
