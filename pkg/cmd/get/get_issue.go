package get

import (
	"regexp"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/issues"
	"github.com/jenkins-x/jx/pkg/kube"
)

// GetIssueOptions contains the command line options
type GetIssueOptions struct {
	GetOptions

	Dir string
	Id  string
}

var (
	GetIssueLong = templates.LongDesc(`
		Display the status of an issue for a project.

`)

	GetIssueExample = templates.Examples(`
		# Get the status of an issue for a project
		jx get issue --id ISSUE_ID
	`)
)

// NewCmdGetIssue creates the command
func NewCmdGetIssue(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetIssueOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "issue [flags]",
		Short:   "Display the status of an issue",
		Long:    GetIssueLong,
		Example: GetIssueExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Id, "id", "i", "", "The issue ID")
	cmd.Flags().StringVarP(&options.Dir, "dir", "d", "", "The root project directory")

	options.AddGetFlags(cmd)
	return cmd
}

// Run implements this command
func (o *GetIssueOptions) Run() error {
	tracker, err := o.CreateIssueProvider(o.Dir)
	if err != nil {
		return errors.Wrap(err, "failed to create the issue tracker")
	}

	issue, err := tracker.GetIssue(o.Id)
	if err != nil {
		return errors.Wrap(err, "issue not found")
	}

	table := o.CreateTable()
	table.AddRow("ISSUE", "STATUS", "APPLICATION", "ENVIRONMENT")

	client, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "cannot create the JX client")
	}

	envList, err := client.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "cannot list the environments")
	}

	kubeClient, _, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to create the Kubernetes client")
	}

	found := false
	for _, env := range envList.Items {
		envNs, err := kube.GetEnvironmentNamespace(client, ns, env.Name)
		if err != nil {
			continue
		}
		releaseList, err := client.JenkinsV1().Releases(envNs).List(metav1.ListOptions{})
		if err != nil {
			return errors.Wrap(err, "cannot list the releases")
		}
		rel := o.findRelease(tracker, issue, releaseList.Items)
		if rel == nil {
			continue
		}

		apps, err := o.getApplications(kubeClient, &env, envNs)
		if err != nil {
			continue
		}
		for _, app := range apps {
			if o.match(issue.URL, app) {
				table.AddRow(issue.URL, *issue.State, app, env.Name)
				found = true
			}
		}
	}
	if !found {
		table.AddRow(issue.URL, *issue.State, "", "")
	}
	table.Render()
	return nil
}

func (o *GetIssueOptions) findRelease(tracker issues.IssueProvider, issue *gits.GitIssue, releases []v1.Release) *v1.Release {
	for _, rel := range releases {
		prs := rel.Spec.PullRequests
		// checks all the PRs and the issues linked into their bodies
		for _, pr := range prs {
			if pr.URL == issue.URL {
				return &rel
			} else {
				issueKind := issues.GetIssueProvider(tracker)
				issueIDs := o.parseIssueIDs(pr, issueKind)
				issueURLs := o.convertIssueIDsToURLs(tracker, issueIDs)
				for _, issueURL := range issueURLs {
					if issueURL == issue.URL {
						return &rel
					}
				}
			}
		}
		// checks all the issues which are not pull requests
		issues := rel.Spec.Issues
		for _, is := range issues {
			if is.URL == issue.URL {
				return &rel
			}
		}
	}
	return nil
}

func (o *GetIssueOptions) parseIssueIDs(issue v1.IssueSummary, issueKind string) []string {
	regex := regexp.MustCompile(`(\#\d+)`)
	if issueKind == issues.Jira {
		regex = regexp.MustCompile(`[A-Z][A-Z]+-(\d+)`)
	}
	issues := []string{}
	foundIssues := map[string]bool{}
	matches := regex.FindAllStringSubmatch(issue.Body, -1)
	for _, match := range matches {
		for _, result := range match {
			id := strings.TrimPrefix(result, "#")
			found, ok := foundIssues[id]
			if !found || !ok {
				issues = append(issues, id)
				foundIssues[id] = true
			}
		}
	}
	return issues
}

func (o *GetIssueOptions) convertIssueIDsToURLs(tracker issues.IssueProvider, issueIDs []string) []string {
	issueURLs := []string{}
	for _, id := range issueIDs {
		issue, err := tracker.GetIssue(id)
		if err != nil {
			continue
		}
		issueURLs = append(issueURLs, issue.URL)
	}
	return issueURLs
}

func (o *GetIssueOptions) getApplications(client kubernetes.Interface, env *v1.Environment, envNs string) ([]string, error) {
	apps := []string{}
	deployments, err := kube.GetDeployments(client, envNs)
	if err != nil {
		return apps, errors.Wrap(err, "failed to retrieve the application deployments")
	}

	for k := range deployments {
		appName := kube.GetAppName(k, envNs)
		if env.Spec.Kind == v1.EnvironmentKindTypeEdit {
			if appName == kube.DeploymentExposecontrollerService {
				continue
			}
			currUsername, _ := o.GetUsername(o.Username)
			if currUsername != env.Spec.PreviewGitSpec.User.Username {
				continue
			}
			appName = kube.GetEditAppName(appName)
		} else if env.Spec.Kind == v1.EnvironmentKindTypePreview {
			appName = env.Spec.PullRequestURL
		}
		apps = append(apps, appName)
	}

	return apps, nil
}

func (o *GetIssueOptions) match(issueURL string, appName string) bool {
	isssueURLParts := strings.Split(issueURL, "/")
	for _, issueURLPart := range isssueURLParts {
		if issueURLPart == appName {
			return true
		}
	}
	return false
}
