package create

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/blang/semver"

	"github.com/jenkins-x/jx/pkg/packages"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultIstioNamespace             = "istio-system"
	defaultIstioReleaseName           = "istio"
	defaultIstioPassword              = "istio"
	defaultIstioConfigDir             = "/istio_service_dir"
	defaultIstioVersion               = ""
	defaultIstioIngressGatewayService = "istio-ingressgateway"
)

var (
	createAddonIstioLong = templates.LongDesc(`
		Creates the istio addon for service mesh on Kubernetes
`)

	createAddonIstioExample = templates.Examples(`
		# Create the istio addon 
		jx create addon istio

		# Create the istio addon in a custom namespace
		jx create addon istio -n mynamespace
	`)
)

// CreateAddonIstioOptions the options for the create spring command
type CreateAddonIstioOptions struct {
	CreateAddonOptions

	Chart                 string
	Password              string
	ConfigDir             string
	NoInjectorWebhook     bool
	Dir                   string
	IngressGatewayService string
}

// NewCmdCreateAddonIstio creates a command object for the "create" command
func NewCmdCreateAddonIstio(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonIstioOptions{
		CreateAddonOptions: CreateAddonOptions{
			CreateOptions: options.CreateOptions{
				CommonOptions: commonOpts,
			},
		},
	}

	cmd := &cobra.Command{
		Use:     "istio",
		Short:   "Create the Istio addon for service mesh",
		Aliases: []string{"env"},
		Long:    createAddonIstioLong,
		Example: createAddonIstioExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, DefaultIstioNamespace, defaultIstioReleaseName, defaultIstioVersion)

	cmd.Flags().StringVarP(&options.Password, "password", "p", defaultIstioPassword, "The default password to use for Istio")
	cmd.Flags().StringVarP(&options.ConfigDir, "config-dir", "d", defaultIstioConfigDir, "The config directory to use")
	cmd.Flags().StringVarP(&options.Chart, optionChart, "c", "", "The name of the chart to use")
	cmd.Flags().BoolVarP(&options.NoInjectorWebhook, "no-injector-webhook", "", false, "Disables the injector webhook")
	cmd.Flags().StringVarP(&options.IngressGatewayService, "ingress-gateway-service", "", defaultIstioIngressGatewayService, "The name of the ingress gateway service created by Istio")
	return cmd
}

// Run implements the command
func (o *CreateAddonIstioOptions) Run() error {
	if o.Chart == "" {
		// lets git clone the source to find the istio charts as they are not published anywhere yet
		dir, err := o.getIstioChartsFromGitHub()
		if err != nil {
			return err
		}
		chartDir := filepath.Join(dir, kube.ChartIstio)
		exists, err := util.DirExists(chartDir)
		if !exists {
			return fmt.Errorf("Could not find folder %s inside istio clone at %s", kube.ChartIstio, dir)
		}
		o.Dir = dir
		o.Chart = kube.ChartIstio
	}
	if o.ReleaseName == "" {
		return util.MissingOption(optionRelease)
	}
	if o.Chart == "" {
		return util.MissingOption(optionChart)
	}
	err := o.EnsureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that Helm is present")
	}
	client, err := o.KubeClient()
	if err != nil {
		return err
	}

	err = o.generateSecrets()
	if err != nil {
		return err
	}

	_, devNamespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return fmt.Errorf("cannot find a dev team namespace to get existing exposecontroller config from. %v", err)
	}

	log.Logger().Infof("found dev namespace %s", devNamespace)

	values := []string{}
	if o.NoInjectorWebhook {
		values = append(values, "sidecarInjectorWebhook.enabled=false")
	}
	setValues := strings.Split(o.SetValues, ",")
	values = append(values, setValues...)
	log.Logger().Infof("installing istio-init")
	err = o.InstallChartAt(o.Dir, o.ReleaseName, o.Chart+"-init", o.Version, o.Namespace, true, values, nil, "")
	if err != nil {
		return fmt.Errorf("istio-init deployment failed: %v", err)
	}
	log.Logger().Infof("installing istio")
	err = o.InstallChartAt(o.Dir, o.ReleaseName, o.Chart, o.Version, o.Namespace, true, values, nil, "")
	if err != nil {
		return fmt.Errorf("istio deployment failed: %v", err)
	}

	// get the ip of the Istio ingress gateway
	c := make(chan string, 1)
	go func() {
		for {
			svc, err := client.CoreV1().Services(o.Namespace).Get(o.IngressGatewayService, metav1.GetOptions{})
			if err != nil {
				log.Logger().Warnf("Error getting Istio ingress gateway %s/%s: %s", o.Namespace, o.IngressGatewayService, err)
				c <- ""
			} else {
				if len(svc.Status.LoadBalancer.Ingress) > 0 {
					c <- svc.Status.LoadBalancer.Ingress[0].IP
					return
				}
				log.Logger().Infof("Waiting for Istio ingress gateway ip %s/%s", o.Namespace, o.IngressGatewayService)
			}
			time.Sleep(5 * time.Second)
		}
	}()

	select {
	case ip := <-c:
		if ip != "" {
			log.Logger().Infof("Istio ingress gateway service ip: %s", ip)
		}
	case <-time.After(1 * time.Minute):
		log.Logger().Infof("Istio ingress gateway service ip is not yet ready, you can get it with `kubectl -n istio-system get service istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}'`")
	}

	return nil
}

// getIstioChartsFromGitHub lets download the release of istio that we need
func (o *CreateAddonIstioOptions) getIstioChartsFromGitHub() (string, error) {
	answer := ""
	var err error
	var actualVersion semver.Version
	if o.Version == "" {
		actualVersion, err = util.GetLatestVersionFromGitHub("istio", "istio")
		if err != nil {
			return answer, fmt.Errorf("unable to get %s version for github.com/%s/%s %v", o.Version, "istio", "istio", err)
		}
	} else {
		actualVersion, err = semver.Make(o.Version)
		if err != nil {
			return answer, fmt.Errorf("unable to parse version %s %v", o.Version, err)
		}
	}

	binDir, err := util.JXBinLocation()
	if err != nil {
		return answer, err
	}
	cacheDir, err := util.CacheDir()
	if err != nil {
		return answer, err
	}

	binaryFile := "istioctl"
	extension := ""
	switch runtime.GOOS {
	case "windows":
		extension = "win.zip"
		binaryFile += ".exe"
	case "darwin":
		extension = "osx.tar.gz"
	default:
		extension = "linux.tar.gz"
	}

	clientURL := fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istio-%s-%s", actualVersion, actualVersion, extension)

	outputDir := filepath.Join(cacheDir, "istio-"+actualVersion.String())
	os.RemoveAll(outputDir)

	answer = outputDir

	err = os.MkdirAll(outputDir, util.DefaultWritePermissions)
	if err != nil {
		return answer, err
	}

	tarPath := filepath.Join(cacheDir, fmt.Sprintf("istio-%s-%s", actualVersion, extension))
	fi, err := os.Stat(tarPath)
	if os.IsNotExist(err) || fi.Size() == 0 {
		err = packages.DownloadFile(clientURL, tarPath)
		if err != nil {
			return answer, err
		}
	} else {
		log.Logger().Infof("Istio package already downloaded: %s", tarPath)
	}

	if strings.HasSuffix(extension, ".zip") {
		err = util.Unzip(tarPath, cacheDir)
		if err != nil {
			return answer, err
		}
	} else {
		err = util.UnTargzAll(tarPath, cacheDir)
		if err != nil {
			return answer, err
		}
	}
	f := filepath.Join(outputDir, "bin", binaryFile)
	exists, err := util.FileExists(f)
	if err != nil {
		return answer, err
	}
	if exists {
		binaryDest := filepath.Join(binDir, binaryFile)
		os.Remove(binaryDest)
		err = os.Rename(f, binaryDest)
		if err != nil {
			return answer, err
		}
	}
	return answer, nil
}

func (o *CreateAddonIstioOptions) generateSecrets() error {
	// generate secret for kiali && grafana
	return nil
}
