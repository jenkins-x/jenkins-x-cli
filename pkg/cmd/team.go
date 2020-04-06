package cmd

import (
	"fmt"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/spf13/cobra"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/kube"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
)

type TeamOptions struct {
	*opts.CommonOptions
}

var (
	teamLong = templates.LongDesc(`
		Displays or changes the current team.

		For more documentation on Teams see: [https://jenkins-x.io/about/features/#teams](https://jenkins-x.io/about/features/#teams)

`)
	teamExample = templates.Examples(`
		# view the current team
		jx team -b

		# pick which team to switch to
		jx team

		# Change the current team to 'cheese'
		jx team cheese
`)
)

func NewCmdTeam(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &TeamOptions{
		CommonOptions: commonOpts,
	}
	cmd := &cobra.Command{
		Use:     "team",
		Aliases: []string{"env"},
		Short:   "View or change the current team in the current Kubernetes cluster",
		Long:    teamLong,
		Example: teamExample,
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	return cmd
}

func (o *TeamOptions) Run() error {
	kubeClient, currentTeam, err := o.KubeClientAndNamespace()
	if err != nil {
		return err
	}
	apisClient, err := o.ApiExtensionsClient()
	if err != nil {
		return err
	}
	kube.RegisterEnvironmentCRD(apisClient)
	_, teamNames, err := kube.GetTeams(kubeClient)
	if err != nil {
		return err
	}

	config, po, err := o.Kube().LoadConfig()
	if err != nil {
		return err
	}
	team := ""
	args := o.Args
	if len(args) > 0 {
		team = args[0]
	}
	if team == "" && !o.BatchMode {
		pick, err := util.PickName(teamNames, "Pick Team: ", "", o.GetIOFileHandles())
		if err != nil {
			return err
		}
		team = pick
	}
	info := util.ColorInfo
	if team != "" && team != currentTeam {
		newConfig := *config
		ctx := kube.CurrentContext(config)
		if ctx == nil {
			return errors.New("there is no context defined in your Kubernetes configuration")
		}
		if ctx.Namespace == team {
			return nil
		}
		ctx.Namespace = team
		err = clientcmd.ModifyConfig(po, newConfig, false)
		if err != nil {
			return fmt.Errorf("Failed to update the kube config %s", err)
		}
		fmt.Fprintf(o.Out, "Now using team '%s' on server '%s'.\n", info(team), info(kube.Server(config, ctx)))
	} else {
		ns := kube.CurrentNamespace(config)
		server := kube.CurrentServer(config)
		if team == "" {
			team = currentTeam
		}
		if team == "" {
			fmt.Fprintf(o.Out, "Using namespace '%s' from context named '%s' on server '%s'.\n", info(ns), info(config.CurrentContext), info(server))
		} else {
			fmt.Fprintf(o.Out, "Using team '%s' on server '%s'.\n", info(team), info(server))
		}
	}
	return nil
}
