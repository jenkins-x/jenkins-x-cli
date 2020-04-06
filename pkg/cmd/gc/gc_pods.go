package gc

import (
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GCPodsOptions containers the CLI options
type GCPodsOptions struct {
	*opts.CommonOptions

	Selector  string
	Namespace string
	Age       time.Duration
}

var (
	GCPodsLong = templates.LongDesc(`
		Garbage collect old Pods that have completed or failed
`)

	GCPodsExample = templates.Examples(`
		# garbage collect old pods of the default age
		jx gc pods

		# garbage collect pods older than 10 minutes
		jx gc pods -a 10m

`)
)

// NewCmdGCPods creates the command object
func NewCmdGCPods(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GCPodsOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "pods",
		Short:   "garbage collection for pods",
		Aliases: []string{"pod"},
		Long:    GCPodsLong,
		Example: GCPodsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Selector, "selector", "s", "", "The selector to use to filter the pods")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The namespace to look for the pods. Defaults to the current namespace")
	cmd.Flags().DurationVarP(&options.Age, "age", "a", time.Hour, "The minimum age of pods to garbage collect. Any newer pods will be kept")
	return cmd
}

// Run implements this command
func (o *GCPodsOptions) Run() error {
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}

	if o.Namespace != "" {
		ns = o.Namespace
	}

	opts := metav1.ListOptions{
		LabelSelector: o.Selector,
	}
	podInterface := kubeClient.CoreV1().Pods(ns)
	podList, err := podInterface.List(opts)
	if err != nil {
		return err
	}

	deleteOptions := &metav1.DeleteOptions{}
	errors := []error{}
	for _, pod := range podList.Items {
		matches, age := o.MatchesPod(&pod)
		if matches {
			err := podInterface.Delete(pod.Name, deleteOptions)
			if err != nil {
				log.Logger().Warnf("Failed to delete pod %s in namespace %s: %s", pod.Name, ns, err)
				errors = append(errors, err)
			} else {
				ageText := strings.TrimSuffix(age.Round(time.Minute).String(), "0s")
				log.Logger().Infof("Deleted pod %s in namespace %s with phase %s as its age is: %s", pod.Name, ns, string(pod.Status.Phase), ageText)
			}
		}
	}
	return util.CombineErrors(errors...)
}

// MatchesPod returns true if this pod can be garbage collected
func (o *GCPodsOptions) MatchesPod(pod *corev1.Pod) (bool, time.Duration) {
	phase := pod.Status.Phase
	now := time.Now()

	finished := now.Add(-1000 * time.Hour)
	for _, s := range pod.Status.ContainerStatuses {
		terminated := s.State.Terminated
		if terminated != nil {
			if terminated.FinishedAt.After(finished) {
				finished = terminated.FinishedAt.Time
			}
		}
	}
	age := now.Sub(finished)
	if phase != corev1.PodSucceeded && phase != corev1.PodFailed {
		return false, age
	}
	return age > o.Age, age
}
