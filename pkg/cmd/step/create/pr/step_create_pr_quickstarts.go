package pr

import (
	"fmt"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits/operations"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/quickstarts"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

var (
	createPullRequestQuickStartsLong = templates.LongDesc(`
		Creates a Pull Request a version stream to include all the quickstarts found in a github organisation
`)

	createPullRequestQuickStartsExample = templates.Examples(`
					`)
)

// StepCreatePullRequestQuickStartsOptions contains the command line flags
type StepCreatePullRequestQuickStartsOptions struct {
	StepCreatePrOptions

	Location v1.QuickStartLocation
}

// NewCmdStepCreatePullRequestQuickStarts Creates a new Command object
func NewCmdStepCreatePullRequestQuickStarts(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestQuickStartsOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "quickstarts",
		Short:   "Creates a Pull Request on a version stream to include all the quickstarts found in a github organisation",
		Long:    createPullRequestQuickStartsLong,
		Example: createPullRequestQuickStartsExample,
		Aliases: []string{"quickstart", "qs"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Location.Owner, "owner", "n", "jenkins-x-quickstarts", "The name of the git owner (user or organisation) to query for quickstart git repositories")
	cmd.Flags().StringVarP(&options.Location.GitKind, "git-kind", "k", "github", "The kind of git provider")
	cmd.Flags().StringVarP(&options.Location.GitURL, "git-server", "", "https://github.com", "The git server to find the quickstarts")
	cmd.Flags().StringArrayVarP(&options.Location.Includes, "filter", "f", []string{"*"}, "The name patterns to include - such as '*' for all names")
	cmd.Flags().StringArrayVarP(&options.Location.Excludes, "excludes", "x", []string{"WIP-*"}, "The name patterns to exclude")
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	return cmd
}

// ValidateQuickStartsOptions validates the common options for quickStarts pr steps
func (o *StepCreatePullRequestQuickStartsOptions) ValidateQuickStartsOptions() error {
	if len(o.GitURLs) == 0 {
		// Default in the versions repo
		o.GitURLs = []string{config.DefaultVersionsURL}
	}
	if err := o.ValidateOptions(true); err != nil {
		return errors.WithStack(err)
	}
	return nil
}

// Run implements this command
func (o *StepCreatePullRequestQuickStartsOptions) Run() error {
	authConfig := auth.NewMemoryAuthConfigService()
	model, err := o.LoadQuickStartsFromLocations([]v1.QuickStartLocation{o.Location}, authConfig.Config())
	if err != nil {
		return fmt.Errorf("failed to load quickstarts: %s", err)
	}

	found := model.Quickstarts
	if len(found) == 0 {
		log.Logger().Warnf("did not find any quickstarts for the location %#v", o.Location)
		return nil
	}

	for _, q := range model.Quickstarts {
		o.SrcGitURL = o.sourceGitURL(q)
		break
	}

	if err := o.ValidateQuickStartsOptions(); err != nil {
		return errors.WithStack(err)
	}

	type Change struct {
		Pro        operations.PullRequestOperation
		QuickStart quickstarts.Quickstart
		ChangeFn   func(from *quickstarts.Quickstart, dir string, gitInfo *gits.GitRepository) ([]string, error)
	}

	modifyFns := []Change{}
	for _, name := range model.SortedNames() {
		q := model.Quickstarts[name]
		if q == nil {
			continue
		}
		version := q.Version
		o.SrcGitURL = o.sourceGitURL(q)

		pro := operations.PullRequestOperation{
			CommonOptions: o.CommonOptions,
			GitURLs:       o.GitURLs,
			SrcGitURL:     o.SrcGitURL,
			Base:          o.Base,
			BranchName:    o.BranchName,
			Version:       version,
			DryRun:        o.DryRun,
		}

		authorName, authorEmail, err := gits.EnsureUserAndEmailSetup(o.Git())
		if err != nil {
			pro.AuthorName = authorName
			pro.AuthorEmail = authorEmail
		}
		callback := func(from *quickstarts.Quickstart, dir string, gitInfo *gits.GitRepository) ([]string, error) {
			quickstarts, err := versionstream.GetQuickStarts(dir)
			if err != nil {
				return nil, errors.Wrapf(err, "loading quickstarts from version stream in dir %s", dir)
			}
			if quickstarts.DefaultOwner == "" {
				quickstarts.DefaultOwner = o.Location.Owner
			}
			o.upsertQuickStart(from, quickstarts)
			err = versionstream.SaveQuickStarts(dir, quickstarts)
			if err != nil {
				return nil, errors.Wrapf(err, "saving quickstarts to the version stream in dir %s", dir)
			}
			return nil, nil
		}

		modifyFns = append(modifyFns, Change{
			QuickStart: *q,
			ChangeFn:   callback,
			Pro:        pro,
		})
	}

	o.SrcGitURL = ""    // there is no src url for the overall PR
	o.SkipCommit = true // As we've done all the commits already
	return o.CreatePullRequest("versionstream", func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		for _, fn := range modifyFns {
			changeFn := fn.Pro.WrapChangeFilesWithCommitFn("quickstart", func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
				return fn.ChangeFn(&fn.QuickStart, dir, gitInfo)
			})
			_, err := changeFn(dir, gitInfo)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
		return nil, nil
	})
}

func (o *StepCreatePullRequestQuickStartsOptions) upsertQuickStart(from *quickstarts.Quickstart, qs *versionstream.QuickStarts) {
	upsert := func(from *quickstarts.Quickstart, to *versionstream.QuickStart) {
		if to.Name == "" {
			to.Name = from.Name
		}
		if from.Version != "" {
			to.Version = from.Version
		}
		if from.DownloadZipURL != "" {
			to.DownloadZipURL = from.DownloadZipURL
		}
		if from.Language != "" {
			to.Language = from.Language
		}
		if from.Framework != "" {
			to.Framework = from.Framework
		}
		if len(from.Tags) == 0 {
			to.Tags = from.Tags
		}
	}
	found := false
	for _, to := range qs.QuickStarts {
		if from.Name == to.Name {
			if from.Owner == to.Owner || (from.Owner == qs.DefaultOwner && to.Owner == "") {
				upsert(from, to)
				found = true
			}

		}
	}
	if !found {
		to := &versionstream.QuickStart{}
		upsert(from, to)
		qs.QuickStarts = append(qs.QuickStarts, to)

		// lets sort the quickstarts in name order
		qs.Sort()
	}
}

func (o *StepCreatePullRequestQuickStartsOptions) sourceGitURL(qs *quickstarts.Quickstart) string {
	owner := qs.Owner
	if owner == "" {
		owner = o.Location.Owner
	}
	return util.UrlJoin(o.Location.GitURL, owner, qs.Name) + ".git"
}
