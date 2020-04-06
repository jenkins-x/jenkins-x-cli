package step

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx/pkg/cmd/opts/step"

	"github.com/jenkins-x/jx/pkg/cmd/helper"

	"github.com/jenkins-x/jx/pkg/buildnum"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/spf13/cobra"
)

const (
	optionBranch = "branch"
	optionOwner  = "owner"
)

// StepNextBuildNumberOptions contains the command line flags
type StepNextBuildNumberOptions struct {
	step.StepOptions

	Owner      string
	Repository string
	Branch     string
}

var (
	StepNextBuildNumberLong = templates.LongDesc(`
		Generates the next build unique number for a pipeline
`)

	StepNextBuildNumberExample = templates.Examples(`
		jx step next-buildnumber 
`)
)

func NewCmdStepNextBuildNumber(commonOpts *opts.CommonOptions) *cobra.Command {
	options := StepNextBuildNumberOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}
	cmd := &cobra.Command{
		Use:     "next-buildnumber",
		Short:   "Generates the next build unique number for a pipeline.",
		Long:    StepNextBuildNumberLong,
		Example: StepNextBuildNumberExample,
		Aliases: []string{"next-buildno"},
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Owner, optionOwner, "o", "", "The Git repository owner")
	cmd.Flags().StringVarP(&options.Repository, optionRepo, "r", "", "The Git repository name")
	cmd.Flags().StringVarP(&options.Branch, optionBranch, "", "master", "The Git branch")
	return cmd
}

func (o *StepNextBuildNumberOptions) Run() error {
	if o.Owner == "" {
		return util.MissingOption(optionOwner)
	}
	if o.Repository == "" {
		return util.MissingOption(optionRepo)
	}

	jxClient, ns, err := o.JXClientAndDevNamespace()
	if err != nil {
		return err
	}
	buildNumGen := buildnum.NewCRDBuildNumGen(jxClient, ns)

	pID := kube.NewPipelineID(o.Owner, o.Repository, o.Branch)

	attempts := 100
	for i := 0; i < attempts; i++ {
		buildNum, err := buildNumGen.NextBuildNumber(pID)
		if err == nil {
			log.Logger().Infof("%s", buildNum)
			return nil
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("Failed after %d attempts to create a new build number for pipeline %s. "+
		"The last error was: %s", attempts, pID.ID, err)
}
