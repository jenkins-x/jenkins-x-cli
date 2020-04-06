package jenkins

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	gojenkins "github.com/jenkins-x/golang-jenkins"
	jenkauth "github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
)

func GetJenkinsClient(url string, batch bool, configService jenkauth.ConfigService, handles util.IOFileHandles) (gojenkins.JenkinsClient, error) {
	if url == "" {
		return nil, errors.New("no external Jenkins URL found in the development namespace!\nAre you sure you installed Jenkins X? Try: https://jenkins-x.io/getting-started/")
	}
	tokenUrl := JenkinsTokenURL(url)

	auth := jenkauth.CreateAuthUserFromEnvironment("JENKINS")
	username := auth.Username
	var err error
	config := configService.Config()

	showForm := false
	if auth.IsInvalid() {
		// lets try load the current auth
		config, err = configService.LoadConfig()
		if err != nil {
			return nil, err
		}
		auths := config.FindUserAuths(url)
		if batch {
			if len(auths) > 0 {
				auth = *auths[0]
			} else {
				urls := []string{}
				for _, svr := range config.Servers {
					urls = append(urls, svr.URL)
				}
				return nil, fmt.Errorf("Could not find any user auths for jenkins server %s has server URLs %s", url, strings.Join(urls, ", "))
			}
		} else {
			if len(auths) > 1 {
				// TODO choose an auth
			}
			showForm = true
			a := config.FindUserAuth(url, username)
			if a != nil {
				if a.IsInvalid() {
					auth, err = EditUserAuth(url, configService, config, a, tokenUrl, batch, handles)
					if err != nil {
						return nil, err
					}
				} else {
					auth = *a
				}
			} else {
				// lets create a new Auth
				auth, err = EditUserAuth(url, configService, config, &auth, tokenUrl, batch, handles)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if auth.IsInvalid() {
		if showForm {
			return nil, fmt.Errorf("No valid Username and API Token specified for Jenkins server: %s\n", url)
		} else {
			log.Logger().Warnf("No $JENKINS_USERNAME and $JENKINS_TOKEN environment variables defined!")
			PrintGetTokenFromURL(os.Stdout, tokenUrl)
			if batch {
				log.Logger().Infof("Then run this command on your terminal and try again:\n")
				log.Logger().Infof("export JENKINS_TOKEN=myApiToken\n")
				return nil, errors.New("No environment variables (JENKINS_USERNAME and JENKINS_TOKEN) or JENKINS_BEARER_TOKEN defined")
			}
		}
	}

	jauth := &gojenkins.Auth{
		Username:    auth.Username,
		ApiToken:    auth.ApiToken,
		BearerToken: auth.BearerToken,
	}
	jenkins := gojenkins.NewJenkins(jauth, url)

	httpClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	jenkins.SetHTTPClient(httpClient)
	return jenkins, nil
}

func PrintGetTokenFromURL(out io.Writer, tokenUrl string) (int, error) {
	return fmt.Fprintf(out, "Please go to %s and click %s to get your API Token\n", util.ColorInfo(tokenUrl), util.ColorInfo("Show API Token"))
}

func JenkinsTokenURL(url string) string {
	tokenUrl := util.UrlJoin(url, "/me/configure")
	return tokenUrl
}

func JenkinsApiURL(url string) string {
	return util.UrlJoin(url, "/api")
}

// JenkinsLoginURL returns the Jenkins login URL
func JenkinsLoginURL(url string) string {
	return util.UrlJoin(url, "/login")
}

func EditUserAuth(url string, configService jenkauth.ConfigService, config *jenkauth.AuthConfig, auth *jenkauth.UserAuth, tokenUrl string, batchMode bool, handles util.IOFileHandles) (jenkauth.UserAuth, error) {

	log.Logger().Infof("\nTo be able to connect to the Jenkins server we need a username and API Token\n")

	f := func(username string) error {
		log.Logger().Infof("\nPlease go to %s and click %s to get your API Token", util.ColorInfo(tokenUrl), util.ColorInfo("Show API Token"))
		log.Logger().Infof("Then COPY the API token so that you can paste it into the form below:\n")
		return nil
	}

	defaultUsername := "admin"

	err := config.EditUserAuth("Jenkins", auth, defaultUsername, true, batchMode, f, handles)
	if err != nil {
		return *auth, err
	}
	err = configService.SaveUserAuth(url, auth)
	return *auth, err
}

// IsMultiBranchProject returns true if this job is a multi branch project
func IsMultiBranchProject(job *gojenkins.Job) bool {
	return job != nil && job.Class == "org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject"
}

// LoadAllJenkinsJobs Loads all the jobs in full from the Jenkins client
func LoadAllJenkinsJobs(jenkinsClient gojenkins.JenkinsClient) ([]*gojenkins.Job, error) {
	answer := []*gojenkins.Job{}
	jobs, err := jenkinsClient.GetJobs()
	if err != nil {
		return answer, err
	}

	for _, j := range jobs {
		childJobs, err := loadChildJobs(jenkinsClient, j.Name)
		if err != nil {
			return answer, err
		}
		answer = append(answer, childJobs...)
	}
	return answer, nil
}

func loadChildJobs(jenkinsClient gojenkins.JenkinsClient, name string) ([]*gojenkins.Job, error) {
	answer := []*gojenkins.Job{}
	job, err := jenkinsClient.GetJob(name)
	if err != nil {
		return answer, err
	}
	answer = append(answer, &job)

	if job.Jobs != nil {
		for _, child := range job.Jobs {
			childJobs, err := loadChildJobs(jenkinsClient, job.FullName+"/"+child.Name)
			if err != nil {
				return answer, err
			}
			answer = append(answer, childJobs...)
		}
	}
	return answer, nil
}

// JobName returns the Jenkins job name starting with the given prefix
func JobName(prefix string, j *gojenkins.Job) string {
	name := j.FullName
	if name == "" {
		name = j.Name
	}
	if prefix != "" {
		name = prefix + "/" + name
	}
	return name
}

// IsPipeline checks if the job is a pipeline job
func IsPipeline(j *gojenkins.Job) bool {
	return strings.Contains(j.Class, "Job")
}

// SwitchJenkinsBaseURL sometimes a Jenkins server does not know its external URL so lets switch the base URL of the job
// URL to use the known working baseURL of the jenkins server
func SwitchJenkinsBaseURL(jobURL string, baseURL string) string {
	if jobURL == "" {
		return baseURL
	}
	if baseURL == "" {
		return jobURL
	}
	u, err := url.Parse(jobURL)
	if err != nil {
		log.Logger().Warnf("failed to parse Jenkins Job URL %s due to: %s", jobURL, err)
		return jobURL
	}

	u2, err := url.Parse(baseURL)
	if err != nil {
		log.Logger().Warnf("failed to parse Jenkins base URL %s due to: %s", baseURL, err)
		return jobURL
	}
	u.Host = u2.Host
	u.Scheme = u2.Scheme
	return u.String()
}
