package semrel

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/Masterminds/semver"

	"github.com/jenkins-x/jx/pkg/gits"
)

var commitPattern = regexp.MustCompile("^(\\w*)(?:\\((.*)\\))?\\: (.*)$")
var breakingPattern = regexp.MustCompile("BREAKING CHANGES?")

type change struct {
	Major, Minor, Patch bool
}

type conventionalCommit struct {
	*gits.GitCommit
	MessageLines []string
	Type         string
	Scope        string
	MessageBody  string
	Change       change
}

type release struct {
	SHA     string
	Version *semver.Version
}

func calculateChange(commits []*conventionalCommit, latestRelease *release) change {
	var change change
	for _, commit := range commits {
		if latestRelease.SHA == commit.SHA {
			break
		}
		change.Major = change.Major || commit.Change.Major
		change.Minor = change.Minor || commit.Change.Minor
		change.Patch = change.Patch || commit.Change.Patch
	}
	return change
}

func applyChange(version *semver.Version, change change) *semver.Version {
	if version.Major() == 0 {
		change.Major = true
	}
	if !change.Major && !change.Minor && !change.Patch {
		return nil
	}
	var newVersion semver.Version
	preRel := version.Prerelease()
	if preRel == "" {
		switch {
		case change.Major:
			newVersion = version.IncMajor()
			break
		case change.Minor:
			newVersion = version.IncMinor()
			break
		case change.Patch:
			newVersion = version.IncPatch()
			break
		}
		return &newVersion
	}
	preRelVer := strings.Split(preRel, ".")
	if len(preRelVer) > 1 {
		idx, err := strconv.ParseInt(preRelVer[1], 10, 32)
		if err != nil {
			idx = 0
		}
		preRel = fmt.Sprintf("%s.%d", preRelVer[0], idx+1)
	} else {
		preRel += ".1"
	}
	newVersion, _ = version.SetPrerelease(preRel)
	return &newVersion
}

// GetNewVersion uses the conventional commits in the range of latestTagRev..endSha to increment the version from latestTag
func GetNewVersion(dir string, endSha string, gitter gits.Gitter, latestTag string, latestTagRev string) (*semver.Version, error) {
	version, err := semver.NewVersion(strings.TrimPrefix(latestTag, "v"))
	if err != nil {
		return nil, errors.Wrapf(err, "parsing %s as semantic version", latestTag)
	}
	release := release{
		SHA:     latestTagRev,
		Version: version,
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	rawCommits, err := gitter.GetCommits(dir, release.SHA, endSha)
	if err != nil {
		return nil, errors.Wrapf(err, "getting commits in range %s..%s", release.SHA, endSha)
	}
	commits := make([]*conventionalCommit, 0)
	for _, c := range rawCommits {
		commits = append(commits, parseCommit(&c))
	}

	return applyChange(release.Version, calculateChange(commits, &release)), nil
}

func parseCommit(commit *gits.GitCommit) *conventionalCommit {
	c := &conventionalCommit{
		GitCommit: commit,
	}
	c.MessageLines = strings.Split(commit.Message, "\n")
	found := commitPattern.FindAllStringSubmatch(c.MessageLines[0], -1)
	if len(found) < 1 {
		return c
	}
	c.Type = strings.ToLower(found[0][1])
	c.Scope = found[0][2]
	c.MessageBody = found[0][3]
	c.Change = change{
		Major: breakingPattern.MatchString(commit.Message),
		Minor: c.Type == "feat",
		Patch: c.Type == "fix",
	}
	return c
}
