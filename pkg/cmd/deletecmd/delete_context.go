package deletecmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/create/options"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"gopkg.in/AlecAivazis/survey.v1"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	delete_context_long = templates.LongDesc(`
		Deletes one or more Kubernetes contexts.
`)

	delete_context_example = templates.Examples(`
		# Deletes a context for a cluster that no longer exists
		jx delete context something

		# Deletes all contexts containing the word cheese
		# selecting them all by default
		jx delete ctx -a cheese
	`)
)

// DeleteContextOptions the options for the create spring command
type DeleteContextOptions struct {
	options.CreateOptions

	SelectAll      bool
	SelectFilter   string
	DeleteAuthInfo bool
	DeleteCluster  bool
}

// NewCmdDeleteContext creates a command object for the "delete repo" command
func NewCmdDeleteContext(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &DeleteContextOptions{
		CreateOptions: options.CreateOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use:     "contexts",
		Short:   "Deletes one or more Kubernetes contexts",
		Aliases: []string{"context", "ctx"},
		Long:    delete_context_long,
		Example: delete_context_example,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	//addDeleteFlags(cmd, &options.CreateOptions)

	cmd.Flags().BoolVarP(&options.SelectAll, "all", "a", false, "Selects all the matched contexts")
	cmd.Flags().StringVarP(&options.SelectFilter, "filter", "f", "", "Filter the list of contexts to those containing this text")
	cmd.Flags().BoolVarP(&options.DeleteAuthInfo, "delete-user", "", false, "Also delete the user config associated to the context")
	cmd.Flags().BoolVarP(&options.DeleteCluster, "delete-cluster", "", false, "Also delete the cluster config associated to the context")
	return cmd
}

// Run implements the command
func (o *DeleteContextOptions) Run() error {
	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	config, po, err := o.Kube().LoadConfig()
	if err != nil {
		return err
	}

	if config == nil || config.Contexts == nil || len(config.Contexts) == 0 {
		return fmt.Errorf("No Kubernetes contexts available! Try create or connect to cluster?")
	}

	names := []string{}
	allNames := []string{}

	args := o.Args
	for k := range config.Contexts {
		allNames = append(allNames, k)
		if matchesFilter(k, args) {
			names = append(names, k)
		}
	}
	sort.Strings(allNames)

	if len(names) == 0 {
		if len(args) == 0 {
			return fmt.Errorf("Failed to find a context!")
		}
		return util.InvalidArg(args[1], allNames)
	}

	selected, err := util.SelectNamesWithFilter(names, "Select the Kubernetes Contexts to delete: ", o.SelectAll, o.SelectFilter, "", o.GetIOFileHandles())
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return nil
	}

	flag := false
	prompt := &survey.Confirm{
		Message: "Are you sure you want to delete these these Kubernetes Contexts?",
		Default: false,
	}
	err = survey.AskOne(prompt, &flag, nil, surveyOpts)
	if err != nil {
		return err
	}
	if !flag {
		return nil
	}

	newConfig := *config
	for _, name := range selected {
		a := newConfig.Contexts[name].AuthInfo
		if o.DeleteAuthInfo && a != "" {
			log.Logger().Debugf("Deleting user %s for context %s", util.ColorInfo(a), util.ColorInfo(name))
			delete(newConfig.AuthInfos, a)
		}
		c := newConfig.Contexts[name].Cluster
		if o.DeleteCluster && c != "" {
			log.Logger().Debugf("Deleting cluster %s for context %s", util.ColorInfo(c), util.ColorInfo(name))
			delete(newConfig.Clusters, c)
		}

		log.Logger().Debugf("Deleting context %s", util.ColorInfo(name))
		delete(newConfig.Contexts, name)
	}
	err = clientcmd.ModifyConfig(po, newConfig, false)
	if err != nil {
		return fmt.Errorf("Failed to update the kube config %s", err)
	}

	log.Logger().Infof("Deleted Kubernetes contexts: %s", util.ColorInfo(strings.Join(selected, ", ")))
	return nil
}

// matchesFilter returns true if there are no filters or the text contains one of the filters
func matchesFilter(text string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, f := range filters {
		if strings.Contains(text, f) {
			return true
		}
	}
	return false
}
