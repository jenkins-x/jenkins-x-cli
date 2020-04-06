package extensions

import (
	"fmt"
	"strconv"
	"strings"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
)

func NotifyCommitStatus(commitRef jenkinsv1.CommitStatusCommitReference, state string, targetUrl string, description string, comment string, context string, gitProvider gits.GitProvider, gitRepoInfo *gits.GitRepository) (status *gits.GitRepoStatus, err error) {

	if commitRef.SHA == "" {
		return &gits.GitRepoStatus{}, fmt.Errorf("SHA cannot be empty on %v", commitRef)
	}
	if err != nil {
		return &gits.GitRepoStatus{}, err
	}

	status = &gits.GitRepoStatus{
		Description: description,
		State:       state,
		TargetURL:   targetUrl,
		Context:     context,
	}

	oldStatuses, err := gitProvider.ListCommitStatus(gitRepoInfo.Organisation, gitRepoInfo.Name, commitRef.SHA)
	if err != nil {
		return &gits.GitRepoStatus{}, err
	}
	oldStatus := &gits.GitRepoStatus{}
	for _, o := range oldStatuses {
		if o.Context == context {
			oldStatus = o
			// List is sorted in reverse chronological order - most recent statuses first
			break
		}
	}
	if oldStatus.ID != "" {
		status.ID = oldStatus.ID
	}
	// check for for forbidden status transitions
	if strings.HasPrefix(strings.ToLower(oldStatus.Description), strings.ToLower("Overridden")) {
		// If the status has been overridden, then we should not automatically update it again
		log.Logger().Infof("commit status is overridden for pull request %s (%s) on %s so not updating", commitRef.PullRequest, commitRef.SHA, commitRef.GitURL)
		return oldStatus, nil
	}
	if oldStatus.Description != status.Description && oldStatus.State != status.State {

		log.Logger().Infof("Status %s for commit status for pull request %s (%s) on %s", state, commitRef.PullRequest, commitRef.SHA, commitRef.GitURL)
		_, err = gitProvider.UpdateCommitStatus(gitRepoInfo.Organisation, gitRepoInfo.Name, commitRef.SHA, status)
		if err != nil {
			return &gits.GitRepoStatus{}, err
		}
		if comment != "" {
			prn, err := strconv.Atoi(strings.TrimPrefix(commitRef.PullRequest, "PR-"))
			if err != nil {
				return &gits.GitRepoStatus{}, err
			}
			pr, err := gitProvider.GetPullRequest(gitRepoInfo.Organisation, gitRepoInfo, prn)
			if err != nil {
				return &gits.GitRepoStatus{}, err
			}
			err = gitProvider.AddPRComment(pr, comment)
			if err != nil {
				return &gits.GitRepoStatus{}, err
			}
		}
	}
	return status, nil
}
