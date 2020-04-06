package create

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pborman/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

type CreateAddonPrometheusOptions struct {
	options.CreateOptions

	Namespace   string
	Version     string
	ReleaseName string
	HelmUpdate  bool
	SetValues   string
	Password    string
}

func NewCmdCreateAddonPrometheus(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CreateAddonPrometheusOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:   "prometheus",
		Short: "Creates a prometheus addon",
		Long: `Creates a prometheus addon.

By default Prometheus Server is exposed via Ingress entry http://prometheus.jx.your.domain.com secured
with basic authentication. Admin username is 'admin' and default password is 'admin' (see --password flag).
`,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.addFlags(cmd, kube.DefaultNamespace, "prometheus")
	return cmd
}

func (options *CreateAddonPrometheusOptions) addFlags(cmd *cobra.Command, defaultNamespace string, defaultOptionRelease string) {
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", defaultNamespace, "The Namespace to install into")
	cmd.Flags().StringVarP(&options.ReleaseName, optionRelease, "r", defaultOptionRelease, "The chart release name")
	cmd.Flags().BoolVarP(&options.HelmUpdate, "helm-update", "", true, "Should we run helm update first to ensure we use the latest version")
	cmd.Flags().StringVarP(&options.SetValues, "set", "s", "", "The chart set values (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().StringVarP(&options.Password, "password", "", "admin", "Admin password to access Prometheus web UI.")
}

func (o *CreateAddonPrometheusOptions) Run() error {
	err := o.EnsureHelm()
	if err != nil {
		return errors.Wrap(err, "failed to ensure that helm is present")
	}

	data := make(map[string][]byte)
	hash := util.HashPassword(o.Password)
	data[kube.AUTH] = []byte(fmt.Sprintf("admin:{SHA}%s", hash))
	sec := &core_v1.Secret{
		Data: data,
		ObjectMeta: v1.ObjectMeta{
			Name: "prometheus-ingress",
		},
	}
	client, err := o.KubeClient()
	if err != nil {
		return err
	}
	_, err = client.CoreV1().Secrets(o.Namespace).Create(sec)
	if err != nil {
		return fmt.Errorf("cannot create secret %s in target namespace %s: %v", "prometheus-ingress", o.Namespace, err)
	}

	ingressConfig, err := client.CoreV1().ConfigMaps(o.Namespace).Get("ingress-config", meta_v1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Cannot get ingress config map.")
	}

	values := map[string]map[string]map[string]interface{}{
		"server": {
			"ingress": {
				"enabled": true,
				"hosts":   []string{"prometheus.jx." + ingressConfig.Data["domain"]},
				"annotations": map[string]string{
					"kubernetes.io/ingress.class":             "nginx",
					"nginx.ingress.kubernetes.io/auth-type":   "basic",
					"nginx.ingress.kubernetes.io/auth-secret": "prometheus-ingress",
					"nginx.ingress.kubernetes.io/auth-realm":  "Authentication required to access Prometheus.",
				},
			},
		},
	}
	valuesBytes, err := yaml.Marshal(values)
	if err != nil {
		return err
	}
	prometheusIngressConfig := path.Join("/tmp", "prometheusIngressConfig_"+uuid.New())
	err = ioutil.WriteFile(prometheusIngressConfig, valuesBytes, 0644)
	if err != nil {
		return err
	}

	setValues := strings.Split(o.SetValues, ",")
	err = o.InstallChartWithOptions(helm.InstallChartOptions{
		ReleaseName: o.ReleaseName,
		Chart:       "stable/prometheus",
		Version:     o.Version,
		Ns:          o.Namespace,
		HelmUpdate:  o.HelmUpdate,
		ValueFiles:  []string{prometheusIngressConfig},
		SetValues:   setValues,
	})
	if err != nil {
		return fmt.Errorf("Failed to install chart %s: %s", "prometheus", err)
	}
	return nil
}
