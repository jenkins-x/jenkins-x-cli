// +build unit

package features

import (
	"testing"
	"time"

	"github.com/jenkins-x/jx/pkg/issues"
	"github.com/jenkins-x/jx/pkg/tests"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/stretchr/testify/assert"
)

func TestDisableIssuesForOrg(t *testing.T) {
	type repo struct {
		gits.GitRepository
		issues      map[int]*gits.FakeIssue
		projects    []gits.GitProject
		wikiEnabled bool
	}
	type args struct {
		repos    []repo
		dryRun   bool
		includes []string
		excludes []string
	}

	ts := []struct {
		name        string
		args        args
		wantErr     bool
		wantIssues  bool
		wantProject bool
		wantWiki    bool
	}{
		{
			name: "closeIssues",
			args: args{
				repos: []repo{
					{
						GitRepository: gits.GitRepository{
							Name:         "roadrunner",
							Organisation: "acme",
							HasIssues:    true,
							HasWiki:      false,
							HasProjects:  false,
						},
					},
				},
				dryRun:   false,
				includes: nil,
				excludes: nil,
			},
			wantErr:     false,
			wantIssues:  false,
			wantProject: false,
			wantWiki:    false,
		},
		{
			name: "closeProjects",
			args: args{
				repos: []repo{
					{
						GitRepository: gits.GitRepository{
							Name:         "roadrunner",
							Organisation: "acme",
							HasIssues:    false,
							HasWiki:      false,
							HasProjects:  true,
						},
					},
				},
				dryRun:   false,
				includes: nil,
				excludes: nil,
			},
			wantErr:     false,
			wantIssues:  false,
			wantProject: false,
			wantWiki:    false,
		},
		{
			name: "closeWiki",
			args: args{
				repos: []repo{
					{
						GitRepository: gits.GitRepository{

							Name:         "roadrunner",
							Organisation: "acme",
							HasIssues:    false,
							HasWiki:      true,
							HasProjects:  false,
						},
					},
				},
				dryRun:   false,
				includes: nil,
				excludes: nil,
			},
			wantErr:     false,
			wantIssues:  false,
			wantProject: false,
			wantWiki:    false,
		},
		{
			name: "keepIssuesOpen",
			args: args{
				repos: []repo{
					{
						GitRepository: gits.GitRepository{
							Name:         "roadrunner",
							Organisation: "acme",
							HasIssues:    true,
							HasWiki:      false,
							HasProjects:  false,
						},
						issues: map[int]*gits.FakeIssue{
							1: {
								Issue: &gits.GitIssue{
									Title: "First issue",
									State: &issues.IssueOpen,
								},
							},
							2: {
								Issue: &gits.GitIssue{
									Title: "Second issue",
									State: &issues.IssueClosed,
								},
							},
						},
					},
				},
				dryRun:   false,
				includes: nil,
				excludes: nil,
			},
			wantErr:     false,
			wantIssues:  true,
			wantProject: false,
			wantWiki:    false,
		},
		{
			name: "keepProjectsOpen",
			args: args{
				repos: []repo{
					{
						GitRepository: gits.GitRepository{
							Name:         "roadrunner",
							Organisation: "acme",
							HasIssues:    false,
							HasWiki:      false,
							HasProjects:  true,
						},
						projects: []gits.GitProject{
							{
								Name:        "Project 1",
								Description: "",
								Number:      1,
								State:       "open",
							},
						},
					},
				},
				dryRun:   false,
				includes: nil,
				excludes: nil,
			},
			wantErr:     false,
			wantIssues:  false,
			wantProject: true,
			wantWiki:    false,
		},
		{
			name: "keepWikiOpen",
			args: args{
				repos: []repo{
					{
						GitRepository: gits.GitRepository{
							Name:         "roadrunner",
							Organisation: "acme",
							HasIssues:    false,
							HasWiki:      true,
							HasProjects:  false,
						},
						wikiEnabled: true,
					},
				},
				dryRun:   false,
				includes: nil,
				excludes: nil,
			},
			wantErr:     false,
			wantIssues:  false,
			wantProject: false,
			wantWiki:    true,
		},
	}
	for _, tt := range ts {
		t.Run(tt.name, func(t *testing.T) {
			tests.Retry(t, 5, 5*time.Second, func(r *tests.R) {
				fakeRepos := make([]*gits.FakeRepository, 0)
				for _, r := range tt.args.repos {
					fr, err := gits.NewFakeRepository(r.Organisation, r.Name, nil, nil)
					assert.NoError(t, err)
					fr.GitRepo.HasIssues = r.HasIssues
					fr.GitRepo.HasWiki = r.HasWiki
					fr.GitRepo.HasProjects = r.HasProjects
					fr.Issues = r.issues
					fr.Projects = r.projects
					fr.WikiEnabled = r.wikiEnabled
					fakeRepos = append(fakeRepos, fr)
				}
				provider := gits.NewFakeProvider(fakeRepos...)

				terminal := tests.NewTerminal(t, nil)

				handles := util.IOFileHandles{
					Err: terminal.Err,
					In:  terminal.In,
					Out: terminal.Out,
				}
				err := DisableFeaturesForOrg("acme", tt.args.includes, tt.args.excludes, tt.args.dryRun, true, provider, handles)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				if err != nil {
					return
				}
				for _, r := range tt.args.repos {
					repo, err := provider.GetRepository(r.Organisation, r.Name)
					assert.NoError(t, err)
					if tt.wantIssues {
						assert.True(t, repo.HasIssues)
					}
					if tt.wantProject {
						assert.True(t, repo.HasProjects)
					}
					if tt.wantWiki {
						assert.True(t, repo.HasWiki)
					}
				}

			})
		})
	}
}
