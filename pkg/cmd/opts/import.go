package opts

import (
	"fmt"
	"net/url"
	"time"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/pipelinescheduler"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

// ImportProject imports a MultiBranchProject into Jenkins for the given git URL
func (o *CommonOptions) ImportProject(gitURL string, dir string, jenkinsfile string, branchPattern, credentials string,
	failIfExists bool, gitProvider gits.GitProvider, authConfigSvc auth.ConfigService, isEnvironment bool, batchMode bool) error {
	jc, err := o.JenkinsClient()
	if err != nil {
		return errors.Wrap(err, "creating Jenkins client")
	}

	if gitURL == "" {
		return fmt.Errorf("no Git repository URL provided")
	}

	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "parsing git URL %q", gitURL)
	}

	if branchPattern == "" {
		branchPattern, err = o.getBranchPattern(gitProvider, dir)
		if err != nil {
			return errors.Wrapf(err, "getting branch pattern in dir %q", dir)
		}
	}

	credentials, err = o.updateJenkinsCredentials(credentials, jc, gitProvider)
	if err != nil {
		return errors.Wrapf(err, "updating credentials %q in Jenkins", credentials)
	}

	if err := o.createJenkinsJob(jc, gitInfo.Organisation); err != nil {
		return errors.Wrap(err, "creating Jenkins job")
	}

	opts := &JenkinsProjectOptions{
		GitInfo:       gitInfo,
		GitProvider:   gitProvider,
		Credentials:   credentials,
		BranchPattern: branchPattern,
		Jenkinsfile:   jenkinsfile,
	}
	if err := o.createJenkinsProject(opts, jc, gitInfo.Organisation, failIfExists, isEnvironment); err != nil {
		return errors.Wrap(err, "creating Jenkins project")
	}

	if err := o.registerJenkinsWebhook(jc, gitProvider, gitInfo, gitURL); err != nil {
		return errors.Wrap(err, "registering Jenkins webhook")
	}

	return nil
}

func (o *CommonOptions) createJenkinsJob(jc gojenkins.JenkinsClient, organisation string) error {
	return o.Retry(10, time.Second*10, func() error {
		folder, err := jc.GetJob(organisation)
		if err != nil {
			// could not find folder so lets try create it
			jobUrl := util.UrlJoin(jc.BaseURL(), jc.GetJobURLPath(organisation))
			folderXML := jenkins.CreateFolderXML(jobUrl, organisation)
			err = jc.CreateJobWithXML(folderXML, organisation)
			if err != nil {
				return errors.Wrapf(err, "creting the %q folder in Jenkins", organisation)
			}
		} else {
			c := folder.Class
			if c != "com.cloudbees.hudson.plugins.folder.Folder" {
				log.Logger().Warnf("The folder %s is of class %s", organisation, c)
			}
		}
		return nil
	})
}

// JenkinsProjectOptions store the required options to create a Jenkins project
type JenkinsProjectOptions struct {
	GitInfo       *gits.GitRepository
	GitProvider   gits.GitProvider
	Credentials   string
	BranchPattern string
	Jenkinsfile   string
}

func (o *CommonOptions) createJenkinsProject(opts *JenkinsProjectOptions, jc gojenkins.JenkinsClient,
	organisation string, failIfExists bool, isEnvironment bool) error {
	return o.Retry(10, time.Second*10, func() error {
		projectXml := jenkins.CreateMultiBranchProjectXml(
			opts.GitInfo,
			opts.GitProvider,
			opts.Credentials,
			opts.BranchPattern,
			opts.Jenkinsfile)
		jobName := opts.GitInfo.Name
		job, err := jc.GetJobByPath(organisation, jobName)
		if err != nil {
			err = jc.CreateFolderJobWithXML(projectXml, organisation, jobName)
			if err != nil {
				return errors.Wrapf(err, "creating MultiBranchProject job %q in folder %q",
					jobName, organisation)
			}
			job, err = jc.GetJobByPath(organisation, jobName)
			if err != nil {
				return errors.Wrapf(err, "checking the MultiBranchProject job %q in folder %q exists",
					jobName, organisation)
			}
			log.Logger().Infof("Created Jenkins Project: %s", util.ColorInfo(job.Url))

			o.LogImportedProject(isEnvironment, opts.GitInfo)

			params := url.Values{}
			err = jc.Build(job, params)
			if err != nil {
				return errors.Wrapf(err, "triggering the job %q", job.Url)
			}
			log.Logger().Infof("Triggered Jenkins job:  %s", job.Url)

			return nil
		}

		if failIfExists {
			return fmt.Errorf("job already exists in Jenkins at %s", job.Url)
		} else {
			log.Logger().Infof("Job already exists in Jenkins at %s", job.Url)
		}

		return nil
	})
}

func (o *CommonOptions) registerJenkinsWebhook(jc gojenkins.JenkinsClient, gitProvider gits.GitProvider,
	gitInfo *gits.GitRepository, gitURL string) error {
	suffix := gitProvider.JenkinsWebHookPath(gitURL, "")
	jenkBaseURL := o.ExternalJenkinsBaseURL
	if jenkBaseURL == "" {
		jenkBaseURL = jc.BaseURL()
	}
	isInsecureSSL, err := o.IsInsecureSSLWebhooks()
	if err != nil {
		return errors.Wrapf(err, "checking if we need to setup insecure SSL webhook")
	}

	webhookUrl := util.UrlJoin(jenkBaseURL, suffix)
	webhook := &gits.GitWebHookArguments{
		Owner:       gitInfo.Organisation,
		Repo:        gitInfo,
		URL:         webhookUrl,
		InsecureSSL: isInsecureSSL,
	}
	return gitProvider.CreateWebHook(webhook)
}

func (o *CommonOptions) getBranchPattern(gitProvider gits.GitProvider, dir string) (string, error) {
	var branchPattern string
	patterns, err := o.TeamBranchPatterns()
	if err != nil {
		return branchPattern, errors.Wrap(err, "getting branch pattern from team settings")
	}
	branchPattern = patterns.DefaultBranchPattern
	if branchPattern != "" {
		return branchPattern, nil
	}

	log.Logger().Debugf("Checking if the repository is a fork at %s with kind %s", gitProvider.ServerURL(), gitProvider.Kind())
	fork, err := o.Git().IsFork(dir)
	if err != nil {
		return branchPattern, errors.Wrap(err, "checking git repository is a fork")
	}
	if fork {
		branch, err := o.Git().Branch(dir)
		if err != nil {
			return branchPattern, errors.Wrapf(err, "getting the current branch in dir %q", dir)
		}
		if branch == "" {
			return branchPattern, fmt.Errorf("no branch found in dir %q", dir)
		}
		branchPattern = branch
	} else {
		branchPattern = jenkins.BranchPattern(gitProvider.Kind())
	}
	log.Logger().Debugf("Using branch pattern: %s", branchPattern)
	return branchPattern, nil
}

func (o *CommonOptions) updateJenkinsCredentials(credentials string, jc gojenkins.JenkinsClient, gitProvider gits.GitProvider) (string, error) {
	if credentials == "" {
		credentials = gitProvider.ServerURL()
	}
	if credentials == "" {
		return credentials, fmt.Errorf("no server configured in the git provider")
	}

	_, err := jc.GetCredential(credentials)
	if err == nil {
		return credentials, nil
	}

	auth := gitProvider.UserAuth()
	if err := jc.CreateCredential(credentials, auth.Username, auth.ApiToken); err != nil {
		return credentials, errors.Wrapf(err, "creating credentials %q in Jenkins", credentials)
	}

	return credentials, nil
}

// GenerateProwConfig regenerates the Prow configurations after we have created a SourceRepository
func (o *CommonOptions) GenerateProwConfig(currentNamespace string, devEnv *v1.Environment) error {
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	jxClient, _, err := o.JXClient()
	if err != nil {
		return err
	}

	defaultSchedulerName := devEnv.Spec.TeamSettings.DefaultScheduler.Name
	config, plugins, err := pipelinescheduler.GenerateProw(false, true, jxClient, currentNamespace, defaultSchedulerName, devEnv, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to update the Prow 'config' and 'plugins' ConfigMaps when regenerating prow config from source repositories")
	}
	err = pipelinescheduler.ApplyDirectly(kubeClient, currentNamespace, config, plugins)
	if err != nil {
		return errors.Wrapf(err, "applying Prow config in namespace %s", currentNamespace)
	}
	log.Logger().Infof("regenerated Prow configuration")
	return nil
}

// LogImportedProject logs more details for an imported project
func (o *CommonOptions) LogImportedProject(isEnvironment bool, gitInfo *gits.GitRepository) {
	log.Blank()
	if !isEnvironment {
		log.Logger().Infof("Watch pipeline activity via:    %s", util.ColorInfo(fmt.Sprintf("jx get activity -f %s -w", gitInfo.Name)))
		log.Logger().Infof("Browse the pipeline log via:    %s", util.ColorInfo(fmt.Sprintf("jx get build logs %s", gitInfo.PipelinePath())))
		log.Logger().Infof("You can list the pipelines via: %s", util.ColorInfo("jx get pipelines"))
		log.Logger().Infof("When the pipeline is complete:  %s", util.ColorInfo("jx get applications"))
		log.Blank()
		log.Logger().Infof("For more help on available commands see: %s", util.ColorInfo("https://jenkins-x.io/developing/browsing/"))
		log.Blank()
	}
	log.Logger().Info(util.ColorStatus("Note that your first pipeline may take a few minutes to start while the necessary images get downloaded!\n"))
}
