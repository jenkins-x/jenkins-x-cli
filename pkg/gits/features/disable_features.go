package features

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// DisableFeaturesForOrg iterates over all the repositories in org (except those that match excludes) and disables issue
// trackers, projects and wikis if they are not in use.
//
// Issue trackers are not in use if they have no open or closed issues
// Projects are not in use if there are no open projects
// Wikis are not in use if the provider returns that the wiki is not enabled
//
// Note that the requirement for issues is no issues at all so that we don't close issue trackers that have historic info
//
// If includes is not empty only those that match an include will be operated on. If dryRun is true, the operations to
// be done will printed and but nothing done. If batchMode is false, then each change will be prompted.
func DisableFeaturesForOrg(org string, includes []string, excludes []string, dryRun bool, batchMode bool, provider gits.GitProvider, handles util.IOFileHandles) error {

	type closeTypes struct {
		*gits.GitRepository
		closeIssues  bool
		closeProject bool
		closeWiki    bool
		keepIssues   bool
		keepProject  bool
		keepWiki     bool
	}

	repos, err := provider.ListRepositories(org)
	if err != nil {
		return errors.Wrapf(err, "listing repositories in %s", org)
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})
	// dedupe
	dedupedRepos := make([]*gits.GitRepository, 0)
	previous := ""
	for _, r := range repos {
		if r.Name != previous {
			dedupedRepos = append(dedupedRepos, r)
		}
		previous = r.Name
	}
	ct := make([]closeTypes, 0)

	log.Logger().Infof("Analysing repositories\n")

	for _, repo := range dedupedRepos {
		c := closeTypes{
			GitRepository: repo,
			closeIssues:   false,
			closeProject:  false,
			closeWiki:     false,
		}
		if c.Archived {
			// Ignore archived repos
			continue
		}
		if !util.Contains(excludes, fmt.Sprintf("%s/%s", repo.Organisation, repo.Name)) && (len(includes) == 0 || util.Contains(includes, fmt.Sprintf("%s/%s", repo.Organisation, repo.Name))) {
			issues := ""
			if repo.HasIssues {
				openIssues, err := provider.SearchIssues(repo.Organisation, repo.Name, "open")
				if err != nil {
					return errors.Wrapf(err, "finding open issues in %s/%s", repo.Organisation, repo.Name)
				}
				closedIssues, err := provider.SearchIssues(repo.Organisation, repo.Name, "closed")
				if err != nil {
					return errors.Wrapf(err, "finding open issues in %s/%s", repo.Organisation, repo.Name)
				}
				open := len(openIssues)
				all := len(openIssues) + len(closedIssues)
				stat := fmt.Sprintf("%d/%d", open, all)
				if open > 0 {
					stat = util.ColorInfo(stat)
				}
				if all == 0 {
					c.closeIssues = true
				} else {
					c.keepIssues = true
				}
				issues = fmt.Sprintf("%s issues are open", stat)
			} else {
				issues = "Disabled"
			}
			projects := ""
			if repo.HasProjects {
				allProjects, err := provider.GetProjects(repo.Organisation, repo.Name)
				if err != nil {
					return errors.Wrapf(err, "getting projects for %s/%s", repo.Organisation, repo.Name)
				}
				open := 0
				for _, p := range allProjects {
					if p.State == gits.ProjectOpen {
						open++
					}
				}
				stat := fmt.Sprintf("%d/%d", open, len(allProjects))
				if open > 0 {
					stat = util.ColorInfo(stat)
					c.keepProject = true
				} else {
					c.closeProject = true
				}
				projects = fmt.Sprintf("%s open projects", stat)
			} else {
				projects = "Disabled"
			}
			wikis := ""
			if repo.HasWiki {
				enabled, err := provider.IsWikiEnabled(repo.Organisation, repo.Name)
				if err != nil {
					return errors.Wrapf(err, "checking if wiki for %s/%s is enabled", repo.Organisation, repo.Name)
				}
				if enabled {
					wikis = util.ColorInfo("In use")
					c.keepWiki = true
				} else {
					wikis = "Not in use"
					c.closeWiki = true
				}
			} else {
				wikis = "Disabled"
			}
			log.Logger().Infof(`
%s
Issues: %s
Projects: %s
Wiki Pages: %s
`, util.ColorBold(fmt.Sprintf("%s/%s", repo.Organisation, repo.Name)), issues, projects, wikis)
			ct = append(ct, c)
		}
	}
	log.Logger().Infof("\n\n Analysis complete\n")
	for _, c := range ct {
		toClose := make([]string, 0)
		toKeep := make([]string, 0)
		var wiki, issues, project *bool
		disabled := false
		if c.closeProject {
			toClose = append(toClose, util.ColorWarning("project"))
			project = &disabled
		}
		if c.keepProject {
			toKeep = append(toKeep, util.ColorInfo("project"))
		}
		if c.closeWiki {
			toClose = append(toClose, util.ColorWarning("wiki"))
			wiki = &disabled
		}
		if c.keepWiki {
			toKeep = append(toKeep, util.ColorInfo("wiki"))
		}
		if c.closeIssues {
			toClose = append(toClose, util.ColorWarning("issues"))
			issues = &disabled
		}
		if c.keepIssues {
			toKeep = append(toKeep, util.ColorInfo("issues"))
		}
		if len(toClose) > 0 || len(toKeep) > 0 {
			if dryRun {
				log.Logger().Infof("Would disable %s on %s", strings.Join(toClose, ", "), util.ColorInfo(fmt.Sprintf("%s/%s", c.Organisation, c.Name)))
			} else {

				if !batchMode {
					if answer, err := util.Confirm(fmt.Sprintf("Are you sure you want to disable %s on %s", strings.Join(toClose, ","), util.ColorInfo(fmt.Sprintf("%s/%s", c.Organisation, c.Name))), true, "", handles); err != nil {
						return err
					} else if !answer {
						continue
					}
				}
				logStr := ""
				if len(toClose) > 0 {
					logStr = fmt.Sprintf("Disabling %s", strings.Join(toClose, ", "))
				}
				if len(toKeep) > 0 {
					if len(logStr) > 0 {
						logStr += "; "
					}
					logStr += fmt.Sprintf("Keeping %s", strings.Join(toKeep, ", "))
				}
				if len(logStr) > 0 {
					log.Logger().Infof("%s: %s", util.ColorInfo(fmt.Sprintf("%s/%s", c.Organisation, c.Name)), logStr)
				}
				_, err = provider.ConfigureFeatures(c.Organisation, c.Name, issues, project, wiki)
				if err != nil {
					return errors.Wrapf(err, "disabling %s on %s/%s", strings.Join(toClose, ", "), c.Organisation, c.Name)
				}
			}
		}
	}
	return nil
}
