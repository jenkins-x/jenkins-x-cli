package metapipeline

import "fmt"

// PullRef defines all required information for checking out the source in the required state for the pipeline execution
type PullRef struct {
	sourceURL    string
	baseBranch   string
	baseSHA      string
	pullRequests []PullRequestRef
}

// PullRequestRef defines  a pull request which needs to be merged.
type PullRequestRef struct {
	ID       string
	MergeSHA string
}

// NewPullRef creates a pull ref for a master/feature build with no pull requests.
func NewPullRef(sourceURL string, baseBranch string, baseSHA string) PullRef {
	return PullRef{
		sourceURL:  sourceURL,
		baseBranch: baseBranch,
		baseSHA:    baseSHA,
	}
}

// NewPullRefWithPullRequest creates a pull ref for a pull request.
func NewPullRefWithPullRequest(sourceURL string, baseBranch string, baseSHA string, prRef ...PullRequestRef) PullRef {
	return PullRef{
		sourceURL:    sourceURL,
		baseBranch:   baseBranch,
		baseSHA:      baseSHA,
		pullRequests: prRef,
	}
}

// SourceURL returns the source URL of this pull ref
func (p *PullRef) SourceURL() string {
	return p.sourceURL
}

// BaseBranch returns the base branch of this pull ref.
func (p *PullRef) BaseBranch() string {
	return p.baseBranch
}

// BaseSHA returns the base sha of this pull ref.
func (p *PullRef) BaseSHA() string {
	return p.baseSHA
}

// PullRequests returns the pull request for this pull ref.
func (p *PullRef) PullRequests() []PullRequestRef {
	return p.pullRequests
}

// String returns a string representation of this pull ref in Prow PullRef format.
func (p *PullRef) String() string {
	s := fmt.Sprintf("%s:%s", p.baseBranch, p.baseSHA)
	for _, pr := range p.pullRequests {
		s = fmt.Sprintf("%s,%s:%s", s, pr.ID, pr.MergeSHA)
	}
	return s
}
