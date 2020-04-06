package step

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	"github.com/rollout/rox-go/core/utils"

	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicateOptions contains the command line flags
type ReplicateOptions struct {
	step.StepOptions
	ReplicateToNamepace []string
	CreateNamespace     bool
}

const (
	annotationReplicationAllowed           = "replicator.v1.mittwald.de/replication-allowed"
	annotationEeplicationAllowedNamespaces = "replicator.v1.mittwald.de/replication-allowed-namespaces"
	configMap                              = "configmap"
	secret                                 = "secret" // pragma: allowlist secret
	replicateToNamespaceFlag               = "replicate-to-namespace"

	// Description common text for long command descriptions around storage
	Description = `
Annotates a secret or configmap so it can be replicated across an environment
`
)

var (
	stepReplicateLong = templates.LongDesc(`

Works with the replicator app https://github.com/jenkins-x-charts/kubernetes-replicator

jx add app replicator

This step will annotate a secret or configmap so that the replicator can replicate the data into another namespace  
`)

	stepReplicateExample = templates.Examples(`
		NOTE: quote and namespaces that include a wildcard
		# lets collect some files to the team's default storage location (which if not configured uses the current git repository's gh-pages branch)
		jx step replicate configmap foo --replicate-to-namespace jx-staging --replicate-to-namespace "foo-preview*"

		# lets collect some files to the team's default storage location (which if not configured uses the current git repository's gh-pages branch)
		jx step replicate secret bar --replicate-to-namespace jx-staging --replicate-to-namespace "foo-preview*"

`)
)

// NewCmdStepReplicate creates the CLI command
func NewCmdStepReplicate(commonOpts *opts.CommonOptions) *cobra.Command {
	options := ReplicateOptions{
		StepOptions: step.StepOptions{

			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "replicate",
		Short:   Description,
		Long:    stepReplicateLong,
		Example: stepReplicateExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringArrayVarP(&options.ReplicateToNamepace, replicateToNamespaceFlag, "r", nil, "Specify a list of namespaces to replicate data into")
	cmd.Flags().BoolVarP(&options.CreateNamespace, "create-namespace", "", false, "Should create any missing namespaces")

	return cmd
}

// Run runs the command
func (o *ReplicateOptions) Run() error {
	if len(o.Args) != 2 {
		return util.MissingArgument("configmap or secret")
	}
	if o.Args[0] != secret && o.Args[0] != configMap {
		return util.MissingArgument("configmap or secret")
	}
	if len(o.ReplicateToNamepace) == 0 {
		return util.MissingOption(replicateToNamespaceFlag)
	}

	client, currentNamespace, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	for _, ns := range o.ReplicateToNamepace {
		//  if there's a wildcard in the name let's not validate it exists
		if strings.Contains(ns, "*") {
			continue
		}
		_, err := client.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})
		if err != nil {
			if o.CreateNamespace {
				err = kube.EnsureNamespaceCreated(client, ns, nil, nil)
				if err != nil {
					return err
				}
			} else {
				return util.InvalidOptionError(replicateToNamespaceFlag, ns, err)
			}
		}
	}
	resourceType := o.Args[0]
	resourceName := o.Args[1]

	if resourceType == secret {
		// find all secrets issued by letsencrypt
		if strings.Contains(resourceName, "*") {
			secrets, err := client.CoreV1().Secrets(currentNamespace).List(metav1.ListOptions{})
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("unable to list secrets '%s'", resourceName))
			}

			re := regexp.MustCompile(resourceName)

			for _, secretItem := range secrets.Items {
				if re.MatchString(secretItem.Name) {
					secret, err := client.CoreV1().Secrets(currentNamespace).Get(secretItem.Name, metav1.GetOptions{})
					if err != nil {
						return errors.Wrap(err, fmt.Sprintf("unable to get secret '%s'", secretItem.Name))
					}

					secret.Annotations = setReplicatorAnnotations(secret.Annotations, o.ReplicateToNamepace)
					_, err = client.CoreV1().Secrets(currentNamespace).Update(secret)
					if err != nil {
						return errors.Wrap(err, fmt.Sprintf("unable to update secret '%s'", secretItem.Name))
					}
				}
			}
		} else {
			secret, err := client.CoreV1().Secrets(currentNamespace).Get(resourceName, metav1.GetOptions{})
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("unable to get secret '%s'", resourceName))
			}
			secret.Annotations = setReplicatorAnnotations(secret.Annotations, o.ReplicateToNamepace)
			_, err = client.CoreV1().Secrets(currentNamespace).Update(secret)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("unable to update secret '%s'", resourceName))
			}
		}
	}

	if resourceType == configMap {
		cm, err := client.CoreV1().ConfigMaps(currentNamespace).Get(resourceName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("unable to get configmap %s", resourceName))
		}
		cm.Annotations = setReplicatorAnnotations(cm.Annotations, o.ReplicateToNamepace)
		_, err = client.CoreV1().ConfigMaps(currentNamespace).Update(cm)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("unable to update configmap %s", resourceName))
		}
	}
	return nil
}

func setReplicatorAnnotations(annotations map[string]string, namespaces []string) map[string]string {
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotationReplicationAllowed] = "true"
	allowedNamespaces := annotations[annotationEeplicationAllowedNamespaces]
	var nsSlice []string
	if allowedNamespaces != "" {
		nsSlice = strings.Split(allowedNamespaces, ",")
	}
	for _, replicateToNamespace := range namespaces {
		if !utils.ContainsString(nsSlice, replicateToNamespace) {
			nsSlice = append(nsSlice, replicateToNamespace)
		}
	}
	annotations[annotationEeplicationAllowedNamespaces] = strings.Join(nsSlice, ",")

	return annotations
}
