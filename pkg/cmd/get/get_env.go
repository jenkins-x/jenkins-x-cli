package get

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetEnvOptions containers the CLI options
type GetEnvOptions struct {
	GetOptions

	PromotionStrategy string
	PreviewOnly       bool
}

var (
	getEnvLong = templates.LongDesc(`
		Display one or more environments.
` + helper.SeeAlsoText("jx get previews"))

	getEnvExample = templates.Examples(`
		# List all environments
		jx get environments

		# List all environments using the shorter alias
		jx get env
	`)
)

// NewCmdGetEnv creates the new command for: jx get env
func NewCmdGetEnv(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &GetEnvOptions{
		GetOptions: GetOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "environments",
		Short:   "Display one or more Environments",
		Aliases: []string{"envs", "environment", "env"},
		Long:    getEnvLong,
		Example: getEnvExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}

	options.AddGetFlags(cmd)

	cmd.Flags().StringVarP(&options.PromotionStrategy, "promote", "p", "", "Filters the environments by promotion strategy. Possible values: "+strings.Join(v1.PromotionStrategyTypeValues, ", "))
	cmd.Flags().SetAnnotation("promote", cobra.BashCompCustom, []string{"__jx_get_promotionstrategies"})

	return cmd
}

// Run implements this command
func (o *GetEnvOptions) Run() error {
	client, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	kubeClient, err := o.KubeClient()
	if err != nil {
		return err
	}

	args := o.Args
	if len(args) > 0 {
		e := args[0]
		env, err := client.JenkinsV1().Environments(ns).Get(e, metav1.GetOptions{})
		if err != nil {
			envNames, err := kube.GetEnvironmentNames(client, ns)
			if err != nil {
				return err
			}
			return util.InvalidArg(e, envNames)
		}

		// lets output one environment
		spec := &env.Spec

		table := o.CreateTable()
		table.AddRow("NAME", "LABEL", "KIND", "NAMESPACE", "SOURCE", "REF", "PR")
		table.AddRow(e, spec.Label, spec.Namespace, kindString(spec), spec.Source.URL, spec.Source.Ref, spec.PullRequestURL)
		table.Render()
		log.Blank()

		ens := env.Spec.Namespace
		if ens != "" {
			deps, err := kubeClient.AppsV1().Deployments(ens).List(metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("Could not find deployments in namespace %s: %s", ens, err)
			}
			table = o.CreateTable()
			table.AddRow("APP", "VERSION", "DESIRED", "CURRENT", "UP-TO-DATE", "AVAILABLE", "AGE")
			for _, d := range deps.Items {
				replicas := ""
				if d.Spec.Replicas != nil {
					replicas = formatInt32(*d.Spec.Replicas)
				}
				table.AddRow(d.Name, kube.GetVersion(&d.ObjectMeta), replicas,
					formatInt32(d.Status.ReadyReplicas), formatInt32(d.Status.UpdatedReplicas), formatInt32(d.Status.AvailableReplicas), "")
			}
			table.Render()
		}
	} else {
		envs, err := client.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
		if err != nil {
			return err
		}
		if len(envs.Items) == 0 {
			log.Logger().Infof("No environments found.\nTo create an environment use: jx create env")
			return nil
		}

		environments := o.filterEnvironments(envs.Items)
		kube.SortEnvironments(environments)

		if o.Output != "" {
			envs.Items = environments
			return o.renderResult(envs, o.Output)
		}
		table := o.CreateTable()
		if o.PreviewOnly {
			table.AddRow("PULL REQUEST", "NAMESPACE", "APPLICATION")
		} else {
			table.AddRow("NAME", "LABEL", "KIND", "PROMOTE", "NAMESPACE", "ORDER", "CLUSTER", "SOURCE", "REF", "PR")
		}

		for _, env := range environments {
			spec := &env.Spec
			if o.PreviewOnly {
				table.AddRow(spec.PullRequestURL, spec.Namespace, util.ColorInfo(spec.PreviewGitSpec.ApplicationURL))
			} else {
				table.AddRow(env.Name, spec.Label, kindString(spec), string(spec.PromotionStrategy), spec.Namespace, util.Int32ToA(spec.Order), spec.Cluster, spec.Source.URL, spec.Source.Ref, spec.PullRequestURL)
			}
		}
		table.Render()
	}
	return nil
}

func kindString(spec *v1.EnvironmentSpec) string {
	answer := string(spec.Kind)
	if answer == "" {
		return string(v1.EnvironmentKindTypePermanent)
	}
	return answer
}

func (o *GetEnvOptions) filterEnvironments(envs []v1.Environment) []v1.Environment {
	answer := []v1.Environment{}
	for _, e := range envs {
		preview := e.Spec.Kind == v1.EnvironmentKindTypePreview
		if o.matchesFilter(&e) && preview == o.PreviewOnly {
			answer = append(answer, e)
		}
	}
	return answer
}

func (o *GetEnvOptions) matchesFilter(env *v1.Environment) bool {
	if o.PromotionStrategy == "" {
		return true
	}
	return env.Spec.PromotionStrategy == v1.PromotionStrategyType(o.PromotionStrategy)
}
