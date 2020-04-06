package get

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/versionstream"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

const (
	defaultRepoName            = "origin"
	defaultStableVersionBranch = "master"
)

// StepGetVersionChangeSetOptions contains the command line flags
type StepGetVersionChangeSetOptions struct {
	step.StepOptions
	VersionsDir        string
	VersionsRepository string
	VersionsGitRef     string
	TestingBranch      string
	StableBranch       string
	PR                 string
}

var (
	// StepGetVersionChangeSetLong command long description
	StepGetVersionChangeSetLong = templates.LongDesc(`
		This pipeline step generates environment variables from the differences of versions between jenkins-x-version branches

`)
	// StepGetVersionChangeSetExample command example
	StepGetVersionChangeSetExample = templates.Examples(`
		# This pipeline step generates environment variables from the differences of versions between jenkins-x-version PR21 and the master branch
		jx step get version-changeset --pr 21

        # This pipeline step generates environment variables from the differences of versions between jenkins-x-version PR21 and a branch called stuff
        jx step get version-changeset --stable-branch stuff --pr 21
`)
)

// NewCmdStepGetVersionChangeSet create the 'step git envs' command
func NewCmdStepGetVersionChangeSet(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepGetVersionChangeSetOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "version-changeset",
		Short:   "Creates environment variables from the differences of versions between jenkins-x-version branches",
		Long:    StepGetVersionChangeSetLong,
		Example: StepGetVersionChangeSetExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.VersionsRepository, "repo", "r", "", "Jenkins X versions Git repo")
	cmd.Flags().StringVarP(&options.VersionsGitRef, "versions-ref", "", "", "Jenkins X versions Git repository reference (tag, branch, sha etc)")
	cmd.Flags().StringVarP(&options.StableBranch, "stable-branch", "", defaultStableVersionBranch, "the versions git repository branch to compare against")
	cmd.Flags().StringVarP(&options.TestingBranch, "testing-branch", "", defaultStableVersionBranch, "the versions git repository branch to clone")
	cmd.Flags().StringVarP(&options.VersionsDir, "versions-dir", "", "", "the directory containing the versions repo")
	cmd.Flags().StringVarP(&options.PR, "pr", "", "", "the PR in the versions repository top clone")

	return cmd
}

// Run implements the command
func (o *StepGetVersionChangeSetOptions) Run() error {
	if o.VersionsDir == "" {
		versionDir, _, err := o.CloneJXVersionsRepo(o.VersionsRepository, o.VersionsGitRef)
		if err != nil {
			return err
		}
		o.VersionsDir = versionDir
	}
	if o.PR != "" {
		o.TestingBranch = o.PR
		o.Git().FetchBranch(o.VersionsDir, defaultRepoName, "pull/"+o.PR+"/head:"+o.PR)
	}

	if o.StableBranch != defaultStableVersionBranch {
		o.Git().FetchBranch(o.VersionsDir, defaultRepoName, o.StableBranch+":"+o.StableBranch)
	}

	o.Git().Checkout(o.VersionsDir, o.TestingBranch)
	bulkChange, _ := o.Git().ListChangedFilesFromBranch(o.VersionsDir, o.StableBranch)
	changeSets := strings.Split(bulkChange, "\n")
	appUpdatedVersions := make([]string, 0)
	appPreviousVersions := make([]string, 0)
	for _, line := range changeSets {
		output := strings.Split(line, "\t")
		if len(output) == 2 {
			modifier := output[0]
			if modifier == "A" || modifier == "M" {
				file := output[1]
				if strings.HasSuffix(file, ".yml") {
					stableVersion, err := versionstream.LoadStableVersionFile(filepath.Join(o.VersionsDir, file))
					if err == nil {
						if modifier == "M" {
							fileData, _ := o.Git().LoadFileFromBranch(o.VersionsDir, o.StableBranch, file)
							oldStableVersion, _ := versionstream.LoadStableVersionFromData([]byte(fileData))
							oldApp := formatVersion(file, oldStableVersion)
							appPreviousVersions = append(appPreviousVersions, oldApp)
						}
						changedApp := formatVersion(file, stableVersion)
						appUpdatedVersions = append(appUpdatedVersions, changedApp)
					}
				}
			}

		}
	}
	updateEnv := strings.Join(appUpdatedVersions, ",")
	previousEnv := strings.Join(appPreviousVersions, ",")
	fmt.Fprintf(o.Out, "JX_CHANGED_VERSIONS=\"%s\"\n", updateEnv)
	fmt.Fprintf(o.Out, "JX_STABLE_VERSIONS=\"%s\"\n", previousEnv)
	return nil
}

func formatVersion(fileName string, stableVersion *versionstream.StableVersion) string {
	formattedVersion := strings.Replace(fileName, string(versionstream.KindChart)+"/", string(versionstream.KindChart)+":", 1)
	formattedVersion = strings.Replace(formattedVersion, ".yml", "", 1)
	formattedVersion = formattedVersion + ":" + stableVersion.Version
	return formattedVersion
}
