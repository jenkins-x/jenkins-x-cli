package fake

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gojenkins "github.com/jenkins-x/golang-jenkins"
)

// FakeJenkins contains the state of the fake JenkinsClient
type FakeJenkins struct {
	baseURL string
	Jobs    []gojenkins.Job
	JobMap  map[string]*gojenkins.Job
}

// NewFakeJenkins creates a fake JenkinsClient that can be used in tests
func NewFakeJenkins() *FakeJenkins {
	return &FakeJenkins{
		JobMap: map[string]*gojenkins.Job{},
	}
}

// GetJobs returns the jobs
func (j *FakeJenkins) GetJobs() ([]gojenkins.Job, error) {
	return j.Jobs, nil
}

// GetJob gets a job by name
func (j *FakeJenkins) GetJob(name string) (gojenkins.Job, error) {
	for _, job := range j.Jobs {
		if job.Name == name {
			return job, nil
		}
	}
	return gojenkins.Job{}, j.notFoundf("job: %s", name)
}

// CreateJobWithXML create a job from XML
func (j *FakeJenkins) CreateJobWithXML(jobXml string, folder string) error {
	folderJob := j.getOrCreateFolderJob(folder)
	if folderJob != nil {
		return nil
	}
	return j.notFoundf("job: %s", folder)
}

// CreateFolderJobWithXML creates a folder based job from XML
func (j *FakeJenkins) CreateFolderJobWithXML(jobXml string, folder string, jobName string) error {
	folderJob := j.getOrCreateFolderJob(folder)

	for _, job := range folderJob.Jobs {
		if job.Name == jobName {
			return nil
		}
	}
	folderJob.Jobs = append(folderJob.Jobs, gojenkins.Job{
		Name: jobName,
		Url:  folderJob.Url + "/job/" + folder,
	})
	return nil
}

func (j *FakeJenkins) getOrCreateFolderJob(folder string) *gojenkins.Job {
	var folderJob *gojenkins.Job
	for i, job := range j.Jobs {
		if job.Name == folder {
			folderJob = &j.Jobs[i]
			break
		}
	}
	if folderJob == nil {
		j.Jobs = append(j.Jobs, gojenkins.Job{
			Name: folder,
			Url:  "/job/" + folder,
		})
		folderJob = &j.Jobs[len(j.Jobs)-1]
	}
	return folderJob
}

// GetJobURLPath gets the job URL patjh
func (j *FakeJenkins) GetJobURLPath(name string) string {
	return gojenkins.FullPath(name)
}

// IsErrNotFound returns true if the error is not found
func (j *FakeJenkins) IsErrNotFound(err error) bool {
	te, ok := err.(gojenkins.APIError)
	return ok && te.StatusCode == 404
}

// BaseURL returns the server base URL
func (j *FakeJenkins) BaseURL() string {
	return j.baseURL
}

// SetHTTPClient sets the http client
func (j *FakeJenkins) SetHTTPClient(*http.Client) {
}

// Post posts an object
func (j *FakeJenkins) Post(string, url.Values, interface{}) (err error) {
	return nil
}

// GetJobConfig gets the job config for the given name
func (j *FakeJenkins) GetJobConfig(name string) (gojenkins.JobItem, error) {
	return gojenkins.JobItem{}, j.notImplemented()
}

// GetBuild gets the build for a specific job and build number
func (j *FakeJenkins) GetBuild(gojenkins.Job, int) (gojenkins.Build, error) {
	return gojenkins.Build{}, j.notImplemented()
}

// GetLastBuild returns the last build of the job
func (j *FakeJenkins) GetLastBuild(gojenkins.Job) (gojenkins.Build, error) {
	return gojenkins.Build{}, j.notImplemented()
}

// StopBuild stops the build
func (j *FakeJenkins) StopBuild(gojenkins.Job, int) error {
	return nil
}

// GetMultiBranchJob gets a multi branch job of the given name
func (j *FakeJenkins) GetMultiBranchJob(string, string, string) (gojenkins.Job, error) {
	return gojenkins.Job{}, j.notImplemented()
}

// GetJobByPath fake
func (j *FakeJenkins) GetJobByPath(names ...string) (gojenkins.Job, error) {
	jobs := j.Jobs
	lastIdx := len(names) - 1
	for idx, name := range names {
		found := false
		for _, job := range jobs {
			if job.Name == name {
				if idx >= lastIdx {
					return job, nil
				}
				jobs = job.Jobs
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return gojenkins.Job{}, j.notFoundf("job for path %s", strings.Join(names, "/"))
}

// GetOrganizationScanResult returns the organisation scan result
func (j *FakeJenkins) GetOrganizationScanResult(int, gojenkins.Job) (string, error) {
	return "", j.notImplemented()
}

// CreateJob creates a job
func (j *FakeJenkins) CreateJob(gojenkins.JobItem, string) error {
	return nil
}

// Reload reloads the fake server
func (j *FakeJenkins) Reload() error {
	return nil
}

// Restart restarts the fake server
func (j *FakeJenkins) Restart() error {
	return nil
}

// SafeRestart safely restarts the fake server
func (j *FakeJenkins) SafeRestart() error {
	return nil
}

// QuietDown quiets down
func (j *FakeJenkins) QuietDown() error {
	return nil
}

// GetCredential get the credential of the given name
func (j *FakeJenkins) GetCredential(string) (*gojenkins.Credentials, error) {
	return nil, j.notImplemented()
}

// CreateCredential creates a credential
func (j *FakeJenkins) CreateCredential(string, string, string) error {
	return nil
}

// DeleteJob deletes a job
func (j *FakeJenkins) DeleteJob(gojenkins.Job) error {
	return nil
}

// UpdateJob updates a job
func (j *FakeJenkins) UpdateJob(gojenkins.JobItem, string) error {
	return nil
}

// RemoveJob removes a job
func (j *FakeJenkins) RemoveJob(string) error {
	return nil
}

// AddJobToView adds a job to the view
func (j *FakeJenkins) AddJobToView(string, gojenkins.Job) error {
	return nil
}

// CreateView creates a view
func (j *FakeJenkins) CreateView(gojenkins.ListView) error {
	return nil
}

// Build triggers a build
func (j *FakeJenkins) Build(gojenkins.Job, url.Values) error {
	return nil
}

// GetBuildConsoleOutput get the console output
func (j *FakeJenkins) GetBuildConsoleOutput(gojenkins.Build) ([]byte, error) {
	return nil, j.notImplemented()
}

// GetQueue gets the build queue
func (j *FakeJenkins) GetQueue() (gojenkins.Queue, error) {
	return gojenkins.Queue{}, j.notImplemented()
}

// GetArtifact gets an artifact
func (j *FakeJenkins) GetArtifact(gojenkins.Build, gojenkins.Artifact) ([]byte, error) {
	return nil, j.notImplemented()
}

// SetBuildDescription sets the build description
func (j *FakeJenkins) SetBuildDescription(gojenkins.Build, string) error {
	return nil
}

// GetComputerObject gets the computer
func (j *FakeJenkins) GetComputerObject() (gojenkins.ComputerObject, error) {
	return gojenkins.ComputerObject{}, j.notImplemented()
}

// GetComputers gets the computers
func (j *FakeJenkins) GetComputers() ([]gojenkins.Computer, error) {
	return nil, j.notImplemented()
}

// GetComputer gets the computer
func (j *FakeJenkins) GetComputer(string) (gojenkins.Computer, error) {
	return gojenkins.Computer{}, j.notImplemented()
}

// GetBuildURL gets the build URL
func (j *FakeJenkins) GetBuildURL(gojenkins.Job, int) string {
	return ""
}

// GetLogFromURL gets the log from a URL
func (j *FakeJenkins) GetLogFromURL(string, int64, *gojenkins.LogData) error {
	return nil
}

// TailLog tails the log
func (j *FakeJenkins) TailLog(string, io.Writer, time.Duration, time.Duration) error {
	return nil
}

// TailLogFunc tails the log function
func (j *FakeJenkins) TailLogFunc(string, io.Writer) gojenkins.ConditionFunc {
	return nil
}

// NewLogPoller creates a new log poller
func (j *FakeJenkins) NewLogPoller(string, io.Writer) *gojenkins.LogPoller {
	return nil
}

func (j *FakeJenkins) notImplemented() error {
	return fmt.Errorf("not implemented")
}

func (j *FakeJenkins) notFound(message string) error {
	return fmt.Errorf("not found: %s", message)
}

func (j *FakeJenkins) notFoundf(message string, arguments ...interface{}) error {
	return j.notFound(fmt.Sprintf(message, arguments...))
}
