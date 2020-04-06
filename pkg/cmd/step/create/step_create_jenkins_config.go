package create

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	createJenkinsConfigLong = templates.LongDesc(`
		Creates the Jenkins config.xml file from a number of ConfigMaps for Pod Templates
`)

	createJenkinsConfigExample = templates.Examples(`
		jx step create jenkins config

			`)
)

// StepCreateJenkinsConfigOptions contains the command line flags
type StepCreateJenkinsConfigOptions struct {
	step.StepOptions

	Output string
}

// NewCmdStepCreateJenkinsConfig Creates a new Command object
func NewCmdStepCreateJenkinsConfig(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepCreateJenkinsConfigOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "jenkins config",
		Short:   "Creates the Jenkins config.xml file from a number of ConfigMaps for Pod Templates",
		Long:    createJenkinsConfigLong,
		Example: createJenkinsConfigExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.Output, "output", "o", "config.xml", "the output file generated")
	return cmd
}

// Run implements this command
func (o *StepCreateJenkinsConfigOptions) Run() error {
	kubeClient, ns, err := o.KubeClientAndDevNamespace()
	if err != nil {
		return err
	}
	configMapInterface := kubeClient.CoreV1().ConfigMaps(ns)
	selector := kube.LabelKind + "=" + kube.ValueKindPodTemplateXML
	list, err := configMapInterface.List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to load Pod Template XML ConfigMaps with selector %s in namespace %s", selector, ns)
	}
	cmName := kube.SecretJenkins
	cm, err := configMapInterface.Get(cmName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to load ConfigMap %s in namespace %s", cmName, ns)
	}

	configXml := ""
	if cm.Data != nil {
		configXml = cm.Data["config.xml"]
	}
	if configXml == "" {
		return fmt.Errorf("no config.xml key in ConfigMap %s in namespace %s", cmName, ns)
	}

	var buffer strings.Builder
	found := false
	lines := strings.Split(configXml, "\n")
	for _, line := range lines {
		buffer.WriteString(line)
		buffer.WriteString("\n")
		if strings.TrimSpace(line) == "<templates>" && !found {
			found = true
			for _, cm := range list.Items {
				data := cm.Data
				if data != nil {
					configXML := data["config.xml"]
					if configXML != "" {
						for _, cl := range strings.Split(configXML, "\n") {
							buffer.WriteString("        ")
							buffer.WriteString(cl)
							buffer.WriteString("\n")
						}
					}
				}
			}
		}
	}

	fileXML := buffer.String()

	err = ioutil.WriteFile(o.Output, []byte(fileXML), util.DefaultWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", o.Output)
	}
	log.Logger().Infof("generated Jenkins configuration file %s", util.ColorInfo(o.Output))
	return nil
}
