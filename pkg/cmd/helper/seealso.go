package helper

import (
	"fmt"
	"strings"
)

// SeeAlsoText returns text to describe which other commands to look at which are related to the current command
func SeeAlsoText(commands ...string) string {
	if len(commands) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nSee Also:\n\n")

	for _, command := range commands {
		u := "https://jenkins-x.io/commands/" + strings.Replace(command, " ", "_", -1)
		sb.WriteString(fmt.Sprintf("* %s : [%s](%s)\n", command, u, u))
	}
	sb.WriteString("\n")
	return sb.String()
}
