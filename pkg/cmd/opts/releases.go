package opts

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/jenkins-x/jx/pkg/dependencymatrix"
	"github.com/jenkins-x/jx/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/jenkins-x/jx/pkg/gits/releases"

	"github.com/jenkins-x/jx/pkg/gits"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/pkg/errors"
)

// ParseDependencyUpdateMessage parses commit messages, and if it's a dependency update message parses it
//
// A complete update message looks like:
//
// chore(dependencies): update ((<owner>/)?<repo>|https://<gitHost>/<owner>/<repo>) from <fromVersion> to <toVersion>
//
// <description of update method>
//
// <fromVersion>, <toVersion> and <repo> are required fields. The markdown URL format is optional, and a plain <owner>/<repo>
// can be used.
func (o *CommonOptions) ParseDependencyUpdateMessage(msg string, commitURL string) (*v1.DependencyUpdate, *dependencymatrix.DependencyUpdates, error) {
	parsedMessage, err := dependencymatrix.ParseDependencyMessage(msg)
	if parsedMessage == nil {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	commitInfo, err := gits.ParseGitURL(commitURL)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "parsing %s", commitURL)
	}
	if parsedMessage.Owner == "" {
		parsedMessage.Owner = commitInfo.Organisation
	}
	if parsedMessage.Scheme == "" {
		parsedMessage.Scheme = commitInfo.Scheme
	}
	if parsedMessage.Host == "" {
		parsedMessage.Host = commitInfo.Host
	}
	update := &v1.DependencyUpdate{
		DependencyUpdateDetails: v1.DependencyUpdateDetails{
			Owner:       parsedMessage.Owner,
			Repo:        parsedMessage.Repo,
			Host:        parsedMessage.Host,
			URL:         fmt.Sprintf("%s://%s/%s/%s", parsedMessage.Scheme, parsedMessage.Host, parsedMessage.Owner, parsedMessage.Repo),
			FromVersion: parsedMessage.FromVersion,
			ToVersion:   parsedMessage.ToVersion,
			Component:   parsedMessage.Component,
		},
	}
	provider, _, err := o.CreateGitProviderForURLWithoutKind(update.URL)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "creating git provider for %s", update.URL)
	}

	var upstreamUpdates dependencymatrix.DependencyUpdates
	toRelease, err := releases.GetRelease(update.ToVersion, update.Owner, update.Repo, provider)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if toRelease != nil {
		update.ToReleaseHTMLURL = toRelease.HTMLURL
		update.ToReleaseName = toRelease.Name
		if toRelease.Assets != nil {
			for _, asset := range *toRelease.Assets {
				if asset.Name == dependencymatrix.DependencyUpdatesAssetName {
					resp, err := http.Get(asset.BrowserDownloadURL)
					if err != nil {
						return nil, nil, errors.Wrapf(err, "retrieving dependency updates from %s", asset.BrowserDownloadURL)
					}
					defer resp.Body.Close()

					// Write the body
					var b bytes.Buffer
					_, err = io.Copy(&b, resp.Body)
					str := b.String()
					log.Logger().Debugf("Dependency update yaml is %s", str)
					if err != nil {
						return nil, nil, errors.Wrapf(err, "retrieving dependency updates from %s", asset.BrowserDownloadURL)
					}
					err = yaml.Unmarshal([]byte(str), &upstreamUpdates)
					if err != nil {
						return nil, nil, errors.Wrapf(err, "unmarshaling dependency updates from %s", asset.BrowserDownloadURL)
					}
				}
			}
		}
	}
	fromRelease, err := releases.GetRelease(update.FromVersion, update.Owner, update.Repo, provider)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	if fromRelease != nil {
		update.FromReleaseHTMLURL = fromRelease.HTMLURL
		update.FromReleaseName = fromRelease.Name
	}
	return update, &upstreamUpdates, nil
}
