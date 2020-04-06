package namespace

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/spf13/cobra"

	"github.com/pkg/errors"

	"github.com/jenkins-x/jx/pkg/kube"

	api_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"sort"

	"github.com/jenkins-x/jx/pkg/util"
	"gopkg.in/AlecAivazis/survey.v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd/api"

	core_v1 "k8s.io/api/core/v1"
)

type NamespaceOptions struct {
	*opts.CommonOptions

	Create bool
}

var (
	namespaceLong = templates.LongDesc(`
		Displays or changes the current namespace.`)
	namespaceExample = templates.Examples(`
		# view the current namespace
		jx --batch-mode ns

		# interactively select the namespace to switch to
		jx ns

		# change the current namespace to 'cheese'
		jx ns cheese

		# change the current namespace to 'brie' creating it if necessary
		jx ns --create brie`)
)

func NewCmdNamespace(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &NamespaceOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "namespace",
		Aliases: []string{"ns"},
		Short:   "View or change the current namespace context in the current Kubernetes cluster",
		Long:    namespaceLong,
		Example: namespaceExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	cmd.Flags().BoolVarP(&options.Create, "create", "c", false, "Creates the specified namespace if it does not exist")
	return cmd
}

func (o *NamespaceOptions) Run() error {
	config, pathOptions, err := o.Kube().LoadConfig()
	if err != nil {
		return errors.Wrap(err, "loading Kubernetes configuration")
	}
	client, err := o.KubeClient()
	if err != nil {
		return errors.Wrap(err, "creating kubernetes client")
	}
	currentNS := kube.CurrentNamespace(config)

	ns := namespace(o)

	if ns == "" && !o.BatchMode {
		ns, err = pickNamespace(o, client, currentNS)
		if err != nil {
			return err
		}
	}

	info := util.ColorInfo
	if ns != "" && ns != currentNS {
		ctx, err := changeNamespace(client, config, pathOptions, ns, o.Create)
		if err != nil {
			return err
		}
		if ctx == nil {
			_, _ = fmt.Fprintf(o.Out, "No kube context - probably in a unit test or pod?\n")
		} else {
			_, _ = fmt.Fprintf(o.Out, "Now using namespace '%s' on server '%s'.\n", info(ctx.Namespace), info(kube.Server(config, ctx)))
		}
	} else {
		ns := kube.CurrentNamespace(config)
		server := kube.CurrentServer(config)
		if config == nil {
			_, _ = fmt.Fprintf(o.Out, "Using namespace '%s' on server '%s'. No context - probably a unit test or pod?\n", info(ns), info(server))

		} else {
			_, _ = fmt.Fprintf(o.Out, "Using namespace '%s' from context named '%s' on server '%s'.\n", info(ns), info(config.CurrentContext), info(server))
		}
	}
	return nil
}

func namespace(o *NamespaceOptions) string {
	ns := ""
	args := o.Args
	if len(args) > 0 {
		ns = args[0]
	}
	return ns
}

func changeNamespace(client kubernetes.Interface, config *api.Config, pathOptions clientcmd.ConfigAccess, ns string, create bool) (*api.Context, error) {
	_, err := client.CoreV1().Namespaces().Get(ns, meta_v1.GetOptions{})
	if err != nil {
		switch err.(type) {
		case *api_errors.StatusError:
			err = handleStatusError(err, client, ns, create)
			if err != nil {
				return nil, err
			}
		default:
			return nil, errors.Wrapf(err, "getting namespace %q", ns)
		}
	}
	newConfig := *config
	ctx := kube.CurrentContext(config)
	if ctx == nil {
		log.Logger().Warnf("there is no context defined in your Kubernetes configuration - we may be inside a test case or pod?\n")
		return ctx, nil
	}
	if ctx.Namespace == ns {
		return ctx, nil
	}
	ctx.Namespace = ns
	err = clientcmd.ModifyConfig(pathOptions, newConfig, false)
	if err != nil {
		return nil, fmt.Errorf("failed to update the kube config %s", err)
	}
	return ctx, nil
}

func handleStatusError(err error, client kubernetes.Interface, ns string, create bool) error {
	statusErr, _ := err.(*api_errors.StatusError)
	if statusErr.Status().Reason == meta_v1.StatusReasonNotFound && create {
		err = createNamespace(client, ns)
		if err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

func createNamespace(client kubernetes.Interface, ns string) error {
	namespace := core_v1.Namespace{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: ns,
		},
	}
	_, err := client.CoreV1().Namespaces().Create(&namespace)
	if err != nil {
		return errors.Wrapf(err, "unable to create namespace %s", ns)
	}
	return nil
}

func pickNamespace(o *NamespaceOptions, client kubernetes.Interface, defaultNamespace string) (string, error) {
	names, err := getNamespaceNames(client)
	if err != nil {
		return "", errors.Wrap(err, "retrieving namespace the names of the namespaces")
	}

	selectedNamespace, err := pick(o, names, defaultNamespace)
	if err != nil {
		return "", errors.Wrap(err, "picking the namespace")
	}
	return selectedNamespace, nil
}

// getNamespaceNames returns the sorted list of environment names
func getNamespaceNames(client kubernetes.Interface) ([]string, error) {
	var names []string
	list, err := client.CoreV1().Namespaces().List(meta_v1.ListOptions{})
	if err != nil {
		return names, fmt.Errorf("loading namespaces %s", err)
	}
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	return names, nil
}

func pick(o *NamespaceOptions, names []string, defaultNamespace string) (string, error) {
	if len(names) == 0 {
		return "", nil
	}
	if len(names) == 1 {
		return names[0], nil
	}
	name := ""
	prompt := &survey.Select{
		Message: "Change namespace:",
		Options: names,
		Default: defaultNamespace,
	}

	surveyOpts := survey.WithStdio(o.In, o.Out, o.Err)
	err := survey.AskOne(prompt, &name, nil, surveyOpts)
	return name, err
}
