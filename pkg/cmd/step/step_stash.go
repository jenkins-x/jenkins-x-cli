package step

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/jx/pkg/builds"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/kube/naming"

	"github.com/jenkins-x/jx/pkg/collector"
	"github.com/jenkins-x/jx/pkg/gits"

	jenkinsv1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

// StepStashOptions contains the command line flags
type StepStashOptions struct {
	step.StepOptions
	Pattern         []string
	Dir             string
	ToPath          string
	Basedir         string
	StorageLocation jenkinsv1.StorageLocation
	ProjectGitURL   string
	ProjectBranch   string
}

const (
	envVarSourceURL = "SOURCE_URL"

	// storageSupportDescription common text for long command descriptions around storage
	StorageSupportDescription = `
Currently Jenkins X supports storing files into a branch of a git repository or in cloud blob storage like S3, GCS, Azure blobs etc.

When using Cloud Storage we use URLs like 's3://nameOfBucket' on AWS, 'gs://anotherBucket' on GCP or on Azure 'azblob://thatBucket'
`
)

var (
	stepStashLong = templates.LongDesc(`
		This pipeline step stashes the specified files from the build into some stable storage location.
` + StorageSupportDescription + helper.SeeAlsoText("jx step unstash", "jx edit storage"))

	stepStashExample = templates.Examples(`
		# lets collect some files to the team's default storage location (which if not configured uses the current git repository's gh-pages branch)
		jx step stash -c tests -p "target/test-reports/*"

		# lets collect some files to a specific Git URL for a repository
		jx step stash -c tests -p "target/test-reports/*" --git-url https://github.com/myuser/myrepo.git

		# lets collect some files with the file names relative to the 'target/test-reports' folder and store in a Git URL
		jx step stash -c tests -p "target/test-reports/*" --basedir target/test-reports --git-url https://github.com/myuser/myrepo.git

		# lets collect some files to a specific AWS cloud storage bucket
		jx step stash -c coverage -p "build/coverage/*" --bucket-url s3://my-aws-bucket

		# lets collect some files to a specific cloud storage bucket
		jx step stash -c tests -p "target/test-reports/*" --bucket-url gs://my-gcp-bucket

		# lets collect some files to a specific cloud storage bucket and specify the path to store them inside
		jx step stash -c tests -p "target/test-reports/*" --bucket-url gs://my-gcp-bucket --to-path tests/mystuff

`)
)

// NewCmdStepStash creates the CLI command
func NewCmdStepStash(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepStashOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "stash",
		Short:   "Stashes local files generated as part of a pipeline into long term storage",
		Aliases: []string{"collect"},
		Long:    stepStashLong,
		Example: stepStashExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	addStorageLocationFlags(cmd, &options.StorageLocation)

	cmd.Flags().StringArrayVarP(&options.Pattern, "pattern", "p", nil, "Specify the pattern to use to look for files")
	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The source directory to try detect the current git repository or branch. Defaults to using the current directory")
	cmd.Flags().StringVarP(&options.ToPath, "to-path", "t", "", "The path within the storage to store the files. If not specified it defaults to 'jenkins-x/$category/$owner/$repoName/$branch/$buildNumber'")
	cmd.Flags().StringVarP(&options.Basedir, "basedir", "", "", "The base directory to use to create relative output file names. e.g. if you specify '--pattern \"target/*.xml\" then you may want to supply '--basedir target' to strip the 'target/' prefix from all collected files")
	cmd.Flags().StringVarP(&options.ProjectGitURL, "project-git-url", "", "", "The project git URL to collect for. Used to default the organisation and repository folders in the storage. If not specified its discovered from the local '.git' folder")
	cmd.Flags().StringVarP(&options.ProjectBranch, "project-branch", "", "", "The project git branch of the project to collect for. Used to default the branch folder in the storage. If not specified its discovered from the local '.git' folder")
	return cmd
}

func addStorageLocationFlags(cmd *cobra.Command, location *jenkinsv1.StorageLocation) {
	cmd.Flags().StringVarP(&location.Classifier, "classifier", "c", "", "A name which classifies this type of file. Example values: "+kube.ClassificationValues)
	cmd.Flags().StringVarP(&location.BucketURL, "bucket-url", "", "", "Specify the cloud storage bucket URL to send each file to. e.g. use 's3://nameOfBucket' on AWS, gs://anotherBucket' on GCP or on Azure 'azblob://thatBucket'")
	cmd.Flags().StringVarP(&location.GitURL, "git-url", "", "", "Specify the Git URL to of the repository to use for storage")
	cmd.Flags().StringVarP(&location.GitBranch, "git-branch", "", "gh-pages", "The branch to use to store files in the git repository")
}

// Run runs the command
func (o *StepStashOptions) Run() error {
	if len(o.Pattern) == 0 {
		return util.MissingOption("pattern")
	}
	classifier := o.StorageLocation.Classifier
	if classifier == "" {
		return util.MissingOption("classifier")
	}
	var err error
	if o.Dir == "" {
		o.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	settings, err := o.TeamSettings()
	if err != nil {
		return err
	}
	if o.StorageLocation.IsEmpty() {
		// lets try get the location from the team settings
		o.StorageLocation = settings.StorageLocationOrDefault(classifier)

		if o.StorageLocation.IsEmpty() {
			// we have no team settings so lets try detect the git repository using an env var or local file system
			sourceURL := os.Getenv(envVarSourceURL)
			if sourceURL == "" {
				_, gitConf, err := o.Git().FindGitConfigDir(o.Dir)
				if err != nil {
					log.Logger().Warnf("Could not find a .git directory: %s", err)
				} else {
					sourceURL, err = o.DiscoverGitURL(gitConf)
				}
			}
			if sourceURL == "" {
				return fmt.Errorf("Missing option --git-url and we could not detect the current git repository URL")
			}
			o.StorageLocation.GitURL = sourceURL
		}
	}
	if o.StorageLocation.IsEmpty() {
		return fmt.Errorf("Missing option --git-url and we could not detect the current git repository URL")
	}

	coll, err := collector.NewCollector(o.StorageLocation, o.Git())
	if err != nil {
		return errors.Wrapf(err, "failed to create the collector for storage settings %s", o.StorageLocation.Description())
	}

	client, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "cannot create the JX client")
	}

	buildNo := builds.GetBuildNumber()
	var projectGitInfo *gits.GitRepository
	gitURL := o.ProjectGitURL
	if gitURL == "" {
		gitURL = o.StorageLocation.GitURL
	}
	if gitURL != "" {
		projectGitInfo, err = gits.ParseGitURL(gitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to parse the git URL %s", gitURL)
		}
	} else {
		dir := ""
		projectGitInfo, err = o.FindGitInfo(dir)
		if err != nil {
			return errors.Wrapf(err, "failed to find the git information in the directory %s", dir)
		}
	}
	projectOrg := projectGitInfo.Organisation
	projectRepoName := projectGitInfo.Name

	projectBranchName, err := o.determineProjectBranchName(o.ProjectBranch, gitURL)
	if err != nil {
		return err
	}

	storagePath := o.ToPath
	if storagePath == "" {
		storagePath = filepath.Join("jenkins-x", classifier, projectOrg, projectRepoName, projectBranchName, buildNo)
	}

	urls, err := coll.CollectFiles(o.Pattern, storagePath, o.Basedir)
	if err != nil {
		return errors.Wrapf(err, "failed to collect patterns %s to path %s", strings.Join(o.Pattern, ", "), storagePath)
	}

	for _, u := range urls {
		log.Logger().Infof("stashed: %s", util.ColorInfo(u))
	}

	// TODO this pipeline name construction needs moving to a shared lib, and other things refactoring to use it
	pipeline := fmt.Sprintf("%s-%s-%s-%s", projectOrg, projectRepoName, projectBranchName, buildNo)

	if pipeline != "" && buildNo != "" {
		name := naming.ToValidName(pipeline)
		key := &kube.PromoteStepActivityKey{
			PipelineActivityKey: kube.PipelineActivityKey{
				Name:     name,
				Pipeline: pipeline,
				Build:    buildNo,
				GitInfo: &gits.GitRepository{
					Organisation: projectOrg,
					Name:         projectRepoName,
				},
			},
		}
		a, _, err := key.GetOrCreate(client, ns)
		if err != nil {
			return err
		}
		a.Spec.Attachments = append(a.Spec.Attachments, jenkinsv1.Attachment{
			Name: classifier,
			URLs: urls,
		})
		_, err = client.JenkinsV1().PipelineActivities(ns).PatchUpdate(a)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *StepStashOptions) determineProjectBranchName(projectBranchName string, gitURL string) (string, error) {
	if projectBranchName != "" {
		return projectBranchName, nil
	}
	// If there isn't a bucket URL, use the configured git branch
	if o.StorageLocation.BucketURL == "" {
		return o.StorageLocation.GitBranch, nil
	}
	// If there is a bucket URL, try using the BRANCH_NAME env var.
	if projectBranchName == "" {
		projectBranchName = os.Getenv(util.EnvVarBranchName)
	}
	if projectBranchName == "" {
		// lets try find the branch name via git
		if gitURL == "" {
			var err error
			projectBranchName, err = o.Git().Branch(o.Dir)
			if err != nil {
				return "", err
			}
		}
	}

	if projectBranchName == "" {
		return "", fmt.Errorf("environment variable %s is empty, and couldn't find a branch from %s as a git repository", util.EnvVarBranchName, o.Dir)
	}

	return projectBranchName, nil
}
