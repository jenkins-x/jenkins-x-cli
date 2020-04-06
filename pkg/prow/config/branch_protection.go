package config

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"k8s.io/test-infra/prow/config"
)

// AddRepoToBranchProtection adds a repository to the Branch Protection section of a prow config
func AddRepoToBranchProtection(bp *config.BranchProtection, repoSpec string, context string, kind Kind) error {
	bp.ProtectTested = true
	if bp.Orgs == nil {
		bp.Orgs = make(map[string]config.Org, 0)
	}
	url, err := gits.ParseGitURL(repoSpec)
	if err != nil {
		return err
	}
	requiredOrg, requiredRepo := url.Organisation, url.Name
	if _, ok := bp.Orgs[requiredOrg]; !ok {
		bp.Orgs[requiredOrg] = config.Org{}
	}
	if bp.Orgs[requiredOrg].Repos == nil {
		org := bp.Orgs[requiredOrg]
		org.Repos = make(map[string]config.Repo, 0)
		bp.Orgs[requiredOrg] = org
	}
	if _, ok := bp.Orgs[requiredOrg].Repos[requiredRepo]; !ok {
		bp.Orgs[requiredOrg].Repos[requiredRepo] = config.Repo{
			Policy: config.Policy{
				RequiredStatusChecks: &config.ContextPolicy{},
			},
		}

	}
	if bp.Orgs[requiredOrg].Repos[requiredRepo].Policy.RequiredStatusChecks.Contexts == nil {
		bp.Orgs[requiredOrg].Repos[requiredRepo].Policy.RequiredStatusChecks.Contexts = make([]string, 0)
	}
	contexts := bp.Orgs[requiredOrg].Repos[requiredRepo].Policy.RequiredStatusChecks.Contexts
	switch kind {
	case Application:
		if !util.Contains(contexts, ServerlessJenkins) {
			contexts = append(contexts, ServerlessJenkins)
		}
	case Environment, RemoteEnvironment:
		if !util.Contains(contexts, PromotionBuild) {
			contexts = append(contexts, PromotionBuild)
		}
	case Protection:
		if !util.Contains(contexts, context) {
			contexts = append(contexts, context)
		}
	default:
		return fmt.Errorf("unknown Prow config kind %s", kind)
	}
	bp.Orgs[requiredOrg].Repos[requiredRepo].Policy.RequiredStatusChecks.Contexts = contexts
	return nil
}

// RemoveRepoFromBranchProtection removes a repository to the Branch Protection section of a prow config
func RemoveRepoFromBranchProtection(bp *config.BranchProtection, repoSpec string) error {
	if bp.Orgs == nil {
		return errors.New("no orgs in BranchProtection object")
	}
	url, err := gits.ParseGitURL(repoSpec)
	if err != nil {
		return err
	}
	requiredOrg, requiredRepo := url.Organisation, url.Name

	repos := bp.Orgs[requiredOrg].Repos
	if repos == nil {
		return errors.New("no repos found for org " + requiredOrg)
	}
	if _, ok := repos[requiredRepo]; !ok {
		return errors.New(fmt.Sprintf("repo %s not found in org %s", requiredRepo, requiredOrg))
	}
	delete(repos, requiredRepo)
	return nil
}

// GetAllBranchProtectionContexts gets all the contexts that have branch protection for a repo
func GetAllBranchProtectionContexts(org string, repo string, prowConfig *config.Config) ([]string, error) {
	prowOrg, ok := prowConfig.BranchProtection.Orgs[org]
	if !ok {
		prowOrg = config.Org{}
	}
	if prowOrg.Repos == nil {
		prowOrg.Repos = make(map[string]config.Repo, 0)
	}
	prowRepo, ok := prowOrg.Repos[repo]
	if !ok {
		prowRepo = config.Repo{}
	}
	if prowRepo.RequiredStatusChecks == nil {
		prowRepo.RequiredStatusChecks = &config.ContextPolicy{}
	}
	return prowRepo.RequiredStatusChecks.Contexts, nil
}

// GetBranchProtectionContexts gets the branch protection contexts for a repo
func GetBranchProtectionContexts(org string, repo string, prowConfig *config.Config) ([]string, error) {
	result := make([]string, 0)
	contexts, err := GetAllBranchProtectionContexts(org, repo, prowConfig)
	if err != nil {
		return result, errors.Wrap(err, "getting branch protection contexts")
	}
	for _, c := range contexts {
		if c != ServerlessJenkins && c != PromotionBuild {
			result = append(result, c)
		}
	}
	return result, nil
}
