package dependencymatrix

import (
	"regexp"

	"github.com/pkg/errors"
)

var (
	dependencyUpdateRegex = regexp.MustCompile(`^(?m:chore\((?:deps|dependencies)\): (?:bump|update) (.*) from ([\w\.]*) to ([\w\.]*)$)`)
	slugLinkRegex         = regexp.MustCompile(`^(?:([\w-]*?)?\/?([\w-]+)|(https?):\/\/([\w\.]*)\/([\w-]*)\/([\w-]*)(?:\.git)?)(?::([\w-]*))?$`)
)

// DependencyMessage is the parsed representation of a dependency update message on a commit
type DependencyMessage struct {
	URL         string
	Owner       string
	Host        string
	Repo        string
	FromVersion string
	ToVersion   string
	Component   string
	Scheme      string
}

// ParseDependencyMessage parses a dependency update message on a commit and returns the DependencyMessage struct
func ParseDependencyMessage(msg string) (*DependencyMessage, error) {
	matches := dependencyUpdateRegex.FindStringSubmatch(msg)
	if matches == nil {
		// string does not match at all
		return nil, nil
	}
	if len(matches) != 4 {
		return nil, errors.Errorf("parsing %s as dependency update message", msg)
	}
	slug := matches[1]
	var urlScheme, urlHost, owner, repo string
	slugMatches := slugLinkRegex.FindStringSubmatch(slug)
	if len(slugMatches) == 8 {
		if slugMatches[6] != "" {
			owner = slugMatches[5]
			repo = slugMatches[6]
			urlScheme = slugMatches[3]
			urlHost = slugMatches[4]
		} else {
			owner = slugMatches[1]
			repo = slugMatches[2]
		}
		update := &DependencyMessage{
			Owner:       owner,
			Repo:        repo,
			Host:        urlHost,
			Scheme:      urlScheme,
			FromVersion: matches[2],
			ToVersion:   matches[3],
			Component:   slugMatches[7],
		}
		return update, nil
	}
	return nil, nil
}
