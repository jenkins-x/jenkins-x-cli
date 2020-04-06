package upgrade

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/create"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/ghodss/yaml"
	"github.com/jenkins-x/jx/pkg/addon"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	upgradeAddonsLong = templates.LongDesc(`
		Upgrades any Addons added to Jenkins X if there are any new releases available
`)

	upgradeAddonsExample = templates.Examples(`
		# Upgrades any Addons added to Jenkins X
		jx upgrade addons
	`)
)

// UpgradeAddonsOptions the options for the create spring command
type UpgradeAddonsOptions struct {
	options.CreateOptions

	Namespace   string
	Set         string
	VersionsDir string

	InstallFlags create.InstallFlags
}

// NewCmdUpgradeAddons defines the command
func NewCmdUpgradeAddons(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &UpgradeAddonsOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "addons",
		Short:   "Upgrades any Addons added to Jenkins X if there are any new releases available",
		Aliases: []string{"addon"},
		Long:    upgradeAddonsLong,
		Example: upgradeAddonsExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	options.addFlags(cmd)
	options.InstallFlags.AddCloudEnvOptions(cmd)

	cmd.AddCommand(NewCmdUpgradeAddonProw(commonOpts))

	return cmd
}

func (options *UpgradeAddonsOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "The Namespace to upgrade")
	cmd.Flags().StringVarP(&options.Set, "set", "s", "", "The Helm parameters to pass in while upgrading")
	cmd.Flags().StringVarP(&options.VersionsDir, "versions-dir", "", "", "The directory containing the versions repo")
}

// Run implements the command
func (o *UpgradeAddonsOptions) Run() error {
	err := o.Helm().UpdateRepo()
	if err != nil {
		return err
	}

	_, _, err = o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}

	ns := o.Namespace
	if ns == "" {
		_, ns, err = o.JXClientAndDevNamespace()
		if err != nil {
			return err
		}
	}

	addonConfig, err := addon.LoadAddonsConfig()
	if err != nil {
		return err
	}
	addonEnabled := map[string]bool{}
	for _, addon := range addonConfig.Addons {
		if addon.Enabled {
			addonEnabled[addon.Name] = true
		}
	}
	releases, _, err := o.Helm().ListReleases(ns)
	if err != nil {
		log.Logger().Warnf("Failed to find Helm installs: %s", err)
	}

	charts := kube.AddonCharts
	keys := []string{}
	if len(o.Args) > 0 {
		for _, k := range o.Args {
			chart := charts[k]
			if chart == "" {
				return errors.New(fmt.Sprintf("Could not find addon %s in the list of addons", k))
			}
			status := releases[k].Status
			if status == "" {
				return errors.New(fmt.Sprintf("Could not find %s chart %s installed in namespace %s\n", util.ColorInfo(k), util.ColorInfo(chart), util.ColorInfo(ns)))
			}
			keys = append(keys, k)
		}
	} else {
		keys = util.SortedMapKeys(charts)
	}

	for _, k := range keys {
		if k == "jx-prow" {
			//installing prow
			log.Logger().Infof("Upgrading prow")
			upgradeAddonProwOptions := &UpgradeAddonProwOptions{
				UpgradeAddonsOptions: *o,
			}
			upgradeAddonProwOptions.Upgrade()
			continue
		}
		chart := charts[k]
		status := releases[k].Status
		name := k
		if status != "" {
			log.Logger().Infof("Upgrading %s chart %s...", util.ColorInfo(name), util.ColorInfo(chart))

			valueFiles := []string{}
			valueFiles, err = helm.AppendMyValues(valueFiles)
			if err != nil {
				return errors.Wrap(err, "failed to append the myvalues.yaml file")
			}

			values := []string{}
			if o.Set != "" {
				values = append(values, o.Set)
			}

			config := &v1.ConfigMap{}
			plugins := &v1.ConfigMap{}
			if k == kube.DefaultProwReleaseName {
				// lets backup any Prow config as we should never replace this, eventually we'll move config to a git repo so this is temporary
				config, plugins, err = o.backupConfigs()
				if err != nil {
					return errors.Wrap(err, "backing up the prow config")
				}
			}
			helmOptions := helm.InstallChartOptions{
				Chart:          chart,
				ReleaseName:    k,
				Ns:             ns,
				NoForce:        true,
				ValueFiles:     valueFiles,
				SetValues:      values,
				UpgradeOnly:    true,
				VersionsDir:    o.VersionsDir,
				VersionsGitURL: o.InstallFlags.VersionsRepository,
				VersionsGitRef: o.InstallFlags.VersionsGitRef,
			}
			// TODO this will fail to upgrade file system charts like Istio
			err = o.InstallChartWithOptions(helmOptions)
			if err != nil {
				return errors.Wrapf(err, "Failed to upgrade %s chart %s: %v\n", name, chart, err)
			}

			if k == kube.DefaultProwReleaseName {
				err = o.restoreConfigs(config, plugins)
				if err != nil {
					return err
				}
			}

			log.Logger().Infof("Upgraded %s chart %s", util.ColorInfo(name), util.ColorInfo(chart))
		}
	}
	return nil
}

func (o *UpgradeAddonsOptions) restoreConfigs(config *v1.ConfigMap, plugins *v1.ConfigMap) error {
	client, devNamespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}
	var err1 error
	if config != nil {
		_, err = client.CoreV1().ConfigMaps(devNamespace).Get("config", metav1.GetOptions{})
		if err != nil {
			_, err = client.CoreV1().ConfigMaps(devNamespace).Create(config)
			if err != nil {
				b, _ := yaml.Marshal(config)
				err1 = fmt.Errorf("error restoring config %s\n", string(b))
			}
		}
	}
	if plugins != nil {
		_, err = client.CoreV1().ConfigMaps(devNamespace).Get("plugins", metav1.GetOptions{})
		if err != nil {
			_, err = client.CoreV1().ConfigMaps(devNamespace).Create(plugins)
			if err != nil {
				b, _ := yaml.Marshal(plugins)
				err = fmt.Errorf("%v/nerror restoring plugins %s\n", err1, string(b))
			}
		}
	}
	return err
}

func (o *UpgradeAddonsOptions) backupConfigs() (*v1.ConfigMap, *v1.ConfigMap, error) {
	client, devNamespace, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return nil, nil, err
	}
	config, _ := client.CoreV1().ConfigMaps(devNamespace).Get("config", metav1.GetOptions{})
	plugins, _ := client.CoreV1().ConfigMaps(devNamespace).Get("plugins", metav1.GetOptions{})
	config = config.DeepCopy()
	config.ResourceVersion = ""
	plugins = plugins.DeepCopy()
	plugins.ResourceVersion = ""
	return config, plugins, nil
}
