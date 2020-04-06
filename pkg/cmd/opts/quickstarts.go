package opts

import (
	"fmt"
	"strings"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/quickstarts"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
)

// LoadQuickStartsModel Load all quickstarts
func (o *CommonOptions) LoadQuickStartsModel(gitHubOrganisations []string, ignoreTeam bool) (*quickstarts.QuickstartModel, error) {
	authConfigSvc, err := o.GitLocalAuthConfigService()
	if err != nil {
		return nil, err
	}
	resolver, err := o.GetVersionResolver()
	if err != nil {
		return nil, err
	}

	config := authConfigSvc.Config()

	locations, err := o.loadQuickStartLocations(gitHubOrganisations, ignoreTeam)
	if err != nil {
		return nil, err
	}

	model, err := o.LoadQuickStartsFromLocations(locations, config)
	if err != nil {
		return nil, fmt.Errorf("failed to load quickstarts: %s", err)
	}
	quickstarts, err := versionstream.GetQuickStarts(resolver.VersionsDir)
	if err != nil {
		return nil, errors.Wrapf(err, "loading quickstarts from version stream in dir %s", resolver.VersionsDir)
	}
	quickstarts.DefaultMissingValues()
	model.LoadQuickStarts(quickstarts)
	return model, nil
}

// LoadQuickStartsFromLocations Load all quickstarts from the given locatiotns
func (o *CommonOptions) LoadQuickStartsFromLocations(locations []v1.QuickStartLocation, config *auth.AuthConfig) (*quickstarts.QuickstartModel, error) {
	gitMap := map[string]map[string]v1.QuickStartLocation{}
	for _, loc := range locations {
		m := gitMap[loc.GitURL]
		if m == nil {
			m = map[string]v1.QuickStartLocation{}
			gitMap[loc.GitURL] = m
		}
		m[loc.Owner] = loc
	}
	model := quickstarts.NewQuickstartModel()

	for gitURL, m := range gitMap {
		for _, location := range m {
			kind := location.GitKind
			if kind == "" {
				kind = gits.KindGitHub
			}

			// If this is a default quickstart location but there's no github.com credentials, skip and rely on the version stream alone.
			server := config.GetOrCreateServer(gitURL)
			userAuth := config.CurrentUser(server, o.InCluster())
			if kube.IsDefaultQuickstartLocation(location) && (userAuth == nil || userAuth.IsInvalid()) {
				continue
			}
			gitProvider, err := o.GitProviderForGitServerURL(gitURL, kind, "")
			if err != nil {
				return model, err
			}
			log.Logger().Debugf("Searching for repositories in Git server %s owner %s includes %s excludes %s as user %s ", gitProvider.ServerURL(), location.Owner, strings.Join(location.Includes, ", "), strings.Join(location.Excludes, ", "), gitProvider.CurrentUsername())
			err = model.LoadGithubQuickstarts(gitProvider, location.Owner, location.Includes, location.Excludes)
			if err != nil {
				log.Logger().Debugf("Quickstart load error: %s", err.Error())
			}
		}
	}
	return model, nil
}

// loadQuickStartLocations loads the quickstarts
func (o *CommonOptions) loadQuickStartLocations(gitHubOrganisations []string, ignoreTeam bool) ([]v1.QuickStartLocation, error) {
	var locations []v1.QuickStartLocation
	if !ignoreTeam {
		jxClient, ns, err := o.JXClientAndDevNamespace()
		if err != nil {
			return nil, err
		}

		locations, err = kube.GetQuickstartLocations(jxClient, ns)
		if err != nil {
			return nil, err
		}
	}
	// lets add any extra github organisations if they are not already configured
	for _, org := range gitHubOrganisations {
		found := false
		for _, loc := range locations {
			if loc.GitURL == gits.GitHubURL && loc.Owner == org {
				found = true
				break
			}
		}
		if !found {
			locations = append(locations, v1.QuickStartLocation{
				GitURL:   gits.GitHubURL,
				GitKind:  gits.KindGitHub,
				Owner:    org,
				Includes: []string{"*"},
				Excludes: []string{"WIP-*"},
			})
		}
	}
	return locations, nil
}
