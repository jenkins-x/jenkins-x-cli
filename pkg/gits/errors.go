package gits

import "strings"

// IsEmptyCommitError checks if the error during git rebase is caused by the commit being empty at the end of the cherry-pick
func IsEmptyCommitError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, `If you wish to commit it anyway, use:

    git commit --allow-empty

Otherwise, please use 'git reset'`)
}

// IsRepositoryNotExportedError checks if the clone error happens because the repository is not exported
func IsRepositoryNotExportedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "repository not exported")
}
