package opts

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/builds"
	jxjenkins "github.com/jenkins-x/jx/pkg/jenkins"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/kube/services"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// JenkinsSelectorOptions used to represent the options used to refer to a Jenkins.
// if nothing is specified it assumes the current team is using a static Jenkins server as its execution engine.
// otherwise we can refer to other additional Jenkins Apps to implement custom Jenkins servers
type JenkinsSelectorOptions struct {
	UseCustomJenkins  bool
	CustomJenkinsName string

	// cached client
	cachedCustomJenkinsClient gojenkins.JenkinsClient
}

// AddFlags add the command flags for picking a custom Jenkins App to work with
func (o *JenkinsSelectorOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.UseCustomJenkins, "custom", "m", false, "Use a custom Jenkins App instead of the default execution engine in Jenkins X")
	cmd.Flags().StringVarP(&o.CustomJenkinsName, "jenkins-name", "j", "", "The name of the custom Jenkins App if you don't wish to use the default execution engine in Jenkins X")
}

// IsCustom returns true if a custom Jenkins App is specified
func (o *JenkinsSelectorOptions) IsCustom() bool {
	return o.UseCustomJenkins || o.CustomJenkinsName != ""
}

// GetAllPipelineJobNames returns all the pipeline job names
func (o *CommonOptions) GetAllPipelineJobNames(jenkinsClient gojenkins.JenkinsClient, jobNames *[]string, jobName string) error {
	job, err := jenkinsClient.GetJob(jobName)
	if err != nil {
		return err
	}
	if len(job.Jobs) == 0 {
		*jobNames = append(*jobNames, job.FullName)
	}
	for _, j := range job.Jobs {
		err = o.GetAllPipelineJobNames(jenkinsClient, jobNames, job.FullName+"/"+j.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetJenkinsClient sets the JenkinsClient - usually used in testing
func (o *CommonOptions) SetJenkinsClient(jenkinsClient gojenkins.JenkinsClient) {
	o.jenkinsClient = jenkinsClient
}

// JenkinsClient returns the Jenkins client
func (o *CommonOptions) JenkinsClient() (gojenkins.JenkinsClient, error) {
	if o.jenkinsClient == nil {
		kubeClient, ns, err := o.KubeClientAndDevNamespace()
		if err != nil {
			return nil, err
		}

		o.factory.SetBatch(o.BatchMode)
		jenkins, err := o.factory.CreateJenkinsClient(kubeClient, ns, o.GetIOFileHandles())

		if err != nil {
			return nil, err
		}
		o.jenkinsClient = jenkins
	}
	return o.jenkinsClient, nil
}

// CustomJenkinsClient returns the Jenkins client for the custom jenkins app
func (o *CommonOptions) CustomJenkinsClient(jenkinsServiceName string) (gojenkins.JenkinsClient, error) {
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, err
	}
	o.factory.SetBatch(o.BatchMode)
	return o.factory.CreateCustomJenkinsClient(kubeClient, ns, jenkinsServiceName, o.GetIOFileHandles())
}

// CustomJenkinsURL returns the default or the custom Jenkins URL
func (o *CommonOptions) CustomJenkinsURL(jenkinsSelector *JenkinsSelectorOptions, kubeClient kubernetes.Interface, ns string) (string, error) {
	if !jenkinsSelector.UseCustomJenkins {
		return o.factory.GetJenkinsURL(kubeClient, ns)
	}
	customJenkinsName, err := o.PickCustomJenkinsName(jenkinsSelector, kubeClient, ns)
	if err != nil {
		return "", err
	}
	return o.factory.GetCustomJenkinsURL(kubeClient, ns, customJenkinsName)
}

// PickCustomJenkinsName picks the name of a custom jenkins server App if available
func (o *CommonOptions) PickCustomJenkinsName(jenkinsSelector *JenkinsSelectorOptions, kubeClient kubernetes.Interface, ns string) (string, error) {
	if !jenkinsSelector.UseCustomJenkins {
		return "", nil
	}
	customJenkinsName := jenkinsSelector.CustomJenkinsName
	if customJenkinsName == "" {
		serviceInterface := kubeClient.CoreV1().Services(ns)
		selector := kube.LabelKind + "=" + kube.ValueKindJenkins
		serviceList, err := serviceInterface.List(metav1.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return "", errors.Wrapf(err, "failed to list Jenkins services in namespace %s with selector %s", ns, selector)
		}
		switch len(serviceList.Items) {
		case 0:
			return "", fmt.Errorf("No Jenkins App services found in namespace %s with selector %s\nAre you sure you installed a Jenkins App in this namespace?\nTry jx add app jx-app-jenkins", ns, selector)

		case 1:
			customJenkinsName = serviceList.Items[0].Name

		default:
			names := []string{}
			for _, svc := range serviceList.Items {
				names = append(names, svc.Name)
			}
			sort.Strings(names)

			if o.BatchMode {
				return "", util.MissingOptionWithOptions("jenkins-name", names)
			}
			customJenkinsName, err = util.PickName(names, "Pick which custom Jenkins App you wish to use: ", "Jenkins Apps are a way to add custom Jenkins servers into Jenkins X", o.GetIOFileHandles())
			if err != nil {
				return "", err
			}
		}
	}
	jenkinsSelector.CustomJenkinsName = customJenkinsName
	if customJenkinsName == "" {
		return "", fmt.Errorf("failed to find a csutom Jenkins App name in namespace %s", ns)
	}
	return customJenkinsName, nil
}

// CreateCustomJenkinsClient creates either a regular Jenkins client or if useCustom is true creates a JenkinsClient
// for a custom jenkins App. If no customJenkinsName is specified and there is only one available it is used. Otherwise
// the user is prompted to pick the Jenkins App to use if not in batch mode.
func (o *CommonOptions) CreateCustomJenkinsClient(jenkinsSelector *JenkinsSelectorOptions) (gojenkins.JenkinsClient, error) {
	isProw, err := o.IsProw()
	if err != nil {
		return nil, err
	}
	if isProw {
		jenkinsSelector.UseCustomJenkins = true
	}
	if jenkinsSelector == nil || !jenkinsSelector.UseCustomJenkins {
		return o.JenkinsClient()
	}
	if jenkinsSelector.cachedCustomJenkinsClient != nil {
		return jenkinsSelector.cachedCustomJenkinsClient, nil
	}
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, err
	}
	customJenkinsName, err := o.PickCustomJenkinsName(jenkinsSelector, kubeClient, ns)
	if err != nil {
		return nil, err
	}
	jenkinsClient, err := o.CustomJenkinsClient(customJenkinsName)
	if err == nil {
		jenkinsSelector.cachedCustomJenkinsClient = jenkinsClient
	}
	return jenkinsClient, err
}

// getJenkinsURL return the Jenkins URL
func (o *CommonOptions) GetJenkinsURL() (string, error) {
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return "", err
	}

	return o.factory.GetJenkinsURL(kubeClient, ns)
}

// GetJenkinsJobs returns the existing Jenkins jobs
func (o *CommonOptions) GetJenkinsJobs(jenkinsSelector *JenkinsSelectorOptions, filter string) (map[string]gojenkins.Job, error) {
	jobMap := map[string]gojenkins.Job{}
	jenkins, err := o.CreateCustomJenkinsClient(jenkinsSelector)
	if err != nil {
		return jobMap, err
	}
	jobs, err := jenkins.GetJobs()
	if err != nil {
		return jobMap, err
	}
	o.AddJenkinsJobs(jenkins, &jobMap, filter, "", jobs)
	return jobMap, nil
}

// AddJenkinsJobs add the given jobs to Jenkins
func (o *CommonOptions) AddJenkinsJobs(jenkins gojenkins.JenkinsClient, jobMap *map[string]gojenkins.Job, filter string, prefix string, jobs []gojenkins.Job) {
	for _, j := range jobs {
		name := jxjenkins.JobName(prefix, &j)
		if jxjenkins.IsPipeline(&j) {
			if filter == "" || strings.Contains(name, filter) {
				(*jobMap)[name] = j
				continue
			}
		}
		if j.Jobs != nil {
			o.AddJenkinsJobs(jenkins, jobMap, filter, name, j.Jobs)
		} else {
			job, err := jenkins.GetJob(name)
			if err == nil && job.Jobs != nil {
				o.AddJenkinsJobs(jenkins, jobMap, filter, name, job.Jobs)
			}
		}
	}
}

// TailJenkinsBuildLog tail the build log of the given Jenkins jobs name
func (o *CommonOptions) TailJenkinsBuildLog(jenkinsSelector *JenkinsSelectorOptions, jobName string, build *gojenkins.Build) error {
	jenkins, err := o.CreateCustomJenkinsClient(jenkinsSelector)
	if err != nil {
		return nil
	}

	u, err := url.Parse(build.Url)
	if err != nil {
		return err
	}
	buildPath := u.Path
	log.Logger().Infof("%s %s", "tailing the log of", fmt.Sprintf("%s #%d", jobName, build.Number))
	// TODO Logger
	return jenkins.TailLog(buildPath, o.Out, time.Second, time.Hour*100)
}

// GetJenkinsJobName returns the Jenkins job name
func (o *CommonOptions) GetJenkinsJobName() string {
	owner := os.Getenv("REPO_OWNER")
	repo := os.Getenv("REPO_NAME")
	branch := o.GetBranchName("")

	if owner != "" && repo != "" && branch != "" {
		return fmt.Sprintf("%s/%s/%s", owner, repo, branch)
	}

	job := os.Getenv("JOB_NAME")
	if job != "" {
		return job
	}
	return ""
}

func (o *CommonOptions) GetBranchName(dir string) string {
	branch := builds.GetBranchName()
	if branch == "" {
		if dir == "" {
			dir = "."
		}
		var err error
		branch, err = o.Git().Branch(dir)
		if err != nil {
			log.Logger().Warnf("failed to get the git branch name in dir %s", dir)
		}
	}
	return branch
}

// UpdateJenkinsURL updates the Jenkins URL
func (o *CommonOptions) UpdateJenkinsURL(namespaces []string) error {
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	// loop over each namespace and update the Jenkins URL if a Jenkins service is found
	for _, n := range namespaces {
		externalURL, err := services.GetServiceURLFromName(client, "jenkins", n)
		if err != nil {
			// skip namespace if no Jenkins service found
			continue
		}

		log.Logger().Infof("Updating Jenkins with new external URL details %s", externalURL)

		jenkins, err := o.factory.CreateJenkinsClient(client, n, o.GetIOFileHandles())

		if err != nil {
			return err
		}

		data := url.Values{}
		data.Add("script", fmt.Sprintf(groovy, externalURL))

		err = jenkins.Post("/scriptText", data, nil)
	}

	return nil
}
