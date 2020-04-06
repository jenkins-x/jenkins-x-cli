package issues

const (
	Bugzilla = "bugzilla"
	Jira     = "jira"
	Trello   = "trello"
	Git      = "git"
)

var (
	IssueOpen   = "open"
	IssueClosed = "closed"
)

var (
	IssueTrackerKinds = []string{Bugzilla, Jira, Trello}
)
