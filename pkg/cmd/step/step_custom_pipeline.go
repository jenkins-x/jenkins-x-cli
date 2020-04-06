package step

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/jenkinsfile"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepCustomPipelineOptions contains the command line arguments for this command
type StepCustomPipelineOptions struct {
	step.StepOptions

	MultiBranchProject bool
	Dir                string
	Jenkinsfile        string
	JenkinsPath        string
	JenkinsSelector    opts.JenkinsSelectorOptions
}

var (
	stepCustomPipelineLong = templates.LongDesc(`
		This pipeline step lazily creates a Pipeline job inside a custom Jenkins App and then triggers it

`)

	stepCustomPipelineExample = templates.Examples(`
		# triggers the Jenkinsfile in the current directory in the custom Jenkins App
		jx step custom pipeline
`)
)

// NewCmdStepCustomPipeline creates the new command
func NewCmdStepCustomPipeline(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepCustomPipelineOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
		JenkinsSelector: opts.JenkinsSelectorOptions{
			UseCustomJenkins: true,
		},
	}
	cmd := &cobra.Command{
		Use:     "custom pipeline",
		Short:   "Triggers a pipeline in a custom Jenkins server using the local Jenkinsfile",
		Long:    stepCustomPipelineLong,
		Example: stepCustomPipelineExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.JenkinsSelector.CustomJenkinsName, "jenkins-name", "j", "", "The name of the custom Jenkins App if you don't wish to use the default execution engine in Jenkins X")

	cmd.Flags().BoolVarP(&options.MultiBranchProject, "multi-branch-project", "", false, "Use a Multi Branch Project in Jenkins")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the Jenkisnfile inside")
	cmd.Flags().StringVarP(&options.Jenkinsfile, "jenkinsfile", "f", jenkinsfile.Name, "The name of the Jenkinsfile to use")
	cmd.Flags().StringVarP(&options.JenkinsPath, "jenkins-path", "p", "", "The Jenkins folder path to create the pipeline inside. If not specified it defaults to the git 'owner/repoName/branch'")
	return cmd
}

// Run implements the command
func (o *StepCustomPipelineOptions) Run() error {
	jenkinsClient, err := o.CreateCustomJenkinsClient(&o.JenkinsSelector)
	if err != nil {
		return err
	}
	if o.Dir == "" {
		o.Dir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if o.Jenkinsfile == "" {
		o.Jenkinsfile = jenkinsfile.Name
	}
	jenkinsfileName := filepath.Join(o.Dir, o.Jenkinsfile)
	exists, err := util.FileExists(jenkinsfileName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s does not exist", jenkinsfileName)
	}

	gitInfo, err := o.FindGitInfo(o.Dir)
	if err != nil {
		return err
	}

	branch, err := o.Git().Branch(o.Dir)
	if err != nil {
		return err
	}
	if branch == "" {
		branch = "master"
	}

	if o.JenkinsPath == "" {
		o.JenkinsPath = fmt.Sprintf("%s/%s/%s", gitInfo.Organisation, gitInfo.Name, branch)
	}

	paths := strings.Split(o.JenkinsPath, "/")
	last := len(paths) - 1
	for i, path := range paths {
		folderPath := paths[0 : i+1]
		folder, err := jenkinsClient.GetJobByPath(folderPath...)
		fullPath := util.UrlJoin(folderPath...)
		jobURL := util.UrlJoin(jenkinsClient.BaseURL(), fullPath)

		if i < last {
			// lets ensure there's a folder
			err = o.Retry(3, time.Second*10, func() error {
				if err != nil {
					folderXML := jenkins.CreateFolderXML(jobURL, path)
					if i == 0 {
						err = jenkinsClient.CreateJobWithXML(folderXML, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s folder at %s in Jenkins", path, jobURL)
						}
					} else {
						folders := strings.Join(paths[0:i], "/job/")
						err = jenkinsClient.CreateFolderJobWithXML(folderXML, folders, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s folder in folders %s at %s in Jenkins", path, folders, jobURL)
						}
					}
				} else {
					c := folder.Class
					if c != "com.cloudbees.hudson.plugins.folder.Folder" {
						log.Logger().Warnf("Warning the folder %s is of class %s", jobURL, c)
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			gitURL := gitInfo.HttpCloneURL()
			log.Logger().Infof("Using git URL %s and branch %s", util.ColorInfo(gitURL), util.ColorInfo(branch))

			err = o.Retry(3, time.Second*10, func() error {
				if err != nil {
					pipelineXML := jenkins.CreatePipelineXML(gitURL, branch, o.Jenkinsfile)
					if i == 0 {
						err = jenkinsClient.CreateJobWithXML(pipelineXML, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s pipeline at %s in Jenkins", path, jobURL)
						}
					} else {
						folders := strings.Join(paths[0:i], "/job/")
						err = jenkinsClient.CreateFolderJobWithXML(pipelineXML, folders, path)
						if err != nil {
							return errors.Wrapf(err, "failed to create the %s pipeline in folders %s at %s in Jenkins", path, folders, jobURL)
						}
					}
				}
				return nil
			})
			if err != nil {
				return err
			}

			job, err := jenkinsClient.GetJobByPath(paths...)
			if err != nil {
				return err
			}
			job.Url = jenkins.SwitchJenkinsBaseURL(job.Url, jenkinsClient.BaseURL())
			jobPath := strings.Join(paths, "/")
			log.Logger().Infof("triggering pipeline job %s", util.ColorInfo(jobPath))
			err = jenkinsClient.Build(job, url.Values{})
			if err != nil {
				return errors.Wrapf(err, "failed to trigger job %s", jobPath)
			}
		}
	}
	return nil
}
