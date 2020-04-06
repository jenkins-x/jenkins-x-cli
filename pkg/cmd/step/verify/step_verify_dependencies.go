package verify

import (
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/opts/step"
	"github.com/jenkins-x/jx/pkg/dependencymatrix"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// StepVerifyDependenciesOptions contains the command line flags
type StepVerifyDependenciesOptions struct {
	step.StepOptions
	Dir string
}

// NewCmdStepVerifyDependencies creates the `jx step verify pod` command
func NewCmdStepVerifyDependencies(commonOpts *opts.CommonOptions) *cobra.Command {
	options := &StepVerifyDependenciesOptions{
		StepOptions: step.StepOptions{
			CommonOptions: commonOpts,
		},
	}

	cmd := &cobra.Command{
		Use: "dependencies",
		Run: func(cmd *cobra.Command, args []string) {
			options.Cmd = cmd
			options.Args = args
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	cmd.Flags().StringVarP(&options.Dir, "dir", "", "", "The directory of the repository to validate, there should be a dependency-matrix dir in it")
	return cmd
}

// Run implements this command
func (o *StepVerifyDependenciesOptions) Run() error {
	err := dependencymatrix.VerifyDependencyMatrixHasConsistentVersions(o.Dir)
	if err != nil {
		return errors.WithStack(err)
	}
	log.Logger().Infof("Dependencies do not contain any conflicting versions")
	return nil
}
