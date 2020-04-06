package helm

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StepHelmReleaseOptions contains the command line flags
type StepHelmReleaseOptions struct {
	StepHelmOptions
}

var (
	StepHelmReleaseLong = templates.LongDesc(`
		This pipeline step releases the Helm chart in the current directory
`)

	StepHelmReleaseExample = templates.Examples(`
		jx step helm release

`)
)

func NewCmdStepHelmRelease(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepHelmReleaseOptions{
		StepHelmOptions: StepHelmOptions{
			StepOptions: step.StepOptions{
				CommonOptions: commonOpts,
			},
		},
	}
	cmd := &cobra.Command{
		Use:     "release",
		Short:   "Releases the helm chart in the current directory",
		Aliases: []string{""},
		Long:    StepHelmReleaseLong,
		Example: StepHelmReleaseExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addStepHelmFlags(cmd)
	return cmd
}

func (o *StepHelmReleaseOptions) Run() error {
	dir := o.Dir
	valuesFiles, err := o.discoverValuesFiles(dir)
	if err != nil {
		return err
	}
	_, err = o.HelmInitDependencyBuild(dir, o.DefaultReleaseCharts(), valuesFiles)
	if err != nil {
		return errors.Wrapf(err, "failed to build dependencies for chart from directory '%s'", dir)
	}

	o.Helm().SetCWD(dir)
	err = o.Helm().PackageChart()
	if err != nil {
		return errors.Wrapf(err, "failed to package the chart from directory '%s'", dir)
	}

	chartFile := filepath.Join(dir, "Chart.yaml")
	name, version, err := helm.LoadChartNameAndVersion(chartFile)
	if err != nil {
		return errors.Wrap(err, "failed to load chart name and version")
	}

	if name == "" {
		return fmt.Errorf("Could not find name in chart %s", chartFile)
	}
	if version == "" {
		return fmt.Errorf("Could not find version in chart %s", chartFile)
	}
	tarball := fmt.Sprintf("%s-%s.tgz", name, version)
	exists, err := util.FileExists(tarball)
	if err != nil {
		return errors.Wrapf(err, "don't find the chart archive '%s'", tarball)
	}
	if !exists {
		return fmt.Errorf("Generated helm file %s does not exist!", tarball)
	}
	defer os.Remove(tarball)

	chartRepo := o.ReleaseChartRepositoryURL()

	userName := os.Getenv("CHARTMUSEUM_CREDS_USR")
	password := os.Getenv("CHARTMUSEUM_CREDS_PSW")
	if userName == "" || password == "" {
		// lets try load them from the secret directly
		client, ns, err := o.KubeClientAndNamespace()
		if err != nil {
			return errors.Wrap(err, "failed to create the kube client")
		}
		secret, err := client.CoreV1().Secrets(ns).Get(kube.SecretJenkinsChartMuseum, metav1.GetOptions{})
		if err != nil {
			secret, err = client.CoreV1().Secrets(ns).Get(kube.SecretBucketRepo, metav1.GetOptions{})
		}
		if err != nil {
			log.Logger().Warnf("Could not load Secret %s or %s in namespace %s: %s", kube.SecretJenkinsChartMuseum, kube.SecretBucketRepo, ns, err)
		} else {
			if secret != nil && secret.Data != nil {
				if userName == "" {
					userName = string(secret.Data["BASIC_AUTH_USER"])
				}
				if password == "" {
					password = string(secret.Data["BASIC_AUTH_PASS"])
				}
			}
		}
	}
	if userName == "" {
		return fmt.Errorf("No environment variable $CHARTMUSEUM_CREDS_USR defined")
	}
	if password == "" {
		return fmt.Errorf("No environment variable CHARTMUSEUM_CREDS_PSW defined")
	}

	// post the tarball to the chart repository
	client := http.Client{}

	u := util.UrlJoin(chartRepo, "/api/charts")

	file, err := os.Open(tarball)
	if err != nil {
		return errors.Wrapf(err, "failed to open the chart archive '%s'", tarball)
	}
	log.Logger().Infof("Uploading chart file %s to %s", util.ColorInfo(tarball), util.ColorInfo(u))
	req, err := http.NewRequest(http.MethodPost, u, bufio.NewReader(file))
	if err != nil {
		return errors.Wrapf(err, "failed to build the chart upload request for endpoint '%s'", u)
	}
	req.SetBasicAuth(userName, password)
	req.Header.Set("Content-Type", "application/gzip")
	res, err := client.Do(req)
	if err != nil {
		if res == nil {
			return errors.Wrapf(err, "failed to execute the chart upload HTTP request, url: '%s', error: '%v'", u, err)
		}
		errRes, _ := ioutil.ReadAll(res.Body)
		return errors.Wrapf(err, "failed to execute the chart upload HTTP request, url: '%s', status: '%s', response: '%s'", u, res.Status, string(errRes))
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read the response body of chart upload request")
	}
	responseMessage := string(body)
	statusCode := res.StatusCode
	log.Logger().Infof("Received %d response: %s", statusCode, responseMessage)
	if statusCode >= 300 {
		return fmt.Errorf("Failed to post chart to %s due to response %d: %s", u, statusCode, responseMessage)
	}
	return nil
}
