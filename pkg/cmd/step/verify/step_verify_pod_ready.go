package verify

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/builds"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"os"
	"os/exec"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/table"
	"k8s.io/client-go/kubernetes"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	stepStatusLong = templates.LongDesc(`
		This step checks the status of all kubernetes pods
	`)

	stepStatusExample = templates.Examples(`
		jx step verify pod
	`)
)

// StepVerifyPodReadyOptions contains the command line flags
type StepVerifyPodReadyOptions struct {
	step.StepOptions
	Debug            bool
	ExcludeBuildPods bool

	WaitDuration time.Duration
}

// NewCmdStepVerifyPodReady creates the `jx step verify pod` command
func NewCmdStepVerifyPodReady(commonOpts *opts.CommonOptions) *cobra.Command {

	options := StepVerifyPodReadyOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "ready",
		Short:   "Verifies all the pods are ready",
		Long:    stepStatusLong,
		Example: stepStatusExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().BoolVarP(&options.Debug, "debug", "", false, "Output logs of any failed pod")
	cmd.Flags().DurationVarP(&options.WaitDuration, "wait-time", "w", time.Second, "The default wait time to wait for the pods to be ready")

	cmd.Flags().BoolVarP(&options.ExcludeBuildPods, "exclude-build", "", false, "Exclude build pods")
	return cmd
}

// Run the `jx step verify pod ready` command
func (o *StepVerifyPodReadyOptions) Run() error {
	kubeClient, ns, err := o.KubeClientAndNamespace()
	if err != nil {
		return errors.Wrap(err, "failed to get the Kube client")
	}

	log.Logger().Infof("Checking pod statuses")

	table, err := o.waitForReadyPods(kubeClient, ns)
	table.Render()
	if err != nil {
		if o.WaitDuration.Seconds() == 0 {
			return err
		}
		log.Logger().Warnf("%s\n", err.Error())
		log.Logger().Infof("\nWaiting %s for the pods to become ready...\n\n", o.WaitDuration.String())

		err = o.RetryQuietlyUntilTimeout(o.WaitDuration, time.Second*10, func() error {
			var err error
			table, err = o.waitForReadyPods(kubeClient, ns)
			return err
		})
		table.Render()
	}
	return err
}

func (o *StepVerifyPodReadyOptions) waitForReadyPods(kubeClient kubernetes.Interface, ns string) (table.Table, error) {
	table := o.CreateTable()

	var listOptions metav1.ListOptions
	if o.ExcludeBuildPods {
		listOptions = metav1.ListOptions{
			LabelSelector: fmt.Sprintf("!%s", builds.LabelPipelineRunName),
		}
	} else {
		listOptions = metav1.ListOptions{}
	}

	pods, err := kubeClient.CoreV1().Pods(ns).List(listOptions)
	if err != nil {
		return table, errors.Wrapf(err, "failed to list the PODs in namespace '%s'", ns)
	}

	table.AddRow("POD", "STATUS")

	var f *os.File

	if o.Debug {
		log.Logger().Infof("Creating verify-pod.log file")
		f, err = os.Create("verify-pod.log")
		if err != nil {
			return table, errors.Wrap(err, "error creating log file")
		}
		defer f.Close()
	}

	notReadyPods := []string{}

	notReadyPhases := map[string][]string{}

	for _, pod := range pods.Items {
		podName := pod.ObjectMeta.Name
		phase := pod.Status.Phase

		if phase == corev1.PodFailed && o.Debug {
			args := []string{"logs", podName}
			name := "kubectl"
			e := exec.Command(name, args...)
			e.Stderr = o.Err
			var out bytes.Buffer
			e.Stdout = &out
			err := e.Run()
			if err != nil {
				return table, errors.Wrap(err, "failed to get the Kube pod logs")
			}
			_, err = f.WriteString(fmt.Sprintf("Logs for pod %s:\n", podName))
			if err != nil {
				return table, errors.Wrap(err, "error writing log file")
			}
			_, err = f.Write(out.Bytes())
			if err != nil {
				return table, errors.Wrap(err, "error writing log file")
			}
		}
		table.AddRow(podName, string(phase))

		if !kube.IsPodCompleted(&pod) && !kube.IsPodReady(&pod) {
			notReadyPods = append(notReadyPods, pod.Name)
			key := string(phase)
			notReadyPhases[key] = append(notReadyPhases[key], pod.Name)
		}
	}
	if len(notReadyPods) > 0 {
		phaseSlice := []string{}
		for k, list := range notReadyPhases {
			phaseSlice = append(phaseSlice, fmt.Sprintf("%s: %s", k, strings.Join(list, ", ")))
		}
		return table, fmt.Errorf("the following pods are not ready:\n%s", strings.Join(phaseSlice, "\n"))
	}
	return table, nil
}
