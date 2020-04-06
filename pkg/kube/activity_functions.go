package kube

import (
	"time"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func StartPromote(p *v1.PromoteActivityStep) error {
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if p.Status == v1.ActivityStatusTypeNone {
		p.Status = v1.ActivityStatusTypeRunning
	}
	return nil
}

func CompletePromote(p *v1.PromoteActivityStep) error {
	StartPromote(p)
	if p.CompletedTimestamp == nil {
		p.CompletedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	p.Status = v1.ActivityStatusTypeSucceeded
	return nil
}

func FailedPromote(p *v1.PromoteActivityStep) error {
	StartPromote(p)
	if p.CompletedTimestamp == nil {
		p.CompletedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	p.Status = v1.ActivityStatusTypeFailed
	return nil
}

func StartPromotionPullRequest(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
	StartPromote(ps)
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if a.Spec.WorkflowStatus != v1.ActivityStatusTypeRunning {
		a.Spec.WorkflowStatus = v1.ActivityStatusTypeRunning
	}
	if a.Spec.Status != v1.ActivityStatusTypeRunning {
		a.Spec.Status = v1.ActivityStatusTypeRunning
	}
	if p.Status != v1.ActivityStatusTypeRunning {
		p.Status = v1.ActivityStatusTypeRunning
	}
	return nil
}

func StartPromotionUpdate(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
	StartPromote(ps)
	pullRequest := ps.PullRequest
	if pullRequest != nil {
		CompletePromotionPullRequest(a, s, ps, pullRequest)
	}
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if a.Spec.WorkflowStatus != v1.ActivityStatusTypeRunning {
		a.Spec.WorkflowStatus = v1.ActivityStatusTypeRunning
	}
	if a.Spec.Status != v1.ActivityStatusTypeRunning {
		a.Spec.Status = v1.ActivityStatusTypeRunning
	}
	if p.Status != v1.ActivityStatusTypeRunning {
		p.Status = v1.ActivityStatusTypeRunning
	}
	return nil
}

func CompletePromotionPullRequest(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if p.CompletedTimestamp == nil {
		p.CompletedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	p.Status = v1.ActivityStatusTypeSucceeded
	return nil
}

func FailedPromotionPullRequest(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromotePullRequestStep) error {
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if p.CompletedTimestamp == nil {
		p.CompletedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	p.Status = v1.ActivityStatusTypeFailed
	return nil
}

func CompletePromotionUpdate(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
	CompletePromote(ps)
	pullRequest := ps.PullRequest
	if pullRequest != nil {
		CompletePromotionPullRequest(a, s, ps, pullRequest)
	}
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if p.CompletedTimestamp == nil {
		p.CompletedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	p.Status = v1.ActivityStatusTypeSucceeded
	return nil
}

func FailedPromotionUpdate(a *v1.PipelineActivity, s *v1.PipelineActivityStep, ps *v1.PromoteActivityStep, p *v1.PromoteUpdateStep) error {
	FailedPromote(ps)
	pullRequest := ps.PullRequest
	if pullRequest != nil {
		CompletePromotionPullRequest(a, s, ps, pullRequest)
	}
	if p.StartedTimestamp == nil {
		p.StartedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if p.CompletedTimestamp == nil {
		p.CompletedTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	p.Status = v1.ActivityStatusTypeFailed
	return nil
}
