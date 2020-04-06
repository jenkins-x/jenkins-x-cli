package maven

import (
	"strings"

	"fmt"
	"sort"

	"github.com/jenkins-x/jx/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1"
)

const (
	MavenArchetypePluginVersion = "3.0.1"
)

type ArtifactVersions struct {
	GroupId     string
	ArtifactId  string
	Description string
	Versions    []string
}

type GroupArchectypes struct {
	GroupId   string
	Artifacts map[string]*ArtifactVersions
}

type ArchetypeModel struct {
	Groups map[string]*GroupArchectypes
}

type ArtifactData struct {
	GroupId     string
	ArtifactId  string
	Version     string
	Description string
}

type ArchetypeFilter struct {
	GroupIds         []string
	GroupIdFilter    string
	ArtifactIdFilter string
	Version          string
}

type ArchetypeForm struct {
	ArchetypeGroupId    string
	ArchetypeArtifactId string
	ArchetypeVersion    string

	GroupId    string
	ArtifactId string
	Package    string
	Version    string
}

func NewArchetypeModel() ArchetypeModel {
	return ArchetypeModel{
		Groups: map[string]*GroupArchectypes{},
	}
}

func (m *ArchetypeModel) GroupIDs(filter string) []string {
	answer := []string{}
	for group := range m.Groups {
		if filter == "" || strings.Index(group, filter) >= 0 {
			answer = append(answer, group)
		}
	}
	sort.Strings(answer)
	return answer
}

func (m *ArchetypeModel) ArtifactIDs(groupId string, filter string) []string {
	answer := []string{}
	artifact := m.Groups[groupId]
	if artifact != nil {
		for a := range artifact.Artifacts {
			if filter == "" || strings.Index(a, filter) >= 0 {
				answer = append(answer, a)
			}
		}
		sort.Strings(answer)
	}
	return answer
}

func (m *ArchetypeModel) Versions(groupId string, artifactId, filter string) []string {
	answer := []string{}
	artifact := m.Groups[groupId]
	if artifact != nil {
		av := artifact.Artifacts[artifactId]
		if av != nil {
			for _, v := range av.Versions {
				if filter == "" || strings.Index(v, filter) >= 0 {
					answer = append(answer, v)
				}
			}
			// TODO use a version sorter?
			sort.Sort(sort.Reverse(sort.StringSlice(answer)))
		}
	}
	return answer
}

func (m *ArchetypeModel) AddArtifact(a *ArtifactData) *ArtifactVersions {
	groupId := a.GroupId
	artifactId := a.ArtifactId
	version := a.Version
	description := a.Description
	if groupId == "" || artifactId == "" || version == "" {
		return nil
	}

	if m.Groups == nil {
		m.Groups = map[string]*GroupArchectypes{}
	}
	group := m.Groups[groupId]
	if group == nil {
		group = &GroupArchectypes{
			GroupId:   groupId,
			Artifacts: map[string]*ArtifactVersions{},
		}
		m.Groups[groupId] = group
	}
	artifact := group.Artifacts[artifactId]
	if artifact == nil {
		artifact = &ArtifactVersions{
			GroupId:    groupId,
			ArtifactId: artifactId,
			Versions:   []string{},
		}
		group.Artifacts[artifactId] = artifact
	}
	if artifact.Description == "" && description != "" {
		artifact.Description = description
	}
	if util.StringArrayIndex(artifact.Versions, version) < 0 {
		artifact.Versions = append(artifact.Versions, version)
	}
	return artifact
}

func (model *ArchetypeModel) CreateSurvey(data *ArchetypeFilter, pickVersion bool, form *ArchetypeForm, handles util.IOFileHandles) error {
	surveyOpts := survey.WithStdio(handles.In, handles.Out, handles.Err)
	groupIds := data.GroupIds
	if len(data.GroupIds) == 0 {
		filteredGroups := model.GroupIDs(data.GroupIdFilter)
		if len(filteredGroups) == 0 {
			return util.InvalidOption("group-filter", data.GroupIdFilter, model.GroupIDs(""))
		}

		// lets pick from all groups
		prompt := &survey.Select{
			Message: "Group ID:",
			Options: filteredGroups,
		}
		err := survey.AskOne(prompt, &form.ArchetypeGroupId, survey.Required, surveyOpts)
		if err != nil {
			return err
		}
		artifactsWithoutFilter := model.ArtifactIDs(form.ArchetypeGroupId, "")
		if len(artifactsWithoutFilter) == 0 {
			return fmt.Errorf("Could not find any artifacts for group %s", form.ArchetypeGroupId)
		}
	} else {
		// TODO for now lets just support a single group ID being passed in
		form.ArchetypeGroupId = groupIds[0]

		artifactsWithoutFilter := model.ArtifactIDs(form.ArchetypeGroupId, "")
		if len(artifactsWithoutFilter) == 0 {
			return util.InvalidOption("group", form.ArchetypeGroupId, model.GroupIDs(""))
		}
	}
	if form.ArchetypeGroupId == "" {
		return fmt.Errorf("No archetype groupId selected")
	}

	artifactIds := model.ArtifactIDs(form.ArchetypeGroupId, data.ArtifactIdFilter)
	if len(artifactIds) == 0 {
		artifactsWithoutFilter := model.ArtifactIDs(form.ArchetypeGroupId, "")
		return util.InvalidOption("artifact", data.ArtifactIdFilter, artifactsWithoutFilter)
	}

	if len(artifactIds) == 1 {
		form.ArchetypeArtifactId = artifactIds[0]
	} else {
		prompt := &survey.Select{
			Message: "Artifact ID:",
			Options: artifactIds,
		}
		err := survey.AskOne(prompt, &form.ArchetypeArtifactId, survey.Required, surveyOpts)
		if err != nil {
			return err
		}
	}
	if form.ArchetypeArtifactId == "" {
		return fmt.Errorf("No archetype artifactId selected")
	}

	version := data.Version
	versions := model.Versions(form.ArchetypeGroupId, form.ArchetypeArtifactId, version)
	if len(versions) == 0 {
		return util.InvalidOption("version", version, model.Versions(form.ArchetypeGroupId, form.ArchetypeArtifactId, ""))
	}

	if len(versions) == 1 || !pickVersion {
		form.ArchetypeVersion = versions[0]
	} else {
		prompt := &survey.Select{
			Message: "Version:",
			Options: versions,
		}
		err := survey.AskOne(prompt, &form.ArchetypeVersion, survey.Required, surveyOpts)
		if err != nil {
			return err
		}
	}
	if form.ArchetypeVersion == "" {
		return fmt.Errorf("No archetype version selected")
	}

	if form.GroupId == "" {
		q := &survey.Input{
			Message: "Project Group ID:",
			Default: "com.acme",
		}
		err := survey.AskOne(q, &form.GroupId, survey.Required, surveyOpts)
		if err != nil {
			return err
		}
	}
	if form.ArtifactId == "" {
		q := &survey.Input{
			Message: "Project Artifact ID:",
			Default: "",
		}
		err := survey.AskOne(q, &form.ArtifactId, survey.Required, surveyOpts)
		if err != nil {
			return err
		}
	}
	if form.Version == "" {
		q := &survey.Input{
			Message: "Project Version:",
			Default: "1.0.0-SNAPSHOT",
		}
		err := survey.AskOne(q, &form.Version, survey.Required, surveyOpts)
		if err != nil {
			return err
		}
	}
	return nil
}
