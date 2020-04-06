package pr

import (
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/config"

	"github.com/jenkins-x/jx/pkg/helm"

	"github.com/jenkins-x/jx/pkg/gits"

	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/gits/operations"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cloud/gke"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	createVersionPullRequestLong = templates.LongDesc(`
		Creates a Pull Request on the versions git repository for a new versionstream of a chart/package
`)

	createVersionPullRequestExample = templates.Examples(`
		# create a Pull Request to update a chart versionstream
		jx step create pr versions -n jenkins-x/prow -v 1.2.3

		# create a Pull Request to update a chart versionstream to the latest found in the helm repo
		jx step create pr versions -n jenkins-x/prow 

		# create a Pull Request to update all charts matching a filter to the latest found in the helm repo
		jx step create pr versions pr -f "*"

		# create a Pull Request to update all charts in the 'jenkins-x' chart repository to the latest found in the helm repo
		jx step create pr versions -f "jenkins-x/*"

		# create a Pull Request to update all charts in the 'jenkins-x' chart repository and update the BDD test images
		jx step create pr versions -f "jenkins-x/*" --images

			`)
)

// StepCreatePullRequestVersionsOptions contains the command line flags
type StepCreatePullRequestVersionsOptions struct {
	StepCreatePrOptions

	Kinds              []string
	Name               string
	Includes           []string
	Excludes           []string
	UpdateTektonImages bool
}

// NewCmdStepCreatePullRequestVersion Creates a new Command object
func NewCmdStepCreatePullRequestVersion(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreatePullRequestVersionsOptions{
		StepCreatePrOptions: StepCreatePrOptions{
			StepCreateOptions: step.StepCreateOptions{
				StepOptions: step.StepOptions{
					CommonOptions: commonOpts,
				},
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "versions",
		Short:   "Creates a Pull Request on the versions git repository for a new versionstream of a chart/package",
		Long:    createVersionPullRequestLong,
		Example: createVersionPullRequestExample,
		Aliases: []string{"versionstream pullrequest", "versionstream"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringArrayVarP(&options.Kinds, "kind", "k", []string{"charts", "git"}, "The kinds of versionstream. Possible values: "+strings.Join(versionstream.KindStrings, ", ")+".")
	cmd.Flags().StringVarP(&options.Name, "name", "n", "", "The name of the versionstream to update. e.g. the name of the chart like 'jenkins-x/prow'")
	cmd.Flags().StringArrayVarP(&options.Includes, "filter", "f", nil, "The name patterns to include - such as '*' for all names")
	cmd.Flags().StringArrayVarP(&options.Excludes, "excludes", "x", nil, "The name patterns to exclude")
	cmd.Flags().BoolVarP(&options.UpdateTektonImages, "images", "", false, "Update the tekton builder images for the Jenkins X Versions BDD tests")
	AddStepCreatePrFlags(cmd, &options.StepCreatePrOptions)
	return cmd
}

// ValidateVersionsOptions validates the common options for versionstream pr steps
func (o *StepCreatePullRequestVersionsOptions) ValidateVersionsOptions() error {
	if len(o.GitURLs) == 0 {
		// Default in the versions repo
		o.GitURLs = []string{config.DefaultVersionsURL}
	}
	if len(o.Kinds) == 0 {
		return util.MissingOption("kind")
	}
	for _, kind := range o.Kinds {
		if util.StringArrayIndex(versionstream.KindStrings, kind) < 0 {
			return util.InvalidOption("kind", kind, versionstream.KindStrings)
		}
	}

	if o.UpdateTektonImages && o.Name == "" && len(o.Includes) == 0 && util.StringArraysEqual(o.Kinds, []string{"charts", "git"}) {
		// Allow for just running jx step create pr versions --images
		o.Kinds = make([]string, 0)
	} else if len(o.Includes) == 0 && !o.UpdateTektonImages {
		if o.Name == "" && !o.UpdateTektonImages {
			return util.MissingOption("name")
		}
	}
	return nil
}

// Run implements this command
func (o *StepCreatePullRequestVersionsOptions) Run() error {
	if err := o.ValidateVersionsOptions(); err != nil {
		return errors.WithStack(err)
	}

	modifyFns := make([]operations.ChangeFilesFn, 0)

	if o.UpdateTektonImages {

		builderImageVersion, err := findLatestBuilderImageVersion()
		if err != nil {
			return errors.WithStack(err)
		}

		log.Logger().Infof("the latest builder image version is %s\n", util.ColorInfo(builderImageVersion))
		pro := operations.PullRequestOperation{
			CommonOptions: o.CommonOptions,
			GitURLs:       o.GitURLs,
			SrcGitURL:     "https://github.com/jenkins-x/jenkins-x-builders.git",
			Base:          o.Base,
			BranchName:    o.BranchName,
			Version:       builderImageVersion,
			DryRun:        o.DryRun,
		}
		authorName, authorEmail, _ := gits.EnsureUserAndEmailSetup(o.Git())
		if authorName != "" && authorEmail != "" {
			pro.AuthorName = authorName
			pro.AuthorEmail = authorEmail
		}
		fn, err := operations.CreatePullRequestRegexFn(builderImageVersion, "gcr.io/jenkinsxio/builder-(?:maven|go|terraform|go-nodejs):(?P<versions>.+)", "jenkins-x*.yml")
		if err != nil {
			return errors.WithStack(err)
		}
		modifyFns = append(modifyFns, pro.WrapChangeFilesWithCommitFn("versions", fn))
		modifyFns = append(modifyFns, pro.WrapChangeFilesWithCommitFn("versions", operations.CreatePullRequestBuildersFn(builderImageVersion)))
		// Update the pipeline files
		fn, err = operations.CreatePullRequestRegexFn(builderImageVersion, `(?m)^\s*agent:\n\s*image: gcr.io/jenkinsxio/builder-.*:(?P<version>.*)$`, "jenkins-x-*.yml")
		if err != nil {
			return errors.WithStack(err)
		}
		modifyFns = append(modifyFns, pro.WrapChangeFilesWithCommitFn("versions", fn))

		// Machine learning builders have to be handled separately
		mlBuilderImageVersion, err := findLatestMLBuilderImageVersion()
		if err != nil {
			return errors.WithStack(err)
		}

		log.Logger().Infof("the latest machine learning builder image version is %s\n", util.ColorInfo(mlBuilderImageVersion))
		mlPro := operations.PullRequestOperation{
			CommonOptions: o.CommonOptions,
			GitURLs:       o.GitURLs,
			SrcGitURL:     "https://github.com/jenkins-x/jenkins-x-builders-ml.git",
			Base:          o.Base,
			BranchName:    o.BranchName,
			Version:       mlBuilderImageVersion,
			DryRun:        o.DryRun,
		}
		if authorName != "" && authorEmail != "" {
			pro.AuthorName = authorName
			pro.AuthorEmail = authorEmail
		}
		modifyFns = append(modifyFns, mlPro.WrapChangeFilesWithCommitFn("versions", operations.CreatePullRequestMLBuildersFn(mlBuilderImageVersion)))
	}
	if len(o.Includes) > 0 {
		for _, kind := range o.Kinds {
			modifyFns = append(modifyFns, o.CreatePullRequestUpdateVersionFilesFn(o.Includes, o.Excludes, kind, o.Helm()))
		}
	} else {
		pro := operations.PullRequestOperation{
			CommonOptions: o.CommonOptions,
			GitURLs:       o.GitURLs,
			Base:          o.Base,
			BranchName:    o.BranchName,
			Version:       o.Version,
			DryRun:        o.DryRun,
		}
		authorName, authorEmail, _ := gits.EnsureUserAndEmailSetup(o.Git())
		if authorName != "" && authorEmail != "" {
			pro.AuthorName = authorName
			pro.AuthorEmail = authorEmail
		}
		vaultClient, err := o.SystemVaultClient("")
		if err != nil {
			vaultClient = nil
		}
		for _, kind := range o.Kinds {
			switch kind {
			case string(versionstream.KindChart):
				modifyFns = append(modifyFns, pro.WrapChangeFilesWithCommitFn("versions", operations.CreateChartChangeFilesFn(o.Name, o.Version, kind, &pro, o.Helm(), vaultClient, o.GetIOFileHandles())))
			}

		}
	}

	o.SrcGitURL = ""    // there is no src url for the overall PR
	o.SkipCommit = true // As we've done all the commits already
	return o.CreatePullRequest("versionstream", func(dir string, gitInfo *gits.GitRepository) ([]string, error) {
		for _, kind := range o.Kinds {
			if versionstream.VersionKind(kind) == versionstream.KindChart {
				_, err := o.HelmInitDependency(dir, o.DefaultReleaseCharts())
				if err != nil {
					return nil, errors.Wrap(err, "failed to ensure the helm repositories were setup")
				}
				log.Logger().Info("updating helm repositories to find the latest chart versions...\n")
				err = o.Helm().UpdateRepo()
				if err != nil {
					return nil, errors.Wrap(err, "failed to update helm repos")
				}
			}
		}
		for _, fn := range modifyFns {
			// in a versions PR there is no overall "old version" - it's done commit by commit
			_, err := fn(dir, gitInfo)
			if err != nil {
				return nil, errors.WithStack(err)
			}
		}
		return nil, nil
	})
}

func findLatestBuilderImageVersion() (string, error) {
	return findLatestImageVersion("gcr.io/jenkinsxio/builder-maven")
}

func findLatestMLBuilderImageVersion() (string, error) {
	return findLatestImageVersion("gcr.io/jenkinsxio/builder-machine-learning")
}

func findLatestImageVersion(image string) (string, error) {
	cmd := util.Command{
		Name: "gcloud",
		Args: []string{
			"container", "images", "list-tags", image, "--format", "json",
		},
	}
	output, err := cmd.RunWithoutRetry()
	if err != nil {
		return "", errors.WithStack(err)
	}
	return gke.FindLatestImageTag(output)
}

//CreatePullRequestUpdateVersionFilesFn creates the ChangeFilesFn for directory tree of stable version files, applying the includes and excludes
func (o *StepCreatePullRequestVersionsOptions) CreatePullRequestUpdateVersionFilesFn(includes []string, excludes []string, kindStr string, helmer helm.Helmer) operations.ChangeFilesFn {

	return func(dir string, gitInfo *gits.GitRepository) (i []string, e error) {
		answer := make([]string, 0)

		kindDir := filepath.Join(dir, kindStr)
		glob := filepath.Join(kindDir, "*", "*.yml")
		paths, err := filepath.Glob(glob)
		if err != nil {
			return nil, errors.Wrapf(err, "bad glob pattern %s", glob)
		}
		glob = filepath.Join(kindDir, "*", "*", "*.yml")
		morePaths, err := filepath.Glob(glob)
		if err != nil {
			return nil, errors.Wrapf(err, "bad glob pattern %s", glob)
		}
		paths = append(paths, morePaths...)
		for _, path := range paths {
			name, err := versionstream.NameFromPath(kindDir, path)
			if err != nil {
				return nil, errors.WithStack(err)
			}
			if !util.StringMatchesAny(name, includes, excludes) {
				continue
			} else {
				pro := operations.PullRequestOperation{
					CommonOptions: o.CommonOptions,
					GitURLs:       o.GitURLs,
					Base:          o.Base,
					BranchName:    o.BranchName,
					DryRun:        o.DryRun,
				}
				authorName, authorEmail, err := gits.EnsureUserAndEmailSetup(o.Git())
				if err != nil {
					pro.AuthorName = authorName
					pro.AuthorEmail = authorEmail
				}
				var cff operations.ChangeFilesFn
				switch kindStr {
				case string(versionstream.KindChart):

					vaultClient, err := o.SystemVaultClient("")
					if err != nil {
						vaultClient = nil
					}
					cff = pro.WrapChangeFilesWithCommitFn(kindStr, operations.CreateChartChangeFilesFn(name, "", kindStr, &pro, o.Helm(), vaultClient, o.GetIOFileHandles()))
				case string(versionstream.KindGit):
					cff = pro.WrapChangeFilesWithCommitFn(kindStr, pro.CreatePullRequestGitReleasesFn(name))
				}
				a, err := cff(dir, gitInfo)
				if err != nil {
					if isFailedToFindLatest(err) {
						log.Logger().Warnf("Failed to find latest for %s", util.ColorInfo(name))
					} else {
						return nil, errors.WithStack(err)
					}
				}
				answer = append(answer, a...)
			}
		}
		return answer, nil
	}
}

func isFailedToFindLatest(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "failed to find latest version for ")
}
