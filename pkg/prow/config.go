package prow

// Owners keeps the prow OWNERS data
type Owners struct {
	Approvers []string `json:"approvers"`
	Reviewers []string `json:"reviewers"`
}

// OwnersAliases keept the prow OWNERS_ALIASES data
type OwnersAliases struct {
	Aliases       []string `json:"aliases"`
	BestApprovers []string `json:"best-approvers"`
	BestReviewers []string `json:"best-reviewers"`
}
