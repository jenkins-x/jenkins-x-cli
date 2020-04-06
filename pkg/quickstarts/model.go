package quickstarts

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"gopkg.in/AlecAivazis/survey.v1"
)

const (
	// JenkinsXQuickstartsOwner default quickstart owner
	JenkinsXQuickstartsOwner = "jenkins-x-quickstarts"
)

// GitQuickstart returns a github based quickstart
func GitQuickstart(provider gits.GitProvider, owner string, repo string, version string, downloadURL string, language string, framework string, tags ...string) *Quickstart {
	return &Quickstart{
		ID:             owner + "/" + repo,
		Owner:          owner,
		Name:           repo,
		Version:        version,
		Language:       language,
		Framework:      framework,
		Tags:           tags,
		DownloadZipURL: downloadURL,
		GitProvider:    provider,
	}
}

func toGitHubQuickstart(provider gits.GitProvider, owner string, repo *gits.GitRepository) *Quickstart {
	language := repo.Language
	// TODO find this from GitHub???
	framework := ""
	tags := []string{}

	branch := "master"
	repoName := repo.Name
	gitCommits, err := provider.ListCommits(owner, repoName, &gits.ListCommitsArguments{
		SHA:     branch,
		Page:    1,
		PerPage: 1,
	})
	version := ""
	u := ""
	if err != nil {
		log.Logger().Warnf("failed to load commits on branch %s for repo %s/%s due to: %s", branch, owner, repoName, err.Error())
	} else if len(gitCommits) > 0 {
		commit := gitCommits[0]
		sha := commit.ShortSha()
		version = QuickStartVersion(sha)
		u = provider.BranchArchiveURL(owner, repoName, sha)
	}
	if u == "" {
		u = provider.BranchArchiveURL(owner, repoName, "master")
	}
	return GitQuickstart(provider, owner, repoName, version, u, language, framework, tags...)
}

// QuickStartVersion creates a quickstart version string
func QuickStartVersion(sha string) string {
	return "1.0.0+" + sha
}

// LoadGithubQuickstarts Loads quickstarts from github
func (model *QuickstartModel) LoadGithubQuickstarts(provider gits.GitProvider, owner string, includes []string, excludes []string) error {
	repos, err := provider.ListRepositories(owner)
	if err != nil {
		return err
	}
	for _, repo := range repos {
		name := repo.Name
		if util.StringMatchesAny(name, includes, excludes) {
			model.Add(toGitHubQuickstart(provider, owner, repo))
		}
	}
	return nil
}

// NewQuickstartModel creates a new quickstart model
func NewQuickstartModel() *QuickstartModel {
	return &QuickstartModel{
		Quickstarts: map[string]*Quickstart{},
	}
}

// SortedNames returns the sorted names of the quickstarts
func (model *QuickstartModel) SortedNames() []string {
	names := []string{}
	for name := range model.Quickstarts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Add adds the given quickstart to this mode. Returns true if it was added
func (model *QuickstartModel) Add(q *Quickstart) bool {
	if q != nil {
		id := q.ID
		if id != "" {
			model.Quickstarts[id] = q
			return true
		}
	}
	return false
}

// CreateSurvey creates a survey to query pick a quickstart
func (model *QuickstartModel) CreateSurvey(filter *QuickstartFilter, batchMode bool, handles util.IOFileHandles) (*QuickstartForm, error) {
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	language := filter.Language
	if language != "" {
		languages := model.Languages()
		if len(languages) == 0 {
			// lets ignore this filter as there are none available
			filter.Language = ""
		} else {
			lower := strings.ToLower(language)
			lowerLanguages := util.StringArrayToLower(languages)
			if util.StringArrayIndex(lowerLanguages, lower) < 0 {
				return nil, util.InvalidOption("language", language, languages)
			}
		}
	}
	quickstarts := model.Filter(filter)
	names := []string{}
	m := map[string]*Quickstart{}
	for _, q := range quickstarts {
		name := q.SurveyName()
		m[name] = q
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return nil, fmt.Errorf("No quickstarts match filter")
	}
	answer := ""
	if len(names) == 1 {
		// if there's only a single option, use it
		answer = names[0]
	} else if batchMode {
		// should not prompt for selection in batch mode so return an error
		return nil, fmt.Errorf("More than one quickstart matches the current filter options. Try filtering based on other criteria (eg. Owner or Text): %v", names)
	} else {
		// if no single answer after filtering and we're not in batch mode then prompt
		prompt := &survey.Select{
			Message: "select the quickstart you wish to create",
			Options: names,
		}
		err := survey.AskOne(prompt, &answer, survey.Required, surveyOpts)
		if err != nil {
			return nil, err
		}
	}

	if answer == "" {
		return nil, fmt.Errorf("No quickstart chosen")
	}
	q := m[answer]
	if q == nil {
		return nil, fmt.Errorf("Could not find chosen quickstart for %s", answer)
	}
	name := filter.ProjectName
	form := &QuickstartForm{
		Quickstart: q,
		Name:       name,
	}
	return form, nil
}

// Filter filters all the available quickstarts with the filter and return the matches
func (model *QuickstartModel) Filter(filter *QuickstartFilter) []*Quickstart {
	answer := []*Quickstart{}
	for _, name := range model.SortedNames() {
		q := model.Quickstarts[name]
		if filter.Matches(q) {
			// If the filter matches a quickstart name exactly, return only that quickstart
			if q.Name == filter.Text {
				return []*Quickstart{q}
			}
			answer = append(answer, q)
		}
	}
	return answer
}

// Languages returns all the languages in the quickstarts sorted
func (model *QuickstartModel) Languages() []string {
	m := map[string]string{}
	for _, q := range model.Quickstarts {
		l := q.Language
		if l != "" {
			m[l] = l
		}
	}
	return util.SortedMapKeys(m)
}

func (model *QuickstartModel) LoadQuickStarts(quickstarts *versionstream.QuickStarts) error {
	for _, from := range quickstarts.QuickStarts {
		id := from.ID
		if id == "" {
			log.Logger().Warnf("no ID available for quickstart in version stream %#v", from)
			continue
		}
		to := model.Quickstarts[id]
		if to == nil {
			to = &Quickstart{}
		}
		err := model.convertToQuickStart(from, to)
		if err != nil {
			return errors.Wrapf(err, "failed to convert quickstart from the version stream %s", id)
		}
		model.Quickstarts[id] = to
	}
	return nil
}

func (model *QuickstartModel) convertToQuickStart(from *versionstream.QuickStart, to *Quickstart) error {
	s := func(text string, override string) string {
		if override != "" {
			return override
		}
		return text
	}
	ss := func(texts []string, overrides []string) []string {
		answer := append([]string{}, texts...)
		for _, o := range overrides {
			if util.StringArrayIndex(answer, o) < 0 {
				answer = append(answer, o)
			}
		}
		return answer
	}

	to.ID = s(to.ID, from.ID)
	to.Owner = s(to.Owner, from.Owner)
	to.Name = s(to.Name, from.Name)
	to.Version = s(to.Version, from.Version)
	to.DownloadZipURL = s(to.DownloadZipURL, from.DownloadZipURL)
	to.Framework = s(to.Framework, from.Framework)
	to.Language = s(to.Language, from.Language)
	to.Tags = ss(to.Tags, from.Tags)
	return nil
}
