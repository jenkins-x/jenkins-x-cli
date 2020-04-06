package operations

import (
	"bytes"
	"fmt"
	"time"

	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/secreturl"

	"github.com/jenkins-x/jx/pkg/cmd/opts"

	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/dependencymatrix"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/gits/releases"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	// assetRetrievalTimeout controls the amount of time to wait when attempting to retrieve dependency information
	assetRetrievalTimeout = 3 * time.Minute
)

// PullRequestOperation provides a way to execute a PullRequest operation using Git
type PullRequestOperation struct {
	*opts.CommonOptions
	GitURLs       []string
	SrcGitURL     string
	Base          string
	Component     string
	BranchName    string
	Version       string
	DryRun        bool
	SkipCommit    bool
	AuthorName    string
	AuthorEmail   string
	SkipAutoMerge bool
	Labels        []string
}

// ChangeFilesFn is the function called to create the pull request
type ChangeFilesFn func(dir string, gitInfo *gits.GitRepository) ([]string, error)

// CreatePullRequest will fork (if needed) and pull a git repo, then perform the update, and finally create or update a
// PR for the change. Any open PR on the repo with the `updatebot` label will be updated.
func (o *PullRequestOperation) CreatePullRequest(kind string, update ChangeFilesFn) (*gits.PullRequestInfo, error) {
	var result *gits.PullRequestInfo
	for _, gitURL := range o.GitURLs {
		dir, err := ioutil.TempDir("", "create-pr")
		if err != nil {
			return nil, err
		}
		provider, _, err := o.CreateGitProviderForURLWithoutKind(gitURL)
		if err != nil {
			return nil, errors.Wrapf(err, "creating git provider for directory %s", dir)
		}

		dir, _, upstreamInfo, forkInfo, err := gits.ForkAndPullRepo(gitURL, dir, o.Base, o.BranchName, provider, o.Git(), "")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to fork and pull %s", o.GitURLs)
		}

		gitInfo := upstreamInfo
		if forkInfo != nil {
			gitInfo = forkInfo
		}
		commitMessage, details, err := o.updateAndGenerateMessagesAndDependencyMatrix(dir, kind, upstreamInfo.Host, gitInfo, update)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		labels := []string{}
		if !o.SkipAutoMerge {
			labels = append(labels, "updatebot")
		}
		if len(o.Labels) > 0 {
			labels = append(labels, o.Labels...)
		}
		filter := &gits.PullRequestFilter{
			Labels: labels,
		}

		details.Labels = labels
		result, err = gits.PushRepoAndCreatePullRequest(dir, upstreamInfo, forkInfo, o.Base, details, filter, !o.SkipCommit, commitMessage, true, o.DryRun, o.Git(), provider)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create PR for base %s and head branch %s from temp dir %s", o.Base, details.BranchName, dir)
		}
	}
	return result, nil
}

// WrapChangeFilesWithCommitFn wraps the passed ChangeFilesFn in a commit. This is useful for creating multiple commits
// to batch e.g. in a single PR push/creation
func (o *PullRequestOperation) WrapChangeFilesWithCommitFn(kind string, fn ChangeFilesFn) ChangeFilesFn {
	return func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		commitMessage, prDetails, err := o.updateAndGenerateMessagesAndDependencyMatrix(dir, kind, gitInfo.Host, gitInfo, fn)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		err = o.Git().Add(dir, "-A")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		changed, err := o.Git().HasChanges(dir)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if !changed {
			log.Logger().Warnf("No changes made to the source code in %s. Code must be up to date!", dir)
			return nil, nil
		}
		if commitMessage == "" {
			commitMessage = prDetails.Message
		}
		err = o.Git().CommitDir(dir, commitMessage)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return nil, nil
	}
}

func (o *PullRequestOperation) updateAndGenerateMessagesAndDependencyMatrix(dir string, kind string, destHost string, gitInfo *gits.GitRepository, update ChangeFilesFn) (string, *gits.PullRequestDetails, error) {

	oldVersions, err := update(dir, gitInfo)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}
	nonSemantic := make([]string, 0)
	semantic := make([]semver.Version, 0)
	for _, v := range oldVersions {
		sv, err := semver.Parse(v)
		if err != nil {
			nonSemantic = append(nonSemantic, v)
		} else {
			semantic = append(semantic, sv)
		}
	}
	semver.Sort(semantic)
	sort.Strings(nonSemantic)
	dedupedSemantic := make([]string, 0)
	dedupedNonSemantic := make([]string, 0)
	previous := ""
	for _, v := range nonSemantic {
		if v != previous {
			dedupedNonSemantic = append(dedupedNonSemantic, v)
		}
		previous = v
	}
	previous = ""
	for _, v := range semantic {
		vStr := v.String()
		if vStr != previous {
			dedupedSemantic = append(dedupedSemantic, vStr)
		}
		previous = vStr
	}

	oldVersionsStr := strings.Join(dedupedSemantic, ", ")
	if len(dedupedNonSemantic) > 0 {
		if oldVersionsStr != "" {
			oldVersionsStr += " and "
		}
		oldVersionsStr = oldVersionsStr + strings.Join(dedupedNonSemantic, ", ")
	}

	// remove the v prefix if we are using a v tag
	version := strings.TrimPrefix(o.Version, "v")
	commitMessage, details, updateDependency, assets, err := o.CreateDependencyUpdatePRDetails(kind, o.SrcGitURL, destHost, oldVersionsStr, version, o.Component)
	if err != nil {
		return "", nil, errors.WithStack(err)
	}

	var upstreamDependencyAsset *gits.GitReleaseAsset

	for _, asset := range assets {
		if asset.Name == dependencymatrix.DependencyUpdatesAssetName {
			upstreamDependencyAsset = &asset
			break
		}
	}

	if updateDependency != nil {
		err = dependencymatrix.UpdateDependencyMatrix(dir, updateDependency)
		if err != nil {
			return "", nil, errors.WithStack(err)
		}
	}

	if upstreamDependencyAsset != nil {
		updatedPaths, err := AddDependencyMatrixUpdatePaths(upstreamDependencyAsset, updateDependency)
		if err != nil {
			log.Logger().Warnf("adding dependency updates: %q", err)
		} else {
			for _, d := range updatedPaths {
				err = dependencymatrix.UpdateDependencyMatrix(dir, d)
				if err != nil {
					return "", nil, errors.Wrapf(err, "updating dependency matrix with upstream dependency %+v", d)
				}
			}
		}
	}
	return commitMessage, details, nil
}

// AddDependencyMatrixUpdatePaths retrieves the upstreamDependencyAsset and converts it to a slice of DependencyUpdates, prepending the updateDependency to the path
func AddDependencyMatrixUpdatePaths(upstreamDependencyAsset *gits.GitReleaseAsset, updateDependency *v1.DependencyUpdate) ([]*v1.DependencyUpdate, error) {
	var upstreamUpdates dependencymatrix.DependencyUpdates
	var updates []*v1.DependencyUpdate

	log.Logger().Infof("Attempting to retrieve dependency asset information from %q", upstreamDependencyAsset.BrowserDownloadURL)

	err := util.Retry(assetRetrievalTimeout, func() error {
		resp, err := http.Get(upstreamDependencyAsset.BrowserDownloadURL)
		if err != nil {
			return errors.Wrapf(err, "retrieving dependency updates from %s", upstreamDependencyAsset.BrowserDownloadURL)
		}
		defer resp.Body.Close()
		// Write the body
		var b bytes.Buffer
		_, err = io.Copy(&b, resp.Body)
		if err != nil {
			return errors.Wrapf(err, "copying response body after retrieving dependency updates from %s", upstreamDependencyAsset.BrowserDownloadURL)
		}
		bytes := b.Bytes()
		err = yaml.Unmarshal(bytes, &upstreamUpdates)
		if err != nil {
			log.Logger().Errorf("unable to unmarshall from %s", string(bytes))
			return errors.Wrapf(err, "unmarshaling dependency updates from %s", upstreamDependencyAsset.BrowserDownloadURL)
		}
		return nil
	})

	if err != nil {
		return nil, errors.Wrapf(err, "getting dependency updates from %s", upstreamDependencyAsset.BrowserDownloadURL)
	}

	for _, d := range upstreamUpdates.Updates {
		// Need to prepend a path element
		if d.Paths == nil {
			d.Paths = []v1.DependencyUpdatePath{
				{
					{
						Host:               updateDependency.Host,
						Owner:              updateDependency.Owner,
						Repo:               updateDependency.Repo,
						URL:                updateDependency.URL,
						Component:          updateDependency.Component,
						ToReleaseHTMLURL:   updateDependency.ToReleaseHTMLURL,
						ToVersion:          updateDependency.ToVersion,
						FromVersion:        updateDependency.FromVersion,
						FromReleaseName:    updateDependency.FromReleaseName,
						FromReleaseHTMLURL: updateDependency.FromReleaseHTMLURL,
						ToReleaseName:      updateDependency.ToReleaseName,
					},
				},
			}
		} else {
			for j, e := range d.Paths {
				if e == nil {
					d.Paths[j] = v1.DependencyUpdatePath{
						{
							Host:               updateDependency.Host,
							Owner:              updateDependency.Owner,
							Repo:               updateDependency.Repo,
							URL:                updateDependency.URL,
							Component:          updateDependency.Component,
							ToReleaseHTMLURL:   updateDependency.ToReleaseHTMLURL,
							ToVersion:          updateDependency.ToVersion,
							FromVersion:        updateDependency.FromVersion,
							FromReleaseName:    updateDependency.FromReleaseName,
							FromReleaseHTMLURL: updateDependency.FromReleaseHTMLURL,
							ToReleaseName:      updateDependency.ToReleaseName,
						},
					}
				}
				d.Paths[j] = append([]v1.DependencyUpdateDetails{
					{
						Host:               updateDependency.Host,
						Owner:              updateDependency.Owner,
						Repo:               updateDependency.Repo,
						URL:                updateDependency.URL,
						Component:          updateDependency.Component,
						ToReleaseHTMLURL:   updateDependency.ToReleaseHTMLURL,
						ToVersion:          updateDependency.ToVersion,
						FromVersion:        updateDependency.FromVersion,
						FromReleaseName:    updateDependency.FromReleaseName,
						FromReleaseHTMLURL: updateDependency.FromReleaseHTMLURL,
						ToReleaseName:      updateDependency.ToReleaseName,
					},
				}, e...)
			}
		}
		updates = append(updates, &d)
	}
	return updates, nil
}

// CreateDependencyUpdatePRDetails creates the PullRequestDetails for a pull request, taking the kind of change it is (an id)
// the srcRepoUrl for the repo that caused the change, the destRepo for the repo receiving the change, the fromVersion and the toVersion
func (o PullRequestOperation) CreateDependencyUpdatePRDetails(kind string, srcRepoURL string, destHost string, fromVersion string, toVersion string, component string) (string, *gits.PullRequestDetails, *v1.DependencyUpdate, []gits.GitReleaseAsset, error) {
	var commitMessage, title, message strings.Builder
	commitMessage.WriteString("chore(deps): bump ")
	title.WriteString("chore(deps): bump ")
	message.WriteString("Update ")
	var update *v1.DependencyUpdate
	var assets []gits.GitReleaseAsset

	if srcRepoURL != "" {
		provider, srcRepo, err := o.CreateGitProviderForURLWithoutKind(srcRepoURL)
		if err != nil {
			return "", nil, nil, nil, errors.Wrapf(err, "creating git provider for %s", srcRepoURL)
		}
		update = &v1.DependencyUpdate{
			DependencyUpdateDetails: v1.DependencyUpdateDetails{
				Owner: srcRepo.Organisation,
				Repo:  srcRepo.Name,
				URL:   srcRepoURL,
			},
		}
		if srcRepo.Host != destHost {
			commitMessage.WriteString(srcRepoURL)
			title.WriteString(srcRepoURL)
			update.Host = srcRepo.Host
		} else {
			titleStr := fmt.Sprintf("%s/%s", srcRepo.Organisation, srcRepo.Name)
			commitMessage.WriteString(titleStr)
			title.WriteString(titleStr)
			update.Host = destHost
		}
		repoStr := fmt.Sprintf("[%s/%s](%s)", srcRepo.Organisation, srcRepo.Name, srcRepoURL)
		message.WriteString(repoStr)

		if component != "" {
			componentStr := fmt.Sprintf(":%s", component)
			commitMessage.WriteString(componentStr)
			title.WriteString(componentStr)
			message.WriteString(componentStr)
			update.Component = component
		}
		commitMessage.WriteString(" ")
		title.WriteString(" ")
		message.WriteString(" ")

		if fromVersion != "" {
			fromText := fmt.Sprintf("from %s ", fromVersion)
			commitMessage.WriteString(fromText)
			title.WriteString(fromText)
			update.FromVersion = fromVersion
			release, err := releases.GetRelease(fromVersion, srcRepo.Organisation, srcRepo.Name, provider)
			if err != nil {
				return "", nil, nil, nil, errors.Wrapf(err, "getting release from %s/%s", srcRepo.Organisation, srcRepo.Name)
			}
			if release != nil {
				message.WriteString(fmt.Sprintf("from [%s](%s) ", fromVersion, release.HTMLURL))
				update.FromReleaseName = release.Name
				update.FromReleaseHTMLURL = release.HTMLURL
			} else {
				message.WriteString(fmt.Sprintf("from %s ", fromVersion))
			}
		}
		if toVersion != "" {
			toText := fmt.Sprintf("to %s", toVersion)
			commitMessage.WriteString(toText)
			title.WriteString(toText)
			update.ToVersion = toVersion
			release, err := releases.GetRelease(toVersion, srcRepo.Organisation, srcRepo.Name, provider)
			if err != nil {
				return "", nil, nil, nil, errors.Wrapf(err, "getting release from %s/%s", srcRepo.Organisation, srcRepo.Name)
			}
			if release != nil {
				message.WriteString(fmt.Sprintf("to [%s](%s)", toVersion, release.HTMLURL))
				update.ToReleaseHTMLURL = release.HTMLURL
				update.ToReleaseName = release.Name
				if release.Assets != nil {
					assets = *release.Assets
				}
			} else {
				message.WriteString(fmt.Sprintf("to %s", toVersion))
			}
		}
	} else {
		commitMessage.WriteString(" versions")
		title.WriteString(" versions")
		message.WriteString(" versions")
	}
	message.WriteString(fmt.Sprintf("\n\nCommand run was `%s`", strings.Join(os.Args, " ")))
	commitMessage.WriteString(fmt.Sprintf("\n\nCommand run was `%s`", strings.Join(os.Args, " ")))
	if o.AuthorEmail != "" && o.AuthorName != "" {
		commitMessage.WriteString(fmt.Sprintf("\n\nSigned-off-by: %s <%s>", o.AuthorName, o.AuthorEmail))
	}
	return commitMessage.String(), &gits.PullRequestDetails{
		BranchName: fmt.Sprintf("bump-%s-version-%s", kind, string(uuid.NewUUID())),
		Title:      title.String(),
		Message:    message.String(),
	}, update, assets, nil
}

// CreatePullRequestRegexFn creates the ChangeFilesFn that will apply the regex, updating the matches to version over the files
func CreatePullRequestRegexFn(version string, regex string, files ...string) (ChangeFilesFn, error) {
	r, err := regexp.Compile(regex)
	if err != nil {
		return nil, errors.Wrapf(err, "%s does not compile", regex)
	}
	namedCaptures := make([]bool, 0)
	namedCapture := false
	for i, n := range r.SubexpNames() {
		if i == 0 {
			continue
		} else if n == "version" {
			namedCaptures = append(namedCaptures, true)
			namedCapture = true
		} else {
			namedCaptures = append(namedCaptures, false)
		}
	}
	return func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		oldVersions := make([]string, 0)
		for _, glob := range files {

			matches, err := filepath.Glob(filepath.Join(dir, glob))
			if err != nil {
				return nil, errors.Wrapf(err, "applying glob %s", glob)
			}

			// iterate over the glob matches
			for _, path := range matches {

				data, err := ioutil.ReadFile(path)
				if err != nil {
					return nil, errors.Wrapf(err, "reading %s", path)
				}
				info, err := os.Stat(path)
				if err != nil {
					return nil, errors.WithStack(err)
				}
				s := string(data)
				for _, b := range namedCaptures {
					if b {
						namedCapture = true
					}
				}
				answer := util.ReplaceAllStringSubmatchFunc(r, s, func(groups []util.Group) []string {
					answer := make([]string, 0)
					for i, group := range groups {
						if namedCapture {
							// If we are using named capture, then replace only the named captures that have the right name
							if namedCaptures[i] {
								oldVersions = append(oldVersions, group.Value)
								answer = append(answer, version)
							} else {
								answer = append(answer, group.Value)
							}
						} else {
							oldVersions = append(oldVersions, group.Value)
							answer = append(answer, version)
						}

					}
					return answer
				})
				err = ioutil.WriteFile(path, []byte(answer), info.Mode())
				if err != nil {
					return nil, errors.Wrapf(err, "writing %s", path)
				}
			}
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
		return oldVersions, nil
	}, nil
}

// CreatePullRequestBuildersFn creates the ChangeFilesFn that will update the gcr.io/jenkinsxio/builder-*.yml images
func CreatePullRequestBuildersFn(version string) ChangeFilesFn {
	return func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		answer, err := versionstream.UpdateStableVersionFiles(filepath.Join(dir, string(versionstream.KindDocker), "gcr.io", "jenkinsxio", "builder-*.yml"), version, "builder-base.yml", "builder-machine-learning.yml", "builder-machine-learning-gpu.yml")
		if err != nil {
			return nil, errors.Wrap(err, "modifying the builder-*.yml image versions")
		}
		return answer, nil
	}
}

// CreatePullRequestMLBuildersFn creates the ChangeFilesFn that will update the gcr.io/jenkinsxio/builder-machine-learning*.yml images
func CreatePullRequestMLBuildersFn(version string) ChangeFilesFn {
	return func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		answer, err := versionstream.UpdateStableVersionFiles(filepath.Join(dir, string(versionstream.KindDocker), "gcr.io", "jenkinsxio", "builder-machine-learning*.yml"), version)
		if err != nil {
			return nil, errors.Wrap(err, "modifying the builder-machine-learning*.yml image versions")
		}
		return answer, nil
	}
}

// CreatePullRequestGitReleasesFn creates the ChangeFilesFn that will update the git/ directory in the versions repo, using the git provider release api
func (o *PullRequestOperation) CreatePullRequestGitReleasesFn(name string) ChangeFilesFn {
	return func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		u := fmt.Sprintf("https://%s.git", name)
		provider, gitInfo, err := o.CreateGitProviderForURLWithoutKind(u)
		if err != nil {
			return nil, errors.Wrapf(err, "creating git provider for %s", u)
		}
		release, err := provider.GetLatestRelease(gitInfo.Organisation, gitInfo.Name)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find latest version for %s", u)
		}

		version := strings.TrimPrefix(release.Name, "v")
		log.Logger().Infof("found latest version %s for git repo %s", util.ColorInfo(version), util.ColorInfo(u))
		o.Version = version
		if o.SrcGitURL == "" {
			sv, err := versionstream.LoadStableVersion(dir, versionstream.KindGit, name)
			if err != nil {
				return nil, errors.Wrapf(err, "loading stable version")
			}
			o.SrcGitURL = sv.GitURL
			if sv.Component != "" {
				o.Component = sv.Component
			}
		}
		oldVersions, err := versionstream.UpdateStableVersion(dir, string(versionstream.KindGit), name, version)
		if err != nil {
			return nil, errors.Wrapf(err, "updating version %s to %s", u, version)
		}
		return oldVersions, nil

	}
}

// CreateChartChangeFilesFn creates the ChangeFilesFn for updating the chart with name to version. If the version is
// empty it will fetch the latest version using helmer, using the vaultClient to get the repo creds or prompting using
// in, out and outErr
func CreateChartChangeFilesFn(name string, version string, kind string, pro *PullRequestOperation, helmer helm.Helmer,
	vaultClient secreturl.Client, handles util.IOFileHandles) ChangeFilesFn {
	return func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		if version == "" && kind == string(versionstream.KindChart) {
			parts := strings.Split(name, "/")
			searchName := name
			if len(parts) == 2 {
				prefixes, err := versionstream.GetRepositoryPrefixes(dir)
				if err != nil {
					return nil, errors.Wrapf(err, "getting repository prefixes")
				}
				prefix := parts[0]
				urls := prefixes.URLsForPrefix(prefix)
				if len(urls) > 0 {
					if len(urls) > 1 {
						log.Logger().Warnf("helm repo %s has more than one url %+v, using first declared (%s)", prefix, urls, urls[0])
					} else if len(urls) == 0 {
						log.Logger().Warnf("helm repo %s has more than no urls, not adding", prefix)
					}
					prefix, err = helm.AddHelmRepoIfMissing(urls[0], prefix, "", "", helmer, vaultClient, handles)
					if err != nil {
						return nil, errors.Wrapf(err, "adding repository %s with url %s", prefix, urls[0])
					}
				}
				searchName = fmt.Sprintf("%s/%s", prefix, parts[1])
			}
			c, err := helm.FindLatestChart(searchName, helmer)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to find latest chart version for %s", name)
			}
			version = c.ChartVersion
			log.Logger().Infof("found latest version %s for chart %s\n", util.ColorInfo(version), util.ColorInfo(name))
		}
		pro.Version = version
		if pro.SrcGitURL == "" {
			sv, err := versionstream.LoadStableVersion(dir, versionstream.VersionKind(kind), name)
			if err != nil {
				return nil, errors.Wrapf(err, "loading stable version")
			}
			pro.SrcGitURL = sv.GitURL
			if sv.Component != "" {
				pro.Component = sv.Component
			}
		}
		if pro.SrcGitURL == "" {
			err := helm.InspectChart(name, version, "", "", "", helmer, func(dir string) error {
				fileName, err := helm.FindChartFileName(dir)
				if err != nil {
					return errors.Wrapf(err, "find chart file")
				}
				chart, err := helm.LoadChartFile(fileName)
				if err != nil {
					return errors.Wrapf(err, "loading chart file")
				}
				if len(chart.Sources) > 0 {
					pro.SrcGitURL = chart.Sources[0]
				} else {
					return errors.Errorf("chart %s %s has no sources in Chart.yaml", name, version)
				}
				return nil
			})
			if err != nil {
				return nil, errors.Wrapf(err, "failed to find source repo for %s", name)
			}
		}
		if pro.SrcGitURL == "" {
			return nil, errors.Errorf("Unable to determine git url for dependency %s", name)
		}
		answer, err := versionstream.UpdateStableVersion(dir, kind, name, version)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return answer, nil
	}
}
