package get

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
)

// CRDCountOptions the command line options
type CRDCountOptions struct {
	*opts.CommonOptions
}

type tableLine struct {
	name      string
	version   string
	count     int
	namespace string
}

var (
	getCrdCountLong = templates.LongDesc(`
		Count the number of resources for all custom resources definitions

`)

	getCrdCountExample = templates.Examples(`

		# Count the number of resources for all custom resources definitions
		jx get crd count
	`)
)

// NewCmdGetCRDCount creates the command object
func NewCmdGetCRDCount(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &CRDCountOptions{
		CommonOptions: commonOpts,
	}

	cmd := &cobra.Command{
		Use:     "crd count",
		Short:   "Display resources count for all custom resources",
		Long:    getCrdCountLong,
		Example: getCrdCountExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

// Run implements this command
func (o *CRDCountOptions) Run() error {
	results, err := o.getCustomResourceCounts()
	if err != nil {
		return errors.Wrap(err, "cannot get custom resource counts")
	}

	table := o.CreateTable()
	table.AddRow("NAME", "VERSION", "COUNT", "NAMESPACE")

	for _, r := range results {
		table.AddRow(r.name, r.version, strconv.Itoa(r.count), r.namespace)
	}

	table.Render()
	return nil
}

func (o *CRDCountOptions) getCustomResourceCounts() ([]tableLine, error) {

	exClient, err := o.ApiExtensionsClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get api extensions client")
	}
	dynamicClient, _, err := o.GetFactory().CreateDynamicClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dynamic client")
	}

	// lets loop over each arg and validate they are resources, note we could have "--all"
	crdList, err := exClient.ApiextensionsV1beta1().CustomResourceDefinitions().List(v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get a list of custom resource definitions")
	}

	kclient, _, err := o.GetFactory().CreateKubeClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get kubernetes client")
	}

	// get the list of namespaces the user has access to
	namespaces, err := kclient.CoreV1().Namespaces().List(v1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list namespaces")
	}

	log.Logger().Info("Looking for cluster wide custom resource counts and namespaced custom resources in these namespaces:")
	for _, namespace := range namespaces.Items {
		log.Logger().Infof("%s", util.ColorInfo(namespace.Name))
	}
	log.Logger().Info("this operation may take a while depending on how many custom resources exist")

	var results []tableLine
	// loop over each crd and check how many resources exist for them
	for _, crd := range crdList.Items {
		// each crd can have multiple versions
		for _, v := range crd.Spec.Versions {
			r := schema.GroupVersionResource{Group: crd.Spec.Group, Version: v.Name, Resource: crd.Spec.Names.Plural}

			if crd.Spec.Scope == v1beta1.ClusterScoped {
				// get cluster scoped resources
				resources, err := dynamicClient.Resource(r).List(v1.ListOptions{})
				if err != nil {
					return nil, errors.Wrapf(err, "finding resource %s.%s %s", crd.Spec.Names.Plural, crd.Spec.Group, v.Name)
				}

				results = o.addLine(crd, v, resources, results, "cluster scoped")
			} else if crd.Spec.Scope == v1beta1.NamespaceScoped {
				// get namespaced scoped resources
				for _, n := range namespaces.Items {

					resources, err := dynamicClient.Resource(r).Namespace(n.Name).List(v1.ListOptions{})
					if err != nil {
						return nil, errors.Wrapf(err, "finding resource %s.%s %s", crd.Spec.Names.Plural, crd.Spec.Group, v.Name)
					}

					results = o.addLine(crd, v, resources, results, n.Name)
				}
			}
		}
	}
	// sort the entries so resources with the most come at the bottom as it's clearer to see after running the command
	sort.Slice(results, func(i, j int) bool {
		return results[i].count < results[j].count
	})
	return results, nil
}

func (o *CRDCountOptions) addLine(crd v1beta1.CustomResourceDefinition, v v1beta1.CustomResourceDefinitionVersion, resources *unstructured.UnstructuredList, results []tableLine, namespace string) []tableLine {
	line := tableLine{
		name:      fmt.Sprintf("%s.%s", crd.Spec.Names.Plural, crd.Spec.Group),
		version:   v.Name,
		count:     len(resources.Items),
		namespace: namespace,
	}
	return append(results, line)
}
