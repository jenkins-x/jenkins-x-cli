package get

import (
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/ghodss/yaml"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	tbl "github.com/jenkins-x/jx/pkg/table"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

const (
	indentation = "  "
)

// GetActivityOptions containers the CLI options
type GetActivityOptions struct {
	*opts.CommonOptions

	Filter      string
	BuildNumber string
	Watch       bool
	Sort        bool
}

var (
	get_activity_long = templates.LongDesc(`
		Display the current activities for one or more projects.
`)

	get_activity_example = templates.Examples(`
		# List the current activities for all applications in the current team
		jx get activities

		# List the current activities for application 'foo'
		jx get act -f foo

		# Watch the activities for application 'foo'
		jx get act -f foo -w
	`)
)

// NewCmdGetActivity creates the new command for: jx get version
func NewCmdGetActivity(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetActivityOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "activities",
		Short:   "Display one or more Activities on projects",
		Aliases: []string{"activity", "act"},
		Long:    get_activity_long,
		Example: get_activity_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Filter, "filter", "f", "", "Text to filter the pipeline names")
	cmd.Flags().StringVarP(&options.BuildNumber, "build", "", "", "The build number to filter on")
	cmd.Flags().BoolVarP(&options.Watch, "watch", "w", false, "Whether to watch the activities for changes")
	cmd.Flags().BoolVarP(&options.Sort, "sort", "s", false, "Sort activities by timestamp")
	return cmd
}

// Run implements this command
func (o *GetActivityOptions) Run() error {
	client, currentNs, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	ns, _, err := kube.GetDevNamespace(kubeClient, currentNs)
	if err != nil {
		return err
	}
	table := o.CreateTable()
	table.SetColumnAlign(1, util.ALIGN_RIGHT)
	table.SetColumnAlign(2, util.ALIGN_RIGHT)
	table.AddRow("STEP", "STARTED AGO", "DURATION", "STATUS")

	if o.Watch {
		return o.WatchActivities(&table, client, ns)
	}

	list, err := client.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	if o.Sort {
		kube.SortActivities(list.Items)
	}

	for _, activity := range list.Items {
		o.addTableRow(&table, &activity)
	}
	table.Render()

	return nil
}

func (o *GetActivityOptions) addTableRow(table *tbl.Table, activity *v1.PipelineActivity) bool {
	if o.matches(activity) {
		spec := &activity.Spec
		text := ""
		version := activity.Spec.Version
		if version != "" {
			text = "Version: " + util.ColorInfo(version)
		}
		statusText := statusString(activity.Spec.Status)
		if statusText == "" {
			statusText = text
		} else {
			statusText += " " + text
		}
		table.AddRow(spec.Pipeline+" #"+spec.Build,
			timeToString(spec.StartedTimestamp),
			util.DurationString(spec.StartedTimestamp, spec.CompletedTimestamp),
			statusText)
		indent := indentation
		for _, step := range spec.Steps {
			o.addStepRow(table, &step, indent)
		}
		return true
	}
	return false
}

func (o *GetActivityOptions) WatchActivities(table *tbl.Table, jxClient versioned.Interface, ns string) error {
	yamlSpecMap := map[string]string{}
	activity := &v1.PipelineActivity{}
	listWatch := cache.NewListWatchFromClient(jxClient.JenkinsV1().RESTClient(), "pipelineactivities", ns, fields.Everything())
	kube.SortListWatchByName(listWatch)
	_, controller := cache.NewInformer(
		listWatch,
		activity,
		time.Minute*10,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				o.onActivity(table, obj, yamlSpecMap)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.onActivity(table, newObj, yamlSpecMap)
			},
			DeleteFunc: func(obj interface{}) {
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	// Wait forever
	select {}
}

func (o *GetActivityOptions) onActivity(table *tbl.Table, obj interface{}, yamlSpecMap map[string]string) {
	activity, ok := obj.(*v1.PipelineActivity)
	if !ok {
		log.Logger().Infof("Object is not a PipelineActivity %#v", obj)
		return
	}
	data, err := yaml.Marshal(&activity.Spec)
	if err != nil {
		log.Logger().Infof("Failed to marshal Activity.Spec to YAML: %s", err)
	} else {
		text := string(data)
		name := activity.Name
		old := yamlSpecMap[name]
		if old == "" || old != text {
			yamlSpecMap[name] = text
			if o.addTableRow(table, activity) {
				table.Render()
				table.Clear()
			}
		}
	}
}

func (o *GetActivityOptions) addStepRow(table *tbl.Table, parent *v1.PipelineActivityStep, indent string) {
	stage := parent.Stage
	preview := parent.Preview
	promote := parent.Promote
	if stage != nil {
		addStageRow(table, stage, indent)
	} else if preview != nil {
		addPreviewRow(table, preview, indent)
	} else if promote != nil {
		addPromoteRow(table, promote, indent)
	} else {
		log.Logger().Warnf("Unknown step kind %#v", parent)
	}
}

func addStageRow(table *tbl.Table, stage *v1.StageActivityStep, indent string) {
	name := "Stage"
	if stage.Name != "" {
		name = ""
	}
	addStepRowItem(table, &stage.CoreActivityStep, indent, name, "")

	indent += indentation
	for _, step := range stage.Steps {
		addStepRowItem(table, &step, indent, "", "")
	}
}

func addPreviewRow(table *tbl.Table, parent *v1.PreviewActivityStep, indent string) {
	pullRequestURL := parent.PullRequestURL
	if pullRequestURL == "" {
		pullRequestURL = parent.Environment
	}
	addStepRowItem(table, &parent.CoreActivityStep, indent, "Preview", util.ColorInfo(pullRequestURL))
	indent += indentation

	appURL := parent.ApplicationURL
	if appURL != "" {
		addStepRowItem(table, &parent.CoreActivityStep, indent, "Preview Application", util.ColorInfo(appURL))
	}
}

func addPromoteRow(table *tbl.Table, parent *v1.PromoteActivityStep, indent string) {
	addStepRowItem(table, &parent.CoreActivityStep, indent, "Promote: "+parent.Environment, "")
	indent += indentation

	pullRequest := parent.PullRequest
	update := parent.Update
	if pullRequest != nil {
		addStepRowItem(table, &pullRequest.CoreActivityStep, indent, "PullRequest", describePromotePullRequest(pullRequest))
	}
	if update != nil {
		addStepRowItem(table, &update.CoreActivityStep, indent, "Update", describePromoteUpdate(update))
	}
	appURL := parent.ApplicationURL
	if appURL != "" {
		addStepRowItem(table, &update.CoreActivityStep, indent, "Promoted", " Application is at: "+util.ColorInfo(appURL))
	}
}

func addStepRowItem(table *tbl.Table, step *v1.CoreActivityStep, indent string, name string, description string) {
	text := step.Description
	if description != "" {
		if text == "" {
			text = description
		} else {
			text += " " + description
		}
	}
	textName := step.Name
	if textName == "" {
		textName = name
	} else {
		if name != "" {
			textName = name + ":" + textName
		}
	}
	table.AddRow(indent+textName,
		timeToString(step.StartedTimestamp),
		util.DurationString(step.StartedTimestamp, step.CompletedTimestamp),
		statusString(step.Status)+" "+text)
}

func statusString(statusType v1.ActivityStatusType) string {
	text := statusType.String()
	switch statusType {
	case v1.ActivityStatusTypeFailed, v1.ActivityStatusTypeError:
		return util.ColorError(text)
	case v1.ActivityStatusTypeSucceeded:
		return util.ColorInfo(text)
	case v1.ActivityStatusTypeRunning:
		return util.ColorStatus(text)
	}
	return text
}

func describePromotePullRequest(promote *v1.PromotePullRequestStep) string {
	description := ""
	if promote.PullRequestURL != "" {
		description += " PullRequest: " + util.ColorInfo(promote.PullRequestURL)
	}
	if promote.MergeCommitSHA != "" {
		description += " Merge SHA: " + util.ColorInfo(promote.MergeCommitSHA)
	}
	return description
}

func describePromoteUpdate(promote *v1.PromoteUpdateStep) string {
	description := ""
	for _, status := range promote.Statuses {
		url := status.URL
		state := status.Status

		if url != "" && state != "" {
			description += " Status: " + pullRequestStatusString(state) + " at: " + util.ColorInfo(url)
		}
	}
	return description
}

func pullRequestStatusString(text string) string {
	title := strings.Title(text)
	switch text {
	case "success":
		return util.ColorInfo(title)
	case "error", "failed":
		return util.ColorError(title)
	default:
		return util.ColorStatus(title)
	}
}

func timeToString(t *metav1.Time) string {
	if t == nil {
		return ""
	}
	now := &metav1.Time{
		Time: time.Now(),
	}
	return util.DurationString(t, now)
}

func (o *GetActivityOptions) matches(activity *v1.PipelineActivity) bool {
	answer := true
	filter := o.Filter
	if filter != "" {
		answer = strings.Contains(activity.Name, filter) || strings.Contains(activity.Spec.Pipeline, filter)
	}
	build := o.BuildNumber
	if answer && build != "" {
		answer = activity.Spec.Build == build
	}
	return answer
}
