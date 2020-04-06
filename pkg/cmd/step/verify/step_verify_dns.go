package verify

import (
	"net"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/cenkalti/backoff"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	stepVerifyDNSLong = templates.LongDesc(`
		This step checks that dns has propagated for all ingresses
	`)

	stepVerifyDNSExample = templates.Examples(`
        jx step verify dns --timeout 10m
	`)
)

// StepVerifyDNSOptions options for step verify dns command
type StepVerifyDNSOptions struct {
	step.StepOptions

	Timeout time.Duration
}

// NewCmdStepVerifyDNS creates a new verify url command
func NewCmdStepVerifyDNS(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepVerifyDNSOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "dns",
		Short:   "Verifies DNS resolution for ingress rules",
		Long:    stepVerifyDNSLong,
		Example: stepVerifyDNSExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().DurationVarP(&options.Timeout, optionTimeout, "t", 10*time.Minute, "The default timeout for the endpoint to return the expected HTTP code")
	return cmd
}

func (o *StepVerifyDNSOptions) checkFlags() error {
	return nil
}

// Run waits with exponential backoff for an endpoint to return an expected HTTP status code
func (o *StepVerifyDNSOptions) Run() error {
	if err := o.checkFlags(); err != nil {
		return errors.Wrap(err, "checking flags")
	}

	client, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return errors.Wrap(err, "unable to get kubeclient")
	}

	ingresses, err := client.ExtensionsV1beta1().Ingresses(ns).List(v1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "unable to get ingresses")
	}

	for _, i := range ingresses.Items {
		for _, h := range i.Spec.Rules {
			log.Logger().Infof("Checking DNS resolution for %s", h.Host)

			err := retry(o.Timeout, func() error {
				_, err := net.LookupIP(h.Host)
				if err != nil {
					return errors.Wrapf(err, "Could not resolve: %v", h.Host)
				}

				return nil
			}, func(e error, d time.Duration) {
				log.Logger().Infof("resolution failed, backing of for %s", d)
			})
			if err != nil {
				return errors.Wrap(err, "unable to resolve DNS")
			}
			log.Logger().Infof("%s resolved", h.Host)
		}
	}

	return nil
}

// retry retries with exponential backoff the given function
func retry(maxElapsedTime time.Duration, f func() error, n func(error, time.Duration)) error {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = maxElapsedTime
	bo.InitialInterval = 2 * time.Second
	bo.Reset()
	return backoff.RetryNotify(f, bo, n)

}
