package deletecmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	survey "gopkg.in/AlecAivazis/survey.v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteNamespaceOptions are the flags for delete commands
type DeleteNamespaceOptions struct {
	*opts.CommonOptions

	SelectAll    bool
	SelectFilter string
	Confirm      bool
}

var (
	deleteNamespaceLong = templates.LongDesc(`
		Deletes one or more namespaces
`)

	deleteNamespaceExample = templates.Examples(`
		# Delete the named namespace
		jx delete namespace cheese 

		# Delete the namespaces matching the given filter
		jx delete namespace -f foo -a
	`)
)

// NewCmdDeleteNamespace creates a command object
// retrieves one or more resources from a server.
func NewCmdDeleteNamespace(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteNamespaceOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "namespace",
		Short:   "Deletes one or more namespaces and their associated resources (Environments, Jenkins etc)",
		Long:    deleteNamespaceLong,
		Example: deleteNamespaceExample,
		Aliases: []string{"namespaces", "ns"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().BoolVarP(&options.SelectAll, "all", "a", false, "Should we default to selecting all the matched namespaces for deletion")
	cmd.Flags().StringVarP(&options.SelectFilter, "filter", "f", "", "Filters the list of namespaces you can pick from")
	cmd.Flags().BoolVarP(&options.Confirm, "yes", "y", false, "Confirms we should uninstall this installation")
	return cmd
}

// Run implements this command
func (o *DeleteNamespaceOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}
	namespaceInterface := kubeClient.CoreV1().Namespaces()
	nsList, err := namespaceInterface.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	namespaceNames := []string{}
	for _, namespace := range nsList.Items {
		namespaceNames = append(namespaceNames, namespace.Name)
	}
	sort.Strings(namespaceNames)

	names := o.Args
	if len(names) == 0 {
		if o.BatchMode {
			return fmt.Errorf("Missing namespace name argument")
		}
		names, err = util.SelectNamesWithFilter(namespaceNames, "Which namespaces do you want to delete: ", o.SelectAll, o.SelectFilter, "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
	}

	if o.BatchMode {
		if !o.Confirm {
			return fmt.Errorf("In batch mode you must specify the '-y' flag to confirm")
		}
	} else {
		log.Logger().Warnf("You are about to delete the following namespaces '%s'. This operation CANNOT be undone!",
			strings.Join(names, ","))

		flag := false
		prompt := &survey.Confirm{
			Message: "Are you sure you want to delete all these namespaces?",
			Default: false,
		}
		err = survey.AskOne(prompt, &flag, nil, surveyOpts)
		if err != nil {
			return err
		}
		if !flag {
			return nil
		}
	}

	for _, name := range names {
		log.Logger().Infof("Deleting namespace: %s", util.ColorInfo(name))
		err = namespaceInterface.Delete(name, nil)
		if err != nil {
			log.Logger().Warnf("Failed to delete namespace %s: %s", name, err)
		}
	}
	return nil
}
