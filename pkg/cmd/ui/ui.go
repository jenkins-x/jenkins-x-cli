package ui

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/AlecAivazis/survey.v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/browser"
)

// UIOptions are the options to execute the jx cloudbees command
type UIOptions struct {
	*opts.CommonOptions

	OnlyViewURL  bool
	HideURLLabel bool
	LocalPort    string
}

const (
	// DefaultForwardPort is the default port that the UI will be forwarded to
	DefaultForwardPort = "9000"
)

var (
	core_long = templates.LongDesc(`
		Opens the CloudBees JX UI in a browser.

		Which helps you visualise your CI/CD pipelines.
`)
	core_example = templates.Examples(`
		# Open the JX UI dashboard in a browser
		jx ui

		# Print the Jenkins X console URL but do not open a browser
		jx ui -u`)
)

// NewCmdUI creates the "jx ui" command
func NewCmdUI(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UIOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "ui",
		Short:   "Opens the CloudBees Jenkins X UI app for Kubernetes for visualising CI/CD and your environments",
		Long:    core_long,
		Example: core_example,
		Aliases: []string{"cloudbees", "cloudbee", "cb", "jxui"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&options.OnlyViewURL, "url", "u", false, "Only displays the label and the URL and does not open the browser")
	cmd.Flags().BoolVarP(&options.HideURLLabel, "hide-label", "l", false, "Hides the URL label from display")
	cmd.Flags().StringVarP(&options.LocalPort, "local-port", "p", "", "The local port to forward the data to")

	return cmd
}

// Run implements this command
func (o *UIOptions) Run() error {
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}

	listOptions := v1.ListOptions{
		LabelSelector: "jenkins.io/ui-resource=true",
	}

	log.Logger().Debug("Look for Ingress with label ui-resource")
	ingressList, err := kubeClient.ExtensionsV1beta1().Ingresses(ns).List(listOptions)
	if err != nil {
		return err
	}

	if len(ingressList.Items) == 0 {
		log.Logger().Debug("Couldn't find an Ingress for the UI, executing the UI in read-only mode")
		localURL, serviceName, err := o.getLocalURL(listOptions)
		if err != nil {
			return errors.Wrap(err, "there was a problem getting the local URL")
		}

		// We need to run this in a goroutine so it doesn't block the rest of the execution while forwarding
		go o.executePortForwardRoutine(serviceName)

		err = o.waitForForwarding(localURL)
		if err != nil {
			return err
		}

		log.Logger().Info("Opening the UI in the browser...")
		err = o.openURL(localURL, "Jenkins X UI")
		if err != nil {
			return errors.Wrapf(err, "there was a problem opening the UI in the browser from address %s", util.ColorInfo(localURL))
		}

		// We need to keep the main routine running to avoid having the kubectl port-forward one terminating which would make the UI not accessible anymore
		// This channel will block until a SIGINT signal is received (Ctrl-c)
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		s := <-c
		log.Logger().Debugf("Received signal %s", s.String())
		log.Logger().Info("\nStopping port forwarding")
		os.Exit(1)

	} else {
		log.Logger().Warn("Only single-user mode is available for the UI at this time")
	}

	return nil
}

func (o UIOptions) executePortForwardRoutine(serviceName string) {
	outWriter := ioutil.Discard
	if o.Verbose {
		cmd := fmt.Sprintf("kubectl port-forward service/%s %s:80", serviceName, o.LocalPort)
		log.Logger().Debugf("Executed command: %s", util.ColorInfo(cmd))
		outWriter = o.Out
	}
	c := exec.Command("kubectl", "port-forward", fmt.Sprintf("service/%s", serviceName), fmt.Sprintf("%s:80", o.LocalPort)) // #nosec
	c.Stdout = outWriter
	c.Stderr = o.Err
	err := c.Run()
	if err != nil {
		os.Exit(1)
	}
}

func (o *UIOptions) getLocalURL(listOptions v1.ListOptions) (string, string, error) {
	jxClient, ns, err := o.JXClient()
	if err != nil {
		return "", "", err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return "", "", err
	}
	apps, err := jxClient.JenkinsV1().Apps(ns).List(listOptions)
	if err != nil || len(apps.Items) == 0 {
		log.Logger().Errorf("Couldn't find the jx-app-ui app installed in the cluster. Did you add it via %s?", util.ColorInfo("jx add app jx-app-ui"))
		return "", "", err
	}

	services, err := kubeClient.CoreV1().Services(ns).List(listOptions)
	if err != nil || len(services.Items) == 0 {
		log.Logger().Errorf("Couldn't find the ui service in the cluster")
		return "", "", err
	}

	log.Logger().Info("UI not configured to run with TLS - The UI will open in read-only mode with port-forwarding only for the current user")
	err = o.decideLocalForwardPort()
	if err != nil {
		return "", "", errors.Wrap(err, "there was an error obtaining the local port to forward to")
	}

	serviceName := services.Items[0].Name
	log.Logger().Debugf("Found UI service name %s", util.ColorInfo(serviceName))

	return fmt.Sprintf("http://localhost:%s", o.LocalPort), serviceName, nil
}

func (o UIOptions) waitForForwarding(localURL string) error {
	log.Logger().Infof("Waiting for the UI to be ready on %s...", util.ColorInfo(localURL))
	return o.RetryUntilTrueOrTimeout(time.Minute, time.Second*3, func() (b bool, e error) {
		log.Logger().Debugf("Checking the status of %s", localURL)
		resp, err := http.Get(localURL) // #nosec
		if err != nil || (resp != nil && resp.StatusCode != 200) {
			log.Logger().Debugf("Returned err: %+v", err)
			log.Logger().Info(".")
			return false, nil
		}
		return resp.StatusCode == 200, nil
	})
}

func (o *UIOptions) decideLocalForwardPort() error {
	if o.LocalPort == "" {
		if o.BatchMode {
			return errors.New("executing in Batch Mode and no LocalPort flag provided")
		}
		surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
		prompt := &survey.Input{
			Message: "What local port should the UI be forwarded to?",
			Help:    "The local port that will be used by `kubectl port-forward` to make the UI accessible from your localhost",
			Default: DefaultForwardPort,
		}
		err := survey.AskOne(prompt, &o.LocalPort, nil, surveyOpts)
		if err != nil {
			return errors.Wrap(err, "there was a problem getting the local port from the user")
		}
	}
	return nil
}

func (o *UIOptions) openURL(url string, label string) error {
	// TODO Logger
	if o.HideURLLabel {
		fmt.Fprintf(o.Out, "%s\n", util.ColorInfo(url))
	} else {
		fmt.Fprintf(o.Out, "%s: %s\n", label, util.ColorInfo(url))
	}
	if !o.OnlyViewURL {
		browser.OpenURL(url)
	}
	return nil
}
